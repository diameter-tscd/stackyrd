# Technical Reference

Comprehensive reference for **stackyrd-nano** — Go 1.25.3, module path `stackyrd-nano`.

## Configuration Reference

Full `config.yaml` structure:

```yaml
app:
  name: "stackyrd-nano"
  version: "1.0.1"
  debug: false
  env: "development"
  banner_path: "banner.txt"     # Embedded via pkg/assets/ — no external file needed at runtime
  startup_delay: 15             # TUI boot screen duration (seconds)
  quiet_startup: true            # Suppress console logs in TUI mode
  enable_tui: false              # Enable bubbletea terminal UI

server:
  port: "8080"
  services_endpoint: "/api/v1"

middleware:
  request_id: true
  logger: true
  permission_check: true
  cors: true
  jwt: false
  ratelimit: true
  security: true
  audit: true
  encryption: false
  gzip: true

auth:
  type: "apikey"                 # none, jwt, apikey
  secret: ""

postgres:
  enabled: true
  connections:
    - name: "primary"
      enabled: true
      host: "localhost"
      port: 5432
      user: "postgres"
      password: ""
      dbname: "mydb"
      sslmode: "disable"

encryption:
  enabled: false
  algorithm: "aes-256-gcm"
  key: ""
  rotate_keys: false
  key_rotation_interval: "24h"
```

## API Response Format

```json
{
  "success": true,
  "status": 200,
  "message": "Operation completed",
  "data": {},
  "error": null,
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  },
  "timestamp": 1748963400,
  "datetime": "2026-06-03T15:10:00+07:00",
  "correlation_id": "req-1748963400123456789"
}
```

Error:

```json
{
  "success": false,
  "status": 400,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input",
    "details": { "field": "email" }
  },
  "timestamp": 1748963400,
  "datetime": "2026-06-03T15:10:00+07:00",
  "correlation_id": "req-1748963400123456789"
}
```

## Health Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Overall status (server + infra + init progress) |
| `GET /health/infrastructure` | Per-component infrastructure status |
| `GET /health/dependencies` | Registered components + service factories |
| `GET /health/resources` | Memory usage + goroutine count |

## Service Interface

```go
type Service interface {
    Name() string
    WireName() string
    Enabled() bool
    Endpoints() []string
    RegisterRoutes(*gin.RouterGroup)
    Get() interface{}
}
```

Services auto-register via `init()`:
```go
func init() {
    registry.RegisterService("service_name", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        helper := registry.NewServiceHelper(cfg, log, deps)
        if !helper.IsServiceEnabled("service_name") {
            return nil
        }
        return NewService(true, log)
    })
}
```

## Infrastructure Component Interface

```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}

type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
```

Registered via `init()`:
```go
func init() {
    infrastructure.RegisterComponent("name", factory)
}
```

## Infrastructure Components

| Component | Config Key | Dependencies Key | Package |
|-----------|------------|------------------|---------|
| PostgreSQL | `postgres` | `postgres` | `pgx/v5` + `gorm` |

## Middleware

| Name | Config Key | Purpose |
|------|------------|---------|
| Request ID | `request_id` | Generates and propagates unique X-Request-ID per request |
| Logger | `logger` | Request logging |
| Permission | `permission_check` | Block DELETE by default |
| CORS | `cors` | Cross-origin support |
| JWT | `jwt` | JWT authentication |
| Rate Limit | `ratelimit` | Rate limiting |
| Security | `security` | Security headers |
| Audit | `audit` | Audit logging |
| Encryption | `encryption` | Request/response encryption |
| Gzip | `gzip` | Response compression |

## Scripts

| Script | Command | Description |
|--------|---------|-------------|
| Build | `go run scripts/build/build.go` | Build binary with garble/UPX/backup |
| Docker | `go run scripts/docker/docker_build.go` | Multi-stage Docker build (10 targets) |
| Service Gen | `go run scripts/service/service.go` | Interactive service code generator |
| Package Mgr | `go run scripts/pkg/pkg.go <cmd>` | Package install/list/remove/upgrade |

## Common Commands

```bash
# Run development server
go run cmd/app/main.go

# Build binary
go run scripts/build/build.go

# Build with obfuscation and compression
go run scripts/build/build.go -garble -upx

# Docker images
go run scripts/docker/docker_build.go

# Generate service code
go run scripts/service/service.go

# Run tests
go test ./...

# Run with config URL
go run cmd/app/main.go -c https://config.example.com/config.yaml

# Override port
go run cmd/app/main.go -port 9090

# Production build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w -buildid=" -trimpath -o dist/stackyrd-nano ./cmd/app
```
