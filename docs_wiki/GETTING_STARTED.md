# Getting Started

Quick setup guide.

## Prereqs

- Go 1.25.3+
- Docker Compose (optional, for databases)

## Run

```bash
go mod download
go run cmd/app/main.go
```

With docker: `docker-compose up -d`

## Config

Create `config.yaml`:

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

infrastructure:
  redis: false
  postgres: false
```

## Hello Service

```go
type HelloService struct{ enabled bool }

func (s *HelloService) Name() string        { return "Hello" }
func (s *HelloService) Enabled() bool       { return s.enabled }
func (s *HelloService) RegisterRoutes(g *echo.Group) {
    g.GET("/hello", func(c echo.Context) error { return response.Success(c, "👋") })
}
func (s *HelloService) Get() any              { return s }
```

Register:

```go
func init() {
    registry.RegisterService("hello", func(cfg *config.Config, l *logger.Logger) interfaces.Service {
        if !cfg.Services.IsEnabled("hello") { return nil }
        return &HelloService{}
    })
}
```

Enable: `services.hello: true`

Test: `curl http://localhost:8080/api/v1/hello`

## Scripts CLI

Build once:

```bash
cd scripts && go build -o yrd .
./yrd build       # binary → dist/stackyrd
./yrd docker      # multi-stage images
./yrd service     # scaffold new service
./yrd swagger     # generate OpenAPI docs
./yrd pkg ...     # package manager
```