# Resilience Patterns

## Circuit Breaker

```go
cb := resilience.NewCircuitBreaker(
    resilience.DefaultCircuitBreakerConfig("payment-service"),
)

err := cb.Execute(func() error {
    return callPaymentAPI()
})
```

Multiple breakers:

```go
mgr := resilience.NewCircuitBreakerManager()
cb := mgr.GetOrCreate(resilience.DefaultCircuitBreakerConfig("service-a"))
```

## Retry

```go
err := resilience.RetryWithContext(ctx, func() error {
    return flakyNetworkCall()
}, resilience.RetryConfig{
    MaxAttempts: 3,
    InitialDelay: 100 * time.Millisecond,
    Jitter: true,
})
```

## Timeout

```go
err := resilience.WithTimeout(func() error {
    return slowOp()
}, 5*time.Second)
```

## Health Checks

```go
hc := resilience.NewHealthChecker()
hc.RegisterSimpleCheck("db", db.Ping)
report := hc.Check(ctx)
```

Combine: `cb.Execute(resilience.RetryWithContext(...))`.
