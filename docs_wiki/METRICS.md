# Metrics

Prometheus metrics via `pkg/metrics`.

```go
m := metrics.GetMetrics()
e.GET("/metrics", echo.WrapHandler(m.Handler()))
```

## Recording

```go
m.RecordHTTPRequest("GET", "/api/v1/users", 200, duration, reqSize, respSize)
m.RecordCacheHit("redis", "get")
m.SetCircuitBreakerState("payments", 1)
m.RecordWebhookEvent("order.created", "success", duration)
m.SetWebSocketConnections(hub.ConnectedClients())
m.SetActiveConnections(count)
```

Scrape from Prometheus:

```yaml
scrape_configs:
  - job_name: 'stackyrd'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```
