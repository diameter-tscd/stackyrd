# Architecture Overview

stackyrd is a modular Go framework built on **Gin** with auto-discovery patterns.

## Key Concepts

### Auto-Discovery Pattern
Components register themselves via `init()` functions:
- **Services**: Business logic in `internal/services/modules/`
- **Middleware**: HTTP middleware in `internal/middleware/`
- **Infrastructure**: Databases/clients in `pkg/infrastructure/`

### Request Flow
```
Client → Middleware → Service Handler → Response
                ↓
         Infrastructure (DB, Cache, etc.)
```

### Configuration
- YAML config via Viper
- Environment variable overrides (e.g., `SERVER_PORT`)
- Services enabled/disabled via `config.yaml`

## Project Structure
```
stackyrd/
├── cmd/app/              # Entry point
├── config/              # Configuration
├── internal/
│   ├── middleware/      # HTTP middleware
│   ├── server/         # Server setup
│   └── services/modules/ # Business logic
├── pkg/
│   ├── infrastructure/  # Database/clients
│   ├── registry/       # Service registry
│   └── interfaces/     # Service interface
└── config.yaml
```

## Service Pattern
```go
type Service interface {
    Name() string
    RegisterRoutes(*gin.RouterGroup)
    Enabled() bool
}

// Auto-registration
func init() {
    registry.RegisterService("name", factoryFunc)
}
```

## Infrastructure Pattern
```go
type InfrastructureComponent interface {
    Name() string
    Close() error
}

// Auto-registered in init()
func init() {
    infrastructure.RegisterComponent("name", factoryFunc)
}
```

## Key Features
- **Dependency Injection**: Via `Dependencies` container
- **Async Operations**: Worker pools for I/O
- **Multi-tenant**: Multiple DB connections
- **Monitoring**: Built-in dashboard at `/health`