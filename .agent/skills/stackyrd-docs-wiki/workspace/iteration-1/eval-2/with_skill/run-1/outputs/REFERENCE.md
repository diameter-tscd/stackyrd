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
  banner_path: "banner.txt"
  startup_delay: 15             # TUI boot screen duration (seconds)
  quiet_startup: true            # Suppress console logs in TUI mode
  enable_tui: false              # Enable bubbletea terminal UI

server:
  port: "8080"
  services_endpoint: "/api/v1"

services:
  users_service: true
  products_service: true
  tasks_service: true
  broadcast_service: false
  cache_service: true
  encryption_service: false
  grafana_service: false
  mongodb_service: true
  multi_tenant_service: true

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
  swagger: true

auth:
  type: "apikey"                 # none, jwt, apikey
  secret: ""

redis:
  enabled: false
  address: "localhost:6379"
  password: ""
  db: 0

kafka:
  enabled: false
  brokers:
    - "localhost:9092"
  topic: "my-topic"
  group_id: "my-group"

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

mongo:
  enabled: true
  connections:
    - name: "primary"
      enabled: true
      uri: "mongodb://localhost:27017"
      database: "mydb"

grafana:
  enabled: false
  url: "http://localhost:3000"
  api_key: ""
  username: "admin"
  password: "admin"

minio:
  enabled: false
  endpoint: "localhost:9003"
  access_key_id: "minioadmin"
  secret_access_key: "minioadmin"
  use_ssl: false
  bucket_name: "main"

cron:
  enabled: true
  jobs:
    log_cleanup: "0 0 * * *"
    health_check: "*/10 * * * * *"

encryption:
  enabled: false
  algorithm: "aes-256-gcm"
  key: ""
  rotate_keys: false
  key_rotation_interval: "24h"

swagger:
  enabled: false
  base_path: "/swagger"

plugins:
  enabled: true
  default_limits:
    max_timeout_ms: 30000
    max_memory_bytes: 104857600
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
| MongoDB | `mongo` | `mongo` | `mongo-driver` |
| Redis | `redis` | `redis` | `go-redis/v9` |
| Kafka | `kafka` | `kafka` | `sarama` |
| MinIO | `minio` | `minio` | `minio-go/v7` |
| Grafana | `grafana` | `grafana` | HTTP client |
| Cron | `cron` | `cron` | `robfig/cron/v3` |
| Plugins | `plugins` | `plugins` | PluginBridge |

## Plugin System

### REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/plugins` | List all plugins |
| GET | `/api/v1/plugins/:name` | Plugin details |
| POST | `/api/v1/plugins/:name/execute` | Execute plugin |
| PUT | `/api/v1/plugins/:name/scripts/:file` | Upload/replace script |
| GET | `/api/v1/plugins/:name/scripts` | List plugin scripts |
| GET | `/api/v1/plugins/:name/scripts/:file` | Get script content |
| DELETE | `/api/v1/plugins/:name` | Unload plugin |
| GET | `/api/v1/plugins/manager/status` | Manager health metrics |

### Runtime Types

| Prefix | Runtime | Language |
|--------|---------|----------|
| `ts:` | goja (sandboxed) | TypeScript |
| `ext:` | gRPC subprocess | Python, etc. |
| `go:` | Native Go | Go |

### Builtin Plugins

| Plugin | Language | Purpose |
|--------|----------|---------|
| inspector | TypeScript | System inspection |
| aggregator | TypeScript | Data aggregation |
| template_renderer | Python | Template rendering |
| schema_validator | Python | Schema validation |
| data_processor | Python | Data processing |
| metric_computer | Python | Metric computation |
| webhook_transformer | Python | Webhook transformation |
| python_demo | Python | Demo plugin |
| lua_demo | Lua | Lua demo |
| lua_transformer | Lua | Lua transformation |

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
| Swagger | `swagger` | Swagger UI route |

## Scripts

| Script | Command | Description |
|--------|---------|-------------|
| Build | `go run scripts/build/build.go` | Build binary with garble/UPX/backup |
| Docker | `go run scripts/docker/docker_build.go` | Multi-stage Docker build (10 targets) |
| Service Gen | `go run scripts/service/service.go` | Interactive service code generator |
| Swagger Gen | `go run scripts/swagger/swagger.go` | OpenAPI doc generation |
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

# Generate Swagger docs
go run scripts/swagger/swagger.go

# Install infrastructure package
go run scripts/pkg/pkg.go install -pkg cloud/aws/ec2@1.0.0

# List installed packages
go run scripts/pkg/pkg.go list

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
