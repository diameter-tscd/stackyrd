# Architecture Overview

**stackyrd** is an enterprise-grade modular Go framework built on **Gin** with auto-discovery patterns, async infrastructure initialization, a plugin system (TS/Go/Python/Lua), TUI dashboard, and Prometheus metrics.

## Key Concepts

### Auto-Discovery Pattern
Components register themselves via `init()` functions at import time:

- **Services**: Business logic in `internal/services/modules/`
- **Middleware**: HTTP middleware in `internal/middleware/`
- **Infrastructure**: Database/clients in `pkg/infrastructure/`
- **Plugins**: TypeScript/Go/Lua/Python scripts in `pkg/plugin/builtin/`

### Boot Sequence

```
parseFlags → ConfigManager → Application.Run
    │
    ├── loadConfig (Viper YAML + env vars)
    ├── validateConfig
    ├── loadBanner
    ├── checkPort
    ├── initLogger (zerolog)
    │
    └── startApp
         │
         ├── TUI mode: RunBootSequence → LiveTUI
         └── Console mode: direct logs
              │
              └── server.New()
                   │
                   ├── async infra init (all components in parallel)
                   ├── plugin init (bridge → deps["plugins"])
                   ├── middleware auto-discovery
                   ├── service auto-discovery
                   ├── route registration (/api/v1)
                   └── Swagger UI (if enabled)
```

### Request Flow
```
Client → Middleware Chain → Service Handler → Response
                ↓
         Plugin Bridge (optional)
                ↓
         Infrastructure (DB, Cache, Kafka, MinIO, ...)
```

### TUI vs Console Mode
- Set `app.enable_tui: true` in config.yaml for bubbletea TUI (boot animation, live dashboard, log viewer, charts)
- Default (`false`) uses traditional console logging with banner

## Project Structure
```
stackyrd/
├── cmd/app/                        # Entry point, CLI flags, bootstrap
├── config/                         # Viper YAML config loading
├── config.yaml                     # Single source of truth config
├── internal/
│   ├── middleware/                  # Auto-registered HTTP middleware
│   └── server/                     # Gin server setup, health endpoints
├── internal/services/modules/       # Auto-discovered business services
├── pkg/
│   ├── interfaces/                 # Service interface
│   ├── plugin/                     # Plugin system (TS→goja, external gRPC)
│   │   ├── builtin/                # Built-in plugin manifests
│   │   ├── sdk/                    # TS type declarations
│   │   └── python/                 # Python host runtime
│   ├── registry/                   # Service factory registry + DI container
│   ├── infrastructure/             # DB/clients (async-managed)
│   ├── response/                   # Standard API response helpers
│   ├── request/                    # Request binding + validation
│   ├── logger/                     # Zerolog structured logger
│   ├── tui/                        # Bubbletea terminal UI
│   ├── metrics/                    # Prometheus metrics
│   ├── pagination/                 # Cursor-based pagination
│   ├── batch/                      # Batch processing utilities
│   ├── resilience/                 # Circuit breaker, health, retry, timeout
│   ├── webhook/                    # Webhook handler
│   ├── websocket/                  # WebSocket handler
│   ├── logging/                    # Log rotation, sampling, structured
│   ├── testing/                    # Test helpers and mocks
│   └── utils/                      # General utilities
├── scripts/                        # CLI tools (build, docker, service, swagger, pkg)
├── tests/                          # Integration tests
├── docs/                           # Auto-generated Swagger docs
├── docs_wiki/                      # Hand-written project documentation
├── deployments/kubernetes/          # K8s deployment manifests
├── .github/workflows/               # CI (build+test, security scanning)
└── docker-compose.yaml              # Full dev environment
```

## Service Pattern
```go
type Service interface {
    Name() string
    WireName() string
    Enabled() bool
    Endpoints() []string
    RegisterRoutes(*gin.RouterGroup)
    Get() interface{}
}

// Auto-registration with dependency injection
func init() {
    registry.RegisterService("service_name", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        helper := registry.NewServiceHelper(cfg, log, deps)
        if !helper.IsServiceEnabled("service_name") {
            return nil
        }
        return NewService(true, log)
    })
}
```

## Infrastructure Component Pattern
```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}

type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)

// Auto-registration
func init() {
    infrastructure.RegisterComponent("redis", func(cfg *config.Config, log *logger.Logger) (infrastructure.InfrastructureComponent, error) {
        return NewRedisManager(cfg)
    })
}
```

Components are initialized **asynchronously** by `InfraInitManager` with per-component health polling.

## Plugin System
```
PluginRegistry (singleton)
    ├── RuntimeRegistry (prefix-based: "ts:" → goja, "ext:" → gRPC, "go:" → native)
    ├── PluginBridge (InfrastructureComponent → available in deps["plugins"])
    ├── Transpiler (esbuild TS→JS with SHA256 cache)
    ├── Sandbox (timeout + RSS memory enforcement)
    ├── Filesystem (embed.FS + CopyOnWriteFs overlay)
    └── REST API (/api/v1/plugins/*)
```

## Middleware Pattern
```go
type MiddlewareFactory func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error)

func init() {
    middleware.RegisterMiddleware("cors", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
        return corsMiddleware, nil
    })
}
```

## Key Features
- **Dependency Injection**: Dynamic `Dependencies` container with TTL-cached GetAll()
- **Async Initialization**: All infrastructure components init in parallel
- **Multi-connection DB**: Postgres + MongoDB with named connection managers
- **Plugin System**: TypeScript (goja), Python/external (gRPC), Lua, Go
- **TUI Dashboard**: Bubbletea boot sequence + live monitoring dashboard
- **Prometheus Metrics**: HTTP, DB, cache, circuit breaker, webhook, batch, WebSocket
- **Resilience**: Circuit breaker, retry with backoff, health checks, timeouts
- **Pagination**: Cursor-based with base64-encoded cursors
- **Scripts**: Build, Docker, code generation, Swagger, package management tools
