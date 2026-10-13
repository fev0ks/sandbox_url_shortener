// Package mcp implements the Model Context Protocol over a stdio transport.
//
// The wire format is newline-delimited JSON-RPC 2.0: the client writes one
// request object per line on the server's stdin and reads one response object
// per line from its stdout. Nothing else may be written to stdout — logs go to
// stderr, or they corrupt the protocol stream.
package mcp

import "encoding/json"

// protocolVersion is the MCP revision this server implements.
const protocolVersion = "2024-11-05"

// JSON-RPC error codes used by this server.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// Method names of the MCP surface this server answers.
const (
	methodInitialize  = "initialize"
	methodInitialized = "notifications/initialized"
	methodToolsList   = "tools/list"
	methodToolsCall   = "tools/call"
	methodPing        = "ping"
)

// request is an incoming JSON-RPC message. A message without an ID is a
// notification and must not be answered.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *request) isNotification() bool { return len(r.ID) == 0 }

// response is an outgoing JSON-RPC message.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newResult(id json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func newError(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

// initializeResult announces what this server can do.
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

type capabilities struct {
	Tools struct{} `json:"tools"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toolDefinition is one entry of the tools/list response.
//
// Description is the field the model actually reads when deciding what to call,
// so it carries the intended order of use and the boundaries of each tool.
type toolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema map[string]any  `json:"inputSchema"`
	Annotations toolAnnotations `json:"annotations"`
}

// toolAnnotations are behavioural hints a client may surface or act on.
//
// Every tool here reads and nothing writes, which is worth stating in the
// protocol rather than only in the README: a client that auto-approves
// read-only calls can only do so if the server says which ones they are.
type toolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

// readOnlyTool is the annotation set shared by every tool this server exposes.
func readOnlyTool() toolAnnotations {
	return toolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		// The answer depends on an external, changing log index, not on a
		// closed set this server controls.
		OpenWorldHint: true,
	}
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// initializeParams is the half of the client handshake this server reads. The
// requested version is logged rather than negotiated: this server implements
// one revision, and the client decides whether that is acceptable.
type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// toolCallResult carries the tool output back to the model.
//
// A failed execution is reported here with IsError set, not as a JSON-RPC
// error: the model must see the failure text so it can correct the arguments
// and retry. JSON-RPC errors are reserved for protocol-level problems.
type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string, isError bool) toolCallResult {
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}
}
