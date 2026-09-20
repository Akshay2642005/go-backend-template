package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSet(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	t.Run("set and get string", func(t *testing.T) {
		err := Set(c, ctx, "key1", "hello", time.Minute)
		require.NoError(t, err)

		var result string
		err = Get(c, ctx, "key1", &result)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("set and get int", func(t *testing.T) {
		err := Set(c, ctx, "key2", 42, time.Minute)
		require.NoError(t, err)

		var result int
		err = Get(c, ctx, "key2", &result)
		require.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("set and get struct", func(t *testing.T) {
		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		u := User{Name: "Alice", Age: 30}
		err := Set(c, ctx, "key3", u, time.Minute)
		require.NoError(t, err)

		var result User
		err = Get(c, ctx, "key3", &result)
		require.NoError(t, err)
		assert.Equal(t, u, result)
	})

	t.Run("cache miss returns error", func(t *testing.T) {
		var result string
		err := Get(c, ctx, "nonexistent", &result)
		assert.ErrorIs(t, err, ErrCacheMiss)
		assert.True(t, IsCacheMiss(err))
	})

	t.Run("ttl expiry", func(t *testing.T) {
		err := Set(c, ctx, "expiring", "value", 100*time.Millisecond)
		require.NoError(t, err)

		// Should exist immediately
		var result string
		err = Get(c, ctx, "expiring", &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result)

		// Wait for expiry
		time.Sleep(150 * time.Millisecond)
		err = Get(c, ctx, "expiring", &result)
		assert.ErrorIs(t, err, ErrCacheMiss)
	})
}

func TestSetNX(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	ok, err := SetNX(c, ctx, "nx_key", "first", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = SetNX(c, ctx, "nx_key", "second", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)

	var result string
	err = Get(c, ctx, "nx_key", &result)
	require.NoError(t, err)
	assert.Equal(t, "first", result)
}

func TestDelete(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	_ = Set(c, ctx, "del1", "a", time.Minute)
	_ = Set(c, ctx, "del2", "b", time.Minute)

	err := Delete(c, ctx, "del1", "del2")
	require.NoError(t, err)

	var r string
	assert.ErrorIs(t, Get(c, ctx, "del1", &r), ErrCacheMiss)
	assert.ErrorIs(t, Get(c, ctx, "del2", &r), ErrCacheMiss)
}

func TestDeleteByPrefix(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	_ = Set(c, ctx, "ns:item:1", "a", time.Minute)
	_ = Set(c, ctx, "ns:item:2", "b", time.Minute)
	_ = Set(c, ctx, "other:item:3", "c", time.Minute)

	err := DeleteByPrefix(c, ctx, "ns:item")
	require.NoError(t, err)

	var r string
	assert.ErrorIs(t, Get(c, ctx, "ns:item:1", &r), ErrCacheMiss)
	assert.ErrorIs(t, Get(c, ctx, "ns:item:2", &r), ErrCacheMiss)
	assert.NoError(t, Get(c, ctx, "other:item:3", &r))
	assert.Equal(t, "c", r)
}

func TestIncrement(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	_ = Set(c, ctx, "counter", "0", time.Minute)

	val, err := Increment(c, ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	val, err = Increment(c, ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestMetrics(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	_ = Set(c, ctx, "m1", "v", time.Minute)

	var r string
	_ = Get(c, ctx, "m1", &r)   // hit
	_ = Get(c, ctx, "miss", &r) // miss

	snap := c.Metrics()
	assert.Equal(t, int64(1), snap.Hits)
	assert.Equal(t, int64(1), snap.Misses)
}
