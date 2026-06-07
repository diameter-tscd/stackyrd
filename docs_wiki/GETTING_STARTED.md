# Getting Started

Quick setup guide for **stackyrd** — an enterprise-grade modular Go service framework built on **Gin**.

## Prerequisites

- **Go 1.25.3+**
- **Git**
- **Docker + Docker Compose** (for full dev environment with databases)

## Installation

```bash
git clone https://github.com/diameter-tscd/stackyrd.git
cd stackyrd
go mod download
go run cmd/app/main.go
```

With full infrastructure (Redis, PostgreSQL, Kafka, MongoDB, Grafana, MinIO):

```bash
docker-compose up -d
go run cmd/app/main.go
```

## Configuration

Edit `config.yaml`:

```yaml
app:
  name: "My App"
  env: "development"
  enable_tui: false

server:
  port: "8080"
  services_endpoint: /api/v1

services:
  users_service: true
  products_service: true

middleware:
  cors: true
  logger: true
```

## Hello World Service

Create `internal/services/modules/hello_service.go`:

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

type HelloService struct {
    enabled bool
    logger  *logger.Logger
}

func NewHelloService(enabled bool, logger *logger.Logger) *HelloService {
    return &HelloService{enabled: enabled, logger: logger}
}

func (s *HelloService) Name() string        { return "Hello Service" }
func (s *HelloService) WireName() string    { return "hello-service" }
func (s *HelloService) Enabled() bool       { return s.enabled }
func (s *HelloService) Endpoints() []string { return []string{"GET /hello"} }
func (s *HelloService) Get() interface{}    { return s }

func (s *HelloService) RegisterRoutes(g *gin.RouterGroup) {
    g.GET("/hello", s.handleHello)
}

func (s *HelloService) handleHello(c *gin.Context) {
    response.Success(c, map[string]string{"msg": "Hello!"})
}

func init() {
    registry.RegisterService("hello_service", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        if !cfg.Services.IsEnabled("hello_service") {
            return nil
        }
        return NewHelloService(true, log)
    })
}
```

Enable in `config.yaml`:
```yaml
services:
  hello_service: true
```

Test:
```bash
curl http://localhost:8080/api/v1/hello
```

## Using Scripts

The project includes several CLI scripts:

| Script | Command | Purpose |
|--------|---------|---------|
| Build | `go run scripts/build/build.go` | Build binary (garble, UPX, backup) |
| Docker | `go run scripts/docker/docker_build.go` | Multi-stage Docker image builder |
| Service | `go run scripts/service/service.go` | Scaffold new service modules |
| Swagger | `go run scripts/swagger/swagger.go` | Generate OpenAPI docs |
| Package | `go run scripts/pkg/pkg.go` | Install infrastructure packages |

## Database (Optional)

```bash
# Full dev environment
docker-compose up -d

# Or individual services:
docker run -d --name postgres -e POSTGRES_PASSWORD=pass -p 5432:5432 postgres:16
docker run -d --name redis -p 6379:6379 redis:7
docker run -d --name mongo -p 27017:27017 mongo:7
```

Configure in `config.yaml`:
```yaml
postgres:
  enabled: true
  connections:
    - name: "default"
      host: "localhost"
      port: 5432
      user: "postgres"
      password: "pass"
      dbname: "mydb"
      sslmode: "disable"
```
