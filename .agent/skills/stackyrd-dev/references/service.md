# Adding a Service

Services are the primary way to add business logic and HTTP endpoints to stackyrd. They follow the **auto-registration** pattern — create a file in `internal/services/modules/`, implement the interface, register via `init()`, and toggle via `config.yaml`.

## Interface Requirements

Every service must implement `interfaces.Service` (`pkg/interfaces/service.go`):

```go
type Service interface {
    Name() string              // Human-readable display name
    WireName() string          // DI wire name (snake_case, used for config key lookup)
    Enabled() bool             // Config-driven toggle
    Endpoints() []string       // HTTP endpoint patterns this service handles
    RegisterRoutes(g *gin.RouterGroup)  // Register routes on the API group
    Get() interface{}          // Return the underlying instance
}
```

## File Template

Create `internal/services/modules/{name}_service.go`:

```go
package modules

import (
    "stackyrd/config"
    "stackyrd/pkg/interfaces"
    "stackyrd/pkg/logger"
    "stackyrd/pkg/registry"
    "stackyrd/pkg/response"

    "github.com/gin-gonic/gin"
)

type {Name}Service struct {
    enabled bool
    logger  *logger.Logger
}

func New{Name}Service(enabled bool, logger *logger.Logger) *{Name}Service {
    return &{Name}Service{
        enabled: enabled,
        logger:  logger,
    }
}

func (s *{Name}Service) Name() string {
    return "{Name} Service"
}

func (s *{Name}Service) WireName() string {
    return "{wire_name}"
}

func (s *{Name}Service) Enabled() bool {
    return s.enabled
}

func (s *{Name}Service) Endpoints() []string {
    return []string{
        "/{endpoint}",
        "/{endpoint}/:id",
    }
}

func (s *{Name}Service) Get() interface{} {
    return s
}

func (s *{Name}Service) RegisterRoutes(g *gin.RouterGroup) {
    sub := g.Group("/{endpoint}")
    {
        sub.GET("", s.list)
        sub.GET("/:id", s.get)
        sub.POST("", s.create)
        sub.PUT("/:id", s.update)
        sub.DELETE("/:id", s.delete)
    }
}

func (s *{Name}Service) list(c *gin.Context) {
    response.Success(c, nil, "List endpoint")
}

func (s *{Name}Service) get(c *gin.Context) {
    response.Success(c, nil, "Get endpoint")
}

func (s *{Name}Service) create(c *gin.Context) {
    response.Created(c, nil, "Created")
}

func (s *{Name}Service) update(c *gin.Context) {
    response.Success(c, nil, "Updated")
}

func (s *{Name}Service) delete(c *gin.Context) {
    response.Success(c, nil, "Deleted")
}

func init() {
    registry.RegisterService("{name}_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        return New{Name}Service(config.Services.IsEnabled("{name}_service"), logger)
    })
}
```

## Config Toggle

Add to `services:` in `config.yaml`:

```yaml
services:
  {name}_service: true
```

If the service accesses infrastructure components from the `Dependencies` bag, the factory function becomes:

```go
func init() {
    registry.RegisterService("{name}_service", func(cfg *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := New{Name}Service(cfg.Services.IsEnabled("{name}_service"), logger)
        // Access infra components
        if comp, ok := deps.Get("redis"); ok {
            svc.redis = comp.(*infrastructure.RedisManager)
        }
        if comp, ok := deps.Get("postgres"); ok {
            svc.postgres = comp.(*infrastructure.PostgresConnectionManager)
        }
        return svc
    })
}
```

## Testing

Write tests in `tests/services/{name}_service_test.go` (package `services`). The pattern used in existing tests uses Gin directly with `httptest`:

```go
package services

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "stackyrd/internal/services/modules"
    "stackyrd/pkg/logger"
    "stackyrd/pkg/response"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func setup{Name}TestRouter(service *modules.{Name}Service) *gin.Engine {
    gin.SetMode(gin.TestMode)
    r := gin.Default()
    group := r.Group("/api/v1")
    service.RegisterRoutes(group)
    return r
}

func Test{Name}Service_Name(t *testing.T) {
    l := logger.New(false, nil)
    service := modules.New{Name}Service(true, l)
    assert.Equal(t, "{Name} Service", service.Name())
}

func Test{Name}Service_Enabled(t *testing.T) {
    l := logger.New(false, nil)
    service := modules.New{Name}Service(true, l)
    assert.True(t, service.Enabled())
    disabledService := modules.New{Name}Service(false, l)
    assert.False(t, disabledService.Enabled())
}

func Test{Name}Service_Endpoints(t *testing.T) {
    l := logger.New(false, nil)
    service := modules.New{Name}Service(true, l)
    assert.Contains(t, service.Endpoints(), "/{endpoint}")
}

func Test{Name}Service_List(t *testing.T) {
    l := logger.New(false, nil)
    service := modules.New{Name}Service(true, l)
    router := setup{Name}TestRouter(service)

    req, _ := http.NewRequest("GET", "/api/v1/{endpoint}", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    var resp response.Response
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NoError(t, err)
    assert.True(t, resp.Success)
}
```

## Real Examples from the Codebase

Reference these existing services for concrete patterns:

| File | Pattern |
|------|---------|
| `users_service.go` | Full CRUD with validation, pagination, mock DB, sync.Map for concurrent access |
| `products_service.go` | Simple read-only service (minimal template) |
| `tasks_service.go` | Event-driven with PluginBridge access |

## Key Points

- All handlers receive a `*gin.Context` — use `request.Bind()` for request body binding
- Use `response` helpers for consistent API response formatting
- For route-level middleware (e.g., auth on specific routes), apply it inside `RegisterRoutes` on the subgroup
- For authentication on service routes, check `config.Auth.Type` — JWT and API key modes are configured globally
