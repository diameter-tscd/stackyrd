# Development Guide

Learn to add services and extend stackyrd.

## Adding a Service

Create `internal/services/modules/your_service.go`:

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

type YourService struct {
    enabled bool
}

func NewYourService(enabled bool) *YourService {
    return &YourService{enabled: enabled}
}

func (s *YourService) Name() string        { return "Your Service" }
func (s *YourService) WireName() string    { return "your-service" }
func (s *YourService) Enabled() bool       { return s.enabled }
func (s *YourService) Endpoints() []string { return []string{"/your-api"} }
func (s *YourService) Get() interface{}    { return s }

func (s *YourService) RegisterRoutes(g *gin.RouterGroup) {
    g.GET("/your-api", s.getData)
}

func (s *YourService) getData(c *gin.Context) error {
    return response.Success(c, map[string]string{"msg": "Hello"}, "OK")
}

func init() {
    registry.RegisterService("your_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        return NewYourService(cfg.Services.IsEnabled("your_service"))
    })
}
```

Enable in `config.yaml`:
```yaml
services:
  your_service: true
```

## Request Validation

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
}

func (s *YourService) create(c *gin.Context) error {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        return response.BadRequest(c, "Invalid input")
    }
    // process...
    return response.Created(c, req, "Created")
}
```

## Using Dependencies

```go
type YourService struct {
    enabled bool
    db      *infrastructure.PostgresManager
    cache   *infrastructure.RedisManager
}

func init() {
    registry.RegisterService("your_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        return &YourService{
            enabled: cfg.Services.IsEnabled("your_service"),
            db:      deps.Get("postgres").(*infrastructure.PostgresManager),
            cache:   deps.Get("redis").(*infrastructure.RedisManager),
        }
    })
}
```

## Response Helpers

```go
response.Success(c, data, "OK")              // 200
response.Created(c, data, "Created")          // 201
response.NoContent(c)                          // 204
response.BadRequest(c, "Error")               // 400
response.NotFound(c, "Not found")             // 404
response.InternalServerError(c, "Error")       // 500
```

## Enabling in Config

```yaml
services:
  your_service: true

postgres:
  enabled: true
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "mydb"