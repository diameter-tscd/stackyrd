# Development Guide

Add new components by creating files and registering via `init()`.

## Service

Create `internal/services/modules/my_service.go`:

```go
package modules

import (
    "stackyrd/config"
    "stackyrd/pkg/interfaces"
    "stackyrd/pkg/logger"
    "stackyrd/pkg/registry"
    "github.com/labstack/echo/v4"
)

type MyService struct{ enabled bool; log *logger.Logger }

func NewMyService(e bool, l *logger.Logger) *MyService {
    return &MyService{enabled: e, log: l}
}

func (s *MyService) Name() string          { return "My Service" }
func (s *MyService) Enabled() bool         { return s.enabled }
func (s *MyService) RegisterRoutes(g *echo.Group) {
    g.GET("/my", s.handle)
}
func (s *MyService) Get() any              { return s }

func init() {
    registry.RegisterService("my_service", func(cfg *config.Config, l *logger.Logger) interfaces.Service {
        if !cfg.Services.IsEnabled("my_service") { return nil }
        return NewMyService(true, l)
    })
}
```

Enable in `config.yaml`:

```yaml
services:
  my_service: true
```

## Middleware

Create `internal/middleware/my_mw.go`:

```go
package middleware

import (
    "stackyrd/config"
    "stackyrd/pkg/logger"
    "github.com/labstack/echo/v4"
)

func init() {
    RegisterMiddleware("my_mw", func(cfg *config.Config, log *logger.Logger) (echo.MiddlewareFunc, error) {
        return func(next echo.HandlerFunc) echo.HandlerFunc {
            return func(c echo.Context) error {
                return next(c)
            }
        }, nil
    })
}
```

## Infrastructure Component

Create `pkg/infrastructure/my_component.go`:

```go
package infrastructure

import (
    "stackyrd/config"
    "stackyrd/pkg/logger"
)

type MyComponent struct{ enabled bool }

func NewMyComponent(cfg *config.Config) *MyComponent { return &MyComponent{} }

func (c *MyComponent) Name() string            { return "my_component" }
func (c *MyComponent) Close() error            { return nil }
func (c *MyComponent) GetStatus() map[string]any { return nil }

func init() {
    RegisterComponent("my_component", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
        return NewMyComponent(cfg), nil
    })
}
```

Enable in config:

```yaml
infrastructure:
  my_component: true
```

## DI with Dependencies

```go
registry.RegisterServiceWithDeps("my_service", func(cfg *config.Config, l *logger.Logger, deps *registry.Dependencies) interfaces.Service {
    return &MyService{enabled: true, db: deps.Postgres()}
})
```

Available getters: `Redis()`, `Postgres()`, `Mongo()`, `Kafka()`, `Grafana()`, `MinIO()`, `Cron()`.

## Response Helpers

```go
response.Success(c, data)
response.Created(c, data)
response.BadRequest(c, "msg")
response.Error(c, 400, "VALIDATION_ERROR", "msg")
```

## Testing Helpers

```go
c, rec := testing.NewTestContext("GET", "/api/v1/items", nil)
handler(c)
testing.AssertStatus(t, rec, 200)
testing.AssertJSON(t, rec, map[string]any{"success": true})
```

See `pkg/testing/helpers.go` for full list.