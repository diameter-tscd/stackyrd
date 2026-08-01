# Adding a Service

Create `internal/services/modules/{name}_service.go` (package `modules`). Implement `interfaces.Service`, register via `init()`.

## Interface

```go
type Service interface {
    Name() string
    WireName() string
    Enabled() bool
    Endpoints() []string
    RegisterRoutes(g *echo.Group)
    Get() interface{}
}
```

## Skeleton

Services that don't need infrastructure use `RegisterService` with a 2-arg factory:

```go
package modules

import (
    "github.com/labstack/echo/v4"
    "stackyrd/config"
    "stackyrd/pkg/interfaces"
    "stackyrd/pkg/logger"
    "stackyrd/pkg/registry"
    "stackyrd/pkg/response"
)

type ThingService struct { cfg *config.Config; log *logger.Logger }

func (s *ThingService) Name() string     { return "Thing Service" }
func (s *ThingService) WireName() string { return "thing" }
func (s *ThingService) Enabled() bool    { return s.cfg.Services.IsEnabled("thing_service") }
func (s *ThingService) Endpoints() []string { return []string{"/thing", "/thing/:id"} }
func (s *ThingService) Get() interface{} { return s }

func (s *ThingService) RegisterRoutes(g *echo.Group) {
    sub := g.Group("/thing")
    sub.GET("", s.list)
    sub.GET("/:id", s.get)
    sub.POST("", s.create)
    sub.PUT("/:id", s.update)
    sub.DELETE("/:id", s.delete)
}

func (s *ThingService) list(c echo.Context) error   { return response.Success(c, nil, "list") }
func (s *ThingService) get(c echo.Context) error    { return response.Success(c, nil, "get") }
func (s *ThingService) create(c echo.Context) error { return response.Created(c, nil, "created") }
func (s *ThingService) update(c echo.Context) error { return response.Success(c, nil, "updated") }
func (s *ThingService) delete(c echo.Context) error { return response.Success(c, nil, "deleted") }

func init() {
    registry.RegisterService("thing_service", func(cfg *config.Config, log *logger.Logger) interfaces.Service {
        return &ThingService{cfg: cfg, log: log}
    })
}
```

## Config

```yaml
services:
  thing_service: true
```

## Accessing Infrastructure

Services that consume infrastructure use `RegisterServiceWithDeps` and read
**typed getters** (each returns `*T` or `nil`):

```go
func init() {
    registry.RegisterServiceWithDeps("thing_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := &ThingService{cfg: cfg, log: log}
        svc.redis = deps.Redis()      // *infrastructure.RedisManager or nil
        svc.pg = deps.Postgres()      // *infrastructure.PostgresConnectionManager or nil
        return svc
    })
}
```

Typed getters: `Redis()`, `Postgres()`, `Mongo()`, `Kafka()`, `Grafana()`,
`MinIO()`, `Cron()`. Postgres services then pick a connection via
`deps.Postgres().GetDefaultConnection()` or `GetConnection(name)`.

The `Dependencies` container is sealed after boot — services may only read it,
never write. `registry.GetService(name)` returns the running service instance.

## Testing

Write tests in `tests/services/{name}_service_test.go`. Use `echo.New()` and `httptest` to build a router. See `tests/services/users_service_test.go` for the canonical pattern.

## Patterns

- `users_service.go` — full CRUD with validation, pagination, sync.Map
- `products_service.go` — read-only (minimal template)
- `tasks_service.go` — Postgres-backed (uses `RegisterServiceWithDeps` + `deps.Postgres()`)
- `multi_tenant_service.go` / `mongodb_service.go` — per-tenant connections via `deps.Postgres()` / `deps.Mongo()`
- All handlers are `func(echo.Context) error` — use `request.Bind()` for body binding
- Route middleware is applied on the `sub` group inside `RegisterRoutes`
- Error handling: log the technical error with `s.logger.Error(...)`, return a generic message to the client (never `err.Error()`)
