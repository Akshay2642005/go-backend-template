package cache

import (
	"context"
	"time"
)

// Get retrieves a value from the cache by key into dest.
// Returns ErrCacheMiss if the key does not exist.
func Get[T any](c *Cache, ctx context.Context, key string, dest *T) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		c.metrics.RecordMiss()
		return ErrCacheMiss
	}

	c.metrics.RecordHit()

	var zero T
	switch any(zero).(type) {
	case []byte:
		if v, ok := any(dest).(*[]byte); ok {
			*v = data
			return nil
		}
	case string:
		if v, ok := any(dest).(*string); ok {
			*v = string(data)
			return nil
		}
	}

	return UnmarshalValue(data, dest)
}

// Set stores a value in the cache with the given TTL.
func Set[T any](c *Cache, ctx context.Context, key string, value T, ttl time.Duration) error {
	data, err := MarshalValue(value)
	if err != nil {
		c.metrics.RecordError()
		return err
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		c.metrics.RecordError()
		return err
	}

	return nil
}

// SetNX sets a value only if the key does not already exist.
// Returns true if the value was set.
func SetNX(c *Cache, ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := MarshalValue(value)
	if err != nil {
		c.metrics.RecordError()
		return false, err
	}

	ok, err := c.client.SetNX(ctx, key, data, ttl).Result()
	if err != nil {
		c.metrics.RecordError()
		return false, err
	}

	return ok, nil
}

// Delete removes one or more keys from the cache.
func Delete(c *Cache, ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.metrics.RecordError()
		return err
	}

	return nil
}

// DeleteByPrefix removes all keys matching the given prefix using SCAN.
// This is safe for production use as it iterates incrementally.
func DeleteByPrefix(c *Cache, ctx context.Context, prefix string) error {
	var cursor uint64
	pattern := prefix + ":*"

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			c.metrics.RecordError()
			return err
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				c.metrics.RecordError()
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// Increment atomically increments a key by delta. Returns the new value.
func Increment(c *Cache, ctx context.Context, key string, delta int64) (int64, error) {
	val, err := c.client.IncrBy(ctx, key, delta).Result()
	if err != nil {
		c.metrics.RecordError()
		return 0, err
	}

	return val, nil
}

// Expire sets a TTL on an existing key.
func Expire(c *Cache, ctx context.Context, key string, ttl time.Duration) error {
	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		c.metrics.RecordError()
		return err
	}

	return nil
}
