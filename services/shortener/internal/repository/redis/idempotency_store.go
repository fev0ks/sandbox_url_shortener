package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/services/shortener/internal/transport/http/stdlib/link"
)

// IdempotencyStore is a Redis-backed implementation of link.IdempotencyStore.
//
// The store is a thin KV wrapper: callers pass the fully-qualified Redis key
// (e.g., "idempotency:create:abc123"). The store adds no prefixing, so one
// type backs any number of independent actions.
type IdempotencyStore struct {
	client *redis.Client
}

// NewIdempotencyStore creates a Redis-backed idempotency store.
func NewIdempotencyStore(client *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client}
}

// Get returns the cached record or (nil, nil) on miss.
func (s *IdempotencyStore) Get(ctx context.Context, key string) (*link.IdempotencyRecord, error) {
	start := time.Now()
	raw, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencyGet, redisResultMiss, time.Since(start))
			return nil, nil
		}
		metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencyGet, redisResultError, time.Since(start))
		return nil, fmt.Errorf("idempotency get: %w", err)
	}
	var rec link.IdempotencyRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		// Treat corrupt entries as a miss; the caller will refresh.
		metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencyGet, redisResultCorrupt, time.Since(start))
		return nil, nil
	}
	metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencyGet, redisResultHit, time.Since(start))
	return &rec, nil
}

// Set stores the record with the given TTL.
func (s *IdempotencyStore) Set(ctx context.Context, key string, rec *link.IdempotencyRecord, ttl time.Duration) error {
	start := time.Now()
	payload, err := json.Marshal(rec)
	if err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencySet, redisResultMarshalError, time.Since(start))
		return fmt.Errorf("idempotency marshal: %w", err)
	}
	if err := s.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencySet, redisResultError, time.Since(start))
		return fmt.Errorf("idempotency set: %w", err)
	}
	metrics.ObserveRedisOperation(shortenerService, redisOpIdempotencySet, redisResultSuccess, time.Since(start))
	return nil
}
