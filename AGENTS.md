# AGENTS.md

## Project Overview

**Name:** stackyrd-nano
**Language:** Go 1.25.3
**Module:** `stackyrd-nano`
**Purpose:** Lightweight modular service framework built on Gin for rapid Go API applications.
**Architecture:** Layered modular architecture with auto-discovered services, middleware, and infrastructure components. All pre-built services and heavy infrastructure (Kafka, Mongo, MinIO, Grafana, Redis) have been removed for a lean core.

---

## Directory Structure

```
stackyrd-nano/
├── cmd/app/                  # Application entry point
│   ├── main.go               # CLI flags, bootstrap
│   ├── application.go        # App lifecycle: TUI vs console mode
│   ├── config_manager.go     # Config loading
│   ├── constants.go          # App constants
│   └── embed.go              # Embedded banner asset
├── config/
│   └── config.go             # Config structs, Viper setup
├── internal/
│   ├── middleware/            # HTTP middleware (auto-registered via init())
│   │   ├── middleware.go      # Registry, auto-discovery
│   │   ├── audit.go, cors.go, encryption.go, jwt.go
│   │   ├── ratelimit.go, security.go
│   └── server/
│       └── server.go          # Gin server, health endpoints, graceful shutdown
├── pkg/
│   ├── assets/                # Embedded banner.txt
│   ├── interfaces/            # Service interface
│   ├── registry/              # Service factory registry + DI container
│   ├── infrastructure/        # Infra components (Postgres only)
│   │   ├── component.go       # Interface
│   │   ├── registry.go        # ComponentRegistry singleton
│   │   ├── async_init.go      # Async init manager
│   │   ├── async.go           # Async utilities
│   │   ├── route.go           # Route registrar interface
│   │   └── postgres.go        # PostgreSQL + GORM multi-connection
│   ├── logger/                # Structured logger (zerolog)
│   ├── response/              # API response helpers
│   ├── request/               # Request binding + validation
│   ├── tui/                   # Terminal UI (bubbletea)
│   ├── pagination/            # Cursor-based pagination
│   ├── cache/                 # In-memory generic cache
│   ├── batch/                 # Batch processing utilities
│   ├── resilience/            # Circuit breaker, retry, timeout
│   ├── testing/               # Test helpers + mocks
│   ├── utils/                 # System, HTTP, IO, date, numeric, strings
│   ├── webhook/               # Webhook handler
│   └── websocket/             # WebSocket handler (gorilla/websocket)
├── scripts/
│   ├── build/build.go         # Build script
│   ├── docker/docker_build.go
│   ├── pkg/pkg.go             # Package installer
│   └── service/               # Service code generator
├── tests/
│   ├── simple_test.go
│   ├── startup_test.go
│   └── performance_test.go
├── docs_wiki/                 # Project documentation
├── config.yaml
├── go.mod / go.sum
└── .github/workflows/
    ├── go-build.yml
    └── security.yml
```

---

## Core Abstractions

### Service Interface (`pkg/interfaces/service.go`)

```go
type Service interface {
    Name() string
    WireName() string
    Enabled() bool
    Endpoints() []string
    RegisterRoutes(g *gin.RouterGroup)
    Get() interface{}
}
```

Auto-discovered via `init()`. Toggle in `config.yaml` under `services:`.

### InfrastructureComponent Interface (`pkg/infrastructure/component.go`)

```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}
```

Auto-registered via `init()`. Only Postgres ships built-in.

### Middleware Registry (`internal/middleware/middleware.go`)

Auto-registered via `init()`. Factory: `func(*config.Config, *logger.Logger) (gin.HandlerFunc, error)`.

---

## Key Conventions

| Convention | Rule |
|---|---|
| **Package naming** | Services: `package modules`; Middleware: `package middleware`; Infrastructure: `package infrastructure` |
| **File naming** | `{name}_service.go`, `{name}.go`, `{name}.go` |
| **Config naming** | underscore_case matching WireName |
| **init() registration** | `RegisterService`, `RegisterMiddleware`, `RegisterComponent` |

---

## Build & Run

```bash
go mod download
go run cmd/app/main.go          # Run with config.yaml
go test ./...                   # All tests
go run ./scripts/build/         # Build binary
```

---

## Key Dependencies

| Package | Usage |
|---------|-------|
| `gin-gonic/gin` | HTTP router |
| `spf13/viper` | Config loading |
| `rs/zerolog` | Structured logging |
| `jackc/pgx` + `gorm.io/gorm` | PostgreSQL driver + ORM |
| `charmbracelet/bubbletea` | TUI framework |
| `golang-jwt/jwt/v5` | JWT auth |
| `gorilla/websocket` | WebSocket |
| `stretchr/testify` | Test assertions |
