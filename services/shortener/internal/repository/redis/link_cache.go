// Package redis implements domain repository interfaces backed by Redis.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/services/shortener/internal/domain"
)

// LinkCache is a Redis-backed implementation of domain.LinkCache.
type LinkCache struct {
	client *redis.Client
}

// NewLinkCache creates a Redis-backed link cache.
func NewLinkCache(client *redis.Client) *LinkCache {
	return &LinkCache{client: client}
}

// keyFor returns the canonical Redis key for a short code.
func keyFor(shortCode string) string {
	return "link:code:" + shortCode
}

// Get fetches the cached link. Returns (nil, nil) on miss.
func (c *LinkCache) Get(ctx context.Context, shortCode string) (*domain.CachedLink, error) {
	start := time.Now()
	raw, err := c.client.Get(ctx, keyFor(shortCode)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheGet, redisResultMiss, time.Since(start))
			return nil, nil
		}
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheGet, redisResultError, time.Since(start))
		return nil, fmt.Errorf("link cache get: %w", err)
	}

	var cached domain.CachedLink
	if err := json.Unmarshal(raw, &cached); err != nil {
		// Treat corrupt entries as a miss; the caller will refresh from PG.
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheGet, redisResultCorrupt, time.Since(start))
		return nil, nil
	}
	metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheGet, redisResultHit, time.Since(start))
	return &cached, nil
}

// Set stores the cached link with the given TTL. Non-positive TTL is a no-op.
func (c *LinkCache) Set(ctx context.Context, shortCode string, link *domain.CachedLink, ttl time.Duration) error {
	if ttl <= 0 {
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheSet, redisResultSkipped, 0)
		return nil
	}
	start := time.Now()
	payload, err := json.Marshal(link)
	if err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheSet, redisResultMarshalError, time.Since(start))
		return fmt.Errorf("link cache marshal: %w", err)
	}
	if err := c.client.Set(ctx, keyFor(shortCode), payload, ttl).Err(); err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheSet, redisResultError, time.Since(start))
		return fmt.Errorf("link cache set: %w", err)
	}
	metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheSet, redisResultSuccess, time.Since(start))
	return nil
}

// Delete removes the cached entry. Missing keys are not an error.
func (c *LinkCache) Delete(ctx context.Context, shortCode string) error {
	start := time.Now()
	if err := c.client.Del(ctx, keyFor(shortCode)).Err(); err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheDelete, redisResultError, time.Since(start))
		return fmt.Errorf("link cache delete: %w", err)
	}
	metrics.ObserveRedisOperation(shortenerService, redisOpLinkCacheDelete, redisResultSuccess, time.Since(start))
	return nil
}
