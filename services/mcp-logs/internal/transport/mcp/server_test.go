package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/elasticsearch"
	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

type stubLogRepo struct {
	counts map[string]domain.ServiceSummary
}

func (s stubLogRepo) Search(context.Context, elasticsearch.Filters, int) (elasticsearch.Page, error) {
	return elasticsearch.Page{}, nil
}

func (s stubLogRepo) CountLevels(context.Context, []string, time.Time) (map[string]domain.ServiceSummary, error) {
	return s.counts, nil
}

func (s stubLogRepo) DiscoverServices(context.Context, time.Time, int) ([]domain.ServiceInfo, error) {
	return []domain.ServiceInfo{{Service: "shortener"}, {Service: "analytics"}}, nil
}

func (s stubLogRepo) DescribeFields(context.Context, string, time.Time) ([]domain.FieldInfo, int, bool, error) {
	return []domain.FieldInfo{
		{Name: "short_code", Kind: domain.FieldTail, Type: "keyword", ExactFilterable: true, Cardinality: 40},
	}, 100, false, nil
}

type stubTraceRepo struct{}

func (stubTraceRepo) GetTrace(context.Context, string) ([]domain.Span, error) { return nil, nil }

func newTestServer() *Server {
	investigation := service.NewInvestigation(
		stubLogRepo{counts: map[string]domain.ServiceSummary{"shortener": {Service: "shortener", ErrorCount: 3}}},
		stubTraceRepo{},
		service.Options{
			Domains:      map[string][]string{"core": {"shortener"}, "data": {"analytics"}},
			DiscoveryTTL: time.Minute,
		},
	)
	quiet := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewServer(investigation, quiet)
}

// run feeds the given lines through the server and returns the decoded responses.
func run(t *testing.T, lines ...string) []response {
	t.Helper()

	var out bytes.Buffer
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")

	if err := newTestServer().Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve() failed: %v", err)
	}

	var responses []response
	decoder := json.NewDecoder(&out)
	for decoder.More() {
		var resp response
		if err := decoder.Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestInitializeAnnouncesProtocolAndServer(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	if len(responses) != 1 {
		t.Fatalf("want 1 response, got %d", len(responses))
	}
	result := marshalResult(t, responses[0])

	var init initializeResult
	if err := json.Unmarshal(result, &init); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	if init.ProtocolVersion != protocolVersion {
		t.Fatalf("want protocol %q, got %q", protocolVersion, init.ProtocolVersion)
	}
	if init.ServerInfo.Name != serverName {
		t.Fatalf("want server name %q, got %q", serverName, init.ServerInfo.Name)
	}
}

func TestNotificationProducesNoResponse(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	if len(responses) != 0 {
		t.Fatalf("a notification must not be answered, got %d responses", len(responses))
	}
}

func TestToolsListExposesClosedVocabularies(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	var list toolsListResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &list); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if len(list.Tools) != 6 {
		t.Fatalf("want 6 tools, got %d", len(list.Tools))
	}

	byName := make(map[string]toolDefinition, len(list.Tools))
	for _, tool := range list.Tools {
		byName[tool.Name] = tool
		if tool.Description == "" {
			t.Fatalf("tool %q has no description; the model picks tools by description", tool.Name)
		}
	}

	props := byName[toolSearchErrors].InputSchema["properties"].(map[string]any)
	serviceProp := props["service"].(map[string]any)
	// service is deliberately no longer an enum: inlining the vocabulary made
	// the schema grow with the system and re-sent it on every turn. The
	// guarantee moved to the server, which validates and suggests.
	if _, ok := serviceProp["enum"]; ok {
		t.Fatal("service must not inline the vocabulary; it is validated server-side instead")
	}
	windowProp := props["window"].(map[string]any)
	if _, ok := windowProp["enum"]; !ok {
		t.Fatal("window is few and fixed, so it must stay a closed vocabulary")
	}
	domainProp := byName[toolListServices].InputSchema["properties"].(map[string]any)["domain"].(map[string]any)
	if _, ok := domainProp["enum"]; !ok {
		t.Fatal("domains are few and change rarely, so they stay a closed vocabulary")
	}
	if byName[toolSearchErrors].InputSchema["additionalProperties"] != false {
		t.Fatal("schemas must forbid extra arguments")
	}
	for _, tool := range list.Tools {
		if !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint {
			t.Fatalf("tool %q must declare itself read-only; a client cannot infer it from the name", tool.Name)
		}
	}
}

func TestSchemaDoesNotGrowWithServiceCount(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	raw := marshalResult(t, responses[0])

	// The schema is re-sent on every turn of the conversation, so a service
	// name appearing in it is a per-turn cost that scales with the fleet.
	for _, name := range []string{"shortener", "analytics"} {
		if bytes.Contains(raw, []byte(`"`+name+`"`)) {
			t.Fatalf("service name %q must not appear in the tool schema", name)
		}
	}
}

func TestSearchLogsTakesServiceSpecificFiltersAsAnOpenMap(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	var list toolsListResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &list); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	byName := make(map[string]toolDefinition, len(list.Tools))
	for _, tool := range list.Tools {
		byName[tool.Name] = tool
	}

	props := byName[toolSearchLogs].InputSchema["properties"].(map[string]any)
	if _, ok := props["short_code"]; ok {
		t.Fatal("a field belonging to one service must not sit in the shared schema")
	}
	filters, ok := props["filters"].(map[string]any)
	if !ok {
		t.Fatal("service-specific filters need a home that does not grow the schema")
	}
	if _, ok := filters["additionalProperties"].(map[string]any); !ok {
		t.Fatal("filters must declare a value type without naming the keys")
	}
}

func TestDescribeServiceIsExposed(t *testing.T) {
	responses := run(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"describe_service","arguments":{"service":"shortener","window":"1h"}}}`)

	var call toolCallResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if call.IsError {
		t.Fatalf("unexpected tool error: %s", call.Content[0].Text)
	}
	if !strings.Contains(call.Content[0].Text, "short_code") {
		t.Fatalf("describe_service must report the service's own fields: %s", call.Content[0].Text)
	}
}

func TestUndeclaredArgumentIsRejected(t *testing.T) {
	responses := run(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{"window":"1h","service":"shortener"}}}`)

	var call toolCallResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	// Silently dropping an undeclared field leaves the model reasoning about a
	// filter that was never applied.
	if !call.IsError {
		t.Fatal("an argument the schema does not declare must be refused, not ignored")
	}
}

func TestKnownMethodSentAsNotificationIsNotAnswered(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","method":"tools/list"}`)

	if len(responses) != 0 {
		t.Fatalf("a message without an id is a notification whatever its method, got %d responses", len(responses))
	}
}

func TestServeStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		// A reader that never yields stands in for an idle stdin: the client is
		// connected and simply has nothing to say.
		done <- newTestServer().Serve(ctx, blockingReader{}, io.Discard)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() ignored cancellation while blocked on a read; shutdown would need SIGKILL")
	}
}

// blockingReader never returns, like a stdin whose client is idle.
type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) {
	select {}
}

func TestToolCallReturnsResult(t *testing.T) {
	responses := run(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{"window":"1h"}}}`)

	var call toolCallResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if call.IsError {
		t.Fatalf("unexpected tool error: %s", call.Content[0].Text)
	}
	if !strings.Contains(call.Content[0].Text, "shortener") {
		t.Fatalf("result does not mention the service: %s", call.Content[0].Text)
	}
}

func TestToolFailureIsReportedInResultNotAsRPCError(t *testing.T) {
	responses := run(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_errors","arguments":{"service":"billing","window":"1h"}}}`)

	if responses[0].Error != nil {
		t.Fatal("a tool failure must not surface as a JSON-RPC error: the model needs to read the text and retry")
	}

	var call toolCallResult
	if err := json.Unmarshal(marshalResult(t, responses[0]), &call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if !call.IsError {
		t.Fatal("want isError set for a rejected call")
	}
	// The vocabulary is no longer inlined anywhere, the error text included, so
	// the refusal points at the tool that produces it rather than reciting it.
	if !strings.Contains(call.Content[0].Text, "list_services") {
		t.Fatalf("the refusal must route the agent to discovery, got: %s", call.Content[0].Text)
	}
}

func TestUnknownMethodReturnsRPCError(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0","id":1,"method":"tools/destroy"}`)

	if responses[0].Error == nil || responses[0].Error.Code != codeMethodNotFound {
		t.Fatalf("want method-not-found error, got %+v", responses[0].Error)
	}
}

func TestMalformedLineReturnsParseError(t *testing.T) {
	responses := run(t, `{"jsonrpc":"2.0"`)

	if responses[0].Error == nil || responses[0].Error.Code != codeParseError {
		t.Fatalf("want parse error, got %+v", responses[0].Error)
	}
}

func marshalResult(t *testing.T, resp response) []byte {
	t.Helper()
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return raw
}
