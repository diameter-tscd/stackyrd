# Documentation Wiki

## Quick Links

| Document | Description |
|----------|-------------|
| [Getting Started](GETTING_STARTED.md) | Install, run, and configure the framework |
| [Architecture Overview](ARCHITECTURE.md) | Project layout and boot sequence |
| [Development Guide](DEVELOPMENT.md) | Add services, middleware, and infra |
| [API Docs](API_DOCS.md) | Swagger-generated endpoints |

## Core Packages

| Package | Feature |
|---------|---------|
| `pkg/resilience` | Circuit breakers and retry/timeout helpers |
| `pkg/websocket` | Hub‑based real‑time communication |
| `pkg/metrics` | Prometheus metrics collection |
| `pkg/webhook` | HTTP webhook client & server |
| `pkg/infrastructure` | MCP server, redis, postgres, etc |
| `pkg/logger` | Structured JSON logging |

## Operations

| Doc | Purpose |
|------|---------|
| `TESTING.md` | Unit & integration testing patterns |
| `TROUBLESHOOTING.md` | Common issues |

---
Doc brevity kept; see individual files for details.