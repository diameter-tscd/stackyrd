# Metrics & Observability

Prometheus metrics for HTTP requests, cache, circuit breakers, webhooks, WebSocket, batch operations, and logging.

## Quick Start

Metrics are auto-collected via the singleton `metrics.GetMetrics()`. Register the `/metrics` endpoint to expose them:

```go
import "stackyrd/pkg/metrics"

e := echo.New()
m := metrics.GetMetrics()
e.GET("/metrics", echo.WrapHandler(m.Handler()))
```

Access at `http://localhost:8080/metrics`.

## Available Metrics

### HTTP Requests

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | Counter | `method`, `path`, `status` |
| `http_request_duration_seconds` | Histogram | `method`, `path`, `status` |
| `http_request_size_bytes` | Histogram | `method`, `path` |
| `http_response_size_bytes` | Histogram | `method`, `path` |

### Cache

| Metric | Type | Labels |
|--------|------|--------|
| `cache_hits_total` | Counter | `cache`, `operation` |
| `cache_misses_total` | Counter | `cache`, `operation` |

### Circuit Breaker

| Metric | Type | Labels |
|--------|------|--------|
| `circuit_breaker_state` | Gauge | `name` (0=closed, 1=half-open, 2=open) |
| `circuit_breaker_trips_total` | Counter | `name` |

### Webhooks

| Metric | Type | Labels |
|--------|------|--------|
| `webhook_events_total` | Counter | `event_type`, `status` |
| `webhook_duration_seconds` | Histogram | `event_type` |

### WebSocket

| Metric | Type | Description |
|--------|------|-------------|
| `websocket_connections` | Gauge | Current number of WS connections |

### Batch Operations

| Metric | Type | Labels |
|--------|------|--------|
| `batch_operations_total` | Counter | `operation`, `status` |
| `batch_duration_seconds` | Histogram | `operation` |

### Logging

| Metric | Type | Labels |
|--------|------|--------|
| `log_entries_total` | Counter | `level`, `service` |
| `errors_total` | Counter | `error_type`, `service` |

### System

| Metric | Type | Description |
|--------|------|-------------|
| `active_connections` | Gauge | Current active HTTP connections |
| `database_connections` | Gauge | DB connection count by database and state |

## Recording Metrics

### HTTP Requests

```go
func (s *MyService) handler(c echo.Context) error {
    start := time.Now()
    // ... handle request ...
    m.RecordHTTPRequest("GET", "/api/v1/users", 200, time.Since(start), reqSize, respSize)
    return nil
}
```

### Cache Operations

```go
if val, ok := cache.Get(key); ok {
    m.RecordCacheHit("redis", "get")
} else {
    m.RecordCacheMiss("redis", "get")
}
```

### Circuit Breaker

```go
m.SetCircuitBreakerState("payment-service", 1) // half-open
m.RecordCircuitBreakerTrip("payment-service")
```

### WebSocket

```go
m.SetWebSocketConnections(hub.GetConnectedClients())
// Call periodically to update gauge
```

### Batch Operations

```go
start := time.Now()
result, err := processor.Process(ctx, items)
status := "success"
if err != nil { status = "failure" }
m.RecordBatchOperation("process-orders", status, time.Since(start))
```

### Webhook Events

```go
start := time.Now()
resp, err := mgr.Send(ctx, event)
status := "success"
if err != nil { status = "failure" }
m.RecordWebhookEvent("order.created", status, time.Since(start))
```

### Errors

```go
m.RecordError("timeout", "payment-service")
```

## Custom Metrics

Extend the `Metrics` struct with additional Prometheus collectors:

```go
import "github.com/prometheus/client_golang/prometheus"

type CustomMetrics struct {
    OrdersProcessed prometheus.Counter
}

func RegisterCustomMetrics() {
    ordersProcessed := promauto.NewCounter(prometheus.CounterOpts{
        Name: "orders_processed_total",
        Help: "Total number of orders processed",
    })
}
```

## Prometheus Configuration

Scrape from the application:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'stackyrd'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

## Grafana Dashboard

Create dashboards using these queries:

```promql
# Request rate
rate(http_requests_total[5m])

# P99 latency
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Cache hit ratio
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))

# Active connections
active_connections
```

## Singleton Access

```go
// Get the global metrics instance (initialized once via sync.Once)
m := metrics.GetMetrics()
```
