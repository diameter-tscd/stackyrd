# Development Guide

Learn to add services, middleware, infrastructure components, and plugins to stackyrd.

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
    "github.com/diameter-tscd/stackyrd/config"
    "github.com/diameter-tscd/stackyrd/pkg/interfaces"
    "github.com/diameter-tscd/stackyrd/pkg/logger"
    "github.com/diameter-tscd/stackyrd/pkg/registry"
    "github.com/diameter-tscd/stackyrd/pkg/response"
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
    "github.com/diameter-tscd/stackyrd/config"
    "github.com/diameter-tscd/stackyrd/pkg/logger"
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
    "context"
    "github.com/diameter-tscd/stackyrd/config"
    "github.com/diameter-tscd/stackyrd/pkg/logger"
)

type YourComponent struct {
    enabled bool
    logger  *logger.Logger
}

func (c *YourComponent) Name() string                               { return "your_component" }
func (c *YourComponent) Close(ctx context.Context) error             { return nil }
func (c *YourComponent) GetStatus(ctx context.Context) map[string]interface{} { return nil }

func init() {
    RegisterComponent("your_component", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
        return &YourComponent{enabled: true, logger: log}, nil
    })
}
```

Components are auto-initialized asynchronously with health polling. All public methods (`Close`, `GetStatus`) accept `context.Context` for timeout and cancellation propagation.

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
    cache   *infrastructure.RedisManager
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
        var cache *infrastructure.RedisManager
        if r, ok := deps.Get("redis"); ok {
            cache = r.(*infrastructure.RedisManager)
        }
        return &YourService{enabled: true, db: db, cache: cache}
    })
}
```

## Using Plugins from Services

The `PluginBridge` is available in `deps["plugins"]`:

```go
func init() {
    registry.RegisterService("my_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        var bridge *plugin.PluginBridge
        if b, ok := deps.Get("plugins"); ok {
            bridge = b.(*plugin.PluginBridge)
        }
        return NewMyService(true, log, bridge)
    })
}

// At runtime:
if s.bridge != nil && s.bridge.HasPlugin("inspector") {
    result, err := s.bridge.Execute("inspector", map[string]interface{}{
        "mode": "ping",
    })
}
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
import "github.com/diameter-tscd/stackyrd/pkg/resilience"

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
import "github.com/diameter-tscd/stackyrd/pkg/testing"

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

## Code Quality & Linting

The project uses `golangci-lint` with project-wide config in `.golangci.yml`:

```bash
# Run all linters
golangci-lint run ./...

# Run specific linter
golangci-lint run --disable-all --enable=govet ./...
```

CI enforces: `go vet`, `go build`, `go test`, and the golangci-lint pipeline. Static analysis on `staticcheck` runs in the security workflow.

## Error Handling Conventions

Use typed `response.ErrorCode` constants instead of raw strings:

```go
response.Error(c, http.StatusBadRequest, response.ErrorBadRequest, "Invalid input")
```

All services should use `response.*` helpers (Success, BadRequest, NotFound, etc.) for consistent API response shapes.

## Scripts Reference

| Script | Usage |
|--------|-------|
| Build | `go run scripts/build/build.go [-garble] [-upx]` |
| Docker | `go run scripts/docker/docker_build.go` |
| Service Gen | `go run scripts/service/service.go` |
| Swagger Gen | `go run scripts/swagger/swagger.go [-dry-run]` |
| Package Mgr | `go run scripts/pkg/pkg.go install\|list\|remove\|upgrade` |
