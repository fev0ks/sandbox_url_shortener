package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

// Tool names are a contract with the model and with the client configuration.
const (
	toolListServices   = "list_services"
	toolDescribeSvc    = "describe_service"
	toolSearchErrors   = "search_errors"
	toolGetRequestLogs = "get_request_logs"
	toolGetTrace       = "get_trace"
	toolSearchLogs     = "search_logs"
)

// definitions builds the tool declarations sent in tools/list.
//
// The vocabularies are closed with enum: the model cannot invent a service name
// or an unbounded time range, because the schema will not carry one.
func (s *Server) definitions() []toolDefinition {
	windows := windowEnum()
	domains := s.investigation.Domains()

	// service is a plain string, not an enum. Inlining the vocabulary made the
	// schema grow with the system, and the schema is re-sent on every turn; the
	// check moved to the server, which can also suggest the name the caller
	// probably meant. Domains stay an enum because they are few and stable.
	serviceProp := stringProp("Service name, exactly as returned by list_services.")

	listProps := map[string]any{
		"window": enumProp("Time window to count over.", windows),
	}
	if len(domains) > 0 {
		listProps["domain"] = enumProp("Narrow to one area before looking at services. Use when the unfiltered list is long.", domains)
	}

	tools := []toolDefinition{
		{
			Name: toolListServices,
			Description: "Lists known services with their error and warning counts for a time window. " +
				"Call this first, before any other tool: it shows where to look without reading a single log line. " +
				"Does not return log records.",
			InputSchema: object(listProps, []string{"window"}),
			Annotations: readOnlyTool(),
		},
		{
			Name: toolDescribeSvc,
			Description: "Returns the fields one service can be filtered by, with their types, example values " +
				"and how many distinct values each has. Call this before using the filters argument of search_logs, " +
				"and read the distinct_values: a field with one value per record identifies a record and is useless to group by.",
			InputSchema: object(map[string]any{
				"service": serviceProp,
				"window":  enumProp("Time window the examples and counts are taken from.", windows),
			}, []string{"service", "window"}),
			Annotations: readOnlyTool(),
		},
		{
			Name: toolSearchErrors,
			Description: "Returns failures of ONE service, grouped by operation, error kind and message pattern, " +
				"with an occurrence count and a sample request_id. Entry point when no request_id is known yet. " +
				"Covers a single service on purpose; sweeping the whole cluster is not supported. " +
				"Read the coverage field: on a busy window the counts are computed over a sample of the matches, not all of them.",
			InputSchema: object(map[string]any{
				"service": serviceProp,
				"window":  enumProp("Time window to search.", windows),
				"limit":   intProp(fmt.Sprintf("Maximum groups to return, 1..%d.", service.MaxErrorGroups), 1, service.MaxErrorGroups),
			}, []string{"service", "window"}),
			Annotations: readOnlyTool(),
		},
		{
			Name: toolGetRequestLogs,
			Description: "Returns the log records of one request chain, oldest first, and the trace_id if present. " +
				"Call this once a request_id is known, typically from search_errors. " +
				fmt.Sprintf("Only searches the last %s, so an older request returns nothing.", service.RequestChainLookback),
			InputSchema: object(map[string]any{
				"request_id": stringProp("Request identifier taken from an earlier result."),
				"limit":      intProp(fmt.Sprintf("Maximum records to return, 1..%d.", service.MaxRequestLogs), 1, service.MaxRequestLogs),
			}, []string{"request_id"}),
			Annotations: readOnlyTool(),
		},
		{
			Name: toolGetTrace,
			Description: "Returns the spans of one distributed trace with durations and the slowest span. " +
				"Call this when the log chain shows a slow or hanging step and a trace_id is available.",
			InputSchema: object(map[string]any{
				"trace_id": stringProp("Trace identifier, 8 to 32 hexadecimal characters, usually taken from get_request_logs."),
			}, []string{"trace_id"}),
			Annotations: readOnlyTool(),
		},
		{
			Name: toolSearchLogs,
			Description: "Filtered search over one service. Use only when the other tools were not enough. " +
				"Requires at least one narrowing filter beyond service and window. " +
				"Core fields are typed arguments; anything specific to one service goes in filters, and only the " +
				"filtered ones come back in the result.",
			InputSchema: object(map[string]any{
				"service":     serviceProp,
				"window":      enumProp("Time window to search.", windows),
				"operation":   stringProp("Exact operation name, for example redirect_resolve."),
				"error_kind":  stringProp("Exact error kind, for example not_found."),
				"status_code": intProp("Exact HTTP status code.", 100, 599),
				"text":        stringProp("Phrase to match inside the log message."),
				"filters":     mapProp("Service-specific equality filters, for example {\"short_code\": \"abc123\"}. Use tail fields marked exact_filterable by describe_service; unknown or unsupported fields are refused."),
				"limit":       intProp(fmt.Sprintf("Maximum records to return, 1..%d.", service.MaxSearchHits), 1, service.MaxSearchHits),
			}, []string{"service", "window"}),
			Annotations: readOnlyTool(),
		},
	}
	if s.databases != nil {
		tools = append(tools, databaseDefinitions()...)
	}
	return tools
}

// dispatch executes one tool call and returns its rendered result.
func (s *Server) dispatch(ctx context.Context, name string, args json.RawMessage) (any, error) {
	if name == toolListDatabases || name == toolDescribeDatabase || name == toolQueryDatabase {
		return s.dispatchDatabase(ctx, name, args)
	}
	switch name {
	case toolListServices:
		var a struct {
			Window domain.Window `json:"window"`
			Domain string        `json:"domain"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.ListServices(ctx, a.Window, a.Domain)

	case toolDescribeSvc:
		var a struct {
			Service string        `json:"service"`
			Window  domain.Window `json:"window"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.DescribeService(ctx, a.Service, a.Window)

	case toolSearchErrors:
		var a struct {
			Service string        `json:"service"`
			Window  domain.Window `json:"window"`
			Limit   int           `json:"limit"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.SearchErrors(ctx, a.Service, a.Window, a.Limit)

	case toolGetRequestLogs:
		var a struct {
			RequestID string `json:"request_id"`
			Limit     int    `json:"limit"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.GetRequestLogs(ctx, a.RequestID, a.Limit)

	case toolGetTrace:
		var a struct {
			TraceID string `json:"trace_id"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.GetTrace(ctx, a.TraceID)

	case toolSearchLogs:
		var a struct {
			Service    string            `json:"service"`
			Window     domain.Window     `json:"window"`
			Operation  string            `json:"operation"`
			ErrorKind  string            `json:"error_kind"`
			StatusCode int               `json:"status_code"`
			Text       string            `json:"text"`
			Filters    map[string]string `json:"filters"`
			Limit      int               `json:"limit"`
		}
		if err := decodeArgs(args, &a); err != nil {
			return nil, err
		}
		return s.investigation.SearchLogs(ctx, service.SearchQuery{
			Service:    a.Service,
			Window:     a.Window,
			Operation:  a.Operation,
			ErrorKind:  a.ErrorKind,
			StatusCode: a.StatusCode,
			Text:       a.Text,
			Fields:     a.Filters,
			Limit:      a.Limit,
		})

	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

// decodeArgs rejects arguments the schema does not declare.
//
// The schemas say additionalProperties: false, but that is a statement to the
// client, not a check. Accepting an undeclared field silently drops it, and the
// model then reasons about a filter that was never applied.
func decodeArgs(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func windowEnum() []string {
	windows := domain.Windows()
	out := make([]string, 0, len(windows))
	for _, w := range windows {
		out = append(out, string(w))
	}
	return out
}

func object(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func enumProp(description string, values []string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// mapProp declares an open string map: the schema states the value type without
// naming the keys, which is what keeps it from growing with the system.
func mapProp(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": map[string]any{"type": "string"},
	}
}

func intProp(description string, min, max int) map[string]any {
	return map[string]any{"type": "integer", "description": description, "minimum": min, "maximum": max}
}
