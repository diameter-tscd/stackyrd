package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

var (
	jitterMu   sync.Mutex
	jitterRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        bool
	RetryIf       func(error) bool
	OnRetry       func(attempt int, err error)
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryIf:       nil,
		OnRetry:       nil,
	}
}

// Retry executes a function with retry and exponential backoff
func Retry(fn func() error, config ...RetryConfig) error {
	var cfg RetryConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return err
		}

		if attempt < cfg.MaxAttempts {
			delay := calculateDelay(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, err)
			}
			time.Sleep(delay)
		}
	}

	return lastErr
}

// RetryWithContext executes a function with retry and exponential backoff with context
func RetryWithContext(ctx context.Context, fn func() error, config ...RetryConfig) error {
	var cfg RetryConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return err
		}

		if attempt < cfg.MaxAttempts {
			delay := calculateDelay(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, err)
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return lastErr
}

// RetryWithResult executes a function with retry and returns a result
func RetryWithResult[T any](fn func() (T, error), config ...RetryConfig) (T, error) {
	var cfg RetryConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRetryConfig()
	}

	var lastErr error
	var zero T
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts {
			delay := calculateDelay(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, err)
			}
			time.Sleep(delay)
		}
	}

	return zero, lastErr
}

// RetryWithResultContext executes a function with retry and returns a result with context
func RetryWithResultContext[T any](ctx context.Context, fn func() (T, error), config ...RetryConfig) (T, error) {
	var cfg RetryConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRetryConfig()
	}

	var lastErr error
	var zero T
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts {
			delay := calculateDelay(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, err)
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return zero, lastErr
}

// calculateDelay calculates the delay for a retry attempt
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))

	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	if config.Jitter {
		jitterMu.Lock()
		jitter := jitterRand.Float64() * 0.5
		jitterMu.Unlock()
		delay = delay * (1 + jitter)
	}

	return time.Duration(delay)
}

// RetryableError wraps an error to indicate it's retryable
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error) *RetryableError {
	return &RetryableError{Err: err}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	return errors.As(err, &retryableErr)
}

// RetryIfRetryable returns a RetryIf function that retries only retryable errors
func RetryIfRetryable() func(error) bool {
	return func(err error) bool {
		return IsRetryable(err)
	}
}
