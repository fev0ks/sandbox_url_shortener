package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"sandbox_url_shortener/internal/platform/httppolicy"
	"sandbox_url_shortener/internal/platform/metrics"
)

// FixedWindowLimiter is the current production choice for create-rate limiting.
//
// Pros:
// - cheap and easy to inspect in Redis
// - one key per client/window
// - good enough for MVP abuse control
//
// Cons:
// - bursty at window boundaries
// - INCR + EXPIRE is simple but not as strict as a Lua-scripted limiter
// - less fair than sliding-window or token-bucket approaches
type FixedWindowLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
	now    func() time.Time
}

// NewFixedWindowLimiter creates a Redis-backed fixed-window limiter.
func NewFixedWindowLimiter(client *redis.Client, limit int, window time.Duration) *FixedWindowLimiter {
	if window <= 0 {
		window = time.Minute
	}
	return &FixedWindowLimiter{
		client: client,
		limit:  int64(limit),
		window: window,
		now:    time.Now,
	}
}

// Allow records one hit for key and returns the resulting decision.
func (l *FixedWindowLimiter) Allow(ctx context.Context, key string) (httppolicy.Decision, error) {
	start := time.Now()
	now := l.now()
	windowNanos := l.window.Nanoseconds()
	bucket := now.UnixNano() / windowNanos
	redisKey := fmt.Sprintf("%s:%d", key, bucket)

	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		metrics.ObserveRedisOperation(shortenerService, redisOpRateLimitAllow, redisErrorResult(err), time.Since(start))
		return httppolicy.Decision{}, fmt.Errorf("ratelimit incr: %w", err)
	}
	if count == 1 {
		if err := l.client.Expire(ctx, redisKey, l.window).Err(); err != nil {
			metrics.ObserveRedisOperation(shortenerService, redisOpRateLimitAllow, redisErrorResult(err), time.Since(start))
			return httppolicy.Decision{}, fmt.Errorf("ratelimit expire: %w", err)
		}
	}

	if count > l.limit {
		retryAfter := l.window - time.Duration(now.UnixNano()%windowNanos)
		if retryAfter <= 0 {
			retryAfter = l.window
		}
		metrics.ObserveRedisOperation(shortenerService, redisOpRateLimitAllow, redisResultLimited, time.Since(start))
		return httppolicy.Decision{Allowed: false, RetryAfter: retryAfter}, nil
	}

	metrics.ObserveRedisOperation(shortenerService, redisOpRateLimitAllow, redisResultAllowed, time.Since(start))
	return httppolicy.Decision{Allowed: true}, nil
}

func redisErrorResult(err error) string {
	if errors.Is(err, redis.Nil) {
		return redisResultMiss
	}
	return redisResultError
}
