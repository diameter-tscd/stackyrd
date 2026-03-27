# Development Guide

This guide covers how to extend and customize Stackyard for your specific needs. Learn to add new services, integrate databases, handle API requests, and deploy your application.

## Adding New Services

Services are the core building blocks of Stackyard applications. Each service encapsulates business logic and exposes API endpoints.

### Basic Service Structure

Create a new service in `internal/services/modules/service_yourname.go`:

```go
package modules

import (
	"stackyard/config"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/request"
	"stackyard/pkg/response"

	"github.com/labstack/echo/v4"
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

func (s *YourService) RegisterRoutes(g *echo.Group) {
	// Register your API endpoints here
	g.GET("/your-api", s.getData)
	g.POST("/your-api", s.createData)
}

func (s *YourService) getData(c echo.Context) error {
	// Your business logic here
	data := map[string]string{"message": "Hello from your service!"}
	return response.Success(c, data, "Data retrieved")
}

func (s *YourService) createData(c echo.Context) error {
	// Handle POST request
	return response.Created(c, nil, "Data created")
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("your_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewYourService(config.Services.IsEnabled("your_service"))
	})
}
```

### Service Auto-Discovery

Services are automatically discovered and registered when their package is imported. The `init()` function in your service file handles this registration:

```go
// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("your_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewYourService(config.Services.IsEnabled("your_service"))
	})
}
```

**How it works:**
1. When the application starts, Go automatically calls the `init()` function
2. The service factory is registered with the global registry
3. During boot, the registry automatically discovers and creates enabled services
4. No manual registration in `internal/server/server.go` is required

**Service Factory Function:**
- Takes configuration, logger, and dependencies as parameters
- Returns a new service instance
- Checks if the service is enabled via `config.Services.IsEnabled()`
- Enables dependency injection for infrastructure components

### Enable in Configuration

Add to `config.yaml`:

```yaml
services:
  your_service: true
```

### Test Your Service

```bash
# Restart the application
go run cmd/app/main.go

# Test the endpoint
curl http://localhost:8080/api/v1/your-api
```

## API Development

### Request Handling & Validation

Stackyard provides built-in request validation and standardized responses.

#### Basic Request Handling

```go
func (s *YourService) createUser(c echo.Context) error {
    // Parse JSON request
    var req CreateUserRequest
    if err := c.Bind(&req); err != nil {
        return response.BadRequest(c, "Invalid request format")
    }

    // Validate request
    if req.Name == "" {
        return response.BadRequest(c, "Name is required")
    }

    // Process request
    user := User{Name: req.Name, Email: req.Email}
    // Save to database...

    return response.Created(c, user, "User created successfully")
}
```

#### Advanced Validation with Tags

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,username"`
    Email    string `json:"email" validate:"required,email"`
    FullName string `json:"full_name" validate:"required,min=3,max=100"`
}

type UpdateUserRequest struct {
    Username string `json:"username" validate:"omitempty,username"`
    Email    string `json:"email" validate:"omitempty,email"`
    FullName string `json:"full_name" validate:"omitempty,min=3,max=100"`
    Status   string `json:"status" validate:"omitempty,oneof=active inactive suspended"`
}

func (s *YourService) createUser(c echo.Context) error {
    var req CreateUserRequest

    // Bind and validate in one step
    if err := request.Bind(c, &req); err != nil {
        if validationErr, ok := err.(*request.ValidationError); ok {
            return response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
        }
        return response.BadRequest(c, err.Error())
    }

    // Request is valid, proceed...
    user := User{
        ID:        "123",
        Username:  req.Username,
        Email:     req.Email,
        Status:    "active",
        CreatedAt: time.Now().Unix(),
    }

    return response.Created(c, user, "User created successfully")
}
```

#### Custom Validators

Add custom validation rules in `pkg/request/request.go`:

```go
// Add to the validator initialization
validate.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
    phone := fl.Field().String()
    // Your phone validation logic
    matched, _ := regexp.MatchString(`^\+?[1-9]\d{1,14}$`, phone)
    return matched
})

validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
    username := fl.Field().String()
    // Alphanumeric, 3-20 characters
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9]{3,20}$`, username)
    return matched
})
```

### Dependency Injection

Services receive infrastructure dependencies through constructor injection via the service factory function:

```go
type UserService struct {
    enabled bool
    db      *infrastructure.PostgresManager
    cache   *infrastructure.RedisManager
    logger  *logger.Logger
}

func NewUserService(
    db *infrastructure.PostgresManager,
    cache *infrastructure.RedisManager,
    logger *logger.Logger,
    enabled bool,
) *UserService {
    return &UserService{
        enabled: enabled,
        db:      db,
        cache:   cache,
        logger:  logger,
    }
}
```

**Service Factory with Dependencies:**

```go
func init() {
    registry.RegisterService("user_service", func(
        config *config.Config,
        logger *logger.Logger,
        deps *registry.Dependencies,
    ) interfaces.Service {
        return NewUserService(
            deps.Postgres,
            deps.Redis,
            logger,
            config.Services.IsEnabled("user_service"),
        )
    })
}
```

**Available Dependencies:**
- `deps.Postgres` - PostgreSQL database manager
- `deps.Redis` - Redis cache manager
- `deps.MinIO` - Object storage manager
- `deps.Kafka` - Message queue manager
- `deps.MongoDB` - MongoDB database manager
- `deps.Grafana` - Monitoring dashboard manager

**Using Dependencies in Handlers:**

```go
func (s *UserService) createUser(c echo.Context) error {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        return response.ValidationError(c, "Validation failed", err.GetFieldErrors())
    }

    // Use database dependency
    user := User{
        Username: req.Username,
        Email:    req.Email,
    }

    if err := s.db.Create(&user).Error; err != nil {
        s.logger.Error("Failed to create user", "error", err)
        return response.InternalServerError(c, "Database error")
    }

    // Use cache dependency
    s.cache.cacheUser(user.ID, user)

    s.logger.Info("User created successfully", "user_id", user.ID)
    return response.Created(c, user, "User created successfully")
}
```

### Response Types

#### Success Responses

```go
// Simple success
return response.Success(c, data, "Operation completed")

// Success with metadata (pagination)
meta := response.CalculateMeta(page, perPage, total)
return response.SuccessWithMeta(c, data, meta, "Data retrieved")

// Created (201)
return response.Created(c, newResource, "Resource created")

// No content (204)
return response.NoContent(c)
```

#### Error Responses

```go
// Bad request
return response.BadRequest(c, "Invalid input data")

// Not found
return response.NotFound(c, "Resource not found")

// Unauthorized
return response.Unauthorized(c, "Authentication required")

// Forbidden
return response.Forbidden(c, "Access denied")

// Validation error with field details
fieldErrors := map[string]string{
    "email": "Invalid email format",
    "password": "Must be at least 8 characters",
}
return response.ValidationError(c, "Validation failed", fieldErrors)
```

#### Pagination

```go
type PaginationRequest struct {
    Page    int `query:"page" json:"page"`
    PerPage int `query:"per_page" json:"per_page"`
}

func (s *YourService) listUsers(c echo.Context) error {
    var pagination PaginationRequest
    if err := c.Bind(&pagination); err != nil {
        return response.BadRequest(c, "Invalid pagination parameters")
    }

    page := pagination.GetPage()      // Default: 1
    perPage := pagination.GetPerPage() // Default: 10, Max: 100
    offset := pagination.GetOffset()   // Calculated offset

    // Query with pagination
    users, total, err := s.getUsersWithPagination(offset, perPage)
    if err != nil {
        return response.InternalServerError(c, "Failed to fetch users")
    }

    // Return with pagination metadata
    meta := response.CalculateMeta(page, perPage, total)
    return response.SuccessWithMeta(c, users, meta, "Users retrieved")
}
```

## Database Integration

### PostgreSQL with GORM

#### Basic Model & Operations

```go
type User struct {
    gorm.Model
    Name     string `json:"name" gorm:"not null"`
    Email    string `json:"email" gorm:"unique;not null"`
    Password string `json:"-" gorm:"not null"` // Don't serialize
}

func (s *UserService) createUser(c echo.Context) error {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        return response.BadRequest(c, err.Error())
    }

    user := User{
        Name:     req.Name,
        Email:    req.Email,
        Password: hashPassword(req.Password),
    }

    if err := s.db.Create(&user).Error; err != nil {
        return response.InternalServerError(c, "Failed to create user")
    }

    return response.Created(c, user, "User created")
}

func (s *UserService) getUser(c echo.Context) error {
    id := c.Param("id")
    var user User

    if err := s.db.First(&user, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return response.NotFound(c, "User not found")
        }
        return response.InternalServerError(c, "Database error")
    }

    return response.Success(c, user, "User retrieved")
}
```

#### Dependency Injection

Services receive database managers through constructor injection:

```go
type UserService struct {
    enabled bool
    db      *infrastructure.PostgresManager
}

func NewUserService(db *infrastructure.PostgresManager, enabled bool) *UserService {
    return &UserService{
        enabled: enabled,
        db:      db,
    }
}
```

### Redis Caching

#### Basic Caching Operations

```go
type CacheService struct {
    redis *infrastructure.RedisManager
}

func (s *CacheService) cacheUser(userID string, user User) error {
    ctx := context.Background()
    data, err := json.Marshal(user)
    if err != nil {
        return err
    }

    return s.redis.Set(ctx, fmt.Sprintf("user:%s", userID), string(data), time.Hour)
}

func (s *CacheService) getCachedUser(userID string) (*User, error) {
    ctx := context.Background()
    data, err := s.redis.Get(ctx, fmt.Sprintf("user:%s", userID))
    if err != nil {
        return nil, err
    }

    var user User
    if err := json.Unmarshal([]byte(data), &user); err != nil {
        return nil, err
    }

    return &user, nil
}
```

#### Cache-Aside Pattern

```go
func (s *UserService) getUserWithCache(c echo.Context) error {
    userID := c.Param("id")

    // Try cache first
    if user, err := s.cache.getCachedUser(userID); err == nil {
        return response.Success(c, user, "User retrieved from cache")
    }

    // Cache miss - get from database
    var user User
    if err := s.db.First(&user, userID).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return response.NotFound(c, "User not found")
        }
        return response.InternalServerError(c, "Database error")
    }

    // Cache for future requests
    s.cache.cacheUser(userID, user)

    return response.Success(c, user, "User retrieved")
}
```

## Event Streaming

### Server-Sent Events (SSE)

Add real-time capabilities to your services:

```go
type NotificationService struct {
    enabled     bool
    broadcaster *utils.EventBroadcaster
}

func NewNotificationService(enabled bool) *NotificationService {
    return &NotificationService{
        enabled:     enabled,
        broadcaster: utils.NewEventBroadcaster(),
    }
}

func (s *NotificationService) RegisterRoutes(g *echo.Group) {
    g.GET("/notifications/stream", s.streamNotifications)
    g.POST("/notifications/send", s.sendNotification)
}

func (s *NotificationService) streamNotifications(c echo.Context) error {
    // Subscribe to notification stream
    client := s.broadcaster.Subscribe("notifications")
    defer s.broadcaster.Unsubscribe(client.ID)

    // Set SSE headers
    c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
    c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
    c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

    // Listen for events
    for {
        select {
        case event := <-client.Channel:
            // Send SSE event
            c.Response().Write([]byte(fmt.Sprintf("data: %s\n\n", event.Data)))
            c.Response().Flush()
        case <-c.Request().Context().Done():
            return nil
        }
    }
}

func (s *NotificationService) sendNotification(c echo.Context) error {
    var notification map[string]interface{}
    if err := c.Bind(&notification); err != nil {
        return response.BadRequest(c, "Invalid notification data")
    }

    // Broadcast to all subscribers
    s.broadcaster.Broadcast("notifications", "notification", "New notification", notification)

    return response.Success(c, nil, "Notification sent")
}
```

## File Upload Handling

### Basic File Upload

```go
func (s *FileService) uploadFile(c echo.Context) error {
    // Get file from form
    file, err := c.FormFile("file")
    if err != nil {
        return response.BadRequest(c, "No file provided")
    }

    // Open uploaded file
    src, err := file.Open()
    if err != nil {
        return response.InternalServerError(c, "Failed to open file")
    }
    defer src.Close()

    // Upload to storage (MinIO, local, etc.)
    result, err := s.storage.UploadFile(context.Background(),
        fmt.Sprintf("uploads/%s", file.Filename),
        src, file.Size, file.Header.Get("Content-Type"))

    if err != nil {
        return response.InternalServerError(c, "Upload failed")
    }

    return response.Created(c, map[string]interface{}{
        "filename": file.Filename,
        "size":     file.Size,
        "url":      s.storage.GetFileUrl(result.Key),
    }, "File uploaded successfully")
}
```

## Configuration Management

### Service Configuration

Services are enabled/disabled through the `services` section in `config.yaml`:

```yaml
services:
  your_service: true
  user_service: true
  product_service: false
```

### Adding New Configuration Options

Add to `config/config.go`:

```go
type YourServiceConfig struct {
    APIKey    string `yaml:"api_key"`
    Timeout   int    `yaml:"timeout" default:"30"`
    Endpoints []string `yaml:"endpoints"`
}

type Config struct {
    // ... existing fields ...
    YourService YourServiceConfig `yaml:"your_service"`
}
```

Use in `config.yaml`:

```yaml
your_service:
  api_key: "your-api-key"
  timeout: 60
  endpoints:
    - "https://api.example.com"
    - "https://backup.example.com"
```

### Accessing Configuration in Services

```go
func init() {
    registry.RegisterService("your_service", func(
        config *config.Config,
        logger *logger.Logger,
        deps *registry.Dependencies,
    ) interfaces.Service {
        // Access service-specific configuration
        serviceConfig := config.YourService
        
        return NewYourService(
            deps.Postgres,
            logger,
            serviceConfig,
            config.Services.IsEnabled("your_service"),
        )
    })
}

type YourService struct {
    enabled        bool
    db             *infrastructure.PostgresManager
    logger         *logger.Logger
    serviceConfig  YourServiceConfig
}

func NewYourService(
    db *infrastructure.PostgresManager,
    logger *logger.Logger,
    serviceConfig YourServiceConfig,
    enabled bool,
) *YourService {
    return &YourService{
        enabled:       enabled,
        db:            db,
        logger:        logger,
        serviceConfig: serviceConfig,
    }
}
```

### Environment Variable Overrides

Configuration can be overridden using environment variables:

```bash
export YOUR_SERVICE_API_KEY="production-api-key"
export YOUR_SERVICE_TIMEOUT=120
export YOUR_SERVICE_ENDPOINTS='["https://prod-api.example.com"]'

go run cmd/app/main.go
```

**Environment Variable Naming:**
- Use uppercase with underscores
- Prefix with the service name
- Use JSON format for complex types like slices

## Testing

### Unit Tests

```go
func TestUserService_GetUser(t *testing.T) {
    // Setup
    mockDB := &mocks.PostgresManager{}
    service := NewUserService(mockDB, true)

    // Mock expectations
    expectedUser := User{ID: 1, Name: "John"}
    mockDB.On("First", mock.AnythingOfType("*User"), "1").Return(nil).Run(func(args mock.Arguments) {
        user := args.Get(0).(*User)
        *user = expectedUser
    })

    // Test
    c, rec := setupEchoContext()
    c.SetParamNames("id")
    c.SetParamValues("1")

    err := service.getUser(c)
    assert.NoError(t, err)

    // Verify response
    var response map[string]interface{}
    json.Unmarshal(rec.Body.Bytes(), &response)
    assert.True(t, response["success"].(bool))
    assert.Equal(t, "User retrieved", response["message"])
}
```

### Integration Tests

```go
func TestUserAPI(t *testing.T) {
    // Start test server
    e := echo.New()
    // ... setup routes ...

    // Test HTTP requests
    req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("id")
    c.SetParamValues("1")

    // Execute request
    err := userHandler(c)
    assert.NoError(t, err)

    // Verify response
    assert.Equal(t, http.StatusOK, rec.Code)
    var response map[string]interface{}
    json.Unmarshal(rec.Body.Bytes(), &response)
    assert.True(t, response["success"].(bool))
}
```

## Deployment

### Building for Production

```bash
# Build optimized binary
go build -ldflags="-w -s" -o app cmd/app/main.go

# Or use the build script
./scripts/build.sh
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-w -s" -o main cmd/app/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080 9090
CMD ["./main"]
```

Build and run:

```bash
docker build -t myapp .
docker run -p 8080:8080 -p 9090:9090 myapp
```

### Environment Variables

Override configuration with environment variables:

```bash
export APP_DEBUG=false
export SERVER_PORT=3000
export MONITORING_PASSWORD=secure-password
export POSTGRES_PASSWORD=prod-password

go run cmd/app/main.go
```

## Best Practices

### Service Design

1. **Single Responsibility**: Each service should do one thing well
2. **Dependency Injection**: Inject infrastructure dependencies
3. **Error Handling**: Use consistent error responses
4. **Validation**: Always validate input data
5. **Logging**: Log important operations and errors

### API Design

1. **RESTful URLs**: Use consistent URL patterns
2. **HTTP Status Codes**: Use appropriate status codes
3. **JSON Responses**: Stick to the standard response format
4. **Versioning**: Include API versioning in URLs
5. **Documentation**: Document all endpoints

### Performance

1. **Caching**: Cache frequently accessed data
2. **Pagination**: Always paginate large datasets
3. **Async Operations**: Use async operations for slow tasks
4. **Connection Pooling**: Database connections are automatically pooled
5. **Indexes**: Add database indexes for performance

### Security

1. **Input Validation**: Validate all user inputs
2. **Authentication**: Implement proper authentication
3. **Authorization**: Check permissions for operations
4. **HTTPS**: Use HTTPS in production
5. **Secrets**: Never commit secrets to version control

## Service Discovery & Auto-Registration

### How Auto-Discovery Works

Stackyard uses an automatic service discovery system that eliminates the need for manual service registration:

1. **Package Import**: When a service package is imported, Go calls its `init()` function
2. **Factory Registration**: The `init()` function registers a service factory with the global registry
3. **Boot Process**: During application startup, the registry discovers all registered factories
4. **Service Creation**: Enabled services are automatically created and registered
5. **Route Registration**: Services register their routes with the Echo router

### Service Factory Pattern

The service factory function is the core of the auto-discovery system:

```go
func init() {
    registry.RegisterService("your_service", func(
        config *config.Config,
        logger *logger.Logger,
        deps *registry.Dependencies,
    ) interfaces.Service {
        // Service creation logic here
        return NewYourService(/* dependencies */)
    })
}
```

**Factory Function Parameters:**
- `config *config.Config` - Application configuration
- `logger *logger.Logger` - Structured logger instance
- `deps *registry.Dependencies` - Infrastructure dependencies

**Factory Function Responsibilities:**
- Create service instance with dependencies
- Check if service is enabled via `config.Services.IsEnabled()`
- Return `nil` if service should not be registered

### Service Lifecycle

1. **Import Phase**: Service packages are imported, `init()` functions execute
2. **Registration Phase**: Service factories are registered in global registry
3. **Discovery Phase**: Registry scans for all registered factories
4. **Creation Phase**: Enabled services are instantiated with dependencies
5. **Boot Phase**: Services register their routes and start operations
6. **Runtime Phase**: Services handle requests and perform business logic

### Service Dependencies

Services can declare dependencies on infrastructure components:

```go
type YourService struct {
    enabled bool
    db      *infrastructure.PostgresManager
    cache   *infrastructure.RedisManager
    logger  *logger.Logger
}

func NewYourService(
    db *infrastructure.PostgresManager,
    cache *infrastructure.RedisManager,
    logger *logger.Logger,
    enabled bool,
) *YourService {
    return &YourService{
        enabled: enabled,
        db:      db,
        cache:   cache,
        logger:  logger,
    }
}
```

**Available Dependencies:**
- `deps.Postgres` - PostgreSQL database manager
- `deps.Redis` - Redis cache manager
- `deps.MinIO` - Object storage manager
- `deps.Kafka` - Message queue manager
- `deps.MongoDB` - MongoDB database manager
- `deps.Grafana` - Monitoring dashboard manager

### Service Interface Requirements

All services must implement the `interfaces.Service` interface:

```go
type Service interface {
    Name() string                    // Human-readable service name
    WireName() string               // Dependency injection name
    Enabled() bool                  // Whether service is enabled
    Endpoints() []string            // List of endpoint patterns
    RegisterRoutes(g *echo.Group)   // Register API routes
    Get() interface{}               // Return service instance
}
```

### Service Naming Conventions

- **Service Name**: Human-readable name (e.g., "User Service")
- **Wire Name**: Dependency injection identifier (e.g., "user-service")
- **Config Key**: YAML configuration key (e.g., "user_service")
- **Package Name**: Go package name (e.g., "users_service")

### Debugging Service Discovery

If a service isn't registering properly:

1. **Check Package Import**: Ensure the service package is imported
2. **Verify init() Function**: Confirm the `init()` function exists and calls `registry.RegisterService()`
3. **Check Configuration**: Verify the service is enabled in `config.yaml`
4. **Review Dependencies**: Ensure all required dependencies are available
5. **Examine Logs**: Check application logs for service registration messages

### Service Registration Order

Services are registered in the order they are discovered, which depends on:
- Package import order
- `init()` function execution order
- Dependency availability

**Best Practice**: Design services to be independent of registration order when possible.

## Troubleshooting

### Common Development Issues

**Service not registering:**
- Check that the service package is imported (no manual registration needed)
- Verify the `init()` function calls `registry.RegisterService()`
- Ensure the service is enabled in `config.yaml`
- Check for compilation errors in the service package

**Database connection errors:**
- Verify database credentials
- Check network connectivity
- Ensure database server is running
- Confirm dependency injection is working correctly

**API validation errors:**
- Check request JSON structure
- Verify validation tags on struct fields
- Test with valid/invalid data
- Review custom validator implementations

**Performance issues:**
- Add database indexes
- Implement caching
- Check for N+1 query problems
- Monitor memory usage
- Review dependency injection overhead

## Next Steps

Now that you understand how to develop with Stackyard, explore:

- **[Architecture Overview](ARCHITECTURE.md)** - Deep dive into the technical design
- **[API Reference](REFERENCE.md)** - Complete technical documentation

