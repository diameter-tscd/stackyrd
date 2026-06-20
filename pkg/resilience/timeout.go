package resilience

import (
	"context"
	"errors"
	"time"
)

var (
	ErrTimeout = errors.New("operation timed out")
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	Timeout time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Timeout: 30 * time.Second,
	}
}

// WithTimeout executes a function with a timeout
func WithTimeout(fn func() error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return WithContext(ctx, fn)
}

// WithContext executes a function with a context
func WithContext(ctx context.Context, fn func() error) error {
	errChan := make(chan error, 1)

	go func() {
		errChan <- fn()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return ErrTimeout
		}
		return ctx.Err()
	}
}

// WithTimeoutResult executes a function with a timeout and returns a result
func WithTimeoutResult[T any](fn func() (T, error), timeout time.Duration) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return WithContextResult(ctx, fn)
}

// WithContextResult executes a function with a context and returns a result
func WithContextResult[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	resultChan := make(chan struct {
		result T
		err    error
	}, 1)

	go func() {
		result, err := fn()
		resultChan <- struct {
			result T
			err    error
		}{result, err}
	}()

	select {
	case res := <-resultChan:
		return res.result, res.err
	case <-ctx.Done():
		var zero T
		if ctx.Err() == context.DeadlineExceeded {
			return zero, ErrTimeout
		}
		return zero, ctx.Err()
	}
}

// TimeoutFunc wraps a function with timeout
type TimeoutFunc func() error

// WithTimeoutConfig executes a function with timeout configuration
func WithTimeoutConfig(fn func() error, config ...TimeoutConfig) error {
	var cfg TimeoutConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultTimeoutConfig()
	}

	return WithTimeout(fn, cfg.Timeout)
}

// TimeoutFuncResult wraps a function with timeout that returns a result
type TimeoutFuncResult[T any] func() (T, error)

// WithTimeoutConfigResult executes a function with timeout configuration and returns a result
func WithTimeoutConfigResult[T any](fn func() (T, error), config ...TimeoutConfig) (T, error) {
	var cfg TimeoutConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultTimeoutConfig()
	}

	return WithTimeoutResult(fn, cfg.Timeout)
}
