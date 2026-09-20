// Package breaker implements the circuit breaker pattern for protecting
// against cascading failures when downstream services are unhealthy.
//
// States:
//   - Closed:   Normal operation. Failures are counted. When the failure
//     threshold is reached, the breaker transitions to Open.
//   - Open:     All calls are rejected immediately with ErrCircuitOpen.
//     After the cooldown period, the breaker transitions to HalfOpen.
//   - HalfOpen: A limited number of probe requests are allowed through.
//     If they succeed, the breaker transitions to Closed. If any fail,
//     it transitions back to Open.
package breaker

import (
	"sync"
	"time"
)

// State represents the current state of the circuit breaker.
type State int

const (
	// StateClosed is normal operation — requests flow through.
	StateClosed State = iota
	// StateOpen rejects all requests until the cooldown expires.
	StateOpen
	// StateHalfOpen allows a limited number of probe requests.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures the circuit breaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening
	// the breaker. Default: 5.
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successes in half-open
	// state before closing the breaker. Default: 2.
	SuccessThreshold int
	// Cooldown is how long the breaker stays open before allowing probes.
	// Default: 30s.
	Cooldown time.Duration
	// OnStateChange is called when the breaker transitions between states.
	OnStateChange func(name string, from, to State)
}

// Breaker implements the circuit breaker pattern.
type Breaker struct {
	name          string
	config        Config
	state         State
	failures      int
	successes     int
	lastFailure   time.Time
	halfOpenCount int
	mu            sync.RWMutex
}

// New creates a new circuit breaker with the given name and config.
// Zero-value config fields are filled with sensible defaults.
func New(name string, cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}

	return &Breaker{
		name:   name,
		config: cfg,
		state:  StateClosed,
	}
}

// ErrCircuitOpen is returned when a call is rejected because the breaker
// is in the Open state.
type ErrCircuitOpen struct {
	Name     string
	Cooldown time.Duration
}

func (e *ErrCircuitOpen) Error() string {
	return "circuit breaker " + e.Name + " is open"
}

// Allow checks whether a request should be allowed through the breaker.
// Returns nil if allowed, or ErrCircuitOpen if rejected.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return nil

	case StateOpen:
		if time.Since(b.lastFailure) >= b.config.Cooldown {
			b.setState(StateHalfOpen)
			b.halfOpenCount = 0
			return nil
		}
		return &ErrCircuitOpen{Name: b.name, Cooldown: b.config.Cooldown}

	case StateHalfOpen:
		if b.halfOpenCount >= b.config.SuccessThreshold+b.config.FailureThreshold {
			return &ErrCircuitOpen{Name: b.name, Cooldown: b.config.Cooldown}
		}
		b.halfOpenCount++
		return nil
	}

	return nil
}

// Success records a successful call. In half-open state, consecutive
// successes close the breaker.
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		b.successes++
		if b.successes >= b.config.SuccessThreshold {
			b.setState(StateClosed)
			b.resetCounts()
		}
	case StateClosed:
		b.failures = 0
	}
}

// Failure records a failed call. In closed state, consecutive failures
// open the breaker. In half-open state, any failure re-opens it.
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastFailure = time.Now()

	switch b.state {
	case StateClosed:
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.setState(StateOpen)
		}
	case StateHalfOpen:
		b.setState(StateOpen)
	}
}

// State returns the current state of the breaker.
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Stats returns a snapshot of the breaker's internal counters.
type Stats struct {
	Name      string
	State     State
	Failures  int
	Successes int
}

// Stats returns the current breaker stats.
func (b *Breaker) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Stats{
		Name:      b.name,
		State:     b.state,
		Failures:  b.failures,
		Successes: b.successes,
	}
}

func (b *Breaker) setState(to State) {
	from := b.state
	b.state = to
	if b.config.OnStateChange != nil {
		go b.config.OnStateChange(b.name, from, to)
	}
}

func (b *Breaker) resetCounts() {
	b.failures = 0
	b.successes = 0
	b.halfOpenCount = 0
}

// Execute runs fn if the breaker allows it. If fn succeeds, it calls
// Success(); if it fails, it calls Failure(). Returns the function's
// error or ErrCircuitOpen if the call was rejected.
func (b *Breaker) Execute(fn func() error) error {
	if err := b.Allow(); err != nil {
		return err
	}

	if err := fn(); err != nil {
		b.Failure()
		return err
	}

	b.Success()
	return nil
}
