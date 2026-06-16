# Documentation Wiki

## Quick Links

| Document | Description |
|----------|-------------|
| [**Getting Started**](GETTING_STARTED.md) | Prerequisites, installation, configuration, hello-world service, scripts overview |
| [**Architecture Overview**](ARCHITECTURE.md) | Boot sequence, request flow, project structure, core abstractions (Service, Infrastructure, Plugin, Middleware) |
| [**Development Guide**](DEVELOPMENT.md) | Adding services/middleware/infrastructure/plugins, request validation, DI, pagination, resilience patterns, testing |
| [**API Documentation**](API_DOCS.md) | Swagger annotations, response format, helpers reference, doc generation |
| [**Technical Reference**](REFERENCE.md) | Full config.yaml reference, health endpoints, component registry, plugin system API, middleware list, common commands |
| [**Plugin System**](PLUGIN_SYSTEM.md) | Four plugin types (TypeScript, Lua, Python, Go): creation, HTTP routes, WebSocket, persistent state, background execution, management API, sandbox, troubleshooting |

## Package Deep Dives

| Document | Package | Description |
|----------|---------|-------------|
| [**Resilience**](RESILIENCE.md) | `pkg/resilience/` | Circuit breaker, health checks, retry, timeout patterns |
| [**WebSocket**](WEBSOCKET.md) | `pkg/websocket/` | Real-time bidirectional communication |
| [**Metrics**](METRICS.md) | `pkg/metrics/` | Prometheus metrics, custom metrics, dashboards |
| [**Batch Processing**](BATCH.md) | `pkg/batch/` | Batch processing with worker pools, writers, readers |
| [**Pagination**](PAGINATION.md) | `pkg/pagination/` | Cursor-based pagination with forward/backward navigation |
| [**Logging**](LOGGING.md) | `pkg/logging/` | Structured logging, log rotation, sampling |
| [**Webhooks**](WEBHOOK.md) | `pkg/webhook/` | Webhook sending/receiving, HMAC signing, event handlers |
| [**Caching**](CACHING.md) | `pkg/caching/` | Redis-backed cache with TTL, cache-aside, batch invalidation |
| [**Security**](SECURITY.md) | *middleware* | Auth modes, security headers, CORS, encryption, best practices |

## Operations

| Document | Description |
|----------|-------------|
| [**Testing Guide**](TESTING.md) | Unit tests, integration tests, mocks, test helpers, CI pipeline |
| [**Troubleshooting**](TROUBLESHOOTING.md) | Common issues, debugging, health checks, error codes |

## Examples

| File | Description |
|------|-------------|
| [`examples/response_examples.go`](examples/response_examples.go) | API response usage examples |