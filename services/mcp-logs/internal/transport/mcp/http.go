package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTP wire contracts are shared by authentication and transport checks.
const (
	httpMCPPath             = "/mcp"
	httpLivePath            = "/health/live"
	httpReadyPath           = "/health/ready"
	httpAuthorizationHeader = "Authorization"
	httpOriginHeader        = "Origin"
	httpSessionHeader       = "Mcp-Session-Id"
	httpBearerScheme        = "Bearer"
	httpAuthenticateHeader  = "WWW-Authenticate"
	httpSessionTimeout      = 15 * time.Minute
	httpDefaultToolTimeout  = 15 * time.Second
	httpMaxBodyBytes        = 1 << 20
)

// HTTPOptions configures the network boundary and per-tool execution deadline.
// Hosts contains hostnames (optional ports are ignored); Origins contains exact
// HTTP(S) origins. An empty origin allowlist permits only requests without Origin.
type HTTPOptions struct {
	Hosts          []string
	Origins        []string
	RequestTimeout time.Duration
}

// HTTPHandler serves authenticated Streamable HTTP and shallow health endpoints.
// Close stops new session admission and releases existing SDK sessions/SSE streams.
type HTTPHandler struct {
	handler  http.Handler
	server   *sdk.Server
	closed   atomic.Bool
	shutdown chan struct{}
	log      *slog.Logger
}

// NewHTTPHandler adapts the existing investigation tools to stateful Streamable
// HTTP. It copies the issued-key registry so callers cannot change live access.
func NewHTTPHandler(core *Server, keys map[string]string, options HTTPOptions, log *slog.Logger) (*HTTPHandler, error) {
	if core == nil || core.investigation == nil || log == nil {
		return nil, errors.New("MCP HTTP requires a core server and logger")
	}
	registry, err := cloneHTTPKeys(keys)
	if err != nil {
		return nil, err
	}
	hosts, origins, err := httpAllowLists(options)
	if err != nil {
		return nil, err
	}
	if options.RequestTimeout < 0 {
		return nil, errors.New("MCP HTTP tool timeout must be positive")
	}
	if options.RequestTimeout == 0 {
		options.RequestTimeout = httpDefaultToolTimeout
	}

	handler := &HTTPHandler{shutdown: make(chan struct{}), log: log}
	server := sdk.NewServer(&sdk.Implementation{Name: serverName, Version: serverVersion}, &sdk.ServerOptions{Logger: log})
	handler.server = server
	for _, definition := range core.definitions() {
		annotations := definition.Annotations
		server.AddTool(&sdk.Tool{
			Name: definition.Name, Description: definition.Description, InputSchema: definition.InputSchema,
			Annotations: &sdk.ToolAnnotations{
				ReadOnlyHint: annotations.ReadOnlyHint, DestructiveHint: &annotations.DestructiveHint,
				IdempotentHint: annotations.IdempotentHint, OpenWorldHint: &annotations.OpenWorldHint,
			},
		}, httpToolAdapter(core, options.RequestTimeout, handler.shutdown, log))
	}
	stream := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{
		JSONResponse: true, SessionTimeout: httpSessionTimeout, Logger: log,
	})
	verify := func(_ context.Context, token string, r *http.Request) (*auth.TokenInfo, error) {
		client, ok := registry[token]
		if !ok || len(r.Header.Values(httpAuthorizationHeader)) != 1 {
			return nil, auth.ErrInvalidToken
		}
		// Issued keys last until removal and restart; the SDK requires an expiry
		// on each verified request, independent of its idle-session expiration.
		return &auth.TokenInfo{UserID: client, Expiration: time.Now().Add(httpSessionTimeout)}, nil
	}
	protected := auth.RequireBearerToken(verify, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !httpAllowedRequest(w, r, hosts, origins) {
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, httpMaxBodyBytes)
		}
		log.DebugContext(r.Context(), "mcp HTTP request", slog.String("client_id", auth.TokenInfoFromContext(r.Context()).UserID), slog.String("method", r.Method))
		stream.ServeHTTP(w, r)
	}))

	mux := http.NewServeMux()
	mux.Handle(httpMCPPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(httpAuthenticateHeader, httpBearerScheme)
		protected.ServeHTTP(w, r)
	}))
	for _, path := range []string{httpLivePath, httpReadyPath} {
		mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !httpAllowedRequest(w, r, hosts, origins) {
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
	}
	handler.handler = mux
	return handler, nil
}

// ServeHTTP rejects new traffic after Close and otherwise routes HTTP requests.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.closed.Load() {
		http.Error(w, "MCP server is shutting down", http.StatusServiceUnavailable)
		return
	}
	// A sessionless request may allocate a session after Close's snapshot.
	// Clean that session when the request finishes, without making shutdown
	// wait on a slow or partial request body. The HTTP owner bounds draining.
	if r.Header.Get(httpSessionHeader) == "" {
		defer func(ctx context.Context) {
			if h.closed.Load() {
				if err := h.Close(); err != nil {
					h.log.WarnContext(ctx, "mcp HTTP session cleanup failed", slog.String("operation", "mcp_session_cleanup"))
				}
			}
		}(r.Context())
	}
	h.handler.ServeHTTP(w, r)
}

// Close cancels all SDK sessions and their idle timers. Call it before waiting
// for HTTP shutdown, because active SSE streams otherwise keep shutdown waiting.
func (h *HTTPHandler) Close() error {
	if !h.closed.Swap(true) {
		close(h.shutdown)
	}
	var errs []error
	for session := range h.server.Sessions() {
		// Initializers may still be registering their transport in the SDK.
		// Let the request's SDK cleanup/deferred check close those sessions,
		// after registration, once bounded HTTP draining unblocks their body.
		if session.InitializeParams() == nil {
			continue
		}
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close MCP HTTP session: %w", err))
		}
	}
	return errors.Join(errs...)
}

func httpToolAdapter(core *Server, timeout time.Duration, shutdown <-chan struct{}, log *slog.Logger) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		go func() {
			select {
			case <-shutdown:
				cancel()
			case <-ctx.Done():
			}
		}()
		clientID := ""
		if req.Extra != nil && req.Extra.TokenInfo != nil {
			clientID = req.Extra.TokenInfo.UserID
		}
		result, err := core.dispatch(ctx, req.Params.Name, req.Params.Arguments)
		if err != nil {
			// Upstream errors and input failures may include sensitive values;
			// audit the identity/tool without copying failure text into logs.
			log.WarnContext(ctx, "mcp HTTP tool failed", slog.String("client_id", clientID), slog.String("tool", req.Params.Name))
			return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}}}, nil
		}
		rendered, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode MCP HTTP tool result: %w", err)
		}
		log.InfoContext(ctx, "mcp HTTP tool completed", slog.String("client_id", clientID), slog.String("tool", req.Params.Name), slog.Int("result_bytes", len(rendered)))
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(rendered)}}}, nil
	}
}

func cloneHTTPKeys(keys map[string]string) (map[string]string, error) {
	if len(keys) == 0 {
		return nil, errors.New("MCP HTTP issued-key registry is empty")
	}
	registry := make(map[string]string, len(keys))
	for key, client := range keys {
		if !validHTTPKey(key) || !validHTTPClientID(client) {
			return nil, errors.New("MCP HTTP issued-key registry contains an invalid key or client identity")
		}
		registry[key] = client
	}
	return registry, nil
}

func validHTTPClientID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !httpASCIIAlphanumeric(c) && !strings.ContainsRune("._:-", c) {
			return false
		}
	}
	return true
}

func validHTTPKey(key string) bool {
	if key == "" {
		return false
	}
	padding := false
	for _, c := range key {
		if c == '=' {
			padding = true
			continue
		}
		if padding || (!httpASCIIAlphanumeric(c) && !strings.ContainsRune("-._~+/", c)) {
			return false
		}
	}
	return key[0] != '='
}

func httpASCIIAlphanumeric(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func httpAllowLists(options HTTPOptions) (map[string]bool, map[string]bool, error) {
	hostNames := options.Hosts
	if len(hostNames) == 0 {
		hostNames = []string{"localhost", "127.0.0.1", "[::1]"}
	}
	hosts, origins := make(map[string]bool), make(map[string]bool)
	for _, host := range hostNames {
		name := httpHostName(host)
		if name == "" {
			return nil, nil, errors.New("MCP HTTP host allowlist contains an invalid hostname")
		}
		hosts[name] = true
	}
	for _, origin := range options.Origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, nil, errors.New("MCP HTTP origin allowlist contains an invalid origin")
		}
		origins[origin] = true
	}
	return hosts, origins, nil
}

func httpHostName(host string) string {
	if net.ParseIP(host) != nil {
		return strings.ToLower(host)
	}
	u, err := url.Parse("//" + host)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func httpAllowedRequest(w http.ResponseWriter, r *http.Request, hosts, origins map[string]bool) bool {
	if !hosts[httpHostName(r.Host)] {
		http.Error(w, "host not allowed", http.StatusForbidden)
		return false
	}
	if values := r.Header.Values(httpOriginHeader); len(values) > 0 && (len(values) != 1 || !origins[values[0]]) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return false
	}
	return true
}
