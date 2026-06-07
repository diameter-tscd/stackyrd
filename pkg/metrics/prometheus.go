// Package metrics provides Prometheus metric collection and exposition.
package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestSize      *prometheus.HistogramVec
	HTTPResponseSize     *prometheus.HistogramVec
	ActiveConnections    prometheus.Gauge
	DatabaseConnections  *prometheus.GaugeVec
	CacheHits            *prometheus.CounterVec
	CacheMisses          *prometheus.CounterVec
	CircuitBreakerState  *prometheus.GaugeVec
	CircuitBreakerTrips  *prometheus.CounterVec
	WebhookEvents        *prometheus.CounterVec
	WebhookDuration      *prometheus.HistogramVec
	WebSocketConnections prometheus.Gauge
	BatchOperations      *prometheus.CounterVec
	BatchDuration        *prometheus.HistogramVec
	LogEntries           *prometheus.CounterVec
	ErrorRate            *prometheus.CounterVec
}

// NewMetrics creates new Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP request size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		),
		HTTPResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		),
		ActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_connections",
				Help: "Number of active connections",
			},
		),
		DatabaseConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "database_connections",
				Help: "Number of database connections",
			},
			[]string{"database", "state"},
		),
		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache", "operation"},
		),
		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache", "operation"},
		),
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_state",
				Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"name"},
		),
		CircuitBreakerTrips: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_trips_total",
				Help: "Total number of circuit breaker trips",
			},
			[]string{"name"},
		),
		WebhookEvents: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "webhook_events_total",
				Help: "Total number of webhook events",
			},
			[]string{"event_type", "status"},
		),
		WebhookDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "webhook_duration_seconds",
				Help:    "Webhook request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"event_type"},
		),
		WebSocketConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "websocket_connections",
				Help: "Number of WebSocket connections",
			},
		),
		BatchOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "batch_operations_total",
				Help: "Total number of batch operations",
			},
			[]string{"operation", "status"},
		),
		BatchDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "batch_duration_seconds",
				Help:    "Batch operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		LogEntries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_entries_total",
				Help: "Total number of log entries",
			},
			[]string{"level", "service"},
		),
		ErrorRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "errors_total",
				Help: "Total number of errors",
			},
			[]string{"type", "service"},
		),
	}
}

// RecordHTTPRequest records HTTP request metrics
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration, requestSize, responseSize int64) {
	statusStr := strconv.Itoa(status)

	m.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	m.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	m.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(cache, operation string) {
	m.CacheHits.WithLabelValues(cache, operation).Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(cache, operation string) {
	m.CacheMisses.WithLabelValues(cache, operation).Inc()
}

// SetCircuitBreakerState sets circuit breaker state
func (m *Metrics) SetCircuitBreakerState(name string, state int) {
	m.CircuitBreakerState.WithLabelValues(name).Set(float64(state))
}

// RecordCircuitBreakerTrip records a circuit breaker trip
func (m *Metrics) RecordCircuitBreakerTrip(name string) {
	m.CircuitBreakerTrips.WithLabelValues(name).Inc()
}

// RecordWebhookEvent records a webhook event
func (m *Metrics) RecordWebhookEvent(eventType, status string, duration time.Duration) {
	m.WebhookEvents.WithLabelValues(eventType, status).Inc()
	m.WebhookDuration.WithLabelValues(eventType).Observe(duration.Seconds())
}

// SetWebSocketConnections sets WebSocket connections count
func (m *Metrics) SetWebSocketConnections(count int) {
	m.WebSocketConnections.Set(float64(count))
}

// RecordBatchOperation records a batch operation
func (m *Metrics) RecordBatchOperation(operation, status string, duration time.Duration) {
	m.BatchOperations.WithLabelValues(operation, status).Inc()
	m.BatchDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordLogEntry records a log entry
func (m *Metrics) RecordLogEntry(level, service string) {
	m.LogEntries.WithLabelValues(level, service).Inc()
}

// RecordError records an error
func (m *Metrics) RecordError(errorType, service string) {
	m.ErrorRate.WithLabelValues(errorType, service).Inc()
}

// SetActiveConnections sets active connections count
func (m *Metrics) SetActiveConnections(count int) {
	m.ActiveConnections.Set(float64(count))
}

// SetDatabaseConnections sets database connections count
func (m *Metrics) SetDatabaseConnections(database, state string, count int) {
	m.DatabaseConnections.WithLabelValues(database, state).Set(float64(count))
}

// Handler returns Prometheus metrics HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

var globalMetrics *Metrics
var metricsOnce sync.Once

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics()
	})
	return globalMetrics
}
