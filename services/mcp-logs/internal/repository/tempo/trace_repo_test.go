package tempo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const oneTrace = `{"batches":[{
	"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"shortener"}}]},
	"scopeSpans":[{"spans":[
		{"spanId":"b","parentSpanId":"a","name":"db.query","startTimeUnixNano":"1500000000000000000","endTimeUnixNano":"1500000000250000000","status":{"code":"STATUS_CODE_OK"}},
		{"spanId":"a","name":"http.request","startTimeUnixNano":"1500000000000000000","endTimeUnixNano":"1500000000900000000","status":{"code":"STATUS_CODE_ERROR"}}
	]}]
}]}`

func newStubTempo(t *testing.T, handler http.HandlerFunc) (*Repo, *string) {
	t.Helper()

	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return NewRepo(srv.Client(), srv.URL, Auth{}), &path
}

func TestGetTraceProjectsSpansOrderedByStart(t *testing.T) {
	repo, _ := newStubTempo(t, func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(oneTrace)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	spans, err := repo.GetTrace(context.Background(), "4f662fd9f6835424")
	if err != nil {
		t.Fatalf("GetTrace() failed: %v", err)
	}

	if len(spans) != 2 {
		t.Fatalf("want 2 spans, got %d", len(spans))
	}
	if spans[0].Service != "shortener" {
		t.Fatalf("the resource attribute must reach the span, got %q", spans[0].Service)
	}
	if spans[0].DurationMS != 250 && spans[1].DurationMS != 250 {
		t.Fatalf("durations must be derived from the unix nano bounds, got %v and %v",
			spans[0].DurationMS, spans[1].DurationMS)
	}
}

func TestGetTraceEscapesTheIdentifier(t *testing.T) {
	repo, path := newStubTempo(t, func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(oneTrace)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	// The caller rejects this shape first; escaping here is the second half of
	// the guard, so a bypass upstream cannot choose a different endpoint.
	if _, err := repo.GetTrace(context.Background(), "../../api/search?q=x"); err != nil {
		t.Fatalf("GetTrace() failed: %v", err)
	}

	if got := *path; got != "/api/traces/..%2F..%2Fapi%2Fsearch%3Fq=x" {
		t.Fatalf("identifier must not steer the request, got %q", got)
	}
}

func TestGetTraceReportsNotFound(t *testing.T) {
	repo, _ := newStubTempo(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := repo.GetTrace(context.Background(), "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetTraceTreatsAnEmptyPayloadAsNotFound(t *testing.T) {
	repo, _ := newStubTempo(t, func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"batches":[]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	if _, err := repo.GetTrace(context.Background(), "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty trace is not a successful lookup, got %v", err)
	}
}

func TestUnixNanoReturnsZeroOnGarbage(t *testing.T) {
	// Tempo sends these as strings; a parse failure must not become a 1970
	// timestamp that then produces a nonsense duration.
	if got := unixNano("not a number"); !got.IsZero() {
		t.Fatalf("want the zero time, got %v", got)
	}
}
