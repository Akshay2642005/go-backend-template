package breaker

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Defaults(t *testing.T) {
	b := New("test", Config{})

	assert.Equal(t, "test", b.name)
	assert.Equal(t, 5, b.config.FailureThreshold)
	assert.Equal(t, 2, b.config.SuccessThreshold)
	assert.Equal(t, 30*time.Second, b.config.Cooldown)
	assert.Equal(t, StateClosed, b.State())
}

func TestNew_CustomConfig(t *testing.T) {
	b := New("custom", Config{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Cooldown:         5 * time.Second,
	})

	assert.Equal(t, 3, b.config.FailureThreshold)
	assert.Equal(t, 1, b.config.SuccessThreshold)
	assert.Equal(t, 5*time.Second, b.config.Cooldown)
}

func TestClosed_Success(t *testing.T) {
	b := New("test", Config{})

	err := b.Execute(func() error { return nil })
	require.NoError(t, err)
	assert.Equal(t, StateClosed, b.State())
}

func TestClosed_ToOpen(t *testing.T) {
	b := New("test", Config{FailureThreshold: 3})

	// First two failures — still closed
	b.Failure()
	assert.Equal(t, StateClosed, b.State())
	b.Failure()
	assert.Equal(t, StateClosed, b.State())

	// Third failure — opens
	b.Failure()
	assert.Equal(t, StateOpen, b.State())
}

func TestOpen_RejectsRequests(t *testing.T) {
	b := New("test", Config{FailureThreshold: 1, Cooldown: 10 * time.Second})

	b.Failure()
	assert.Equal(t, StateOpen, b.State())

	err := b.Allow()
	require.Error(t, err)

	var openErr *ErrCircuitOpen
	require.True(t, errors.As(err, &openErr))
	assert.Equal(t, "test", openErr.Name)
}

func TestOpen_ToHalfOpen(t *testing.T) {
	b := New("test", Config{FailureThreshold: 1, Cooldown: 1 * time.Millisecond})

	b.Failure()
	assert.Equal(t, StateOpen, b.State())

	// Wait for cooldown
	time.Sleep(5 * time.Millisecond)

	err := b.Allow()
	require.NoError(t, err)
	assert.Equal(t, StateHalfOpen, b.State())
}

func TestHalfOpen_ToClosed(t *testing.T) {
	b := New("test", Config{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Cooldown:         1 * time.Millisecond,
	})

	b.Failure()
	time.Sleep(5 * time.Millisecond)

	// First probe succeeds
	b.Allow()
	b.Success()
	assert.Equal(t, StateHalfOpen, b.State())

	// Second probe succeeds — closes
	b.Allow()
	b.Success()
	assert.Equal(t, StateClosed, b.State())
}

func TestHalfOpen_ToOpen(t *testing.T) {
	b := New("test", Config{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Cooldown:         1 * time.Millisecond,
	})

	b.Failure()
	time.Sleep(5 * time.Millisecond)

	// Probe fails — re-opens
	b.Allow()
	b.Failure()
	assert.Equal(t, StateOpen, b.State())
}

func TestExecute_Success(t *testing.T) {
	b := New("test", Config{})

	err := b.Execute(func() error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, StateClosed, b.State())
}

func TestExecute_Failure(t *testing.T) {
	b := New("test", Config{FailureThreshold: 1})

	err := b.Execute(func() error {
		return errors.New("downstream error")
	})
	require.Error(t, err)
	assert.Equal(t, "downstream error", err.Error())
	assert.Equal(t, StateOpen, b.State())
}

func TestExecute_RejectedWhenOpen(t *testing.T) {
	b := New("test", Config{FailureThreshold: 1, Cooldown: 10 * time.Second})

	b.Failure()

	err := b.Execute(func() error {
		t.Fatal("should not be called")
		return nil
	})
	require.Error(t, err)
	assert.IsType(t, &ErrCircuitOpen{}, err)
}

func TestClosed_ResetsFailureCountOnSuccess(t *testing.T) {
	b := New("test", Config{FailureThreshold: 3})

	b.Failure()
	b.Failure()
	assert.Equal(t, 2, b.failures)

	b.Success()
	assert.Equal(t, 0, b.failures)
}

func TestStats(t *testing.T) {
	b := New("test", Config{FailureThreshold: 3})

	b.Failure()
	b.Failure()

	stats := b.Stats()
	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 2, stats.Failures)
}

func TestState_String(t *testing.T) {
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())
	assert.Equal(t, "unknown", State(99).String())
}

func TestOnStateChange(t *testing.T) {
	var mu sync.Mutex
	var transitions []string
	done := make(chan struct{})

	b := New("test", Config{
		FailureThreshold: 1,
		OnStateChange: func(name string, from, to State) {
			mu.Lock()
			defer mu.Unlock()
			transitions = append(transitions, from.String()+"->"+to.String())
			select {
			case done <- struct{}{}:
			default:
			}
		},
	})

	b.Failure() // closed -> open

	// Wait for the async callback
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, transitions, 1)
	assert.Equal(t, "closed->open", transitions[0])
}

func TestHalfOpen_LimitsProbes(t *testing.T) {
	b := New("test", Config{
		FailureThreshold: 1,
		SuccessThreshold: 100, // High threshold to prevent closing
		Cooldown:         1 * time.Millisecond,
	})

	b.Failure()
	time.Sleep(5 * time.Millisecond)

	// Allow through (enters half-open)
	require.NoError(t, b.Allow())

	// Subsequent allows within the limit
	require.NoError(t, b.Allow())

	// After exceeding the limit, should reject
	for i := 0; i < 100; i++ {
		b.Allow()
	}

	err := b.Allow()
	require.Error(t, err)
	assert.IsType(t, &ErrCircuitOpen{}, err)
}
