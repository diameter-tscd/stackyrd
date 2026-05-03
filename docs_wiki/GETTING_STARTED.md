# Getting Started

Quick setup guide for stackyrd - a Go framework built on **Gin**.

## Prerequisites

- **Go 1.21+**
- **Git**

## Installation

```bash
git clone https://github.com/your-repo/stackyrd.git
cd stackyrd
go mod download
go run cmd/app/main.go
```

## Configuration

Edit `config.yaml`:

```yaml
app:
  name: "My App"
  debug: true

server:
  port: "8080"

services:
  users_service: true
  products_service: true
```

## Hello World Service

Create `internal/services/modules/hello_service.go`:

```go
package modules

import (
    "stackyrd/pkg/response"
    "github.com/gin-gonic/gin"
)

type HelloService struct {
    enabled bool
}

func (s *HelloService) Name() string        { return "Hello Service" }
func (s *HelloService) WireName() string    { return "hello" }
func (s *HelloService) Enabled() bool       { return s.enabled }
func (s *HelloService) Endpoints() []string { return []string{"/hello"} }
func (s *HelloService) Get() interface{}    { return s }

func (s *HelloService) RegisterRoutes(g *gin.RouterGroup) {
    g.GET("/hello", func(c *gin.Context) {
        response.Success(c, map[string]string{"msg": "Hello!"}, "OK")
    })
}

func init() {
    registry.RegisterService("hello", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        return &HelloService{enabled: cfg.Services.IsEnabled("hello")}
    })
}
```

Enable in `config.yaml`:
```yaml
services:
  hello: true
```

Test:
```bash
curl http://localhost:8080/api/v1/hello
```

## Database (Optional)

```bash
# PostgreSQL
docker run -d --name postgres -e POSTGRES_PASSWORD=password -p 5432:5432 postgres:15

# Redis
docker run -d --name redis -p 6379:6379 redis:7
```

Configure in `config.yaml`:
```yaml
postgres:
  enabled: true
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "postgres"