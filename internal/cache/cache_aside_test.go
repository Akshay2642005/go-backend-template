package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrSet(t *testing.T) {
	tr, cleanup := SetupTestRedis(t)
	defer cleanup()

	c := NewTestCache(t, tr)
	ctx := context.Background()

	t.Run("populates cache on miss", func(t *testing.T) {
		called := false
		val, err := GetOrSet(c, ctx, "aside:1", time.Minute, func() (string, error) {
			called = true
			return "from_origin", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "from_origin", val)
		assert.True(t, called)

		// Second call should hit cache
		called = false
		val, err = GetOrSet(c, ctx, "aside:1", time.Minute, func() (string, error) {
			called = true
			return "should_not_run", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "from_origin", val)
		assert.False(t, called)
	})

	t.Run("singleflight prevents concurrent origin calls", func(t *testing.T) {
		var callCount atomic.Int32
		var wg sync.WaitGroup

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = GetOrSet(c, ctx, "sf:test", time.Minute, func() (int, error) {
					callCount.Add(1)
					time.Sleep(100 * time.Millisecond)
					return 42, nil
				})
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(1), callCount.Load())
	})

	t.Run("fail-open on origin error", func(t *testing.T) {
		val, err := GetOrSet(c, ctx, "fail:origin", time.Minute, func() (string, error) {
			return "", assert.AnError
		})
		assert.Error(t, err)
		assert.Equal(t, "", val)
	})
}
