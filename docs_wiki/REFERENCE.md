# Technical Reference

Essential technical details for stackyrd, built on **Gin**.

## Configuration Reference

```yaml
app:
  name: "stackyrd"
  debug: true
  env: "development"

server:
  port: "8080"

services:
  users_service: true
  products_service: true
  mongodb_service: true

middleware:
  logger: true
  cors: true
  jwt: false

auth:
  type: "none" # none, jwt, apikey
  secret: ""

redis:
  enabled: false
  address: "localhost:6379"

postgres:
  enabled: true
  connections:
    - name: "default"
      host: "localhost"
      port: 5432
      user: "postgres"
      password: ""
      dbname: "postgres"

mongo:
  enabled: true
  connections:
    - name: "default"
      uri: "mongodb://localhost:27017"
      database: "mydb"
```

## API Response Format

All responses follow:
```json
{
  "success": true,
  "message": "Operation completed",
  "data": {},
  "timestamp": 1640995200
}
```

Error responses:
```json
{
  "success": false,
  "error": "Error message",
  "timestamp": 1640995200
}
```

## Key Endpoints

| Service | Endpoint | Method | Description |
|---------|-----------|----------|-------------|
| Users | `/api/v1/users` | GET | List users |
| Users | `/api/v1/users` | POST | Create user |
| Products | `/api/v1/products` | GET | List products |
| MongoDB | `/api/v1/products/{tenant}` | GET | Tenant products |

## Service Pattern

Services auto-register via `init()`:
```go
func init() {
    registry.RegisterService("service_name", func(cfg *config.Config, log *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        return NewService(cfg.Services.IsEnabled("service_name"))
    })
}
```

Interface:
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

## Infrastructure Pattern

Components auto-register:
```go
func init() {
    infrastructure.RegisterComponent("name", func(cfg *config.Config, log *logger.Logger) (infrastructure.InfrastructureComponent, error) {
        return NewComponent(cfg)
    })
}
```

Interface:
```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}
```

## Common Commands

```bash
# Run app
go run cmd/app/main.go

# Generate swagger docs
swag init -g cmd/app/main.go -o docs

# Build binary
go build -o app cmd/app/main.go

# Run tests
go test ./...