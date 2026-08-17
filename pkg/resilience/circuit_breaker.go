package resilience

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name                string
	MaxFailures         int
	ResetTimeout        time.Duration
	HalfOpenMaxRequests int
	OnStateChange       func(name string, from State, to State)
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:                name,
		MaxFailures:         5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxRequests: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           State
	failures        int
	successes       int
	lastFailureTime time.Time
	halfOpenCount   int
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 1
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = 1
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.AllowRequest() {
		return errors.New("circuit breaker is open")
	}

	err := fn()

	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}

// ExecuteWithFallback executes a function with circuit breaker protection and fallback
func (cb *CircuitBreaker) ExecuteWithFallback(fn func() error, fallback func() error) error {
	if !cb.AllowRequest() {
		if fallback != nil {
			return fallback()
		}
		return errors.New("circuit breaker is open")
	}

	err := fn()

	if err != nil {
		cb.RecordFailure()
		if fallback != nil {
			return fallback()
		}
		return err
	}

	cb.RecordSuccess()
	return nil
}

// AllowRequest checks if a request is allowed.
// The Open -> HalfOpen transition happens here, atomically, so that after the
// reset timeout exactly HalfOpenMaxRequests probes are let through (no
// unbounded request wave), and a probe that succeeds closes the breaker.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
			cb.transition(StateHalfOpen)
			cb.halfOpenCount = 1 // this probe consumes the allowance
			cb.successes = 0
			return true
		}
		return false
	case StateHalfOpen:
		if cb.halfOpenCount >= cb.config.HalfOpenMaxRequests {
			return false
		}
		cb.halfOpenCount++
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	var change *[2]State
	cb.successes++

	switch cb.state {
	case StateHalfOpen:
		if cb.halfOpenCount >= cb.config.HalfOpenMaxRequests {
			change = cb.transition(StateClosed)
			cb.failures = 0
			cb.halfOpenCount = 0
		}
	case StateClosed:
		cb.failures = 0
	}
	cb.mu.Unlock()

	cb.notify(change)
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	var change *[2]State

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen {
		change = cb.transition(StateOpen)
		cb.halfOpenCount = 0
	} else if cb.state == StateClosed && cb.failures >= cb.config.MaxFailures {
		change = cb.transition(StateOpen)
		cb.halfOpenCount = 0
	}
	cb.mu.Unlock()

	cb.notify(change)
}

// transition sets the new state (caller holds mu) and reports the change.
func (cb *CircuitBreaker) transition(newState State) *[2]State {
	if cb.state == newState {
		return nil
	}
	old := cb.state
	cb.state = newState
	return &[2]State{old, newState}
}

// notify invokes OnStateChange outside the lock so a callback that re-enters
// the breaker (GetState/RecordFailure/...) cannot deadlock.
func (cb *CircuitBreaker) notify(change *[2]State) {
	if change != nil && cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, change[0], change[1])
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]any {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]any{
		"name":              cb.config.Name,
		"state":             cb.state.String(),
		"failures":          cb.failures,
		"successes":         cb.successes,
		"last_failure_time": cb.lastFailureTime,
		"half_open_count":   cb.halfOpenCount,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenCount = 0
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (m *CircuitBreakerManager) GetOrCreate(config CircuitBreakerConfig) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists := m.breakers[config.Name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(config)
	m.breakers[config.Name] = cb
	return cb
}

// Get returns a circuit breaker by name
func (m *CircuitBreakerManager) Get(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cb, exists := m.breakers[name]
	return cb, exists
}

// GetAll returns all circuit breakers
func (m *CircuitBreakerManager) GetAll() map[string]*CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for k, v := range m.breakers {
		result[k] = v
	}
	return result
}

// ResetAll resets all circuit breakers
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cb := range m.breakers {
		cb.Reset()
	}
}
