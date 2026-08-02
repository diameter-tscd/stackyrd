# Development Guide

Learn to add services, middleware, and infrastructure components to stackyrd.

## Adding a Service

### Via Code Generator (Recommended)

```bash
./scripts/yrd service
```

Interactive prompts guide you through: service name, wire name, pattern selection (6 patterns: Basic CRUD, Read-Only, Write-Only, Event-Driven, WebSocket, Batch Processing), custom routes, GORM model, and test generation.

### Manually

Create `internal/services/modules/your_service.go`:

```go
package modules

import (
    "stackyrd/config"
    "stackyrd/pkg/interfaces"
    "stackyrd/pkg/logger"
    "stackyrd/pkg/registry"
    "stackyrd/pkg/response"
    "github.com/labstack/echo/v4"
)

type YourService struct {
    enabled bool
    logger  *logger.Logger
}

func NewYourService(enabled bool, logger *logger.Logger) *YourService {
    return &YourService{enabled: enabled, logger: logger}
}

func (s *YourService) Name() string        { return "Your Service" }
func (s *YourService) WireName() string    { return "your-service" }
func (s *YourService) Enabled() bool       { return s.enabled }
func (s *YourService) Endpoints() []string { return []string{"GET /your-api"} }
func (s *YourService) Get() interface{}    { return s }

func (s *YourService) RegisterRoutes(g *echo.Group) {
    g.GET("/your-api", s.handleGet)
}

func (s *YourService) handleGet(c echo.Context) error {
    return response.Success(c, map[string]string{"msg": "Hello"})
}

func init() {
    registry.RegisterService("your_service", func(cfg *config.Config, log *logger.Logger) interfaces.Service {
        if !cfg.Services.IsEnabled("your_service") {
            return nil
        }
        return NewYourService(true, log)
    })
}
```

Enable in `config.yaml`:
```yaml
services:
  your_service: true
```

### Service Interface

```go
type Service interface {
    Name() string                             // Human-readable name
    WireName() string                         // DI wire name
    Enabled() bool                            // Toggle
    Endpoints() []string                      // Endpoint patterns
    RegisterRoutes(g *echo.Group)             // Register routes
    Get() interface{}                         // Return underlying instance
}
```

## Adding Middleware

Create `internal/middleware/your_middleware.go`:

```go
package middleware

import (
    "stackyrd/config"
    "stackyrd/pkg/logger"
    "github.com/labstack/echo/v4"
)

func init() {
    RegisterMiddleware("your_middleware", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
        return func(next echo.HandlerFunc) echo.HandlerFunc {
            return func(c echo.Context) error {
                // before request
                if err := next(c); err != nil {
                    c.Error(err)
                }
                // after request
                return nil
            }
        }, nil
    })
}
```

Enable/disable in `config.yaml`:
```yaml
middleware:
  your_middleware: true
```

## Adding Infrastructure Components

Create `pkg/infrastructure/your_component.go`:

```go
package infrastructure

import (
    "stackyrd/config"
    "stackyrd/pkg/logger"
)

type YourComponent struct {
    enabled bool
    logger  *logger.Logger
}

func (c *YourComponent) Name() string                     { return "your_component" }
func (c *YourComponent) Close() error                     { return nil }
func (c *YourComponent) GetStatus() map[string]interface{} { return nil }

func init() {
    RegisterComponent("your_component", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
        return &YourComponent{enabled: true, logger: log}, nil
    })
}
```

Components are auto-initialized asynchronously with health polling.

## Request Validation

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
}

func (s *YourService) create(c echo.Context) error {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        if validationErr, ok := err.(*request.ValidationError); ok {
            return response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
        }
        return response.BadRequest(c, err.Error())
    }
    return response.Created(c, req)
}
```

## Using Dependencies

Services that need infrastructure components use `RegisterServiceWithDeps` and access them via typed getters on `Dependencies` (nil if the component is missing):

```go
type YourService struct {
    enabled bool
    db      *infrastructure.PostgresConnectionManager
    cache   *infrastructure.RedisManager
}

func init() {
    registry.RegisterServiceWithDeps("your_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        if !cfg.Services.IsEnabled("your_service") {
            return nil
        }
        return &YourService{enabled: true, db: deps.Postgres(), cache: deps.Redis()}
    })
}
```

Available getters: `Redis()`, `Postgres()`, `Mongo()`, `Kafka()`, `Grafana()`, `MinIO()`, `Cron()`.

The `Dependencies` container is **sealed** after boot — `Set()` is a no-op once infrastructure registration completes.

## Using the Cache

The `pkg/cache/` package provides an in-memory `cache.Cache[T]` (with a `ShardedCache[T]` variant). Instantiate it directly in a service constructor:

```go
import "stackyrd/pkg/cache"

func init() {
    registry.RegisterService("my_service", func(cfg *config.Config, log *logger.Logger) interfaces.Service {
        if !cfg.Services.IsEnabled("my_service") {
            return nil
        }
        return NewMyService(true, log, cache.New[string]())
    })
}
```

For a Redis-backed cache, access the shared connection via `deps.Redis()` in a `RegisterServiceWithDeps` factory.

## Response Helpers

```go
response.Success(c, data)                          // 200
response.Success(c, data, "message")               // 200 + message
response.SuccessWithMeta(c, data, meta)            // 200 + pagination
response.Created(c, data)                          // 201
response.NoContent(c)                              // 204
response.BadRequest(c, "msg")                      // 400
response.Unauthorized(c)                           // 401
response.Forbidden(c)                              // 403
response.NotFound(c)                               // 404
response.Conflict(c, "msg")                        // 409
response.ValidationError(c, "msg", details)        // 422
response.InternalServerError(c)                    // 500
response.ServiceUnavailable(c)                     // 503
response.Error(c, statusCode, errCode, msg)        // custom
```

## Pagination

Cursor-based pagination via `pkg/pagination/`:

```go
page := pagination.NewCursorPagination(db.Model(&User{}), 20)
result, err := page.First(10)
// result.Edges, result.PageInfo.HasNextPage, result.PageInfo.EndCursor
```

## Resilience Patterns

```go
import "stackyrd/pkg/resilience"

// Circuit breaker
cb := resilience.NewCircuitBreaker("my-service", 5, time.Minute)

// Retry with backoff
err := resilience.RetryWithBackoff(context.Background(), 3, time.Second, func() error {
    return doSomething()
})

// Timeout
ctx, cancel := resilience.WithTimeout(context.Background(), 5*time.Second)
```

## Testing

```go
import "stackyrd/pkg/testing"

func TestHandler(t *testing.T) {
    c, w := testing.NewTestContext("GET", "/api/v1/users", nil)
    handler(c)
    testing.AssertStatus(t, w, 200)
    testing.AssertJSON(t, w, map[string]interface{}{"success": true})
}
```

## Configuration Loading

Config can be loaded from a local file or remote URL:

```bash
go run cmd/app/main.go -c https://config.example.com/config.yaml
go run cmd/app/main.go -port 9090 -env production
```

FLags: `-c` (config URL), `-port`, `-verbose`, `-env`.

## Scripts Reference

| Command | Usage |
|---------|-------|
| CLI | `cd scripts && go build -o yrd .` (one-time build → `./scripts/yrd`) |
| Build | `./scripts/yrd build [-garble] [-upx]` |
| Docker | `./scripts/yrd docker` |
| Service Gen | `./scripts/yrd service` |
| Swagger Gen | `./scripts/yrd swagger [-dry-run]` |
| Package Mgr | `./scripts/yrd pkg install\|list\|remove\|upgrade` |
