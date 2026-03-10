# Architecture Overview

This document provides a high-level overview of Stackyard's architecture, design decisions, and key concepts. Understanding this foundation will help you build effectively with the framework.

## System Overview

Stackyard is a **modular, service-oriented Go application framework** built on top of the Echo web framework. It emphasizes **clean architecture**, **dependency injection**, and **production readiness** with comprehensive monitoring and infrastructure integrations.

## Core Architecture Principles

### 1. Clean Architecture

Stackyard follows clean architecture principles with clear separation of concerns:

```
┌─────────────────────────────────────┐
│          Delivery Layer             │
│  (HTTP Handlers, Middleware)        │
├─────────────────────────────────────┤
│          Use Case Layer             │
│  (Business Logic, Services)         │
├─────────────────────────────────────┤
│        Infrastructure Layer         │
│  (Databases, External APIs, Utils)  │
└─────────────────────────────────────┘
```

**Benefits:**
- **Testability**: Each layer can be tested independently
- **Maintainability**: Changes in one layer don't affect others
- **Flexibility**: Easy to swap implementations (e.g., different databases)

### 2. Service-Oriented Design

Applications are built as **composable services** that can be enabled/disabled via configuration:

- **Modularity**: Services encapsulate related functionality
- **Independence**: Services can be developed and deployed separately
- **Configuration-Driven**: Runtime behavior controlled by `config.yaml`
- **Dependency Injection**: Services receive dependencies through constructors

### 3. Infrastructure Abstraction

All external dependencies are abstracted through **infrastructure managers**:

```go
// Abstract interface
type DatabaseManager interface {
    Connect() error
    Query(query string, args ...interface{}) (interface{}, error)
    Close() error
}

// Concrete implementation
type PostgresManager struct {
    db *sqlx.DB
    config PostgresConfig
}
```

**Benefits:**
- **Testability**: Easy to mock infrastructure in tests
- **Flexibility**: Can swap implementations without changing business logic
- **Consistency**: All infrastructure follows the same patterns

## Key Components

### Application Structure

```
stackyard/
├── cmd/app/           # Application entry point
├── config/            # Configuration management
├── internal/          # Private application code
│   ├── middleware/    # HTTP middleware
│   ├── monitoring/    # Monitoring system
│   ├── server/        # HTTP server setup
│   └── services/      # Business services
│       └── modules/   # Service implementations
├── pkg/               # Public reusable packages
│   ├── infrastructure/# External service integrations
│   ├── logger/        # Logging utilities
│   ├── request/       # Request handling & validation
│   ├── response/      # Standardized API responses
│   └── utils/         # General utilities
├── scripts/           # Build and deployment scripts
└── web/               # Static web assets
```

### Request Flow

```
1. HTTP Request → 2. Echo Router → 3. Middleware → 4. Handler → 5. Response

   ↓                    ↓                ↓              ↓             ↓
   Client            Routing          Auth/Logging   Business Logic  JSON
   (Browser/Mobile)  (URL matching)   (Validation)    (Services)     (Response)
```

### Service Registration

Services are registered dynamically through a **service registry**:

```go
// Service interface
type Service interface {
    Name() string                    // Human-readable name
    RegisterRoutes(*echo.Group)      // Register HTTP routes
    Enabled() bool                   // Service activation status
    Endpoints() []string            // API endpoints list
}

// Registration
registry := services.NewServiceRegistry()
registry.Register(modules.NewUserService(config))
registry.Register(modules.NewProductService(config))
registry.Boot(echoInstance) // Wire up all services
```

## Infrastructure Managers

### Database Managers

Stackyard supports multiple database types through abstracted managers:

#### PostgreSQL Manager
- **Multi-tenant support**: Dynamic database switching
- **GORM integration**: Full ORM capabilities with auto-migration
- **Connection pooling**: Efficient connection management
- **Async operations**: Non-blocking database operations

#### MongoDB Manager
- **Document database**: NoSQL capabilities
- **Multi-tenant**: Database-level isolation
- **Aggregation pipelines**: Complex data processing
- **Async operations**: Worker pool-based execution

#### Redis Manager
- **Caching**: High-performance key-value storage
- **Pub/Sub**: Message broadcasting capabilities
- **Batch operations**: Efficient bulk operations
- **Async execution**: Worker pool processing

### Message Queue Managers

#### Kafka Manager
- **Event streaming**: High-throughput message processing
- **Consumer groups**: Load balancing and fault tolerance
- **Topic management**: Dynamic topic creation and configuration
- **Async operations**: Non-blocking message publishing

### Object Storage Managers

#### MinIO Manager
- **S3-compatible**: AWS S3 API compatibility
- **File uploads**: Efficient multipart upload handling
- **Access control**: Bucket and object permissions
- **Async operations**: Background file processing

### Monitoring & Analytics

#### Grafana Manager
- **Dashboard creation**: Programmatic dashboard generation
- **Data source integration**: Connect various data sources
- **Annotation support**: Timeline event marking
- **Health monitoring**: Service status tracking

## Async Architecture

### Worker Pools

All infrastructure operations use **worker pools** for concurrency control:

```go
type WorkerPool struct {
    workers   int
    jobQueue  chan func()
    stopChan  chan struct{}
    stopped   chan struct{}
}

// Usage
result := manager.AsyncOperation(ctx, data)
// Operation runs in worker pool, returns immediately
value, err := result.Wait() // Block until complete
```

**Benefits:**
- **Resource control**: Limit concurrent operations
- **Performance**: Prevent resource exhaustion
- **Reliability**: Graceful error handling and recovery

### Async Results

Operations return **AsyncResult** types for flexible execution:

```go
type AsyncResult[T any] struct {
    Value T
    Error error
    Done  chan struct{}
}

// Synchronous usage
result := manager.GetUserAsync(ctx, userID)
user, err := result.Wait()

// Timeout support
user, err := result.WaitWithTimeout(5 * time.Second)

// Non-blocking check
if result.IsDone() {
    user, err := result.Wait()
    // Process result
}
```

## Configuration System

### Hierarchical Configuration

Configuration is managed through a **hierarchical YAML structure**:

```yaml
app:          # Application-level settings
  name: "MyApp"
  debug: true

server:       # HTTP server configuration
  port: "8080"

services:     # Service enable/disable flags
  user_service: true
  product_service: false

postgres:     # Infrastructure-specific config
  enabled: true
  connections:
    - name: "primary"
      host: "localhost"
      database: "myapp"
```

### Environment Override

Configuration can be overridden with **environment variables**:

```bash
export APP_DEBUG=false
export SERVER_PORT=3000
export POSTGRES_PASSWORD=prod-password
```

### Validation & Defaults

Configuration is **validated at startup** with sensible defaults:

```go
type Config struct {
    App     AppConfig     `yaml:"app"`
    Server  ServerConfig  `yaml:"server"`
    Postgres PostgresConfig `yaml:"postgres" validate:"required_if=Enabled true"`
}

func (c *Config) Validate() error {
    // Custom validation logic
    return validate.Struct(c)
}
```

## API Design Patterns

### Standardized Responses

All API responses follow a **consistent JSON structure**:

```json
{
  "success": true,
  "message": "Operation completed",
  "data": { /* response data */ },
  "meta": { /* pagination metadata */ },
  "timestamp": 1642598400
}
```

### Request Validation

Requests are validated using **struct tags** with automatic error formatting:

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=2,max=50"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=18,lte=120"`
}

func (h *Handler) createUser(c echo.Context) error {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        return err // Automatic validation error response
    }
    // Process valid request...
}
```

### Error Handling

Errors are handled consistently with **standardized error codes**:

```go
// Automatic error responses
return response.NotFound(c, "User not found")
return response.BadRequest(c, "Invalid input")
return response.InternalServerError(c, "Database error")
```

## Security Architecture

### Authentication & Authorization

- **API Key authentication**: Simple key-based auth
- **Session management**: Secure session handling
- **Role-based access**: Permission-based authorization

### Data Protection

- **API Obfuscation**: Base64 encoding for data in transit
- **Encryption**: AES-256-GCM encryption for sensitive data
- **Input validation**: Comprehensive request validation

### Infrastructure Security

- **Connection encryption**: TLS for database connections
- **Secure defaults**: Conservative security settings
- **Audit logging**: Comprehensive operation logging

## Monitoring & Observability

### Web Dashboard

The monitoring dashboard provides:
- **Real-time metrics**: System resource usage
- **API monitoring**: Endpoint performance and errors
- **Service health**: Individual service status
- **Log viewing**: Real-time application logs

### Terminal UI (TUI)

The TUI provides:
- **Boot sequence visualization**: Service initialization status
- **Live log monitoring**: Real-time log streaming with filtering
- **Interactive controls**: Keyboard shortcuts for navigation
- **System monitoring**: Resource usage and performance metrics

### Health Checks

Comprehensive health endpoints:
- **Application health**: `/health`
- **Infrastructure health**: `/health/infrastructure`
- **Service-specific health**: `/health/{service}`

## Build & Deployment

### Multi-Stage Docker Builds

```dockerfile
FROM golang:1.21-alpine AS builder
# Build stage - compile application

FROM alpine:latest AS runtime
# Runtime stage - minimal production image

FROM gcr.io/distroless/static AS ultra-minimal
# Ultra-minimal production image
```

### Build Scripts

Cross-platform build scripts provide:
- **Binary compilation**: Optimized Go builds
- **Asset bundling**: Include static assets in binary
- **Backup management**: Automated backup of previous builds
- **Code obfuscation**: Optional binary obfuscation

### Deployment Options

- **Binary deployment**: Direct server deployment
- **Docker containers**: Containerized deployment
- **Kubernetes**: Orchestrated deployment
- **Serverless**: Function-as-a-service deployment

## Performance Characteristics

### Concurrency Model

- **Goroutines**: Lightweight thread management
- **Worker pools**: Controlled concurrency for I/O operations
- **Async processing**: Non-blocking request handling
- **Connection pooling**: Efficient resource utilization

### Caching Strategy

- **Multi-level caching**: Memory, Redis, CDN
- **Cache invalidation**: TTL-based and manual invalidation
- **Cache warming**: Pre-population of frequently accessed data

### Database Optimization

- **Connection pooling**: Efficient database connection management
- **Query optimization**: Index usage and query planning
- **Batch operations**: Bulk data operations
- **Read/write splitting**: Separate read and write databases

## Scalability Features

### Horizontal Scaling

- **Stateless design**: Services can be scaled independently
- **Load balancing**: Distribute requests across instances
- **Database sharding**: Horizontal database scaling
- **Caching layers**: Reduce database load

### Vertical Scaling

- **Resource optimization**: Efficient memory and CPU usage
- **Async processing**: Handle high concurrency
- **Connection pooling**: Optimize external service connections

## Development Workflow

### Local Development

```bash
# Quick start
go run cmd/app/main.go

# With custom config
go run cmd/app/main.go -config=config.dev.yaml

# Enable debug logging
export APP_DEBUG=true
go run cmd/app/main.go
```

### Testing Strategy

- **Unit tests**: Individual component testing
- **Integration tests**: End-to-end API testing
- **Performance tests**: Load and stress testing
- **Security tests**: Penetration testing and vulnerability scanning

### CI/CD Pipeline

```yaml
# GitHub Actions example
- name: Test
  run: go test ./...

- name: Build
  run: ./scripts/build.sh

- name: Docker Build
  run: ./scripts/docker_build.sh

- name: Deploy
  run: kubectl apply -f k8s/
```

## Best Practices

### Code Organization

1. **Service boundaries**: Clear separation of business logic
2. **Dependency injection**: Constructor-based dependency injection
3. **Interface segregation**: Small, focused interfaces
4. **Error handling**: Consistent error handling patterns

### Performance

1. **Async operations**: Use async for I/O operations
2. **Caching**: Implement appropriate caching strategies
3. **Pagination**: Always paginate large datasets
4. **Monitoring**: Monitor performance metrics

### Security

1. **Input validation**: Validate all user inputs
2. **Secure defaults**: Conservative security settings
3. **Regular updates**: Keep dependencies updated
4. **Audit logging**: Log security-relevant events

## Migration & Extensibility

### Adding New Services

1. **Implement Service interface**
2. **Register in service registry**
3. **Configure via config.yaml**
4. **Add tests and documentation**

### Infrastructure Extensions

1. **Create infrastructure manager**
2. **Implement async operations**
3. **Add configuration support**
4. **Update dependency injection**

### API Extensions

1. **Add new endpoints**
2. **Implement request/response types**
3. **Add validation rules**
4. **Update API documentation**

## Conclusion

Stackyard's architecture emphasizes **modularity**, **scalability**, and **maintainability** through clean architecture principles, service-oriented design, and comprehensive infrastructure abstractions. The framework provides a solid foundation for building production-ready applications while maintaining developer productivity and code quality.

The combination of **async processing**, **dependency injection**, and **configuration-driven behavior** makes Stackyard suitable for applications ranging from simple APIs to complex, multi-tenant SaaS platforms.

For detailed implementation guides, see the **[Development Guide](DEVELOPMENT.md)**. For complete API reference, see the **[API Reference](REFERENCE.md)**.
