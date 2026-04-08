// Package redis implements domain repository interfaces backed by Redis.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

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
	raw, err := c.client.Get(ctx, keyFor(shortCode)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("link cache get: %w", err)
	}

	var cached domain.CachedLink
	if err := json.Unmarshal(raw, &cached); err != nil {
		// Treat corrupt entries as a miss; the caller will refresh from PG.
		return nil, nil
	}
	return &cached, nil
}

// Set stores the cached link with the given TTL. Non-positive TTL is a no-op.
func (c *LinkCache) Set(ctx context.Context, shortCode string, link *domain.CachedLink, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	payload, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("link cache marshal: %w", err)
	}
	if err := c.client.Set(ctx, keyFor(shortCode), payload, ttl).Err(); err != nil {
		return fmt.Errorf("link cache set: %w", err)
	}
	return nil
}

// Delete removes the cached entry. Missing keys are not an error.
func (c *LinkCache) Delete(ctx context.Context, shortCode string) error {
	if err := c.client.Del(ctx, keyFor(shortCode)).Err(); err != nil {
		return fmt.Errorf("link cache delete: %w", err)
	}
	return nil
}
