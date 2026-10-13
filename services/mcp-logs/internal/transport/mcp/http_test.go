package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

const (
	httpTestKeyA           = "fixture-key-client-a"
	httpTestKeyB           = "fixture-key-client-b"
	httpTestSessionHeader  = "Mcp-Session-Id"
	httpTestProtocolHeader = "Mcp-Protocol-Version"
)

type httpLogRepo struct {
	stubLogRepo
	calls atomic.Int64
	count func(context.Context) error
}

func (r *httpLogRepo) DiscoverServices(ctx context.Context, since time.Time, limit int) ([]domain.ServiceInfo, error) {
	r.calls.Add(1)
	return r.stubLogRepo.DiscoverServices(ctx, since, limit)
}

func (r *httpLogRepo) CountLevels(ctx context.Context, services []string, since time.Time) (map[string]domain.ServiceSummary, error) {
	r.calls.Add(1)
	if r.count != nil {
		if err := r.count(ctx); err != nil {
			return nil, err
		}
	}
	return r.stubLogRepo.CountLevels(ctx, services, since)
}

func newHTTPCore(repo *httpLogRepo) *Server {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewServer(service.NewInvestigation(repo, stubTraceRepo{}, service.Options{DiscoveryTTL: time.Minute}), log)
}

func newHTTPTestHandler(t *testing.T, repo *httpLogRepo, options HTTPOptions, keys map[string]string) *HTTPHandler {
	t.Helper()
	if keys == nil {
		keys = map[string]string{httpTestKeyA: "client-a", httpTestKeyB: "client-b"}
	}
	h, err := NewHTTPHandler(newHTTPCore(repo), keys, options, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Errorf("close HTTP handler: %v", err)
		}
	})
	return h
}

type httpKeyTransport struct {
	base       http.RoundTripper
	key        string
	mu         sync.Mutex
	methods    map[string]int
	getStarted chan struct{}
}

func (t *httpKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(httpAuthorizationHeader, httpBearerScheme+" "+t.key)
	t.mu.Lock()
	t.methods[r.Method]++
	t.mu.Unlock()
	if r.Method == http.MethodGet {
		select {
		case t.getStarted <- struct{}{}:
		default:
		}
	}
	return t.base.RoundTrip(r)
}

func connectHTTPClient(t *testing.T, server *httptest.Server, key string, standalone bool) (*sdk.ClientSession, *httpKeyTransport) {
	t.Helper()
	transport := &httpKeyTransport{base: server.Client().Transport, key: key, methods: make(map[string]int), getStarted: make(chan struct{}, 1)}
	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint: server.URL + httpMCPPath, HTTPClient: &http.Client{Transport: transport},
		DisableStandaloneSSE: !standalone, MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, transport
}

func rawHTTPRequest(t *testing.T, h http.Handler, method, authorization, session, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://localhost"+httpMCPPath, strings.NewReader(body))
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(httpTestProtocolHeader, "2025-11-25")
	if authorization != "" {
		r.Header.Set(httpAuthorizationHeader, authorization)
	}
	if session != "" {
		r.Header.Set(httpTestSessionHeader, session)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPRealSDKClientSharesExistingTools(t *testing.T) {
	repo := &httpLogRepo{stubLogRepo: stubLogRepo{counts: map[string]domain.ServiceSummary{"shortener": {Service: "shortener", ErrorCount: 3}}}}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	session, transport := connectHTTPClient(t, server, httpTestKeyA, true)
	if session.ID() == "" || session.InitializeResult().ServerInfo.Name != serverName {
		t.Fatalf("missing stateful SDK handshake: %+v", session.InitializeResult())
	}
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != len(newTestServer().definitions()) {
		t.Fatalf("SDK tools/list: %+v, %v", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("read-only annotation missing from %s", tool.Name)
		}
		for _, definition := range newTestServer().definitions() {
			if definition.Name == tool.Name {
				// Both transports must declare the same input contract; the test
				// core has no domains, so omit that one conditional property.
				want := definition.InputSchema
				if tool.Name == toolListServices {
					delete(want["properties"].(map[string]any), "domain")
				}
				gotJSON, err := json.Marshal(tool.InputSchema)
				if err != nil {
					t.Fatal(err)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotJSON, wantJSON) {
					t.Fatalf("%s input contract differs between transports: %s != %s", tool.Name, gotJSON, wantJSON)
				}
			}
		}
	}
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: toolListServices, Arguments: map[string]any{"window": "15m"}})
	if err != nil || result.IsError {
		t.Fatalf("SDK tool call: %+v, %v", result, err)
	}
	text := result.Content[0].(*sdk.TextContent).Text
	if !strings.Contains(text, `"error_count": 3`) {
		t.Fatalf("shared dispatcher did not return investigation data: %s", text)
	}
	result, err = session.CallTool(context.Background(), &sdk.CallToolParams{Name: toolSearchLogs, Arguments: map[string]any{"service": "shortener", "window": "15m"}})
	if err != nil || !result.IsError {
		t.Fatalf("use-case errors must remain tool errors: %+v, %v", result, err)
	}
	// The real SDK opens a GET stream and closes its session with DELETE.
	select {
	case <-transport.getStarted:
	case <-time.After(time.Second):
		t.Fatal("SDK did not start its GET/SSE stream")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.methods[http.MethodGet] == 0 || transport.methods[http.MethodDelete] == 0 {
		t.Fatalf("SDK must support GET/SSE and DELETE: %v", transport.methods)
	}
}

func TestHTTPAllMCPMethodsRejectBadKeysWithoutBackendCalls(t *testing.T) {
	repo := &httpLogRepo{}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	bodies := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client-a","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_services","arguments":{"window":"15m"}}}`,
	}
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete, http.MethodOptions} {
		for _, credential := range []string{"", "Bearer", "Bearer ", "Basic " + httpTestKeyA, "Bearer unknown-key", "Bearer " + httpTestKeyA + " extra"} {
			for _, body := range bodies {
				w := rawHTTPRequest(t, h, method, credential, "pretend-session", body)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("%s must reject invalid credentials before protocol handling: %d %s", method, w.Code, w.Body.String())
				}
				if strings.Contains(w.Body.String(), httpTestKeyA) || strings.Contains(w.Body.String(), "unknown-key") {
					t.Fatal("unauthorized response must not echo keys")
				}
			}
		}
	}
	if repo.calls.Load() != 0 {
		t.Fatal("unauthenticated requests called a backend")
	}
}

func TestHTTPRegistryIsCopiedAndInvalidRegistriesFailClosed(t *testing.T) {
	for _, keys := range []map[string]string{{}, {"": "client"}, {"key with space": "client"}, {"key\vcontrol": "client"}, {"key\"quoted": "client"}, {"key": ""}, {"key": "bad identity\n"}} {
		if _, err := NewHTTPHandler(newHTTPCore(&httpLogRepo{}), keys, HTTPOptions{}, slog.New(slog.NewJSONHandler(io.Discard, nil))); err == nil {
			t.Fatal("invalid registry must fail constructor")
		}
	}
	keys := map[string]string{httpTestKeyA: "client-a"}
	h := newHTTPTestHandler(t, &httpLogRepo{}, HTTPOptions{}, keys)
	delete(keys, httpTestKeyA)
	keys[httpTestKeyB] = "client-b"
	if w := rawHTTPRequest(t, h, http.MethodDelete, "Bearer "+httpTestKeyA, "missing", ""); w.Code != http.StatusNotFound {
		t.Fatalf("original key must still authenticate after caller map mutation: %d", w.Code)
	}
	if w := rawHTTPRequest(t, h, http.MethodDelete, "Bearer "+httpTestKeyB, "missing", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("caller map mutation must not issue a live key: %d", w.Code)
	}
	removed := newHTTPTestHandler(t, &httpLogRepo{}, HTTPOptions{}, map[string]string{httpTestKeyB: "client-b"})
	if w := rawHTTPRequest(t, removed, http.MethodDelete, "Bearer "+httpTestKeyA, "missing", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("restart with removed key must revoke it: %d", w.Code)
	}
}

func TestHTTPNetworkBoundaryAndBodyCap(t *testing.T) {
	h := newHTTPTestHandler(t, &httpLogRepo{}, HTTPOptions{Origins: []string{"http://localhost:3000"}}, nil)
	for _, tc := range []struct {
		host, origin string
		status       int
	}{
		{"localhost:8085", "", http.StatusBadRequest},
		{"localhost", "http://localhost:3000", http.StatusBadRequest},
		{"attacker.example", "", http.StatusForbidden},
		{"localhost", "https://attacker.example", http.StatusForbidden},
		{"localhost", "null", http.StatusForbidden},
	} {
		r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+httpMCPPath, strings.NewReader("{}"))
		r.Header.Set(httpAuthorizationHeader, "Bearer "+httpTestKeyA)
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("Content-Type", "application/json")
		if tc.origin != "" {
			r.Header.Set(httpOriginHeader, tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("host/origin %q/%q: got %d want %d", tc.host, tc.origin, w.Code, tc.status)
		}
	}
	for _, options := range []HTTPOptions{{Hosts: []string{"http://localhost"}}, {Origins: []string{"http://localhost/path"}}, {Origins: []string{"null"}}, {RequestTimeout: -time.Second}} {
		if _, err := NewHTTPHandler(newHTTPCore(&httpLogRepo{}), map[string]string{httpTestKeyA: "client-a"}, options, slog.New(slog.NewJSONHandler(io.Discard, nil))); err == nil {
			t.Fatal("invalid HTTP options must fail constructor")
		}
	}
	w := rawHTTPRequest(t, h, http.MethodPost, "Bearer "+httpTestKeyA, "", strings.Repeat(" ", httpMaxBodyBytes+1)+"{}")
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("oversized MCP body must be rejected: %d", w.Code)
	}
}

func TestHTTPRejectsUnsupportedProtocolBeforeBackendAccess(t *testing.T) {
	repo := &httpLogRepo{}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	r := httptest.NewRequest(http.MethodPost, "http://localhost"+httpMCPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	r.Header.Set(httpAuthorizationHeader, httpBearerScheme+" "+httpTestKeyA)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set(httpTestProtocolHeader, "2099-01-01")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || repo.calls.Load() != 0 {
		t.Fatalf("unsupported protocol must fail before backend access: %d %s", w.Code, w.Body.String())
	}
}

func TestHTTPSessionsBoundToAuthenticatedClientAndRecoverAfterRestart(t *testing.T) {
	repo := &httpLogRepo{}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	a, _ := connectHTTPClient(t, server, httpTestKeyA, false)
	b, _ := connectHTTPClient(t, server, httpTestKeyB, false)
	if a.ID() == b.ID() {
		t.Fatal("clients must get distinct session IDs")
	}
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		w := rawHTTPRequest(t, h, method, "Bearer "+httpTestKeyB, a.ID(), `{"jsonrpc":"2.0","id":10,"method":"tools/list"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-client %s session use: %d %s", method, w.Code, w.Body.String())
		}
	}
	if repo.calls.Load() != 0 {
		t.Fatal("cross-client requests must not reach backends")
	}
	fresh := newHTTPTestHandler(t, &httpLogRepo{}, HTTPOptions{}, nil)
	w := rawHTTPRequest(t, fresh, http.MethodPost, "Bearer "+httpTestKeyA, a.ID(), `{"jsonrpc":"2.0","id":10,"method":"tools/list"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("stale session must require fresh initialization: %d", w.Code)
	}
	freshServer := httptest.NewServer(fresh)
	t.Cleanup(freshServer.Close)
	newSession, _ := connectHTTPClient(t, freshServer, httpTestKeyA, false)
	if newSession.ID() == "" || newSession.ID() == a.ID() {
		t.Fatal("fresh initialize must create new session")
	}
}

func TestHTTPToolCancellationAndDeadline(t *testing.T) {
	for _, explicitCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "client cancellation"}[explicitCancel], func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			repo := &httpLogRepo{count: func(ctx context.Context) error { close(started); <-ctx.Done(); close(stopped); return ctx.Err() }}
			timeout := 30 * time.Millisecond
			if explicitCancel {
				timeout = 5 * time.Second
			}
			h := newHTTPTestHandler(t, repo, HTTPOptions{RequestTimeout: timeout}, nil)
			server := httptest.NewServer(h)
			t.Cleanup(server.Close)
			session, _ := connectHTTPClient(t, server, httpTestKeyA, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolListServices, Arguments: map[string]any{"window": "15m"}})
				if err == nil && !result.IsError {
					err = errors.New("cancelled tool unexpectedly succeeded")
				}
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("tool did not start")
			}
			if explicitCancel {
				cancel()
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("backend context did not cancel")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("tool call did not finish")
			}
		})
	}
}

func TestHTTPHealthIsShallowAndClosureStopsTraffic(t *testing.T) {
	repo := &httpLogRepo{count: func(context.Context) error { return errors.New("database down") }}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	for _, path := range []string{httpLivePath, httpReadyPath} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("shallow health should succeed without auth/backends: %d", w.Code)
		}
	}
	if repo.calls.Load() != 0 {
		t.Fatal("health called upstream")
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	if w := rawHTTPRequest(t, h, http.MethodPost, "Bearer "+httpTestKeyA, "", "{}"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed handler accepted traffic: %d", w.Code)
	}
}

func TestHTTPShutdownDoesNotWaitForPartialInitializeBody(t *testing.T) {
	repo := &httpLogRepo{}
	h := newHTTPTestHandler(t, repo, HTTPOptions{}, nil)
	active, finished := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		close(finished)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateActive {
			select {
			case active <- struct{}{}:
			default:
			}
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	request := "POST /mcp HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer " + httpTestKeyA + "\r\nContent-Type: application/json\r\nAccept: application/json, text/event-stream\r\nContent-Length: 1000\r\n\r\n{\"jsonrpc\":"
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-active:
	case <-time.After(time.Second):
		t.Fatal("partial request did not reach server")
	}
	// Wait for SDK session allocation, which precedes reading the partial body.
	// This makes the regression exercise an admitted initializer rather than a
	// request that happens to arrive after the shutdown flag was set.
	allocated := false
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	allocationTimeout := time.NewTimer(time.Second)
	defer allocationTimeout.Stop()
	for !allocated {
		for range h.server.Sessions() {
			allocated = true
		}
		if allocated {
			break
		}
		select {
		case <-ticker.C:
		case <-allocationTimeout.C:
			t.Fatal("initializer did not allocate its SDK session")
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- h.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler Close blocked on partial request body")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := server.Config.Shutdown(ctx); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if err := server.Config.Close(); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("forced HTTP close did not unblock partial body")
	}
	for range h.server.Sessions() {
		t.Fatal("late initialization leaked a session after shutdown")
	}
	if repo.calls.Load() != 0 {
		t.Fatal("partial initialize reached a backend")
	}
}

func TestHTTPShutdownCancelsActiveToolBeforeClosingSessions(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	repo := &httpLogRepo{count: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}}
	h := newHTTPTestHandler(t, repo, HTTPOptions{RequestTimeout: time.Minute}, nil)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	session, _ := connectHTTPClient(t, server, httpTestKeyA, false)
	go func() {
		_, _ = session.CallTool(context.Background(), &sdk.CallToolParams{Name: toolListServices, Arguments: map[string]any{"window": "15m"}})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- h.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for tool timeout instead of canceling")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("active backend context was not cancelled")
	}
}

func TestHTTPAuthDoesNotLogRawKey(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h, err := NewHTTPHandler(newHTTPCore(&httpLogRepo{}), map[string]string{httpTestKeyA: "client-a"}, HTTPOptions{}, log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	w := rawHTTPRequest(t, h, http.MethodDelete, "Bearer "+httpTestKeyA, "missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("authenticated request did not pass auth: %d", w.Code)
	}
	if strings.Contains(logs.String(), httpTestKeyA) {
		t.Fatalf("raw key leaked into logs: %s", logs.String())
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil || entry["client_id"] != "client-a" {
		t.Fatalf("expected safe client audit ID: %s (%v)", logs.String(), err)
	}
}
