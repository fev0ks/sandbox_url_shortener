// Package tempo reads projected trace spans from the Tempo query API.
package tempo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// serviceNameAttr is the OpenTelemetry resource attribute carrying the service name.
const serviceNameAttr = "service.name"

// Auth carries an optional bearer token for the Tempo query API. The zero
// value means an unauthenticated endpoint, which is what the local stack runs.
type Auth struct {
	Token string
}

// Repo queries Tempo for a single trace.
type Repo struct {
	client  *http.Client
	baseURL string
	auth    Auth
}

// NewRepo creates a trace repository.
func NewRepo(client *http.Client, baseURL string, auth Auth) *Repo {
	return &Repo{client: client, baseURL: baseURL, auth: auth}
}

// ErrNotFound is returned when Tempo has no trace with the given identifier.
var ErrNotFound = fmt.Errorf("trace not found")

// GetTrace returns the spans of one trace ordered by start time.
//
// Only timing and identity are projected: a raw Tempo trace carries every span
// attribute, which is far more than an investigator needs to see where time went.
func (r *Repo) GetTrace(ctx context.Context, traceID string) ([]domain.Span, error) {
	// The identifier reaches this server through log content, so it is escaped
	// rather than concatenated: an unescaped "?" or "../" would let the caller
	// choose which Tempo endpoint is actually queried. The caller validates the
	// shape as well; this is the second half of that guard.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/traces/"+url.PathEscape(traceID), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if r.auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.auth.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call tempo: %w", err)
	}
	// Closing the response releases resources independently of trace projection.
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read tempo response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tempo returned %d", resp.StatusCode)
	}

	var payload traceResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode tempo response: %w", err)
	}

	spans := projectSpans(payload)
	if len(spans) == 0 {
		return nil, ErrNotFound
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].StartTime.Before(spans[j].StartTime) })
	return spans, nil
}

// traceResponse mirrors the OTLP-shaped payload Tempo returns for a trace.
type traceResponse struct {
	Batches []struct {
		Resource struct {
			Attributes []attribute `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				SpanID            string      `json:"spanId"`
				ParentSpanID      string      `json:"parentSpanId"`
				Name              string      `json:"name"`
				StartTimeUnixNano string      `json:"startTimeUnixNano"`
				EndTimeUnixNano   string      `json:"endTimeUnixNano"`
				Status            spanStatus  `json:"status"`
				Attributes        []attribute `json:"attributes"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"batches"`
}

type spanStatus struct {
	Code string `json:"code"`
}

type attribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
	} `json:"value"`
}

func projectSpans(payload traceResponse) []domain.Span {
	var spans []domain.Span
	for _, batch := range payload.Batches {
		service := attrValue(batch.Resource.Attributes, serviceNameAttr)
		for _, scope := range batch.ScopeSpans {
			for _, s := range scope.Spans {
				start := unixNano(s.StartTimeUnixNano)
				end := unixNano(s.EndTimeUnixNano)
				duration := end.Sub(start)
				spans = append(spans, domain.Span{
					SpanID:     s.SpanID,
					ParentID:   s.ParentSpanID,
					Service:    service,
					Name:       s.Name,
					StartTime:  start,
					DurationMS: float64(duration.Microseconds()) / 1000,
					StatusCode: s.Status.Code,
				})
			}
		}
	}
	return spans
}

func attrValue(attrs []attribute, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.StringValue
		}
	}
	return ""
}

func unixNano(raw string) time.Time {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}
