# Development Guide

Learn to add services, middleware, infrastructure components to stackyrd-nano.

## Adding a Service

### Via Code Generator (Recommended)

```bash
go run scripts/service/service.go
```

Interactive prompts guide you through: service name, wire name, pattern selection (6 patterns: Basic CRUD, Read-Only, Write-Only, Event-Driven, WebSocket, Batch Processing), custom routes, GORM model, and test generation.

### Manually

Create `internal/services/modules/your_service.go`:

```go
package modules

import (
    "stackyrd-nano/config"
    "stackyrd-nano/pkg/interfaces"
    "stackyrd-nano/pkg/logger"
    "stackyrd-nano/pkg/registry"
    "stackyrd-nano/pkg/response"
    "github.com/gin-gonic/gin"
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

func (s *YourService) RegisterRoutes(g *gin.RouterGroup) {
    g.GET("/your-api", s.handleGet)
}

func (s *YourService) handleGet(c *gin.Context) {
    response.Success(c, map[string]string{"msg": "Hello"})
}

func init() {
    registry.RegisterService("your_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        helper := registry.NewServiceHelper(cfg, log, deps)
        if !helper.IsServiceEnabled("your_service") {
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
    RegisterRoutes(g *gin.RouterGroup)        // Register routes
    Get() interface{}                         // Return underlying instance
}
```

## Adding Middleware

Create `internal/middleware/your_middleware.go`:

```go
package middleware

import (
    "stackyrd-nano/config"
    "stackyrd-nano/pkg/logger"
    "github.com/gin-gonic/gin"
)

func init() {
    RegisterMiddleware("your_middleware", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
        return func(c *gin.Context) {
            // before request
            c.Next()
            // after request
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
    "stackyrd-nano/config"
    "stackyrd-nano/pkg/logger"
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

func (s *YourService) create(c *gin.Context) {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        if validationErr, ok := err.(*request.ValidationError); ok {
            response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
            return
        }
        response.BadRequest(c, err.Error())
        return
    }
    response.Created(c, req)
}
```

## Using Dependencies

Services receive infrastructure components via the `Dependencies` container:

```go
type YourService struct {
    enabled bool
    db      *infrastructure.PostgresConnectionManager
}

func init() {
    registry.RegisterService("your_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        if !cfg.Services.IsEnabled("your_service") {
            return nil
        }
        var db *infrastructure.PostgresConnectionManager
        if d, ok := deps.Get("postgres"); ok {
            db = d.(*infrastructure.PostgresConnectionManager)
        }
        return &YourService{enabled: true, db: db}
    })
}
```

## Using the In-Memory Cache

The `pkg/cache/` package provides a generic in-memory cache:

```go
import "stackyrd-nano/pkg/cache"

// Create a cache with default TTL
c := cache.NewCache[string, User](5 * time.Minute)

// Set a value
c.Set("user:123", userData)

// Get a value
user, ok := c.Get("user:123")

// Delete
c.Delete("user:123")

// Clear all entries
c.Clear()
```

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
import "stackyrd-nano/pkg/resilience"

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
import "stackyrd-nano/pkg/testing"

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

| Script | Usage |
|--------|-------|
| Build | `go run scripts/build/build.go [-garble] [-upx]` |
| Docker | `go run scripts/docker/docker_build.go` |
| Service Gen | `go run scripts/service/service.go` |
| Package Mgr | `go run scripts/pkg/pkg.go install|list|remove|upgrade` |
