package link

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// IdempotencyKeyPrefix namespaces Idempotency-Key entries for the create
// action in the shared idempotency store.
const IdempotencyKeyPrefix = "idempotency:create:"

// IdempotencyStoreKey returns the fully-qualified store key for a given
// client-supplied Idempotency-Key header value.
func IdempotencyStoreKey(key string) string {
	return IdempotencyKeyPrefix + key
}

// IdempotencyStore caches the response for an Idempotency-Key + body-hash pair
// so safe retries return the same answer instead of creating duplicate links.
//
// The interface is defined in the link package because the cached value is
// HTTP-shaped (status + body). Implementations live in repository/redis or
// in tests as mocks.
type IdempotencyStore interface {
	// Get returns the stored record, or (nil, nil) on miss.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)
	// Set stores the record under the given TTL. Errors are surfaced so the
	// caller can decide whether to fail open.
	Set(ctx context.Context, key string, rec *IdempotencyRecord, ttl time.Duration) error
}

// IdempotencyRecord is the cached response for a single key.
type IdempotencyRecord struct {
	BodyHash     string          `json:"body_hash"`
	StatusCode   int             `json:"status_code"`
	ResponseBody json.RawMessage `json:"response_body"`
}

// idempotencyTTL is the retry window. 24h matches Stripe's default and is
// long enough for any reasonable client retry strategy.
const idempotencyTTL = 24 * time.Hour

// maxIdempotencyKeyLen bounds the header value so a malicious client cannot
// blow up Redis with multi-megabyte keys.
const maxIdempotencyKeyLen = 255

// bufferingWriter captures the downstream handler's response so we can both
// replay it from the cache later and forward it to the real client now.
//
// We do not implement http.Hijacker / http.Flusher / http.Pusher because the
// create handler does not need them; if a future handler does, switch to
// httptest.ResponseRecorder or a richer wrapper.
type bufferingWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferingWriter() *bufferingWriter {
	return &bufferingWriter{header: http.Header{}}
}

func (b *bufferingWriter) Header() http.Header { return b.header }

func (b *bufferingWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferingWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

// flushTo writes the captured response to the real ResponseWriter.
func (b *bufferingWriter) flushTo(w http.ResponseWriter) {
	for k, v := range b.header {
		w.Header()[k] = v
	}
	if b.status == 0 {
		b.status = http.StatusOK
	}
	w.WriteHeader(b.status)
	if _, err := w.Write(b.body.Bytes()); err != nil {
		return
	}
}

// hashBody returns the hex sha256 of body. Empty body hashes to a stable value.
func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// readAllAndRestore drains r.Body into a byte slice and replaces r.Body with
// a fresh reader so downstream handlers can still decode it normally.
func readAllAndRestore(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if err := r.Body.Close(); err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
