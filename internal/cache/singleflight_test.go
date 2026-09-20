package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSingleflightDo(t *testing.T) {
	g := newGroup()

	t.Run("deduplicates concurrent calls", func(t *testing.T) {
		var callCount atomic.Int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				val, err := g.Do("same", func() (interface{}, error) {
					callCount.Add(1)
					time.Sleep(50 * time.Millisecond)
					return "result", nil
				})
				assert.NoError(t, err)
				assert.Equal(t, "result", val)
			}()
		}
		wg.Wait()
		assert.Equal(t, int32(1), callCount.Load())
	})

	t.Run("allows subsequent calls after completion", func(t *testing.T) {
		g := newGroup()
		var count atomic.Int32

		_, _ = g.Do("key", func() (interface{}, error) {
			count.Add(1)
			return 1, nil
		})

		_, _ = g.Do("key", func() (interface{}, error) {
			count.Add(1)
			return 2, nil
		})

		assert.Equal(t, int32(2), count.Load())
	})

	t.Run("propagates errors", func(t *testing.T) {
		g := newGroup()
		_, err := g.Do("err", func() (interface{}, error) {
			return nil, assert.AnError
		})
		assert.ErrorIs(t, err, assert.AnError)
	})
}
