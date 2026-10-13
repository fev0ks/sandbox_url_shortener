// Package service implements the incident-investigation use cases exposed as
// MCP tools: vocabulary validation, projection limits, grouping and redaction.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/elasticsearch"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/tempo"
)

// Result caps. They exist to protect the agent's context window, not the
// upstream cluster: every record returned here is re-sent on every later turn.
const (
	MaxErrorGroups  = 20
	MaxRequestLogs  = 50
	MaxSearchHits   = 30
	MaxTraceSpans   = 100
	defaultLogLimit = 20
)

// RequestChainLookback bounds how far back a request chain is searched. A
// request_id is unique, so the bound is not there to narrow the result set; it
// keeps the query off the cold end of the index. It is stated in the tool
// description because an agent that does not know about it reads an empty
// result as "no such request" rather than "older than the bound".
const RequestChainLookback = 24 * time.Hour

// errorLevels are the levels search_errors considers interesting.
var errorLevels = []string{elasticsearch.LevelError, elasticsearch.LevelWarn}

// Identifier shapes. Both values reach this server through log content, and
// request_id additionally comes from the inbound X-Request-ID header, so it is
// whatever a caller chose to send. They are checked before being echoed into a
// result, an outbound URL or a next-step instruction.
var (
	traceIDPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8,32}$`)
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:\-]{1,128}$`)
)

// LogRepo reads projected log records and discovers what can be read.
type LogRepo interface {
	Search(ctx context.Context, f elasticsearch.Filters, limit int) (elasticsearch.Page, error)
	CountLevels(ctx context.Context, services []string, since time.Time) (map[string]domain.ServiceSummary, error)
	RegistryRepo
}

// TraceRepo reads projected trace spans.
type TraceRepo interface {
	GetTrace(ctx context.Context, traceID string) ([]domain.Span, error)
}

// Options configure the discovery behaviour of the service.
type Options struct {
	// Allowlist scopes the server to a subset of what the index holds. Empty
	// means everything discovered is reachable. This is operator scoping, not
	// authorization: over stdio every caller shares one identity, so the only
	// boundary available is the one drawn when the process starts.
	Allowlist []string

	// Domains groups services for the first narrowing step. The log schema
	// carries no domain field, so the grouping is configured rather than
	// discovered.
	Domains map[string][]string

	// DiscoveryTTL is how long a discovered registry stays usable.
	DiscoveryTTL time.Duration

	KibanaURL string
}

// Investigation orchestrates the incident-investigation use cases.
type Investigation struct {
	logs      LogRepo
	traces    TraceRepo
	registry  *registry
	kibanaURL string
	now       func() time.Time
}

// NewInvestigation creates the use-case service.
func NewInvestigation(logs LogRepo, traces TraceRepo, opts Options) *Investigation {
	now := time.Now
	return &Investigation{
		logs:      logs,
		traces:    traces,
		registry:  newRegistry(logs, opts.Allowlist, opts.Domains, opts.DiscoveryTTL, now),
		kibanaURL: strings.TrimSuffix(opts.KibanaURL, "/"),
		now:       now,
	}
}

// ServicesOverview reports which services are noisy inside the window.
type ServicesOverview struct {
	Window   domain.Window           `json:"window"`
	Domain   string                  `json:"domain,omitempty"`
	Services []domain.ServiceSummary `json:"services"`

	// Domains is offered when the answer was too wide to be useful, so the
	// agent can narrow instead of picking from a long list of names it does not
	// recognise.
	Domains   []string          `json:"domains,omitempty"`
	Truncated domain.Truncation `json:"truncation"`
	NextStep  string            `json:"next_step"`
	KibanaURL string            `json:"kibana_url,omitempty"`
}

// maxListedServices bounds one list_services answer. Past this the list stops
// being a starting point and becomes something to page through, which is what
// the domain narrowing exists to avoid.
const maxListedServices = 12

// ListServices answers "where should I look" using an aggregation rather than
// documents, so the first step of an investigation costs almost no context.
func (s *Investigation) ListServices(ctx context.Context, window domain.Window, domainName string) (*ServicesOverview, error) {
	since, err := s.since(window)
	if err != nil {
		return nil, err
	}

	known, err := s.registry.Services(ctx)
	if err != nil {
		return nil, err
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("%w: no service has written to the log index in the last %s",
			domain.ErrUnknownValue, DiscoveryWindow)
	}

	domainName = strings.TrimSpace(domainName)
	if domainName != "" {
		known, err = s.filterByDomain(known, domainName)
		if err != nil {
			return nil, err
		}
	}

	wanted := names(known)
	counts, err := s.logs.CountLevels(ctx, wanted, since)
	if err != nil {
		return nil, fmt.Errorf("count levels: %w", err)
	}

	summaries := make([]domain.ServiceSummary, 0, len(wanted))
	for _, name := range wanted {
		summary, ok := counts[name]
		if !ok {
			summary = domain.ServiceSummary{Service: name}
		}
		summary.Service = name
		summaries = append(summaries, summary)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].ErrorCount > summaries[j].ErrorCount
	})

	total := len(summaries)
	capped := total > maxListedServices
	if capped {
		// The noisiest come first, so the cut falls on the quiet ones.
		summaries = summaries[:maxListedServices]
	}

	overview := &ServicesOverview{
		Window:    window,
		Domain:    domainName,
		Services:  summaries,
		Truncated: domain.Truncation{Returned: len(summaries), Total: total, Capped: capped},
		KibanaURL: s.kibanaLink(levelQuery()),
	}

	switch {
	case capped && domainName == "" && len(s.registry.Domains()) > 0:
		overview.Domains = s.registry.Domains()
		overview.NextStep = fmt.Sprintf(
			"%d services matched and the %d noisiest are shown. Call list_services again with a domain to see one area in full.",
			total, len(summaries))
	case len(summaries) > 0 && summaries[0].ErrorCount > 0:
		overview.NextStep = fmt.Sprintf("Call search_errors with service=%q and the same window.", summaries[0].Service)
	default:
		overview.NextStep = "No errors in this window. Widen the window or use search_logs with an explicit filter."
	}
	return overview, nil
}

func (s *Investigation) filterByDomain(known []domain.ServiceInfo, want string) ([]domain.ServiceInfo, error) {
	available := s.registry.Domains()
	if len(available) == 0 {
		return nil, fmt.Errorf("%w: domain %q, no domains are configured on this server",
			domain.ErrUnknownValue, clip(want, 64))
	}

	out := make([]domain.ServiceInfo, 0, len(known))
	for _, svc := range known {
		if svc.Domain == want {
			out = append(out, svc)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: domain %q, expected one of %s",
			domain.ErrUnknownValue, clip(want, 64), strings.Join(available, ", "))
	}
	return out, nil
}

// DescribeService reports what one service can be filtered by.
//
// This is the progressive-disclosure half of the design: the model asks about
// the one service it is investigating instead of carrying every service's
// schema in every request.
func (s *Investigation) DescribeService(ctx context.Context, service string, window domain.Window) (*domain.ServiceDescription, error) {
	if err := s.validateService(ctx, service); err != nil {
		return nil, err
	}
	since, err := s.since(window)
	if err != nil {
		return nil, err
	}

	fields, records, truncated, err := s.logs.DescribeFields(ctx, service, since)
	if err != nil {
		return nil, fmt.Errorf("describe service: %w", err)
	}

	fields = sanitizeDescribedFields(fields, records)

	next := "Use tail fields marked exact_filterable in the filters argument of search_logs."
	if len(fields) == 0 {
		next = "No fields found for this service in the window. Widen the window."
	}

	return &domain.ServiceDescription{
		Service:   service,
		Window:    window,
		Records:   records,
		Fields:    fields,
		NextStep:  next,
		Truncated: truncated,
	}, nil
}

// sanitizeDescribedFields creates an output projection without changing shared
// repository/cache metadata. Examples can contain the same secrets as log text.
func sanitizeDescribedFields(fields []domain.FieldInfo, records int) []domain.FieldInfo {
	projected := append([]domain.FieldInfo(nil), fields...)
	for i := range projected {
		field := &projected[i]
		field.Name = clip(field.Name, maxFieldLen)
		field.Type = clip(field.Type, maxFieldLen)
		note := field.Note
		if warning := fieldNote(*field, records); warning != "" {
			if note != "" {
				note += "; "
			}
			note += warning
		}
		field.Note = clip(redact(note), maxFieldLen)
		field.Examples = make([]string, len(fields[i].Examples))
		for j, example := range fields[i].Examples {
			field.Examples[j] = redact(example)
		}
	}
	return projected
}

// fieldNote warns about a field that is not worth grouping by. Cardinality is
// the number an agent needs and the one it never asks for: a field with one
// distinct value per record identifies records, it does not describe them.
func fieldNote(f domain.FieldInfo, records int) string {
	switch {
	case records > 0 && f.Cardinality >= records:
		return "unique per record - use it to look one record up, never to group by"
	case f.Cardinality == 1:
		return "single value in this window - filtering on it changes nothing"
	default:
		return ""
	}
}

// ErrorReport is the grouped result of search_errors.
type ErrorReport struct {
	Service string              `json:"service"`
	Window  domain.Window       `json:"window"`
	Groups  []domain.ErrorGroup `json:"groups"`

	// Truncation counts groups, matching what Groups holds.
	Truncation domain.Truncation `json:"truncation"`

	// Coverage counts the records the groups were built from. Grouping happens
	// in this process, so the counts inside Groups can only describe records
	// that were actually fetched.
	Coverage  domain.Coverage `json:"coverage"`
	NextStep  string          `json:"next_step"`
	KibanaURL string          `json:"kibana_url,omitempty"`
}

// SearchErrors returns grouped failures of a single service.
//
// One service at a time is a deliberate restriction: sweeping the whole cluster
// is exactly the request that fills the context window with noise.
func (s *Investigation) SearchErrors(ctx context.Context, service string, window domain.Window, limit int) (*ErrorReport, error) {
	if err := s.validateService(ctx, service); err != nil {
		return nil, err
	}
	since, err := s.since(window)
	if err != nil {
		return nil, err
	}
	limit = clamp(limit, defaultLogLimit, MaxErrorGroups)

	page, err := s.logs.Search(ctx, elasticsearch.Filters{
		Service: service,
		Since:   since,
		Levels:  errorLevels,
	}, elasticsearch.MaxScanHint)
	if err != nil {
		return nil, fmt.Errorf("search errors: %w", err)
	}

	groups := groupErrors(page.Entries)
	totalGroups := len(groups)
	capped := totalGroups > limit
	if capped {
		groups = groups[:limit]
	}

	// The scan is bounded, so a busy window yields groups counted over a sample.
	// Reporting the group cap alone would leave that invisible, and a count over
	// a sample presented as a total is exactly the kind of number an agent
	// repeats as fact.
	coverage := domain.Coverage{
		Scanned:        len(page.Entries),
		Matched:        page.Total,
		MatchedAtLeast: page.TotalAtLeast,
		Partial:        page.Total > len(page.Entries),
	}
	if coverage.Partial {
		coverage.Note = fmt.Sprintf(
			"Counts below are over the %d most recent matching records, not all %d. "+
				"Treat them as relative weights, and first_seen as the start of the sample, not of the incident. "+
				"Narrow the window for exact counts.",
			coverage.Scanned, coverage.Matched)
	}

	next := "No errors found. Try a wider window, or search_logs with an explicit filter."
	if len(groups) > 0 {
		next = "Call search_logs with the operation from the group above to see surrounding records."
		if id := groups[0].SampleRequestID; id != "" {
			next = fmt.Sprintf("Call get_request_logs with request_id=%q to see the full request chain.", id)
		}
	}

	return &ErrorReport{
		Service:    service,
		Window:     window,
		Groups:     groups,
		Truncation: domain.Truncation{Returned: len(groups), Total: totalGroups, Capped: capped},
		Coverage:   coverage,
		NextStep:   next,
		KibanaURL:  s.kibanaLink(fmt.Sprintf(`service:"%s" and %s`, service, levelQuery())),
	}, nil
}

// RequestChain is the result of get_request_logs.
type RequestChain struct {
	RequestID  string            `json:"request_id"`
	Entries    []domain.LogEntry `json:"entries"`
	Truncation domain.Truncation `json:"truncation"`
	TraceID    string            `json:"trace_id,omitempty"`
	NextStep   string            `json:"next_step"`
	KibanaURL  string            `json:"kibana_url,omitempty"`
}

// GetRequestLogs returns the beginning of one request chain, oldest first.
func (s *Investigation) GetRequestLogs(ctx context.Context, requestID string, limit int) (*RequestChain, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, fmt.Errorf("%w: request_id is required", domain.ErrTooBroad)
	}
	if !requestIDPattern.MatchString(requestID) {
		return nil, fmt.Errorf("%w: request_id %q, expected up to 128 characters of letters, digits, dot, colon, underscore or hyphen",
			domain.ErrMalformedValue, clip(requestID, 64))
	}
	limit = clamp(limit, MaxRequestLogs, MaxRequestLogs)

	// Oldest first at the source, not after the fact. Fetching the newest N and
	// reversing them in process returns the tail of a long chain labelled as its
	// head, which hides the very records that explain how the request failed.
	page, err := s.logs.Search(ctx, elasticsearch.Filters{
		RequestID: requestID,
		Since:     s.now().Add(-RequestChainLookback),
		Oldest:    true,
	}, limit)
	if err != nil {
		return nil, fmt.Errorf("search request chain: %w", err)
	}

	entries := page.Entries
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	var traceID string
	for i := range entries {
		sanitizeEntry(&entries[i])
		if traceID == "" && entries[i].TraceID != "" {
			traceID = entries[i].TraceID
		}
	}

	capped := page.Total > len(entries)

	next := "Chain complete. Report the failing operation and its error_kind."
	switch {
	case len(entries) == 0:
		next = fmt.Sprintf("No records for this request_id within the last %s. "+
			"Check the identifier, or use search_errors to find a current failure.", RequestChainLookback)
	case capped:
		next = fmt.Sprintf("Showing the first %d records of the chain; %d matched. "+
			"The failure may be past the cap - narrow with search_logs on the failing operation.",
			len(entries), page.Total)
	case traceID != "":
		next = fmt.Sprintf("Call get_trace with trace_id=%q to see where the time was spent.", traceID)
	}

	return &RequestChain{
		RequestID:  requestID,
		Entries:    entries,
		Truncation: domain.Truncation{Returned: len(entries), Total: page.Total, Capped: capped, TotalAtLeast: page.TotalAtLeast},
		TraceID:    traceID,
		NextStep:   next,
		KibanaURL:  s.kibanaLink(fmt.Sprintf(`request_id:"%s"`, requestID)),
	}, nil
}

// TraceReport is the result of get_trace.
type TraceReport struct {
	TraceID    string            `json:"trace_id"`
	Spans      []domain.Span     `json:"spans"`
	TotalMS    float64           `json:"total_ms"`
	SlowestID  string            `json:"slowest_span_id,omitempty"`
	Truncation domain.Truncation `json:"truncation"`
	NextStep   string            `json:"next_step"`
}

// GetTrace returns the spans of one trace with the slowest one highlighted.
func (s *Investigation) GetTrace(ctx context.Context, traceID string) (*TraceReport, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("%w: trace_id is required", domain.ErrTooBroad)
	}
	if !traceIDPattern.MatchString(traceID) {
		return nil, fmt.Errorf("%w: trace_id %q, expected 8 to 32 hexadecimal characters",
			domain.ErrMalformedValue, clip(traceID, 64))
	}

	spans, err := s.traces.GetTrace(ctx, traceID)
	if err != nil {
		if errors.Is(err, tempo.ErrNotFound) {
			return nil, fmt.Errorf("trace %q not found: it may have expired or never been sampled", traceID)
		}
		return nil, fmt.Errorf("get trace: %w", err)
	}

	// Aggregates are computed over every span, before the cap. Computing them
	// afterwards silently answers a different question: the slowest span of the
	// first hundred, and a root duration that is zero whenever the root did not
	// make the cut.
	total := len(spans)
	slowest, totalMS := traceAggregates(spans)

	capped := total > MaxTraceSpans
	if capped {
		spans = spans[:MaxTraceSpans]
		spans = ensureSpan(spans, slowest)
	}

	return &TraceReport{
		TraceID:    traceID,
		Spans:      spans,
		TotalMS:    totalMS,
		SlowestID:  slowest.SpanID,
		Truncation: domain.Truncation{Returned: len(spans), Total: total, Capped: capped},
		NextStep:   "Compare the slowest span against the failing operation from the log chain.",
	}, nil
}

// traceAggregates returns the slowest span and the wall-clock duration of the
// trace. With several root spans - orphans are normal in Tempo when a parent
// was not sampled - the longest root is the closest available answer.
func traceAggregates(spans []domain.Span) (slowest domain.Span, totalMS float64) {
	for _, sp := range spans {
		if sp.DurationMS > slowest.DurationMS {
			slowest = sp
		}
		if sp.ParentID == "" && sp.DurationMS > totalMS {
			totalMS = sp.DurationMS
		}
	}
	// A trace whose roots were all dropped still has a span of maximum extent.
	if totalMS == 0 {
		totalMS = slowest.DurationMS
	}
	return slowest, totalMS
}

// ensureSpan keeps a highlighted span present after the list was capped.
// Naming a span the caller cannot see is worse than one extra row.
func ensureSpan(spans []domain.Span, want domain.Span) []domain.Span {
	if want.SpanID == "" {
		return spans
	}
	for _, sp := range spans {
		if sp.SpanID == want.SpanID {
			return spans
		}
	}
	return append(spans, want)
}

// SearchQuery is the argument set of search_logs.
type SearchQuery struct {
	Service    string
	Window     domain.Window
	Operation  string
	ErrorKind  string
	StatusCode int
	Text       string

	// Fields are the service-specific equality filters. They live in a map
	// rather than as typed arguments so that adding a service does not widen
	// the schema every other service has to carry.
	Fields map[string]string
	Limit  int
}

// SearchResult is the result of search_logs.
type SearchResult struct {
	Entries    []domain.LogEntry `json:"entries"`
	Truncation domain.Truncation `json:"truncation"`
	NextStep   string            `json:"next_step"`
	KibanaURL  string            `json:"kibana_url,omitempty"`
}

// SearchLogs is the escape hatch for cases the first three tools do not cover.
//
// It still refuses an unbounded request: without a service and at least one
// narrowing filter the query degenerates into a full index scan.
func (s *Investigation) SearchLogs(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	if err := s.validateService(ctx, q.Service); err != nil {
		return nil, err
	}
	since, err := s.since(q.Window)
	if err != nil {
		return nil, err
	}
	if q.Operation == "" && q.ErrorKind == "" && q.StatusCode == 0 && q.Text == "" && len(q.Fields) == 0 {
		return nil, fmt.Errorf("%w: provide at least one of operation, error_kind, status_code, text, "+
			"or a service-specific filter from describe_service; "+
			"use search_errors for an unfiltered look at failures", domain.ErrTooBroad)
	}
	if q.StatusCode != 0 && (q.StatusCode < 100 || q.StatusCode > 599) {
		return nil, fmt.Errorf("%w: status_code %d, expected an HTTP status between 100 and 599",
			domain.ErrUnknownValue, q.StatusCode)
	}
	if err := s.validateFields(ctx, q.Service, q.Fields); err != nil {
		return nil, err
	}

	limit := clamp(q.Limit, defaultLogLimit, MaxSearchHits)
	page, err := s.logs.Search(ctx, elasticsearch.Filters{
		Service:    q.Service,
		Since:      since,
		Operation:  q.Operation,
		ErrorKind:  q.ErrorKind,
		StatusCode: q.StatusCode,
		Text:       q.Text,
		Fields:     q.Fields,
	}, limit)
	if err != nil {
		return nil, fmt.Errorf("search logs: %w", err)
	}

	entries := page.Entries
	for i := range entries {
		sanitizeEntry(&entries[i])
	}

	next := "Nothing matched. Relax one filter or widen the window."
	if len(entries) > 0 {
		next = "Use get_request_logs with a request_id from the entries above to see the full chain."
	}

	return &SearchResult{
		Entries: entries,
		Truncation: domain.Truncation{
			Returned:     len(entries),
			Total:        page.Total,
			Capped:       page.Total > len(entries),
			TotalAtLeast: page.TotalAtLeast,
		},
		NextStep:  next,
		KibanaURL: s.kibanaLink(fmt.Sprintf(`service:"%s"`, q.Service)),
	}, nil
}

// Domains exposes the configured domain vocabulary for the tool schema. Domains
// are few and change rarely, so unlike service names they can stay an enum.
func (s *Investigation) Domains() []string { return s.registry.Domains() }

// validateService checks a name against the live registry.
//
// The schema no longer carries the list, because inlining it costs tokens on
// every turn once there are dozens of services. The guarantee moves here: an
// unknown name is refused, and the refusal offers the names it was probably
// meant to be rather than only saying no.
func (s *Investigation) validateService(ctx context.Context, service string) error {
	service = strings.TrimSpace(service)
	if service == "" {
		return fmt.Errorf("%w: service is required; call list_services to see which ones exist", domain.ErrTooBroad)
	}

	known, err := s.registry.Services(ctx)
	if err != nil {
		return err
	}
	for _, svc := range known {
		if svc.Service == service {
			return nil
		}
	}

	if near := nearest(service, names(known), 3); len(near) > 0 {
		return fmt.Errorf("%w: service %q. Closest known: %s. Call list_services for the full set",
			domain.ErrUnknownValue, clip(service, 64), strings.Join(near, ", "))
	}
	return fmt.Errorf("%w: service %q is not writing logs. Call list_services to see which ones are",
		domain.ErrUnknownValue, clip(service, 64))
}

// validateFields checks tail filters against what the service actually emits.
//
// Server-side validation is what replaces the typed argument that used to exist
// for short_code. Without it a misspelled key would be sent to Elasticsearch as
// a filter that matches nothing, and an empty result reads to an agent as "this
// did not happen" rather than "you asked the wrong question".
func (s *Investigation) validateFields(ctx context.Context, service string, filters map[string]string) error {
	if len(filters) == 0 {
		return nil
	}

	fields, err := s.registry.Fields(ctx, service)
	if err != nil {
		return err
	}

	available := make([]string, 0, len(fields))
	tail := make(map[string]domain.FieldInfo, len(fields))
	for _, f := range fields {
		if f.Kind == domain.FieldTail && !elasticsearch.IsCoreField(f.Name) {
			if f.ExactFilterable {
				available = append(available, f.Name)
			}
			tail[f.Name] = f
		}
	}

	for name, value := range filters {
		if value == "" {
			return fmt.Errorf("%w: filter %q has no value", domain.ErrTooBroad, clip(name, 64))
		}
		if field, known := tail[name]; known {
			if !field.ExactFilterable {
				return fmt.Errorf("%w: field %q cannot be filtered by exact value; use an exact_filterable field from describe_service", domain.ErrUnknownValue, clip(name, 64))
			}
			continue
		}
		if elasticsearch.IsCoreField(name) {
			return fmt.Errorf("%w: %q is a core field and cannot be used in filters; use the supported tool arguments to narrow the search",
				domain.ErrUnknownValue, clip(name, 64))
		}
		if near := nearest(name, available, 3); len(near) > 0 {
			return fmt.Errorf("%w: %q is not a field of %s. Closest: %s. Call describe_service for the full set",
				domain.ErrUnknownValue, clip(name, 64), service, strings.Join(near, ", "))
		}
		return fmt.Errorf("%w: %q is not a field of %s; call describe_service to see what is",
			domain.ErrUnknownValue, clip(name, 64), service)
	}
	return nil
}

func (s *Investigation) since(window domain.Window) (time.Time, error) {
	d, err := window.Duration()
	if err != nil {
		return time.Time{}, err
	}
	return s.now().Add(-d), nil
}

// levelQuery mirrors the levels search_errors actually queries. A link that
// shows a different set than the tool defeats the point of offering it: it
// exists so a human can check the agent's claim against the same records.
func levelQuery() string {
	return fmt.Sprintf(`level:("%s" or "%s")`, elasticsearch.LevelError, elasticsearch.LevelWarn)
}

// kibanaLink builds a link a human can follow to verify the agent's claim.
//
// The query sits in the URL fragment, so it is percent-escaped rather than
// query-escaped: QueryEscape encodes a space as "+", which a fragment carries
// through literally and Kibana then parses as part of the search term.
func (s *Investigation) kibanaLink(query string) string {
	if s.kibanaURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/app/discover#/?_a=(query:(language:kuery,query:'%s'))",
		s.kibanaURL, strings.ReplaceAll(url.PathEscape(query), "'", "%27"))
}

// groupErrors collapses repeated failures into counted groups, newest first.
func groupErrors(entries []domain.LogEntry) []domain.ErrorGroup {
	index := make(map[string]*domain.ErrorGroup)
	order := make([]string, 0, len(entries))

	for _, e := range entries {
		pattern := normalizeMessage(e.Message)
		key := strings.Join([]string{e.Service, e.Operation, e.ErrorKind, pattern}, "|")

		group, ok := index[key]
		if !ok {
			group = &domain.ErrorGroup{
				Service:         clip(e.Service, maxFieldLen),
				Operation:       clip(e.Operation, maxFieldLen),
				ErrorKind:       clip(e.ErrorKind, maxFieldLen),
				MessagePattern:  redact(pattern),
				FirstSeen:       e.Timestamp,
				LastSeen:        e.Timestamp,
				SampleRequestID: sampleID(e.RequestID, requestIDPattern),
				SampleTraceID:   sampleID(e.TraceID, traceIDPattern),
			}
			index[key] = group
			order = append(order, key)
		}

		if group.SampleError == "" {
			if diagnostic := redact(e.Error); diagnostic != "" {
				group.SampleError = diagnostic
				// A diagnostic and its identifiers form one sample. Drop missing
				// or invalid identifiers rather than borrowing another cause's IDs.
				group.SampleRequestID = sampleID(e.RequestID, requestIDPattern)
				group.SampleTraceID = sampleID(e.TraceID, traceIDPattern)
			}
		}

		group.Count++
		if e.Timestamp.Before(group.FirstSeen) {
			group.FirstSeen = e.Timestamp
		}
		if e.Timestamp.After(group.LastSeen) {
			group.LastSeen = e.Timestamp
		}
		if group.SampleError == "" && group.SampleRequestID == "" {
			group.SampleRequestID = sampleID(e.RequestID, requestIDPattern)
		}
		if group.SampleError == "" && group.SampleTraceID == "" {
			group.SampleTraceID = sampleID(e.TraceID, traceIDPattern)
		}
	}

	groups := make([]domain.ErrorGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, *index[key])
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Count > groups[j].Count })
	return groups
}

// sampleID drops an identifier that does not have the expected shape. A sample
// is offered to the model as the argument of the next call, and a malformed one
// would only be rejected a turn later.
func sampleID(value string, shape *regexp.Regexp) string {
	if shape.MatchString(value) {
		return value
	}
	return ""
}

func clamp(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		return max
	}
	return value
}
