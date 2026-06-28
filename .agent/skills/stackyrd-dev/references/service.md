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
    registry.RegisterService("thing_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
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

```go
func init() {
    registry.RegisterService("thing_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := &ThingService{cfg: cfg, log: log}
        if comp, ok := deps.Get("redis"); ok {
            svc.redis = comp.(*infrastructure.RedisManager)
        }
        return svc
    })
}
```

## Testing

Write tests in `tests/services/{name}_service_test.go`. Use `echo.New()` and `httptest` to build a router. See `tests/services/users_service_test.go` for the canonical pattern.

## Patterns

- `users_service.go` — full CRUD with validation, pagination, sync.Map
- `products_service.go` — read-only (minimal template)
- `tasks_service.go` — event-driven with PluginBridge access
- All handlers are `func(echo.Context) error` — use `request.Bind()` for body binding
- Route middleware is applied on the `sub` group inside `RegisterRoutes`
