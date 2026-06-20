# Getting Started

Quick setup guide for **stackyrd-nano** — a lightweight modular Go service framework built on **Gin**.

## Prerequisites

- **Go 1.25.3+**
- **Git**
- **Docker** (optional, for PostgreSQL dev environment)

## Installation

```bash
git clone https://github.com/diameter-tscd/stackyrd-nano.git
cd stackyrd-nano
go mod download
go run cmd/app/main.go
```

With PostgreSQL:

```bash
docker run -d --name postgres -e POSTGRES_PASSWORD=pass -p 5432:5432 postgres:16
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

middleware:
  cors: true
  logger: true
```

## Hello World Service

Create `internal/services/modules/hello_service.go`:

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
| Package | `go run scripts/pkg/pkg.go` | Install infrastructure packages |

## Database (Optional)

```bash
docker run -d --name postgres -e POSTGRES_PASSWORD=pass -p 5432:5432 postgres:16
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
