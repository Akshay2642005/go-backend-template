package cache

import (
	"context"
	"time"
)

// GetOrSet implements the cache-aside pattern with stampede protection.
// On cache miss, fn is called to load the value from the origin (DB, API, etc.)
// and the result is stored in cache. Concurrent requests for the same key
// share a single origin call via singleflight.
// If Redis is unavailable, the function falls through to the origin (fail-open).
func GetOrSet[T any](c *Cache, ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	// Try cache first
	var dest T
	if err := Get(c, ctx, key, &dest); err == nil {
		return dest, nil
	}

	// Cache miss or Redis error — call origin via singleflight
	val, err := c.sf.Do(key, func() (interface{}, error) {
		result, fnErr := fn()
		if fnErr != nil {
			return nil, fnErr
		}

		// Best-effort cache write (fail-open: don't fail the request)
		storeCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		effectiveTTL := TTLWithJitter(ttl, 0.1)

		if setErr := Set(c, storeCtx, key, result, effectiveTTL); setErr != nil {
			c.logger.Warn().
				Err(setErr).
				Str("key", key).
				Dur("ttl", effectiveTTL).
				Msg("failed to store value in cache")
		} else {
			c.metrics.RecordSet()
		}

		return result, nil
	})

	if err != nil {
		return *new(T), err
	}

	return val.(T), nil
}

// GetOrSetWithMetrics works like GetOrSet but also logs slow origin calls.
func GetOrSetWithMetrics[T any](c *Cache, ctx context.Context, key string, ttl time.Duration, originName string, fn func() (T, error)) (T, error) {
	start := time.Now()

	val, err := GetOrSet(c, ctx, key, ttl, fn)

	duration := time.Since(start)
	c.metrics.RecordSlowOp(duration, c.slowQueryThreshold)

	if duration >= c.slowQueryThreshold {
		c.logger.Warn().
			Str("operation", originName).
			Str("key", key).
			Dur("duration", duration).
			Msg("slow cache-aside operation")
	}

	return val, err
}
