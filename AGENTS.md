# stackyrd

**Language:** Go 1.25.3 — **Module:** `github.com/diameter-tscd/stackyrd`
**Purpose:** Modular service framework built on Echo v4 with auto-discovered services, middleware, and infrastructure.

## Quick Reference

```
go mod download         # Install deps
go run cmd/app/main.go  # Run (needs config.yaml in CWD)
go test ./...           # All tests
docker compose up       # Full dev stack (Redis, PG, Kafka, Mongo, Grafana, MinIO)
cd scripts && go build -o yrd . # Build CLI (output: scripts/yrd)
./scripts/yrd build # Build server binary (output: dist/stackyrd)
```

## Patterns

| Add | Where | Register via | Toggle in `config.yaml` |
|-----|-------|-------------|------------------------|
| Service | `internal/services/modules/{name}_service.go` | `init()` → `registry.RegisterService` | `services.{name}_service` |
| Middleware | `internal/middleware/{name}.go` | `init()` → `RegisterMiddleware` | `middleware.{name}` |
| Infrastructure | `pkg/infrastructure/{name}.go` | `init()` → `RegisterComponent` | _(managed by registry)_ |

**Interfaces:**
- `Service`: `Name()`, `WireName()`, `Enabled()`, `Endpoints()`, `RegisterRoutes(echo.Group)`, `Get()`
- `InfrastructureComponent`: `Name()`, `Close()`, `GetStatus()`
- `MiddlewareFactory`: `func(cfg, logger) (echo.MiddlewareFunc, error)`

**Naming:** `{name}_service.go`, `{name}.go` (infra), `{package}_test.go`

## Directory Layout

```
cmd/app/           # Entry point, CLI flags, bootstrap
config/            # Viper config structs (config.yaml is source of truth)
internal/
  middleware/      # HTTP middleware (auto-registered via init())
  server/          # Echo server, health endpoints, graceful shutdown
  services/modules/ # Business logic services
pkg/
  assets/          # Embedded assets (banner.txt)
  interfaces/      # Service interface
  registry/        # Service factory registry, DI container
  infrastructure/  # Redis, PG, Mongo, Kafka, MinIO, Grafana, Cron, etc.
  logger/          # Zerolog structured logging
  response/        # API response helpers
  request/         # Request binding/validation
  tui/             # Bubbletea terminal UI
  metrics/         # Prometheus
  pagination/      # Cursor-based
  cache/           # Redis-backed
  batch/           # Batch processing
  resilience/      # Circuit breaker, retry, timeout
  testing/         # Test helpers/mocks
  utils/           # System, HTTP, IO, date, numeric, strings, image, broadcast
  webhook/         # Webhook handler
  websocket/       # WebSocket handler
scripts/           # Standalone CLI module (build/service/pkg/swagger/docker), independent go.mod
tests/             # Integration tests (mirrors src layout)
docs/              # Auto-generated Swagger
docs_wiki/         # Hand-written docs (update README.md TOC when changed)
deployments/       # K8s manifests
```

## Config

`config.yaml` at repo root. Loaded via Viper (YAML + env overrides). Struct in `config/config.go` with typed sections: `App`, `Server`, `Services`, `Middleware`, `Auth`, `Redis`, `Kafka`, `Postgres`, `Mongo`, `Grafana`, `Minio`, `Cron`, `Encryption`.

**Never commit secrets in config.yaml** — use env vars in production.

## Boot Order

```
Infra async init → Dependencies → Middleware → AutoDiscoverServices
```

## Never Commit

- `config.yaml` with real secrets
- `dist/` directory
- `.env` or credential files
