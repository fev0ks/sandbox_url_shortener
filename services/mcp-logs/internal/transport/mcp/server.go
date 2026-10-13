package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

// serverName and serverVersion identify this server to the client.
const (
	serverName    = "mcp-logs"
	serverVersion = "0.1.0"
)

// readBufferSize is the initial size of the line reader. A single JSON-RPC
// message can exceed the default bufio.Scanner token limit, which is why this
// server reads with bufio.Reader.ReadBytes instead of a Scanner.
const readBufferSize = 64 << 10

// Server speaks MCP over stdio and delegates every tool call to the
// investigation use cases.
type Server struct {
	investigation *service.Investigation
	databases     *service.DatabaseInvestigation
	log           *slog.Logger
}

// NewServer creates an MCP server. An optional database use case enables the
// same fixed database tools for both transports, even when its registry is empty.
func NewServer(investigation *service.Investigation, log *slog.Logger, databases ...*service.DatabaseInvestigation) *Server {
	s := &Server{investigation: investigation, log: log}
	if len(databases) > 0 {
		s.databases = databases[0]
	}
	return s
}

// line is one read from the input, carrying whatever error ended it.
type line struct {
	data []byte
	err  error
}

// Serve reads requests until the input closes or the context is cancelled.
//
// Requests are handled one at a time. Sequential handling keeps stdout writes
// ordered without a mutex, and an investigation step is never hot enough to
// need concurrency.
//
// Reading happens on its own goroutine so cancellation is not stuck behind it.
// A read from stdin blocks until a client writes, which can be forever; with
// the read inline, a signalled shutdown would wait for a request that is never
// coming, and the process would need SIGKILL to end.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	encoder := json.NewEncoder(out)
	lines := readLines(in)

	for {
		select {
		case <-ctx.Done():
			return nil

		case next, ok := <-lines:
			if !ok {
				return nil
			}
			if len(next.data) > 0 {
				if resp, answer := s.handleLine(ctx, next.data); answer {
					if err := encoder.Encode(resp); err != nil {
						return err
					}
				}
			}
			if next.err != nil {
				if errors.Is(next.err, io.EOF) {
					return nil
				}
				return next.err
			}
		}
	}
}

// readLines pumps the input into a channel. The goroutine ends when the reader
// reports an error, including the EOF that arrives when the client goes away.
func readLines(in io.Reader) <-chan line {
	out := make(chan line)
	go func() {
		defer close(out)
		reader := bufio.NewReaderSize(in, readBufferSize)
		for {
			data, err := reader.ReadBytes('\n')
			out <- line{data: data, err: err}
			if err != nil {
				return
			}
		}
	}()
	return out
}

// handleLine processes one message. The second return value reports whether a
// response must be written; notifications are answered with silence.
func (s *Server) handleLine(ctx context.Context, data []byte) (response, bool) {
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		return newError(nil, codeParseError, "malformed JSON"), true
	}
	if req.Method == "" {
		return newError(req.ID, codeInvalidRequest, "method is required"), true
	}

	s.log.DebugContext(ctx, "mcp request", slog.String("method", req.Method))

	// A message without an id is a notification whatever its method, so the
	// check belongs here rather than in the branch for unknown methods.
	if req.isNotification() {
		if req.Method != methodInitialized {
			s.log.DebugContext(ctx, "ignoring notification", slog.String("method", req.Method))
		}
		return response{}, false
	}

	switch req.Method {
	case methodInitialize:
		s.logClientHandshake(ctx, req)
		return newResult(req.ID, initializeResult{
			ProtocolVersion: protocolVersion,
			ServerInfo:      serverInfo{Name: serverName, Version: serverVersion},
		}), true

	case methodPing:
		return newResult(req.ID, struct{}{}), true

	case methodToolsList:
		return newResult(req.ID, toolsListResult{Tools: s.definitions()}), true

	case methodToolsCall:
		return s.handleToolCall(ctx, req), true

	default:
		return newError(req.ID, codeMethodNotFound, "unknown method "+req.Method), true
	}
}

// logClientHandshake records who connected and which revision they asked for.
// This server implements one revision and answers with it either way; knowing
// what was requested is what makes a version mismatch diagnosable later.
func (s *Server) logClientHandshake(ctx context.Context, req request) {
	var params initializeParams
	if len(req.Params) == 0 {
		return
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}
	s.log.InfoContext(ctx, "mcp client connected",
		slog.String("client", params.ClientInfo.Name),
		slog.String("client_version", params.ClientInfo.Version),
		slog.String("requested_protocol", params.ProtocolVersion),
		slog.String("served_protocol", protocolVersion),
	)
}

func (s *Server) handleToolCall(ctx context.Context, req request) response {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return newError(req.ID, codeInvalidParams, "invalid tool call parameters")
	}

	result, err := s.dispatch(ctx, params.Name, params.Arguments)
	if err != nil {
		// A failed tool is normal control flow: the text goes back to the model
		// so it can fix the arguments, rather than ending the conversation.
		s.log.WarnContext(ctx, "tool call failed",
			slog.String("tool", params.Name),
			slog.String("error", err.Error()),
		)
		return newResult(req.ID, textResult(err.Error(), true))
	}

	rendered, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return newError(req.ID, codeInternalError, "failed to encode tool result")
	}

	s.log.InfoContext(ctx, "tool call completed",
		slog.String("tool", params.Name),
		slog.Int("result_bytes", len(rendered)),
	)
	return newResult(req.ID, textResult(string(rendered), false))
}
