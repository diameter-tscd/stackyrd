# AGENTS.md

## Project Overview

**Name:** stackyrd
**Language:** Go 1.25.3
**Module:** `github.com/diameter-tscd/stackyrd`
**Purpose:** Enterprise-grade modular service framework built on Gin for rapid, configurable, observable microservice-ready Go applications.
**Architecture:** Layered modular architecture with auto-discovered services, middleware, and infrastructure components.

---

## Directory Structure

```
stackyrd/
├── cmd/app/              # Application entry point (CLI flags, bootstrap, config loading)
│   ├── main.go
│   ├── application.go    # App lifecycle: init steps, TUI vs console mode
│   ├── config_manager.go # Config loading from file or URL
│   └── constants.go      # App constants, types, service status enums
├── config/
│   └── config.go         # Config structs, Viper setup, YAML loading
├── internal/
│   ├── middleware/        # HTTP middleware (auto-registered via init())
│   │   ├── middleware.go  # Registry, auto-discovery, core middlewares
│   │   ├── audit.go       # Audit middleware
│   │   ├── cors.go        # CORS middleware
│   │   ├── encryption.go  # Encryption middleware
│   │   ├── jwt.go         # JWT authentication middleware
│   │   ├── ratelimit.go   # Rate limiting middleware
│   │   ├── security.go    # Security headers middleware
│   │   └── swagger.go     # Swagger UI route registration
│   └── server/
│       └── server.go      # Gin server setup, health endpoints, graceful shutdown
├── internal/services/modules/  # Business logic services (auto-discovered)
│   ├── users_service.go
│   ├── products_service.go
│   ├── tasks_service.go
│   ├── broadcast_service.go
│   ├── cache_service.go
│   ├── encryption_service.go
│   ├── grafana_service.go
│   ├── mongodb_service.go
│   └── multi_tenant_service.go
├── pkg/
│   ├── interfaces/
│   │   └── service.go    # Service interface
│   ├── plugin/            # Plugin system (auto-discovered, TS/Go hybrid)
│   │   ├── plugin.go      # Plugin interface, Runtime interface, PluginMeta, Context, Result
│   │   ├── registry.go    # PluginRegistry singleton
│   │   ├── bridge.go      # PluginBridge — infra component so services can call plugins
│   │   ├── runtime.go     # goja JS runtime + injected globals
│   │   ├── runtime_registry.go  # Runtime registry — prefix-based lookup for plugin engines
│   │   ├── transpiler.go  # esbuild TS→JS + SHA256 cache
│   │   ├── sandbox.go     # Timeout + OOM enforcement
│   │   ├── store.go       # afero embed + overlay filesystem
│   │   ├── gin.go         # REST management handlers
│   │   ├── init.go        # Bootstrap: scan builtin/, instantiate, wire routes
│   │   ├── tsplugin.go    # TSScriptPlugin + tsRuntime for TS entrypoints
│   │   ├── external_runtime.go  # ExternalPlugin + externalRuntime for gRPC-based plugins (Python, etc.)
│   │   ├── embed.go       # //go:embed builtin/
│   │   ├── builtin/       # Built-in plugin manifests + scripts (TS and Python)
│   │   └── sdk/           # TypeScript type declarations
│   ├── registry/
│   │   ├── registry.go              # Service factory registry, auto-discovery
│   │   ├── dependencies.go          # Generic DI container (Dependencies)
│   │   └── service_helper.go        # Dependency validation helper
│   ├── infrastructure/   # Infrastructure components (auto-registered via init())
│   │   ├── component.go   # InfrastructureComponent interface + ComponentFactory
│   │   ├── registry.go            # ComponentRegistry singleton
│   │   ├── async_init.go          # Async infra init manager with health checks
│   │   ├── afero.go               # Virtual filesystem abstraction (spf13/afero)
│   │   ├── async.go               # Generic async result/batch utilities
│   │   ├── cron_manager.go        # Cron scheduler wrapper (robfig/cron)
│   │   ├── grafana.go             # Grafana API client
│   │   ├── kafka.go               # Kafka producer/consumer (IBM/sarama)
│   │   ├── minio.go               # MinIO S3-compatible storage client
│   │   ├── mongo.go               # MongoDB driver with multi-connection support
│   │   ├── postgres.go            # PostgreSQL raw SQL + GORM, multi-connection
│   │   └── redis.go               # Redis sync/async/batch client
│   ├── logger/                         # Structured logger (zerolog-based)
│   ├── response/                       # Standard API response helpers
│   ├── request/                        # Request binding and validation helpers
│   ├── tui/                            # Terminal UI (bubbletea + lipgloss)
│   ├── metrics/                        # Prometheus metrics
│   ├── pagination/                     # Cursor-based pagination
│   ├── caching/                        # Redis-backed cache abstraction
│   ├── batch/                          # Batch processing utilities
│   ├── logging/                        # Log rotation, sampling, structured helpers
│   ├── resilience/                     # Circuit breaker, health checks, retry, timeout
│   ├── testing/                        # Test helpers and mocks
│   ├── utils/                          # General utilities (system, http, io, date, numeric, strings, image, params, broadcast)
│   ├── webhook/                        # Webhook handler
│   └── websocket/                      # WebSocket handler
├── scripts/
│   ├── build/build.go          # Build script (garble, backup, archiving)
│   ├── docker/docker_build.go  # Docker build helper
│   ├── pkg/pkg.go              # Infrastructure package installer
│   ├── swagger/swagger.go      # Swagger doc generator
│   └── service/                # Service code generator (6 patterns)
├── tests/
│   ├── services/               # Service integration tests
│   └── infrastructure/         # Infrastructure unit tests
├── docs/                       # Auto-generated Swagger docs
├── docs_wiki/                  # Full project documentation
├── deployments/kubernetes/     # K8s deployment manifest
├── config.yaml                 # Main YAML configuration
├── docker-compose.yaml         # Full dev environment stack
├── Dockerfile                  # Multi-stage build
├── go.mod / go.sum
└── .github/workflows/
    ├── go-build.yml            # CI: build + test
    └── security.yml            # CI: gosec, nancy, govulncheck, trivy, staticcheck, gocritic
```

---

## Core Abstractions

### Service Interface (`pkg/interfaces/service.go`)

All business-logic modules implement this interface:

```go
type Service interface {
    Name() string          // Human-readable name
    WireName() string      // Wire name for DI container
    Enabled() bool         // Enabled/disabled toggle
    Endpoints() []string   // HTTP endpoint patterns handled
    RegisterRoutes(g *gin.RouterGroup)
    Get() interface{}      // Underlying instance
}
```

Services are **auto-discovered** by the registry and registered with Gin's router group under `/api/v1`. Enable/disable via `services:` section in `config.yaml`. Individual service files live in `internal/services/modules/`.

### InfrastructureComponent Interface (`pkg/infrastructure/component.go`)

All infrastructure modules implement this interface:

```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}
```

Components are **auto-registered** via `init()` in their respective files under `pkg/infrastructure/`. Managed by a singleton `ComponentRegistry` with async init and health-check support.

### Middleware Registry (`internal/middleware/middleware.go`)

Middleware is **auto-registered** via `init()` calls. Each middleware is a `MiddlewareFactory` — `func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error)`. Enable/disable via `middleware:` section in `config.yaml`. The registry singleton is accessed via `GetGlobalMiddlewareRegistry()`.

---

## Key Conventions

### Package Structure
- `cmd/app/` — binary entry point only. CLI parsing, bootstrap orchestration.
- `internal/` — Go-internal unexported packages. HTTP handlers, server lifecycle, middleware.
- `pkg/` — exported, library-quality packages. Interfaces, registry, infrastructure, utilities.
- `scripts/` — Go-based CLI tools for build, docker, packaging, service generation.
- `tests/` — flat test tree mirroring source structure (`tests/services/`, `tests/infrastructure/`).

### Naming
- Service files: `{name}_service.go` (e.g. `users_service.go`).
- Infrastructure files: `{name}.go` (e.g. `mongo.go`, `kafka.go`).
- Test files: `{package}_test.go`.

### Configuration
- **Single source of truth:** `config.yaml` at repo root.
- Loaded via **Viper** (`spf13/viper`) — supports YAML file + env var overrides.
- Config struct lives in `config/config.go` with typed sections: `App`, `Server`, `Services`, `Middleware`, `Auth`, `Redis`, `Kafka`, `Postgres` (multi-connection), `Mongo` (multi-connection), `Grafana`, `Minio`, `Cron`, `Encryption`.
- **Never hardcode secrets in config.yaml** — use env vars in production.

### Auto-Registration Pattern
Both middleware and infrastructure components use Go's `init()` function for self-registration. When adding a new:
1. Implement the relevant interface.
2. Export the file under the correct package.
3. Declare `func init() { Register(...) }` in that file.
4. Add toggle to `config.yaml` if applicable.

### TUI vs Console
- Set `app.enable_tui` in config.yaml to switch.
- TUI code lives in `pkg/tui/` (bubbletea splash screen, live dashboard, charts, log broadcast).
- Console fallback: `pkg/tui/simple.go`.

---

## Build & Run

### Prerequisites
- Go 1.25.3
- Docker + Docker Compose (for dev environment)
- Optional: `garble` (`mvdan.cc/garble@latest`) for obfuscated builds

### Local Development
```bash
go mod download              # Install dependencies
go run cmd/app/main.go       # Run with config.yaml in CWD
go test ./...                # Run all tests
```

Using the build script:
```bash
go run scripts/build/build.go
```

### Docker Compose (full dev environment)
```bash
docker-compose up
```
Stack includes: Redis, PostgreSQL, Kafka, MongoDB, Grafana, MinIO, and the stackyrd app.

### Testing
```bash
go test ./...                # All tests
go test -v ./tests/...       # Verbose test output
go test -v ./pkg/testing/... # Run test helpers
```
- Test framework: `testify` (assertions) + `httptest` + Gin test mode.
- Helper library: `pkg/testing/helpers.go` — `NewTestContext`, `AssertStatus`, `AssertJSON`, `ParseResponse`.
- CI: `go test -v ./...`

### CI Pipeline
- **`go-build.yml`**: checkout → `setup-go` → `go mod tidy` → build → `go test -v ./...`
- **`security.yml`**: `gosec`, `nancy`, `govulncheck`, `trivy`, `staticcheck`, `gocritic`

---

## Key Dependencies

| Package | Usage |
|--------|-------|
| `github.com/gin-gonic/gin` | HTTP router & middleware chain |
| `github.com/spf13/viper` | Configuration loading |
| `github.com/rs/zerolog` | Structured JSON logging |
| `github.com/IBM/sarama` | Kafka client |
| `github.com/redis/go-redis/v9` | Redis client |
| `gorm.io/gorm` + `go.mongodb.org/mongo-driver` | DB drivers |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/prometheus/client_golang` | Prometheus metrics |
| `github.com/golang-jwt/jwt/v5` | JWT token auth |
| `github.com/swaggo/swag` + `gin-swagger` | OpenAPI/Swagger docs |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/robfig/cron/v3` | Cron scheduler |
| `spf13/afero` | Virtual filesystem abstraction |
| `github.com/gorilla/websocket` | WebSocket support |
| `github.com/dop251/goja` | JavaScript runtime (plugin execution) |
| `github.com/evanw/esbuild` | In-process TypeScript → JS transpiler |
| `google.golang.org/grpc` | gRPC framework (external plugin communication) |
| `google.golang.org/protobuf` | Protobuf runtime (external plugins) |

---

## Plugin System (`pkg/plugin/`)

The plugin system is a self-contained subsystem that loads TypeScript, Go, or external-language (Python, etc.) plugins from embedded `builtin/` directories. TypeScript plugins are transpiled via esbuild and executed in sandboxed goja VMs. External plugins (prefix `ext:`) are run as subprocesses communicating via gRPC. Plugins access infrastructure components via `$infra.get(name)` and communicate results via `$done()` (TS) or return values (Python).

### Key files

| File | Purpose |
|------|---------|
| `plugin.go` | `Plugin` interface, `Runtime` interface, `PluginMeta`, `ResourceLimits`, `Context`, `Result` |
| `registry.go` | `PluginRegistry` singleton — factory/meta/filesystem maps |
| `bridge.go` | `PluginBridge` — `InfrastructureComponent` so services/infra can call plugins |
| `sandbox.go` | Timeout enforcement, RSS memory monitor (gopsutil), panic recovery |
| `transpiler.go` | SHA256-cached esbuild TS→JS transpilation |
| `runtime.go` | goja VM: fresh per call, injects `$args`, `$logger`, `$infra`, `$limits`, `$done` |
| `runtime_registry.go` | `Runtime` registry — prefix-based lookup for plugin execution engines |
| `tsplugin.go` | `TSScriptPlugin` + `tsRuntime` for TS entrypoints |
| `external_runtime.go` | `ExternalPlugin` + `externalRuntime` for gRPC-based plugins (Python, etc.) |
| `store.go` | `embed.FS` → afero `CopyOnWriteFs` adapter (read-only base + writable overlay) |
| `gin.go` | 7 REST handlers: list, get, execute, upload, listScripts, getScript, unload |
| `init.go` | Bootstrap: scan builtin/, instantiate, wire routes |

### PluginBridge — cross-interaction bridge

`PluginBridge` wraps `PluginRegistry` and implements `InfrastructureComponent`, making it discoverable by both infrastructure components and services:

```go
type PluginBridge struct { ... }

// InfrastructureComponent implementation
func (b *PluginBridge) Name() string    // returns "plugins"
func (b *PluginBridge) Close() error
func (b *PluginBridge) GetStatus() map[string]interface{}  // all plugins + status

// Public API for services & infra components
func (b *PluginBridge) HasPlugin(name string) bool
func (b *PluginBridge) GetMeta(name string) (PluginMeta, bool)
func (b *PluginBridge) Execute(name string, args map[string]interface{}) (*Result, error)
func (b *PluginBridge) ListPlugins() []PluginSummary
```

### How services and infra components interact with plugins

**From a service** — the `PluginBridge` is available in the `Dependencies` bag as `"plugins"`:

```go
// In the service factory (init() function):
registry.RegisterService("my_service", func(cfg *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
    var bridge *plugin.PluginBridge
    if b, ok := deps.Get("plugins"); ok {
        bridge, _ = b.(*plugin.PluginBridge)
    }
    return NewMyService(cfg.Services.IsEnabled("my_service"), logger, bridge)
})

// Then at runtime in the service:
func (s *MyService) handleStatus(c *gin.Context) {
    if s.bridge != nil && s.bridge.HasPlugin("inspector") {
        result, err := s.bridge.Execute("inspector", map[string]interface{}{
            "mode": "ping",
        })
        // ...
    }
}
```

**From an infrastructure component** — access via the global `ComponentRegistry`:

```go
reg := infrastructure.GetGlobalRegistry()
if comp, ok := reg.Get("plugins"); ok {
    if bridge, ok := comp.(*plugin.PluginBridge); ok {
        plugins := bridge.ListPlugins()
    }
}
```

**Convenience accessor** — for any Go code that has access to the logger:

```go
bridge := plugin.GetGlobalPluginBridge()
if bridge != nil && bridge.HasPlugin("inspector") {
    // ...
}
```

### Boot order

Plugin init happens **before** service auto-discovery in `server.go`:

```
Infrastructure async init → populate Dependencies → PLUGIN INIT → bridge→Set("plugins") → Middleware → AutoDiscoverServices (plugins available in deps)
```

### Adding a plugin

**TypeScript plugin:**
1. Create `pkg/plugin/builtin/{name}/plugin.yaml` with manifest, entrypoint `"ts:scripts/handler.ts"`
2. Create `pkg/plugin/builtin/{name}/scripts/handler.ts` using `$infra.get(name)`, `$logger.*`, `$done()` globals
3. See `pkg/plugin/sdk/plugin.d.ts` for TypeScript type declarations

**External language plugin (Python, etc. via gRPC):**
1. Write a Python script implementing a class with an `execute(self, args)` method
2. Create `pkg/plugin/builtin/{name}/plugin.yaml` with entrypoint `"ext:scripts/handler.py"`
3. The python host (`pkg/plugin/python/host.py`) loads the script and serves it via gRPC
4. See `pkg/plugin/python/sdk.py` for the base `Plugin` class

**Go plugin:**
1. Create a flat `.go` file in `pkg/plugin/` implementing the `Plugin` interface with `init()` registration
2. Create `pkg/plugin/builtin/{name}/plugin.yaml` with entrypoint `"go:FuncName"`

---

## Adding a New Service

1. Create `internal/services/modules/{name}_service.go` implementing `interfaces.Service` interface.
2. Add `{name}_service: true/false` to `services:` in `config.yaml`.
3. Optionally write tests in `tests/services/{name}_service_test.go` using `pkg/testing/helpers.go`.
4. The service registry (`pkg/registry/registry.go`) will auto-discover it via `AutoDiscoverServices`.

## Adding New Middleware

1. Create `internal/middleware/{name}.go` with an `init()` that calls `RegisterMiddleware("name", factory)`.
2. Add `{name}: true/false` to `middleware:` in `config.yaml`.

## Adding Infrastructure Components

1. Create a file under `pkg/infrastructure/{name}.go` implementing `InfrastructureComponent`.
2. Register via `init()` calling `RegisterComponent("name", factory)`.
3. Components are initialized async with health-check polling; results appear in TUI dashboard.

---

## Module Path

Module path: `stackyrd` (internal to this repo). All Go import paths are relative to the module root. Binary output goes to `dist/` (git-ignored).

---

## Targets you should never commit

- `config.yaml` with real secrets (rotate secrets; use env vars in production).
- `dist/` directory.
- `.env` or any file containing credentials.
