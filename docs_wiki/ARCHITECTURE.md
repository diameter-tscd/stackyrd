# Architecture Overview

**stackyrd-nano** is a lightweight modular Go framework built on **Gin** with auto-discovery patterns, async infrastructure initialization, TUI dashboard, and Postgres infrastructure.

## Key Concepts

### Auto-Discovery Pattern
Components register themselves via `init()` functions at import time:

- **Services**: Business logic in `internal/services/modules/`
- **Middleware**: HTTP middleware in `internal/middleware/`
- **Infrastructure**: Database/clients in `pkg/infrastructure/`

### Boot Sequence

```mermaid
flowchart TD
    A[parseFlags] --> B[ConfigManager]
    B --> C[Application.Run]
    C --> D[loadConfig<br/>Viper YAML + env vars]
    C --> E[validateConfig]
    C --> F[loadBanner]
    C --> G[checkPort]
    C --> H[initLogger<br/>zerolog]
    C --> I{startApp}
    I -->|TUI mode| J[RunBootSequence]
    J --> K[LiveTUI]
    I -->|Console mode| L[direct logs]
    L --> M[server.New]
    M --> N[async infra init<br/>all components in parallel]
    M --> O[middleware auto-discovery]
    M --> P[service auto-discovery]
    M --> Q[route registration<br/>/api/v1]
```

### Request Flow

```mermaid
flowchart LR
    A[Client] --> B[Middleware Chain]
    B --> C[Service Handler]
    C --> D[Response]
    C --> E[Infrastructure<br/>PostgreSQL]
```

### TUI vs Console Mode
- Set `app.enable_tui: true` in config.yaml for bubbletea TUI (boot animation, live dashboard, log viewer, charts)
- Default (`false`) uses traditional console logging with banner

## Project Structure

```mermaid
flowchart TD
    A[stackyrd-nano/]
    A --> B[cmd/app/<br/>Entry point, CLI, bootstrap]
    A --> C[config/<br/>Viper YAML config loading]
    A --> D[config.yaml]
    A --> E[internal/]
    E --> F[middleware/<br/>Auto-registered HTTP middleware]
    E --> G[server/<br/>Gin setup, health endpoints]
    A --> H[pkg/]
    H --> I[assets/<br/>Embedded application assets (banner.txt)]
    H --> J[interfaces/<br/>Service interface]
    H --> K[registry/<br/>Service registry + DI]
    H --> L[infrastructure/<br/>DB/clients async-managed]
    H --> M[response/<br/>API response helpers]
    H --> N[request/<br/>Binding + validation]
    H --> O[logger/<br/>Zerolog structured logger]
    H --> P[tui/<br/>Bubbletea terminal UI]
    H --> Q[cache/<br/>In-memory generic cache]
    H --> R[pagination/<br/>Cursor-based pagination]
    H --> S[batch/<br/>Batch processing]
    H --> T[resilience/<br/>Circuit breaker, health, retry]
    H --> U[webhook/<br/>Webhook handler]
    H --> V[websocket/<br/>WebSocket handler]
    H --> W[logging/<br/>Log rotation, sampling]
    H --> X[testing/<br/>Test helpers + mocks]
    H --> Y[utils/<br/>General utilities]
    A --> Z[scripts/<br/>CLI tools]
    A --> AA[tests/<br/>Integration tests]
    A --> AB[docs_wiki/<br/>Hand-written documentation]
    A --> AC[.github/workflows/<br/>CI]
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
    infrastructure.RegisterComponent("postgres", func(cfg *config.Config, log *logger.Logger) (infrastructure.InfrastructureComponent, error) {
        return NewPostgresManager(cfg)
    })
}
```

Components are initialized **asynchronously** by `InfraInitManager` with per-component health polling.

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
- **PostgreSQL**: Multi-connection Postgres manager with GORM
- **TUI Dashboard**: Bubbletea boot sequence + live monitoring dashboard
- **Resilience**: Circuit breaker, retry with backoff, health checks, timeouts
- **Pagination**: Cursor-based with base64-encoded cursors
- **Scripts**: Build, Docker, code generation, package management tools
