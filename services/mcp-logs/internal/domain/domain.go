// Package domain holds the incident-investigation types exposed to the agent.
//
// Every type here is a deliberately narrow projection of a much wider upstream
// document. Elasticsearch and Tempo return dozens of fields per record; only
// the ones an investigator actually reads are carried across this boundary,
// because everything returned lands in the model's context window.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrTooBroad is returned when a request would scan more than the server is
// willing to expose. It is a guard against the agent asking for "everything".
var ErrTooBroad = errors.New("query too broad")

// ErrUnknownValue is returned when an argument falls outside a closed vocabulary.
var ErrUnknownValue = errors.New("unknown value")

// ErrMalformedValue is returned when an identifier does not have the shape the
// upstream system uses. Identifiers reach this server through log content and
// are echoed into outbound requests, so their shape is checked rather than
// trusted.
var ErrMalformedValue = errors.New("malformed value")

// Window is the closed vocabulary of supported time ranges. A free-form range
// invites unbounded scans; an enum cannot be widened by the model.
type Window string

// Supported time windows.
const (
	Window15m Window = "15m"
	Window1h  Window = "1h"
	Window6h  Window = "6h"
	Window24h Window = "24h"
)

// Windows lists every supported window in ascending order.
func Windows() []Window {
	return []Window{Window15m, Window1h, Window6h, Window24h}
}

// Duration converts the window to a duration.
func (w Window) Duration() (time.Duration, error) {
	switch w {
	case Window15m:
		return 15 * time.Minute, nil
	case Window1h:
		return time.Hour, nil
	case Window6h:
		return 6 * time.Hour, nil
	case Window24h:
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("%w: window %q, expected one of 15m, 1h, 6h, 24h", ErrUnknownValue, w)
	}
}

// LogEntry is a single projected log record.
//
// The typed fields are the core contract every service emits. Anything specific
// to one service - short_code here, order_id or campaign_id in a larger system -
// lives in Fields instead of growing this struct: a field that belongs to one
// service and is empty for all the others still costs a line in every schema and
// every result once there are dozens of them.
type LogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Service    string    `json:"service"`
	Level      string    `json:"level,omitempty"`
	Operation  string    `json:"operation,omitempty"`
	ErrorKind  string    `json:"error_kind,omitempty"`
	Message    string    `json:"message,omitempty"`
	Error      string    `json:"error,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	TraceID    string    `json:"trace_id,omitempty"`
	StatusCode int       `json:"status_code,omitempty"`
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`

	// Fields carries the service-specific values that were filtered on. Only
	// those: returning every extra field would undo on the response side the
	// context saved on the schema side.
	Fields map[string]string `json:"fields,omitempty"`
}

// ErrorGroup collapses repeated occurrences of the same failure.
//
// A crash loop produces hundreds of identical records; returning them verbatim
// buries the rest of the investigation. Grouping keeps the signal and replaces
// the repetition with a count.
type ErrorGroup struct {
	Service         string    `json:"service"`
	Operation       string    `json:"operation,omitempty"`
	ErrorKind       string    `json:"error_kind,omitempty"`
	MessagePattern  string    `json:"message_pattern"`
	Count           int       `json:"count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	SampleRequestID string    `json:"sample_request_id,omitempty"`
	SampleTraceID   string    `json:"sample_trace_id,omitempty"`
	SampleError     string    `json:"sample_error,omitempty"`
}

// ServiceSummary reports how noisy one service is inside the requested window.
// It answers "where should I look" without reading a single log line.
type ServiceSummary struct {
	Service    string `json:"service"`
	ErrorCount int    `json:"error_count"`
	WarnCount  int    `json:"warn_count"`
}

// Span is a projected Tempo span: enough to see where time went, nothing more.
type Span struct {
	SpanID     string    `json:"span_id"`
	ParentID   string    `json:"parent_span_id,omitempty"`
	Service    string    `json:"service,omitempty"`
	Name       string    `json:"name"`
	StartTime  time.Time `json:"start_time"`
	DurationMS float64   `json:"duration_ms"`
	StatusCode string    `json:"status_code,omitempty"`
}

// Truncation describes how much of a result set was withheld.
//
// Silently trimming a result set is a correctness bug: the agent concludes it
// has seen everything and reasons from a partial picture. Every capped result
// carries this so the omission is visible.
//
// Returned and Total always count the same kind of thing as the field they
// describe: records for a record list, groups for a group list. Mixing the two
// reads as "20 of 900 groups" when it means "20 groups built from 900 records".
type Truncation struct {
	Returned int  `json:"returned"`
	Total    int  `json:"total"`
	Capped   bool `json:"capped"`

	// TotalAtLeast reports that Total is a lower bound rather than an exact
	// count, because counting every match exactly is not worth the cluster time.
	TotalAtLeast bool `json:"total_is_lower_bound,omitempty"`
}

// Coverage describes the sample an in-process aggregation was computed from.
//
// Grouping happens here, not in the index, so it can only see the records that
// were actually fetched. When the window holds more matches than the scan
// limit, every count below is a count over a sample, and saying so is the
// difference between a useful number and a confidently wrong one.
type Coverage struct {
	Scanned int  `json:"scanned_records"`
	Matched int  `json:"matched_records"`
	Partial bool `json:"partial"`

	// MatchedAtLeast reports that Matched is a lower bound.
	MatchedAtLeast bool `json:"matched_records_is_lower_bound,omitempty"`

	// Note spells the consequence out for the model, which reads prose more
	// reliably than it reads a boolean.
	Note string `json:"note,omitempty"`
}

// ServiceInfo is one entry of the discovered service registry.
type ServiceInfo struct {
	Service  string `json:"service"`
	Domain   string `json:"domain,omitempty"`
	Records  int    `json:"records"`
	LastSeen string `json:"last_seen,omitempty"`
}

// FieldKind is the coarse category a field falls into for an investigator.
type FieldKind string

// Field categories.
const (
	// FieldCore is part of the shared log contract and is always projected
	// into a result when present. Some core fields, such as error, are output-only.
	FieldCore FieldKind = "core"
	// FieldTail belongs to one service or a few. Tail fields are filtered
	// through a generic map and returned only when they were filtered on.
	FieldTail FieldKind = "tail"
)

// FieldInfo describes one filterable field of one service.
//
// Cardinality is the reason this type exists. Without it an agent will try to
// group by request_id, which is unique per request: the call succeeds, returns
// thousands of buckets of one, and teaches the agent nothing.
type FieldInfo struct {
	Name string    `json:"name"`
	Kind FieldKind `json:"kind"`
	Type string    `json:"type"`
	// ExactFilterable is derived from the upstream mapping, not inferred from
	// whether the field happened to have sample values in the current window.
	ExactFilterable bool     `json:"exact_filterable"`
	Cardinality     int      `json:"distinct_values"`
	Examples        []string `json:"examples,omitempty"`
	Note            string   `json:"note,omitempty"`
}

// ServiceDescription is what describe_service returns: the shape of one
// service, asked for when it is needed rather than carried at all times.
type ServiceDescription struct {
	Service   string      `json:"service"`
	Window    Window      `json:"window"`
	Records   int         `json:"records"`
	Fields    []FieldInfo `json:"fields"`
	NextStep  string      `json:"next_step"`
	Truncated bool        `json:"fields_truncated,omitempty"`
}
