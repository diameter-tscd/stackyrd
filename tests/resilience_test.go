package main_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"stackyrd/pkg/resilience"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig("test"))
	assert.Equal(t, resilience.StateClosed, cb.GetState())
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cfg := resilience.DefaultCircuitBreakerConfig("test")
	cfg.MaxFailures = 3
	cb := resilience.NewCircuitBreaker(cfg)

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errors.New("fail") })
	}
	assert.Equal(t, resilience.StateOpen, cb.GetState())
}

func TestCircuitBreaker_ClosesAfterSuccess(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig("test"))
	_ = cb.Execute(func() error { return nil })
	assert.Equal(t, resilience.StateClosed, cb.GetState())
}

func TestCircuitBreaker_BlocksWhenOpen(t *testing.T) {
	cfg := resilience.DefaultCircuitBreakerConfig("test")
	cfg.MaxFailures = 1
	cb := resilience.NewCircuitBreaker(cfg)

	_ = cb.Execute(func() error { return errors.New("fail") })
	err := cb.Execute(func() error { return nil })
	assert.ErrorContains(t, err, "circuit breaker is open")
}

func TestCircuitBreaker_FallbackOnOpen(t *testing.T) {
	cfg := resilience.DefaultCircuitBreakerConfig("test")
	cfg.MaxFailures = 1
	cb := resilience.NewCircuitBreaker(cfg)

	_ = cb.Execute(func() error { return errors.New("fail") })

	fallbackCalled := false
	err := cb.ExecuteWithFallback(
		func() error { return errors.New("primary failed") },
		func() error { fallbackCalled = true; return nil },
	)
	assert.NoError(t, err)
	assert.True(t, fallbackCalled)
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	cfg := resilience.DefaultCircuitBreakerConfig("test")
	cfg.MaxFailures = 1
	cfg.ResetTimeout = 1 * time.Millisecond
	cb := resilience.NewCircuitBreaker(cfg)

	_ = cb.Execute(func() error { return errors.New("fail") })
	assert.Equal(t, resilience.StateOpen, cb.GetState())

	time.Sleep(5 * time.Millisecond)
	assert.True(t, cb.AllowRequest())
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig("stats-test"))
	_ = cb.Execute(func() error { return errors.New("fail") })

	stats := cb.GetStats()
	assert.Equal(t, "stats-test", stats["name"])
	assert.Equal(t, "closed", stats["state"])
	assert.Equal(t, 1, stats["failures"])
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := resilience.DefaultCircuitBreakerConfig("test")
	cfg.MaxFailures = 1
	cb := resilience.NewCircuitBreaker(cfg)

	_ = cb.Execute(func() error { return errors.New("fail") })
	assert.Equal(t, resilience.StateOpen, cb.GetState())

	cb.Reset()
	assert.Equal(t, resilience.StateClosed, cb.GetState())
}

func TestCircuitBreakerManager_GetOrCreate(t *testing.T) {
	mgr := resilience.NewCircuitBreakerManager()
	cb := mgr.GetOrCreate(resilience.DefaultCircuitBreakerConfig("svc1"))
	assert.NotNil(t, cb)

	cb2 := mgr.GetOrCreate(resilience.DefaultCircuitBreakerConfig("svc1"))
	assert.Same(t, cb, cb2)
}

func TestCircuitBreakerManager_GetMissing(t *testing.T) {
	mgr := resilience.NewCircuitBreakerManager()
	_, ok := mgr.Get("nonexistent")
	assert.False(t, ok)
}

func TestCircuitBreakerManager_ResetAll(t *testing.T) {
	mgr := resilience.NewCircuitBreakerManager()
	cfg := resilience.DefaultCircuitBreakerConfig("t")
	cfg.MaxFailures = 1
	cb := mgr.GetOrCreate(cfg)
	_ = cb.Execute(func() error { return errors.New("fail") })
	assert.Equal(t, resilience.StateOpen, cb.GetState())

	mgr.ResetAll()
	assert.Equal(t, resilience.StateClosed, cb.GetState())
}

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	attempts := 0
	err := resilience.Retry(func() error {
		attempts++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_RetriesOnFailure(t *testing.T) {
	attempts := 0
	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 1 * time.Millisecond

	err := resilience.Retry(func() error {
		attempts++
		return errors.New("fail")
	}, cfg)
	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	attempts := 0
	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 1 * time.Millisecond

	err := resilience.Retry(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	}, cfg)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestRetry_CustomRetryIf(t *testing.T) {
	attempts := 0
	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 1 * time.Millisecond
	cfg.RetryIf = func(err error) bool { return err.Error() == "retryable" }

	err := resilience.Retry(func() error {
		attempts++
		return errors.New("fatal")
	}, cfg)
	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // didn't retry because RetryIf returned false
}

func TestRetryWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	err := resilience.RetryWithContext(ctx, func() error {
		return errors.New("fail")
	}, cfg)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetryWithResult(t *testing.T) {
	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 2

	result, err := resilience.RetryWithResult(func() (string, error) {
		return "hello", nil
	}, cfg)
	assert.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestRetryWithResult_Failure(t *testing.T) {
	cfg := resilience.DefaultRetryConfig()
	cfg.MaxAttempts = 1

	_, err := resilience.RetryWithResult(func() (string, error) {
		return "", errors.New("fail")
	}, cfg)
	assert.Error(t, err)
}

func TestTimeout_Success(t *testing.T) {
	err := resilience.WithTimeout(func() error {
		return nil
	}, time.Second)
	assert.NoError(t, err)
}

func TestTimeout_Exceeded(t *testing.T) {
	err := resilience.WithTimeout(func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, 1*time.Millisecond)
	assert.ErrorIs(t, err, resilience.ErrTimeout)
}

func TestTimeoutWithConfig(t *testing.T) {
	cfg := resilience.TimeoutConfig{Timeout: time.Second}
	err := resilience.WithTimeoutConfig(func() error {
		return nil
	}, cfg)
	assert.NoError(t, err)
}

func TestTimeoutWithResult(t *testing.T) {
	result, err := resilience.WithTimeoutResult(func() (string, error) {
		return "ok", nil
	}, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestTimeoutWithResult_Exceeded(t *testing.T) {
	_, err := resilience.WithTimeoutResult(func() (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "ok", nil
	}, 1*time.Millisecond)
	assert.ErrorIs(t, err, resilience.ErrTimeout)
}

func TestRetryableError(t *testing.T) {
	baseErr := errors.New("db timeout")
	retryable := resilience.NewRetryableError(baseErr)
	assert.True(t, resilience.IsRetryable(retryable))
	assert.False(t, resilience.IsRetryable(baseErr))
}

func TestRetryIfRetryable(t *testing.T) {
	fn := resilience.RetryIfRetryable()
	assert.True(t, fn(resilience.NewRetryableError(errors.New("x"))))
	assert.False(t, fn(errors.New("x")))
}

func TestStateString(t *testing.T) {
	assert.Equal(t, "closed", resilience.StateClosed.String())
	assert.Equal(t, "half-open", resilience.StateHalfOpen.String())
	assert.Equal(t, "open", resilience.StateOpen.String())
	assert.Equal(t, "unknown", resilience.State(99).String())
}
