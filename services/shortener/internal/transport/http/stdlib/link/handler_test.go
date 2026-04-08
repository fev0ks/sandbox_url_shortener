package link

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/platform/httppolicy"
	"sandbox_url_shortener/services/shortener/internal/domain"
)

// stubService is a minimal Service stub for handler tests.
type stubService struct {
	createFn func(ctx context.Context, in domain.CreateLinkInput) (*domain.Link, error)
}

func (s *stubService) Create(ctx context.Context, in domain.CreateLinkInput) (*domain.Link, error) {
	return s.createFn(ctx, in)
}
func (s *stubService) GetByID(_ context.Context, _ uuid.UUID) (*domain.Link, error) { return nil, nil }
func (s *stubService) Disable(_ context.Context, _ uuid.UUID) (*domain.Link, error) { return nil, nil }
func (s *stubService) Enable(_ context.Context, _ uuid.UUID) (*domain.Link, error)  { return nil, nil }
func (s *stubService) SoftDelete(_ context.Context, _ uuid.UUID) error              { return nil }
func (s *stubService) Resolve(_ context.Context, _ string) (*domain.Link, error)    { return nil, nil }

// stubStore is a minimal in-memory IdempotencyStore.
type stubStore struct {
	mu     sync.Mutex
	data   map[string]*IdempotencyRecord
	getErr error
	setErr error
}

func newStubStore() *stubStore { return &stubStore{data: map[string]*IdempotencyRecord{}} }

func (s *stubStore) Get(_ context.Context, key string) (*IdempotencyRecord, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data[key]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

func (s *stubStore) Set(_ context.Context, key string, rec *IdempotencyRecord, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = rec
	return nil
}

func newCreateRequest(t *testing.T, body string, key string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	return req
}

func quietHandlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newHandler(svc Service, store IdempotencyStore) *Handler {
	return NewHandler(svc, quietHandlerLogger(), "http://x", store, nil)
}

type countingLimiter struct {
	calls   int32
	allowed bool
}

func (l *countingLimiter) Allow(_ context.Context, _ string) (httppolicy.Decision, error) {
	atomic.AddInt32(&l.calls, 1)
	return httppolicy.Decision{Allowed: l.allowed}, nil
}

func successSvc(callCount *int) *stubService {
	return &stubService{
		createFn: func(_ context.Context, _ domain.CreateLinkInput) (*domain.Link, error) {
			*callCount++
			return &domain.Link{ID: uuid.New(), ShortCode: "abc1234", OriginalURL: "https://example.com"}, nil
		},
	}
}

func TestCreate_NoIdempotencyHeader(t *testing.T) {
	calls := 0
	h := newHandler(successSvc(&calls), newStubStore())
	rec := httptest.NewRecorder()
	h.Create(rec, newCreateRequest(t, `{"original_url":"https://example.com"}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d", rec.Code)
	}
	if calls != 1 {
		t.Errorf("svc calls = %d", calls)
	}
}

func TestCreate_FreshIdempotencyKey_StoresResponse(t *testing.T) {
	calls := 0
	store := newStubStore()
	h := newHandler(successSvc(&calls), store)

	rec := httptest.NewRecorder()
	h.Create(rec, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-1"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d", rec.Code)
	}
	if _, ok := store.data[IdempotencyStoreKey("key-1")]; !ok {
		t.Error("expected store to contain key-1")
	}
}

func TestCreate_ReplayOnSecondRequest(t *testing.T) {
	calls := 0
	store := newStubStore()
	h := newHandler(successSvc(&calls), store)

	rec1 := httptest.NewRecorder()
	h.Create(rec1, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-2"))

	rec2 := httptest.NewRecorder()
	h.Create(rec2, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-2"))

	if calls != 1 {
		t.Errorf("svc invoked %d times, want 1 (second should replay)", calls)
	}
	if rec2.Code != http.StatusCreated {
		t.Errorf("replay code = %d", rec2.Code)
	}
	if !bytes.Equal(rec1.Body.Bytes(), rec2.Body.Bytes()) {
		t.Errorf("replay body mismatch:\n  first = %s\n  second = %s", rec1.Body, rec2.Body)
	}
}

func TestCreate_ReplayBypassesRateLimiter(t *testing.T) {
	calls := 0
	store := newStubStore()
	limiter := &countingLimiter{allowed: true}
	h := NewHandler(successSvc(&calls), quietHandlerLogger(), "http://x", store, limiter)

	rec1 := httptest.NewRecorder()
	h.Create(rec1, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-limited"))

	rec2 := httptest.NewRecorder()
	h.Create(rec2, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-limited"))

	if calls != 1 {
		t.Fatalf("svc invoked %d times, want 1", calls)
	}
	if got := atomic.LoadInt32(&limiter.calls); got != 1 {
		t.Fatalf("limiter invoked %d times, want 1 (fresh request only)", got)
	}
	if rec2.Code != http.StatusCreated {
		t.Fatalf("replay code = %d, want 201", rec2.Code)
	}
}

func TestCreate_RateLimitExceeded(t *testing.T) {
	calls := 0
	limiter := &countingLimiter{allowed: false}
	h := NewHandler(successSvc(&calls), quietHandlerLogger(), "http://x", newStubStore(), limiter)

	rec := httptest.NewRecorder()
	h.Create(rec, newCreateRequest(t, `{"original_url":"https://example.com"}`, ""))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	if calls != 0 {
		t.Errorf("svc invoked %d times, want 0 when rate limited", calls)
	}
}

func TestCreate_ConflictOnSameKeyDifferentBody(t *testing.T) {
	calls := 0
	store := newStubStore()
	h := newHandler(successSvc(&calls), store)

	rec1 := httptest.NewRecorder()
	h.Create(rec1, newCreateRequest(t, `{"original_url":"https://a.com"}`, "key-3"))

	rec2 := httptest.NewRecorder()
	h.Create(rec2, newCreateRequest(t, `{"original_url":"https://b.com"}`, "key-3"))

	if rec2.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", rec2.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec2.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestCreate_FailsOpenOnStoreError(t *testing.T) {
	calls := 0
	store := newStubStore()
	store.getErr = errors.New("redis down")
	h := newHandler(successSvc(&calls), store)

	rec := httptest.NewRecorder()
	h.Create(rec, newCreateRequest(t, `{"original_url":"https://example.com"}`, "key-4"))

	if rec.Code != http.StatusCreated {
		t.Errorf("expected fail-open 201, got %d", rec.Code)
	}
	if calls != 1 {
		t.Errorf("svc calls = %d", calls)
	}
}

func TestCreate_KeyTooLong(t *testing.T) {
	h := newHandler(successSvc(new(int)), newStubStore())
	long := bytes.Repeat([]byte("a"), maxIdempotencyKeyLen+1)

	rec := httptest.NewRecorder()
	h.Create(rec, newCreateRequest(t, `{"original_url":"https://example.com"}`, string(long)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d", rec.Code)
	}
}
