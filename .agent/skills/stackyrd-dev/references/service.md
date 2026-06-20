# Adding a Service

Services are the primary way to add business logic and HTTP endpoints. They follow the **auto-registration** pattern — create a file in `internal/services/modules/`, implement the interface, register via `init()`, and toggle via `config.yaml`.

## Interface Requirements

Every service must implement `interfaces.Service` (`pkg/interfaces/service.go`):

```go
type Service interface {
    Name() string              // Human-readable display name
    WireName() string          // DI wire name (snake_case)
    Enabled() bool             // Config-driven toggle
    Endpoints() []string       // HTTP endpoint patterns
    RegisterRoutes(g *gin.RouterGroup)
    Get() interface{}
}
```

## File Template

Create `internal/services/modules/{name}_service.go`:

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

func (s *{Name}Service) Name() string      { return "{Name} Service" }
func (s *{Name}Service) WireName() string  { return "{wire_name}" }
func (s *{Name}Service) Enabled() bool     { return s.enabled }
func (s *{Name}Service) Get() interface{}  { return s }

func (s *{Name}Service) Endpoints() []string {
    return []string{"/{endpoint}", "/{endpoint}/:id"}
}

func (s *{Name}Service) RegisterRoutes(g *gin.RouterGroup) {
    sub := g.Group("/{endpoint}")
    sub.GET("", s.list)
    sub.GET("/:id", s.get)
    sub.POST("", s.create)
    sub.PUT("/:id", s.update)
    sub.DELETE("/:id", s.delete)
}

func (s *{Name}Service) list(c *gin.Context)   { response.Success(c, nil, "List endpoint") }
func (s *{Name}Service) get(c *gin.Context)     { response.Success(c, nil, "Get endpoint") }
func (s *{Name}Service) create(c *gin.Context)  { response.Created(c, nil, "Created") }
func (s *{Name}Service) update(c *gin.Context)  { response.Success(c, nil, "Updated") }
func (s *{Name}Service) delete(c *gin.Context)  { response.Success(c, nil, "Deleted") }

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

If the service accesses infrastructure components from the `Dependencies` bag:

```go
func init() {
    registry.RegisterService("{name}_service", func(cfg *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := New{Name}Service(cfg.Services.IsEnabled("{name}_service"), logger)
        if comp, ok := deps.Get("postgres"); ok {
            svc.postgres = comp.(*infrastructure.PostgresConnectionManager)
        }
        return svc
    })
}
```

## Testing

Write tests in `tests/services/{name}_service_test.go`:

```go
package services

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "stackyrd-nano/internal/services/modules"
    "stackyrd-nano/pkg/logger"
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
}
```

## Key Points

- All handlers receive a `*gin.Context`
- Use `request.Bind()` for request body binding
- Use `response` helpers for consistent API response formatting
- Infrastructure components are accessed from `deps.Get("name")` in the factory
