// Package elasticsearch reads projected log records from the shortener log index.
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// Field names of the standardized log schema. They are a contract with the log
// shipping pipeline, so they live as constants rather than inline literals.
const (
	// fieldTimestamp is added by the shipper, not by the application: Fluent Bit
	// writes it because the Elasticsearch output runs with Logstash_Format On.
	// Sorting and range filtering both depend on it, so an index built by a
	// different pipeline must map it before these queries work.
	fieldTimestamp  = "@timestamp"
	fieldTimeFallbk = "time"
	fieldService    = "service"
	fieldLevel      = "level"
	fieldMessage    = "msg"
	fieldOperation  = "operation"
	fieldErrorKind  = "error_kind"
	fieldError      = "error"
	fieldRequestID  = "request_id"
	fieldTraceID    = "trace_id"
	fieldStatusCode = "status_code"
	fieldMethod     = "method"
	fieldPath       = "path"
)

// ServicePlaceholder marks where a service name goes in the index pattern.
// With logs sharded per service, a query that does not name one is not merely
// broad - it fans out across every shard in the cluster.
const ServicePlaceholder = "{service}"

// CoreFields are the fields in the shared log contract. They are projected
// whenever present, including output-only diagnostics such as error;
// everything else is discovered per service.
var CoreFields = []string{
	fieldTimestamp, fieldService, fieldLevel, fieldMessage, fieldOperation,
	fieldErrorKind, fieldError, fieldRequestID, fieldTraceID, fieldStatusCode, fieldMethod, fieldPath,
}

// IsCoreField reports whether a field is part of the shared contract.
func IsCoreField(name string) bool {
	for _, f := range CoreFields {
		if f == name {
			return true
		}
	}
	return name == fieldTimeFallbk
}

// maxScan bounds how many documents a single search may pull from the index.
// It protects the upstream cluster; the per-tool caps protect the context window.
const maxScan = 500

// MaxScanHint is the widest scan a caller may ask for.
const MaxScanHint = maxScan

// trackTotalHits bounds the exact-count work the cluster does. Beyond this the
// total is reported as a lower bound, which is all an investigator needs: the
// difference between "9 000" and "10 000+" changes no decision, but counting
// every match exactly on a large index is real cluster time.
const trackTotalHits = 10000

// Levels of the standardized schema. slog writes them upper-case, and a terms
// filter is exact, so the constants are the single source of that casing.
const (
	LevelError = "ERROR"
	LevelWarn  = "WARN"
)

// Auth carries optional credentials for the log index. The zero value means an
// unauthenticated endpoint, which is what the local stack runs.
type Auth struct {
	Username string
	Password string
	APIKey   string
}

func (a Auth) header() string {
	switch {
	case a.APIKey != "":
		return "ApiKey " + a.APIKey
	case a.Username != "" || a.Password != "":
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password))
	default:
		return ""
	}
}

// Repo queries the log index over the Elasticsearch REST API.
type Repo struct {
	client  *http.Client
	baseURL string
	index   string
	auth    Auth
}

// NewRepo creates a log repository.
func NewRepo(client *http.Client, baseURL, index string, auth Auth) *Repo {
	return &Repo{client: client, baseURL: baseURL, index: index, auth: auth}
}

// Filters narrows a log search. Since is always applied: a search without a
// time bound scans the whole index for an unbounded period.
type Filters struct {
	Service    string
	Since      time.Time
	Levels     []string
	Operation  string
	ErrorKind  string
	RequestID  string
	StatusCode int
	Text       string

	// Fields carries service-specific equality filters. They are validated
	// against the service's discovered fields before reaching this layer.
	Fields map[string]string

	// Oldest asks for the oldest matches rather than the newest. It matters
	// whenever the caller wants the beginning of something: taking the newest N
	// records of a request chain and reversing them in process yields the tail
	// of the chain presented as its head.
	Oldest bool
}

// Page is one page of matching entries plus what it took to build it.
type Page struct {
	Entries []domain.LogEntry

	// Total is the number of matching documents, capped by trackTotalHits.
	Total int

	// TotalAtLeast reports that Total hit that cap and is a lower bound.
	TotalAtLeast bool
}

// Search returns matching entries along with the total number of matches so the
// caller can report truncation.
func (r *Repo) Search(ctx context.Context, f Filters, limit int) (Page, error) {
	filters := buildFilters(f)
	tail, err := r.tailFilters(ctx, f)
	if err != nil {
		return Page{}, err
	}
	filters = append(filters, tail...)
	if limit <= 0 || limit > maxScan {
		limit = maxScan
	}

	order := "desc"
	if f.Oldest {
		order = "asc"
	}

	body := map[string]any{
		"size":             limit,
		"track_total_hits": trackTotalHits,
		"sort":             []any{map[string]any{fieldTimestamp: map[string]any{"order": order}}},
		"query":            map[string]any{"bool": map[string]any{"filter": filters}},
	}

	raw, err := r.post(ctx, "/"+r.indexFor(f.Service)+"/_search", body)
	if err != nil {
		return Page{}, err
	}

	var resp searchResponse
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&resp); err != nil {
		return Page{}, fmt.Errorf("decode search response: %w", err)
	}

	entries := make([]domain.LogEntry, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		entries = append(entries, projectEntry(hit.Source, sortedKeys(f.Fields)))
	}
	return Page{
		Entries:      entries,
		Total:        resp.Hits.Total.Value,
		TotalAtLeast: resp.Hits.Total.Relation == "gte",
	}, nil
}

// CountLevels returns per-service error and warning counts for the window.
// It answers "which service is noisy" with an aggregation instead of documents,
// which is both cheaper for the cluster and far smaller in the agent's context.
func (r *Repo) CountLevels(ctx context.Context, services []string, since time.Time) (map[string]domain.ServiceSummary, error) {
	body := map[string]any{
		"size": 0,
		"query": map[string]any{"bool": map[string]any{"filter": []any{
			rangeSince(since),
			map[string]any{"terms": map[string]any{keyword(fieldService): services}},
			map[string]any{"terms": map[string]any{keyword(fieldLevel): []string{LevelError, LevelWarn}}},
		}}},
		"aggs": map[string]any{
			"by_service": map[string]any{
				"terms": map[string]any{"field": keyword(fieldService), "size": len(services)},
				"aggs": map[string]any{
					"by_level": map[string]any{
						"terms": map[string]any{"field": keyword(fieldLevel), "size": 2},
					},
				},
			},
		},
	}

	raw, err := r.post(ctx, "/"+r.indexFor("")+"/_search", body)
	if err != nil {
		return nil, err
	}

	var resp aggResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode aggregation response: %w", err)
	}

	out := make(map[string]domain.ServiceSummary, len(resp.Aggregations.ByService.Buckets))
	for _, svc := range resp.Aggregations.ByService.Buckets {
		summary := domain.ServiceSummary{Service: svc.Key}
		for _, lvl := range svc.ByLevel.Buckets {
			switch strings.ToUpper(lvl.Key) {
			case LevelError:
				summary.ErrorCount = lvl.DocCount
			case LevelWarn:
				summary.WarnCount = lvl.DocCount
			}
		}
		out[svc.Key] = summary
	}
	return out, nil
}

func buildFilters(f Filters) []any {
	filters := []any{rangeSince(f.Since)}

	if f.Service != "" {
		filters = append(filters, term(keyword(fieldService), f.Service))
	}
	if len(f.Levels) > 0 {
		levels := make([]string, 0, len(f.Levels))
		for _, l := range f.Levels {
			levels = append(levels, strings.ToUpper(l))
		}
		filters = append(filters, map[string]any{"terms": map[string]any{keyword(fieldLevel): levels}})
	}
	if f.Operation != "" {
		filters = append(filters, term(keyword(fieldOperation), f.Operation))
	}
	if f.ErrorKind != "" {
		filters = append(filters, term(keyword(fieldErrorKind), f.ErrorKind))
	}
	if f.RequestID != "" {
		filters = append(filters, term(keyword(fieldRequestID), f.RequestID))
	}
	if f.StatusCode > 0 {
		filters = append(filters, term(fieldStatusCode, f.StatusCode))
	}
	if f.Text != "" {
		filters = append(filters, map[string]any{
			"match_phrase": map[string]any{fieldMessage: f.Text},
		})
	}
	return filters
}

// tailFilters resolves physical mapping paths locally. No client-supplied
// field path reaches Elasticsearch, and unsupported mappings fail explicitly.
func (r *Repo) tailFilters(ctx context.Context, f Filters) ([]any, error) {
	if len(f.Fields) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(f.Service) == "" {
		return nil, fmt.Errorf("%w: tail filters require one service", domain.ErrTooBroad)
	}
	_, mapped, err := r.mappingFields(ctx, f.Service)
	if err != nil {
		return nil, fmt.Errorf("resolve tail filter mapping: %w", err)
	}
	filters := make([]any, 0, len(f.Fields))
	for _, name := range sortedKeys(f.Fields) {
		property, exists := mapped[name]
		if !exists || property.aggPath(name) == "" || IsCoreField(name) {
			return nil, fmt.Errorf("%w: tail filter is not an approved exact-match scalar field; call describe_service", domain.ErrUnknownValue)
		}
		filters = append(filters, term(property.aggPath(name), f.Fields[name]))
	}
	return filters, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func rangeSince(since time.Time) map[string]any {
	return map[string]any{"range": map[string]any{
		fieldTimestamp: map[string]any{"gte": since.UTC().Format(time.RFC3339)},
	}}
}

func term(field string, value any) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

// keyword targets the exact-match subfield used for filtering in the index.
func keyword(field string) string { return field + ".keyword" }

func (r *Repo) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return r.do(req)
}

func (r *Repo) post(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return r.do(req)
}

func (r *Repo) do(req *http.Request) ([]byte, error) {
	if h := r.auth.header(); h != "" {
		req.Header.Set("Authorization", h)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call elasticsearch: %w", err)
	}
	// Response cleanup cannot change a fully read payload or its existing error.
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read elasticsearch response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elasticsearch returned %d: %s", resp.StatusCode, truncateBody(raw))
	}
	return raw, nil
}

func truncateBody(b []byte) string {
	const maxBytes = 512
	if len(b) <= maxBytes {
		return string(b)
	}
	return string(b[:maxBytes]) + "...(truncated)"
}

type searchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
			// Relation is "eq" for an exact count and "gte" once the count hit
			// the track_total_hits bound.
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type aggResponse struct {
	Aggregations struct {
		ByService struct {
			Buckets []struct {
				Key     string `json:"key"`
				ByLevel struct {
					Buckets []struct {
						Key      string `json:"key"`
						DocCount int    `json:"doc_count"`
					} `json:"buckets"`
				} `json:"by_level"`
			} `json:"buckets"`
		} `json:"by_service"`
	} `json:"aggregations"`
}

// projectEntry keeps the handful of fields an investigator reads and drops the
// rest of the document. This is where the context budget is actually defended.
//
// tail names the service-specific fields the caller filtered on. Those come
// back because the caller asked about them; the rest of the document does not,
// however many fields it has.
func projectEntry(src map[string]any, tail []string) domain.LogEntry {
	entry := projectCore(src)
	for _, name := range tail {
		if v := scalarString(src[name]); v != "" {
			if entry.Fields == nil {
				entry.Fields = make(map[string]string, len(tail))
			}
			entry.Fields[name] = v
		}
	}
	return entry
}

func projectCore(src map[string]any) domain.LogEntry {
	return domain.LogEntry{
		Timestamp:  pickTime(src),
		Service:    pickString(src, fieldService),
		Level:      pickString(src, fieldLevel),
		Operation:  pickString(src, fieldOperation),
		ErrorKind:  pickString(src, fieldErrorKind),
		Message:    pickString(src, fieldMessage),
		Error:      pickString(src, fieldError),
		RequestID:  pickString(src, fieldRequestID),
		TraceID:    pickString(src, fieldTraceID),
		StatusCode: pickInt(src, fieldStatusCode),
		Method:     pickString(src, fieldMethod),
		Path:       pickString(src, fieldPath),
	}
}

func pickTime(src map[string]any) time.Time {
	for _, field := range []string{fieldTimestamp, fieldTimeFallbk} {
		if raw := pickString(src, field); raw != "" {
			if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				return ts
			}
		}
	}
	return time.Time{}
}

func pickString(src map[string]any, field string) string {
	if v, ok := src[field].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func pickInt(src map[string]any, field string) int {
	switch v := src[field].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		value, err := v.Int64()
		if err == nil {
			return int(value)
		}
		return 0
	default:
		return 0
	}
}

func scalarString(value any) string {
	switch scalar := value.(type) {
	case string:
		return strings.TrimSpace(scalar)
	case json.Number:
		return scalar.String()
	case bool:
		return strconv.FormatBool(scalar)
	case float64:
		return strconv.FormatFloat(scalar, 'g', -1, 64)
	case int:
		return strconv.Itoa(scalar)
	case int64:
		return strconv.FormatInt(scalar, 10)
	default:
		return ""
	}
}

// indexFor resolves the configured index pattern for one service.
//
// A pattern containing {service} means logs are sharded per service. Leaving
// the placeholder unresolved would query an index literally named "{service}";
// resolving it to "*" for a query that names no service keeps the fan-out
// explicit rather than accidental.
func (r *Repo) indexFor(service string) string {
	if !strings.Contains(r.index, ServicePlaceholder) {
		return r.index
	}
	if service == "" {
		return strings.ReplaceAll(r.index, ServicePlaceholder, "*")
	}
	return strings.ReplaceAll(r.index, ServicePlaceholder, service)
}

// DiscoverServices returns the services that actually wrote to the index inside
// the window.
//
// Discovery rather than configuration is what lets the tool schema stay the
// same size as the system grows. It also removes a failure mode configuration
// has: a service listed by an operator but never shipping logs reports zero
// errors forever, and an agent reads that as healthy rather than unobserved.
func (r *Repo) DiscoverServices(ctx context.Context, since time.Time, limit int) ([]domain.ServiceInfo, error) {
	if limit <= 0 {
		limit = maxServices
	}

	body := map[string]any{
		"size": 0,
		"query": map[string]any{"bool": map[string]any{"filter": []any{
			rangeSince(since),
		}}},
		"aggs": map[string]any{
			"by_service": map[string]any{
				"terms": map[string]any{"field": keyword(fieldService), "size": limit},
				"aggs": map[string]any{
					"last_seen": map[string]any{"max": map[string]any{"field": fieldTimestamp}},
				},
			},
		},
	}

	raw, err := r.post(ctx, "/"+r.indexFor("")+"/_search", body)
	if err != nil {
		return nil, err
	}

	var resp discoverResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode discovery response: %w", err)
	}

	out := make([]domain.ServiceInfo, 0, len(resp.Aggregations.ByService.Buckets))
	for _, b := range resp.Aggregations.ByService.Buckets {
		out = append(out, domain.ServiceInfo{
			Service:  b.Key,
			Records:  b.DocCount,
			LastSeen: b.LastSeen.AsString,
		})
	}
	return out, nil
}

// maxServices bounds the discovered registry. A cluster with more distinct
// service names than this is misconfigured, not large.
const maxServices = 200

// maxDescribedFields bounds how many fields describe_service reports. The point
// of the tool is to be cheaper than carrying every schema at all times, which a
// hundred-field answer would not be.
const maxDescribedFields = 40

// DescribeFields reports the filterable fields of one service with their types,
// cardinality and a few example values.
//
// Types come from the index mapping, which is authoritative and cheap. Values
// come from aggregations over the window, because a mapping says a field exists
// and never says whether it is worth grouping by.
func (r *Repo) DescribeFields(ctx context.Context, service string, since time.Time) ([]domain.FieldInfo, int, bool, error) {
	names, mapped, err := r.mappingFields(ctx, service)
	if err != nil {
		return nil, 0, false, err
	}

	truncated := len(names) > maxDescribedFields
	if truncated {
		names = names[:maxDescribedFields]
	}

	// Only aggregatable fields get an aggregation. Asking for a terms bucket on
	// an analyzed text field without a keyword subfield is rejected by the
	// cluster, and one such field would fail the whole request rather than lose
	// one row of the answer.
	aggs := make(map[string]any, len(names)*2)
	for _, name := range names {
		path := mapped[name].aggPath(name)
		if path == "" {
			continue
		}
		aggs["card_"+name] = map[string]any{"cardinality": map[string]any{"field": path}}
		aggs["top_"+name] = map[string]any{"terms": map[string]any{"field": path, "size": 3}}
	}

	body := map[string]any{
		"size": 0,
		"query": map[string]any{"bool": map[string]any{"filter": []any{
			rangeSince(since),
			term(keyword(fieldService), service),
		}}},
		"track_total_hits": trackTotalHits,
		"aggs":             aggs,
	}

	raw, err := r.post(ctx, "/"+r.indexFor(service)+"/_search", body)
	if err != nil {
		return nil, 0, false, err
	}

	var resp describeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, 0, false, fmt.Errorf("decode describe response: %w", err)
	}

	fields := make([]domain.FieldInfo, 0, len(names))
	for _, name := range names {
		info := domain.FieldInfo{Name: name, Type: mapped[name].Type, Kind: domain.FieldTail, ExactFilterable: mapped[name].aggPath(name) != ""}
		if mapped[name].aggPath(name) == "" {
			info.Note = "not filterable by exact value"
		}
		if IsCoreField(name) {
			info.Kind = domain.FieldCore
		}
		if card, ok := resp.Aggregations["card_"+name]; ok {
			info.Cardinality = card.Value
		}
		if top, ok := resp.Aggregations["top_"+name]; ok {
			for _, b := range top.Buckets {
				if example := bucketExample(b.Key, b.KeyAsString, mapped[name].Type); example != "" {
					info.Examples = append(info.Examples, example)
				}
			}
		}
		fields = append(fields, info)
	}
	return fields, resp.Hits.Total.Value, truncated, nil
}

// property is one field as the index mapping describes it.
type property struct {
	Type   string
	HasKey bool // has a .keyword subfield
}

// aggPath returns the field path an aggregation can use, or "" when the field
// cannot be aggregated at all.
func (p property) aggPath(name string) string {
	switch p.Type {
	case "keyword":
		return name
	case "text":
		if p.HasKey {
			return keyword(name)
		}
		// Analyzed and with no exact-match subfield: searchable as a phrase,
		// not groupable and not filterable by exact value.
		return ""
	case "byte", "short", "integer", "long", "unsigned_long", "half_float", "float", "double", "scaled_float", "boolean", "date", "date_nanos":
		// Numerics, booleans and dates aggregate on the field itself.
		return name
	default:
		return ""
	}
}

func bucketExample(raw json.RawMessage, asString, fieldType string) string {
	if asString != "" {
		return asString
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return ""
	}
	if fieldType == "boolean" {
		switch scalarString(value) {
		case "0":
			return "false"
		case "1":
			return "true"
		}
	}
	return scalarString(value)
}

// mappingFields reads the field names and types of the index from its mapping.
func (r *Repo) mappingFields(ctx context.Context, service string) ([]string, map[string]property, error) {
	raw, err := r.get(ctx, "/"+url.PathEscape(r.indexFor(service))+"/_mapping")
	if err != nil {
		return nil, nil, err
	}

	var resp map[string]struct {
		Mappings struct {
			Properties map[string]struct {
				Type   string `json:"type"`
				Fields map[string]struct {
					Type string `json:"type"`
				} `json:"fields"`
			} `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode mapping response: %w", err)
	}

	props := make(map[string]property)
	for _, index := range resp {
		for name, prop := range index.Mappings.Properties {
			// Underscore-prefixed fields are the shipper's bookkeeping, not the
			// application's, and nobody filters an investigation by them.
			if strings.HasPrefix(name, "_") || name == fieldTimestamp {
				continue
			}
			sub, hasKeyword := prop.Fields["keyword"]
			candidate := property{Type: prop.Type, HasKey: hasKeyword && sub.Type == "keyword"}
			if existing, seen := props[name]; seen && existing != candidate {
				props[name] = property{}
				continue
			}
			props[name] = candidate
		}
	}

	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	// Core fields first, then the rest alphabetically: the shared contract is
	// what an investigator reaches for before anything service-specific.
	sort.Slice(names, func(i, j int) bool {
		ci, cj := IsCoreField(names[i]), IsCoreField(names[j])
		if ci != cj {
			return ci
		}
		return names[i] < names[j]
	})
	return names, props, nil
}

type discoverResponse struct {
	Aggregations struct {
		ByService struct {
			Buckets []struct {
				Key      string `json:"key"`
				DocCount int    `json:"doc_count"`
				LastSeen struct {
					AsString string `json:"value_as_string"`
				} `json:"last_seen"`
			} `json:"buckets"`
		} `json:"by_service"`
	} `json:"aggregations"`
}

type describeResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations map[string]struct {
		// cardinality
		Value int `json:"value"`
		// terms
		Buckets []struct {
			Key         json.RawMessage `json:"key"`
			KeyAsString string          `json:"key_as_string"`
		} `json:"buckets"`
	} `json:"aggregations"`
}
