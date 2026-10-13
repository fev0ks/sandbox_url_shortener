package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// capture records the request body the repository sent, which is the only place
// the query shape is observable.
type capture struct {
	body     map[string]any
	authSeen string
}

func newStubES(t *testing.T, c *capture, response string) *Repo {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.authSeen = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&c.body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return NewRepo(srv.Client(), srv.URL, "shortener-logs-*", Auth{})
}

func TestSearchSortsOldestFirstWhenAsked(t *testing.T) {
	c := &capture{}
	repo := newStubES(t, c, `{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`)

	if _, err := repo.Search(context.Background(), Filters{Oldest: true, Since: time.Now()}, 10); err != nil {
		t.Fatalf("Search() failed: %v", err)
	}

	sort := c.body["sort"].([]any)[0].(map[string]any)
	order := sort[fieldTimestamp].(map[string]any)["order"]
	if order != "asc" {
		t.Fatalf("want the oldest page from the index itself, got sort order %v", order)
	}
}

func TestSearchDefaultsToNewestFirst(t *testing.T) {
	c := &capture{}
	repo := newStubES(t, c, `{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`)

	if _, err := repo.Search(context.Background(), Filters{Since: time.Now()}, 10); err != nil {
		t.Fatalf("Search() failed: %v", err)
	}

	sort := c.body["sort"].([]any)[0].(map[string]any)
	if order := sort[fieldTimestamp].(map[string]any)["order"]; order != "desc" {
		t.Fatalf("want newest first by default, got %v", order)
	}
}

func TestSearchReportsALowerBoundTotal(t *testing.T) {
	c := &capture{}
	repo := newStubES(t, c, `{"hits":{"total":{"value":10000,"relation":"gte"},"hits":[]}}`)

	page, err := repo.Search(context.Background(), Filters{Since: time.Now()}, 10)
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}

	// Presenting a capped count as exact would let the agent report "10000
	// errors" when the real number is any larger one.
	if !page.TotalAtLeast {
		t.Fatal("a total that hit the tracking bound must be reported as a lower bound")
	}
}

func TestSearchSendsCredentialsWhenConfigured(t *testing.T) {
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.authSeen = r.Header.Get("Authorization")
		if _, err := w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	repo := NewRepo(srv.Client(), srv.URL, "idx", Auth{Username: "reader", Password: "pw"})
	if _, err := repo.Search(context.Background(), Filters{Since: time.Now()}, 1); err != nil {
		t.Fatalf("Search() failed: %v", err)
	}

	if c.authSeen != "Basic cmVhZGVyOnB3" {
		t.Fatalf("basic credentials not sent, got %q", c.authSeen)
	}
}

func TestBuildFiltersAlwaysBoundsTime(t *testing.T) {
	filters := buildFilters(Filters{})

	if len(filters) == 0 {
		t.Fatal("an unfiltered search must still carry a time bound")
	}
	if _, ok := filters[0].(map[string]any)["range"]; !ok {
		t.Fatalf("want a range filter first, got %#v", filters[0])
	}
}

func TestBuildFiltersNormalizesLevelCasing(t *testing.T) {
	filters := buildFilters(Filters{Levels: []string{"error", "Warn"}})

	var levels []string
	for _, f := range filters {
		if terms, ok := f.(map[string]any)["terms"].(map[string]any); ok {
			if v, ok := terms[keyword(fieldLevel)].([]string); ok {
				levels = v
			}
		}
	}
	// A terms filter is exact, and the pipeline writes upper-case levels.
	if len(levels) != 2 || levels[0] != LevelError || levels[1] != LevelWarn {
		t.Fatalf("levels must be normalized for an exact-match filter, got %v", levels)
	}
}

func TestProjectEntryKeepsOnlyTheInvestigationFields(t *testing.T) {
	entry := projectEntry(map[string]any{
		"@timestamp":  "2026-08-20T10:00:00.123Z",
		"service":     " shortener ",
		"level":       "ERROR",
		"msg":         "link missing",
		"status_code": float64(404),
		"short_code":  "abc123",
		"stack_trace": "a very large field nobody asked for",
	}, nil)

	if entry.Service != "shortener" {
		t.Fatalf("values must be trimmed, got %q", entry.Service)
	}
	if entry.StatusCode != 404 {
		t.Fatalf("want status_code 404, got %d", entry.StatusCode)
	}
	if entry.Timestamp.IsZero() {
		t.Fatal("@timestamp must be parsed")
	}
	// short_code belongs to one service. It comes back only when it was asked
	// about, otherwise every service pays for it in every result.
	if len(entry.Fields) != 0 {
		t.Fatalf("a service-specific field must not appear unrequested, got %v", entry.Fields)
	}
}

func TestProjectEntryReturnsTheTailFieldsThatWereFilteredOn(t *testing.T) {
	entry := projectEntry(map[string]any{
		"service":    "shortener",
		"short_code": "abc123",
		"order_id":   "ord-9",
	}, []string{"short_code"})

	if entry.Fields["short_code"] != "abc123" {
		t.Fatalf("a filtered field must come back, got %v", entry.Fields)
	}
	if _, ok := entry.Fields["order_id"]; ok {
		t.Fatal("an unrequested field must stay out of the result")
	}
}

func TestSearchProjectsUnderlyingDatabaseError(t *testing.T) {
	const cause = "link service list recent: link repo list recent: failed to connect to `user=shortener database=shortener`: hostname resolving error: lookup postgres on 127.0.0.11:53: no such host"
	response, err := json.Marshal(map[string]any{
		"hits": map[string]any{
			"total": map[string]any{"value": 1, "relation": "eq"},
			"hits": []any{map[string]any{"_source": map[string]any{
				fieldService: "shortener", fieldMessage: "failed to list recent links", fieldError: cause,
			}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := newStubES(t, &capture{}, string(response))
	page, err := repo.Search(context.Background(), Filters{Service: "shortener", Since: time.Now().Add(-time.Hour)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 || page.Entries[0].Error != cause {
		t.Fatalf("the core projection must preserve the database cause without tail filters: %+v", page.Entries)
	}
	if !IsCoreField(fieldError) || len(page.Entries[0].Fields) != 0 {
		t.Fatal("error must be a core field rather than a service-specific filter")
	}
}

func TestProjectEntryOmitsAbsentOrNonStringError(t *testing.T) {
	for _, value := range []any{nil, "", "   ", float64(500), map[string]any{"message": "untrusted nested object"}} {
		entry := projectEntry(map[string]any{fieldError: value}, nil)
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if _, exists := fields[fieldError]; exists {
			t.Fatalf("empty or non-string error %v must be omitted: %s", value, raw)
		}
	}
}

func TestIndexRoutingResolvesThePlaceholder(t *testing.T) {
	repo := NewRepo(nil, "http://es", "logs-"+ServicePlaceholder+"-*", Auth{})

	if got := repo.indexFor("shortener"); got != "logs-shortener-*" {
		t.Fatalf("want the service routed to its own index, got %q", got)
	}
	// A query that names no service must fan out visibly rather than hit an
	// index literally called "{service}".
	if got := repo.indexFor(""); got != "logs-*-*" {
		t.Fatalf("want an explicit wildcard, got %q", got)
	}
}

func TestIndexRoutingLeavesAPlainPatternAlone(t *testing.T) {
	repo := NewRepo(nil, "http://es", "shortener-logs-*", Auth{})

	if got := repo.indexFor("analytics"); got != "shortener-logs-*" {
		t.Fatalf("a pattern without the placeholder must not be rewritten, got %q", got)
	}
}

func TestBuildFiltersDoesNotGuessServiceSpecificPaths(t *testing.T) {
	filters := buildFilters(Filters{Fields: map[string]string{"short_code": "abc123"}})
	for _, f := range filters {
		if term, ok := f.(map[string]any)["term"].(map[string]any); ok {
			if _, exists := term[keyword("short_code")]; exists {
				t.Fatal("core builder must not guess tail mapping paths")
			}
		}
	}
}

type mappingCapture struct {
	mappingCalls int
	searchCalls  int
	body         map[string]any
}

func newMappedStubES(t *testing.T, mapping, reply string) (*Repo, *mappingCapture) {
	t.Helper()
	capture := &mappingCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := reply
		if strings.HasSuffix(r.URL.Path, "/_mapping") {
			capture.mappingCalls++
			response = mapping
		} else {
			capture.searchCalls++
			if err := json.NewDecoder(r.Body).Decode(&capture.body); err != nil {
				t.Errorf("decode search body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write stub response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return NewRepo(server.Client(), server.URL, "logs-{service}-*", Auth{}), capture
}

func TestSearchTailUsesMappedPathsAndScalarProjection(t *testing.T) {
	mapping := `{"logs-shortener":{"mappings":{"properties":{
		"short_code":{"type":"text","fields":{"keyword":{"type":"keyword"}}},
		"entity_id":{"type":"keyword"},"attempts":{"type":"long"},
		"duration_ms":{"type":"double"},"active":{"type":"boolean"},"event_date":{"type":"date"}
	}}}}`
	reply := `{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_source":{
		"service":"shortener","status_code":500,"attempts":9223372036854775807,"active":false,"duration_ms":1.5,
		"short_code":"abc123","entity_id":"entity-1","unrequested":"private-value"
	}}]}}`
	repo, captured := newMappedStubES(t, mapping, reply)
	fields := map[string]string{"short_code": "abc123", "entity_id": "entity-1", "attempts": "9223372036854775807", "duration_ms": "1.5", "active": "false", "event_date": "2026-10-13T00:00:00Z"}
	before, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.Search(context.Background(), Filters{Service: "shortener", Since: time.Now(), Fields: fields}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if captured.mappingCalls != 1 || captured.searchCalls != 1 {
		t.Fatalf("unexpected calls: %+v", captured)
	}
	terms := make(map[string]any)
	for _, filter := range captured.body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any) {
		if values, ok := filter.(map[string]any)["term"].(map[string]any); ok {
			for path, value := range values {
				terms[path] = value
			}
		}
	}
	for name, value := range fields {
		path := name
		if name == "short_code" {
			path = keyword(name)
		}
		if terms[path] != value {
			t.Fatalf("tail filter %s used wrong path/value: %v", name, terms)
		}
	}
	for _, name := range []string{"entity_id", "attempts", "duration_ms", "active", "event_date"} {
		if _, exists := terms[keyword(name)]; exists {
			t.Fatalf("direct mapped field must not gain keyword suffix: %s", name)
		}
	}
	after, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("mapping resolution mutated caller filters")
	}
	if len(page.Entries) != 1 || page.Entries[0].StatusCode != 500 || page.Entries[0].Fields["attempts"] != "9223372036854775807" || page.Entries[0].Fields["active"] != "false" || page.Entries[0].Fields["duration_ms"] != "1.5" {
		t.Fatalf("scalar projection lost values or precision: %+v", page)
	}
	if _, exists := page.Entries[0].Fields["unrequested"]; exists {
		t.Fatal("unrequested field exposed")
	}
}

func TestSearchTailRejectsUnsupportedMappingsBeforeSearch(t *testing.T) {
	for _, kind := range []string{"text", "object", "nested", "binary", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			mapping := `{"logs-shortener":{"mappings":{"properties":{"tail":{"type":"` + kind + `"}}}}}`
			repo, capture := newMappedStubES(t, mapping, `{"hits":{"total":{"value":0},"hits":[]}}`)
			if _, err := repo.Search(context.Background(), Filters{Service: "shortener", Fields: map[string]string{"tail": "value"}}, 1); err == nil {
				t.Fatal("unsupported mapping accepted")
			}
			if capture.mappingCalls != 1 || capture.searchCalls != 0 {
				t.Fatalf("unsupported tail reached search: %+v", capture)
			}
		})
	}
}

func TestSearchCoreNeedsNoMappingAndTailRequiresOneService(t *testing.T) {
	repo, capture := newMappedStubES(t, `{}`, `{"hits":{"total":{"value":0},"hits":[]}}`)
	if _, err := repo.Search(context.Background(), Filters{Service: "shortener", Operation: "redirect_resolve", ErrorKind: "not_found", StatusCode: 404}, 1); err != nil {
		t.Fatal(err)
	}
	if capture.mappingCalls != 0 || capture.searchCalls != 1 {
		t.Fatal("core search added mapping request")
	}
	if _, err := repo.Search(context.Background(), Filters{Fields: map[string]string{"tail": "value"}}, 1); err == nil {
		t.Fatal("tail search without service accepted")
	}
	if capture.mappingCalls != 0 || capture.searchCalls != 1 {
		t.Fatal("broad tail request called index")
	}
}

func TestDescribeFieldsReadsScalarBucketsAndFilterability(t *testing.T) {
	mapping := `{"logs-shortener":{"mappings":{"properties":{
		"attempts":{"type":"long"},"active":{"type":"boolean"},"event_date":{"type":"date"},
		"short_code":{"type":"keyword"},"detail":{"type":"text"},"payload":{"type":"object"}
	}}}}`
	response := `{"hits":{"total":{"value":10}},"aggregations":{
		"card_attempts":{"value":1},"top_attempts":{"buckets":[{"key":9223372036854775807}]},
		"top_active":{"buckets":[{"key":1,"key_as_string":"true"},{"key":0}]},
		"top_event_date":{"buckets":[{"key":1791849600000,"key_as_string":"2026-10-13T00:00:00Z"}]},
		"top_short_code":{"buckets":[{"key":"abc123"}]}
	}}`
	repo, _ := newMappedStubES(t, mapping, response)
	fields, _, _, err := repo.DescribeFields(context.Background(), "shortener", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]domain.FieldInfo)
	for _, field := range fields {
		byName[field.Name] = field
	}
	if byName["attempts"].Examples[0] != "9223372036854775807" || byName["active"].Examples[0] != "true" || byName["active"].Examples[1] != "false" || byName["event_date"].Examples[0] != "2026-10-13T00:00:00Z" {
		t.Fatalf("scalar buckets lost semantics: %+v", byName)
	}
	for _, name := range []string{"attempts", "active", "event_date", "short_code"} {
		if !byName[name].ExactFilterable {
			t.Fatalf("scalar mapping marked unfilterable: %s", name)
		}
	}
	for _, name := range []string{"detail", "payload"} {
		if byName[name].ExactFilterable || byName[name].Note == "" {
			t.Fatalf("unsupported mapping missing explicit warning: %s", name)
		}
	}
}

func TestConflictingIndexMappingsAreNotExactFilterable(t *testing.T) {
	mapping := `{"logs-old":{"mappings":{"properties":{"tail":{"type":"keyword"}}}},"logs-new":{"mappings":{"properties":{"tail":{"type":"long"}}}}}`
	repo, capture := newMappedStubES(t, mapping, `{"hits":{"total":{"value":0},"hits":[]}}`)
	if _, err := repo.Search(context.Background(), Filters{Service: "shortener", Fields: map[string]string{"tail": "1"}}, 1); err == nil {
		t.Fatal("conflicting mapping silently chose one physical path")
	}
	if capture.searchCalls != 0 {
		t.Fatal("conflicting mapping reached search")
	}
}

func TestDiscoverServicesReadsTheRegistryFromTheIndex(t *testing.T) {
	c := &capture{}
	repo := newStubES(t, c, `{"aggregations":{"by_service":{"buckets":[
		{"key":"shortener","doc_count":120,"last_seen":{"value_as_string":"2026-08-20T10:00:00Z"}},
		{"key":"analytics","doc_count":8,"last_seen":{"value_as_string":"2026-08-20T09:00:00Z"}}
	]}}}`)

	found, err := repo.DiscoverServices(context.Background(), time.Now().Add(-time.Hour), 0)
	if err != nil {
		t.Fatalf("DiscoverServices() failed: %v", err)
	}

	if len(found) != 2 || found[0].Service != "shortener" || found[0].Records != 120 {
		t.Fatalf("want the services that actually wrote logs, got %+v", found)
	}
	if found[0].LastSeen == "" {
		t.Fatal("last_seen tells an operator whether a quiet service is healthy or gone")
	}
}

func TestDescribeFieldsReportsTypesAndCardinality(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/shortener-logs-*/_mapping", func(w http.ResponseWriter, _ *http.Request) {
		// msg is analyzed text with no keyword subfield: aggregating on it is
		// rejected by the cluster, so it must be described but not counted.
		if _, err := w.Write([]byte(`{"shortener-logs-2026.08.20":{"mappings":{"properties":{
			"msg":{"type":"text"},"short_code":{"type":"keyword"},"_shipper":{"type":"keyword"}}}}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	var requestedAggs []string
	mux.HandleFunc("/shortener-logs-*/_search", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Aggs map[string]any `json:"aggs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		for name := range body.Aggs {
			requestedAggs = append(requestedAggs, name)
		}
		if _, err := w.Write([]byte(`{"hits":{"total":{"value":100}},"aggregations":{
			"card_short_code":{"value":40},
			"top_short_code":{"buckets":[{"key":"abc123"},{"key":"zzz999"}]}}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	repo := NewRepo(srv.Client(), srv.URL, "shortener-logs-*", Auth{})
	fields, records, _, err := repo.DescribeFields(context.Background(), "shortener", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("DescribeFields() failed: %v", err)
	}

	if records != 100 {
		t.Fatalf("want the record count the counts are over, got %d", records)
	}
	byName := make(map[string]int, len(fields))
	for _, f := range fields {
		byName[f.Name] = f.Cardinality
		if f.Name == "_shipper" {
			t.Fatal("the shipper's own bookkeeping is not something anyone filters an investigation by")
		}
	}
	if byName["short_code"] != 40 {
		t.Fatalf("want cardinality from the aggregation, got %v", byName)
	}
	for _, name := range requestedAggs {
		if strings.HasSuffix(name, "_msg") {
			t.Fatalf("aggregating on an analyzed field without a keyword subfield fails the whole request: %q", name)
		}
	}
	var msg domain.FieldInfo
	for _, f := range fields {
		if f.Name == "msg" {
			msg = f
		}
	}
	if msg.Name == "" || msg.Note == "" {
		t.Fatalf("a field that cannot be filtered by exact value must still be described, and said to be so: %+v", msg)
	}
}

func TestPickTimeFallsBackToTheApplicationField(t *testing.T) {
	// slog writes "time"; "@timestamp" is added by the shipper, so an index fed
	// by a different pipeline may carry only the former.
	ts := pickTime(map[string]any{fieldTimeFallbk: "2026-08-20T10:00:00Z"})

	if ts.IsZero() {
		t.Fatal("want the application timestamp when the shipper field is absent")
	}
}

func TestPickTimeReturnsZeroForUnparseableValues(t *testing.T) {
	if got := pickTime(map[string]any{fieldTimestamp: "not a time"}); !got.IsZero() {
		t.Fatalf("want the zero time for an unparseable value, got %v", got)
	}
}

func TestCountLevelsMapsBucketsPerService(t *testing.T) {
	c := &capture{}
	repo := newStubES(t, c, `{"aggregations":{"by_service":{"buckets":[
		{"key":"shortener","by_level":{"buckets":[{"key":"ERROR","doc_count":7},{"key":"warn","doc_count":3}]}}
	]}}}`)

	counts, err := repo.CountLevels(context.Background(), []string{"shortener"}, time.Now())
	if err != nil {
		t.Fatalf("CountLevels() failed: %v", err)
	}

	got := counts["shortener"]
	if got.ErrorCount != 7 || got.WarnCount != 3 {
		t.Fatalf("want 7 errors and 3 warnings regardless of bucket casing, got %+v", got)
	}
}

func TestSearchSurfacesUpstreamFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte(`{"error":"unavailable"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	repo := NewRepo(srv.Client(), srv.URL, "idx", Auth{})
	if _, err := repo.Search(context.Background(), Filters{Since: time.Now()}, 1); err == nil {
		t.Fatal("a non-2xx response must be an error, not an empty result")
	}
}
