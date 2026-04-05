# Technical Reference

This comprehensive reference covers all configuration options, API specifications, advanced features, and technical details for stackyrd. Use this as your complete technical resource.

## Configuration Reference

### Complete Configuration Schema

```yaml
# Application Configuration
app:
  name: "stackyrd"         # Application display name
  version: "1.0.0"          # Application version
  debug: true               # Enable debug logging
  env: "development"        # Environment (development, staging, production)
  banner_path: "banner.txt" # Path to startup banner file
  startup_delay: 3          # Seconds to display boot screen (0 to skip)
  quiet_startup: true       # Suppress console logs during startup (TUI only)
  enable_tui: true          # Enable Terminal User Interface

# Server Configuration
server:
  port: "8080"              # HTTP server port

# Service Configuration
services:
  users_service: true       # User management service
  broadcast_service: false  # Event broadcasting service
  cache_service: true       # Redis caching service
  encryption_service: false # API encryption service
  grafana_service: false    # Grafana integration service
  mongodb_service: true     # MongoDB multi-tenant service
  multi_tenant_service: true # Multi-tenant PostgreSQL service
  products_service: true    # Product catalog service
  tasks_service: true       # Task management service

# Authentication
auth:
  type: "apikey"            # Authentication type (apikey, basic, none)
  secret: "super-secret-key" # API key for authentication

# Redis Configuration
redis:
  enabled: false            # Enable Redis
  address: "localhost:6379" # Redis server address
  password: ""              # Redis password (optional)
  db: 0                     # Redis database number

# Kafka Configuration
kafka:
  enabled: false            # Enable Kafka
  brokers:                  # List of Kafka brokers
    - "localhost:9092"
  topic: "my-topic"         # Default topic
  group_id: "my-group"      # Consumer group ID

# PostgreSQL Multi-Connection Configuration
postgres:
  enabled: true
  connections:
    - name: "primary"        # Connection identifier
      enabled: true          # Enable this connection
      host: "localhost"
      port: 5432
      user: "postgres"
      password: "Mypostgres01"
      dbname: "postgres"
      sslmode: "disable"
    - name: "secondary"
      enabled: true
      host: "localhost"
      port: 5433
      user: "postgres"
      password: "Mypostgres01"
      dbname: "postgres"
      sslmode: "disable"

# MongoDB Multi-Connection Configuration
mongo:
  enabled: true
  connections:
    - name: "primary"
      enabled: true
      uri: "mongodb://localhost:27017"
      database: "primary_db"
    - name: "secondary"
      enabled: true
      uri: "mongodb://localhost:27018"
      database: "secondary_db"

# Monitoring Configuration
monitoring:
  enabled: true             # Enable web monitoring dashboard
  port: "9090"              # Monitoring dashboard port
  password: "admin"         # Dashboard login password
  obfuscate_api: true       # Enable API response obfuscation
  title: "stackyrd"        # Dashboard title
  subtitle: "Monitoring Dashboard" # Dashboard subtitle
  max_photo_size_mb: 2      # Maximum photo upload size
  upload_dir: "web/monitoring/uploads" # Upload directory

# MinIO Configuration (Nested under monitoring)
monitoring:
  minio:
    enabled: true           # Enable MinIO integration
    endpoint: "localhost:9003" # MinIO server endpoint
    access_key_id: "minioadmin" # MinIO access key
    secret_access_key: "minioadmin" # MinIO secret key
    use_ssl: false          # Use SSL for MinIO connection
    bucket_name: "main"     # Default bucket name

# External Services Configuration (Nested under monitoring)
monitoring:
  external:
    services:               # List of external services to monitor
      - name: "Google"
        url: "https://google.com"
      - name: "Local API"
        url: "http://localhost:8080/health"

# Cron Jobs Configuration
cron:
  enabled: true             # Enable scheduled jobs
  jobs:
    log_cleanup: "0 0 * * *" # Daily log cleanup at midnight
    health_check: "*/10 * * * *" # Health check every 10 seconds

# Encryption Configuration
encryption:
  enabled: false            # Enable API encryption
  algorithm: "aes-256-gcm"  # Encryption algorithm
  key: ""                   # 32-byte encryption key (base64 encoded)
  rotate_keys: false        # Enable automatic key rotation
  key_rotation_interval: "24h" # Key rotation interval

# Grafana Integration
grafana:
  enabled: true             # Enable Grafana integration
  url: "http://localhost:3000" # Grafana server URL
  api_key: "your-grafana-api-key" # Grafana API key
  username: "admin"         # Grafana username (alternative to API key)
  password: "admin"         # Grafana password (alternative to API key)
```

## API Specifications

### Response Format

All API responses follow this standardized structure:

```json
{
  "success": true,                    // Boolean: operation success status
  "message": "Operation completed",   // String: human-readable message
  "data": {                           // Object: response payload (varies by endpoint)
    "key": "value"
  },
  "meta": {                           // Object: pagination metadata (optional)
    "page": 1,
    "per_page": 10,
    "total": 100,
    "total_pages": 10
  },
  "timestamp": 1642598400             // Number: Unix timestamp
}
```

### Error Response Format

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",             // String: machine-readable error code
    "message": "Human readable error", // String: human-readable error message
    "details": {                      // Object: additional error details (optional)
      "field": "validation error"
    }
  },
  "timestamp": 1642598400
}
```

### Standard Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request parameters |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Access denied |
| `NOT_FOUND` | 404 | Resource not found |
| `ENDPOINT_NOT_FOUND` | 404 | API endpoint doesn't exist |
| `CONFLICT` | 409 | Resource conflict |
| `VALIDATION_ERROR` | 422 | Request validation failed |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

## API Endpoints Reference

### Users Service - `/api/v1/users`

#### GET `/api/v1/users`
List users with pagination.

**Query Parameters:**
- `page` (integer, optional): Page number (default: 1)
- `per_page` (integer, optional): Items per page (default: 10, max: 100)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "username": "john_doe",
      "email": "john@example.com",
      "status": "active",
      "created_at": 1704067200
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

#### POST `/api/v1/users`
Create a new user.

**Request Body:**
```json
{
  "username": "jane_doe",
  "email": "jane@example.com",
  "full_name": "Jane Doe"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "123",
    "username": "jane_doe",
    "email": "jane@example.com",
    "status": "active",
    "created_at": 1704067200
  }
}
```

#### GET `/api/v1/users/:id`
Get a specific user.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "1",
    "username": "john_doe",
    "email": "john@example.com",
    "status": "active",
    "created_at": 1704067200
  }
}
```

#### PUT `/api/v1/users/:id`
Update a user.

**Request Body:**
```json
{
  "username": "john_smith",
  "email": "johnsmith@example.com",
  "status": "inactive"
}
```

#### DELETE `/api/v1/users/:id`
Delete a user.

**Response:**
```json
{
  "success": true,
  "message": "User deleted successfully"
}
```

### Products Service - `/api/v1/products`

#### GET `/api/v1/products`
Get product catalog information.

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "Hello from Service B - Products"
  }
}
```

### MongoDB Service - `/api/v1/products/{tenant}`

#### GET `/api/v1/products/{tenant}`
List products for a specific tenant database.

**Path Parameters:**
- `tenant` (string): Tenant identifier (maps to MongoDB database name)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "_id": "507f1f77bcf86cd799439011",
      "name": "Laptop",
      "description": "Gaming laptop",
      "price": 1299.99,
      "category": "electronics",
      "in_stock": true,
      "quantity": 5,
      "tags": ["gaming", "laptop"]
    }
  ]
}
```

#### POST `/api/v1/products/{tenant}`
Create a product in tenant database.

**Request Body:**
```json
{
  "name": "Smartphone",
  "description": "Latest smartphone",
  "price": 899.99,
  "category": "electronics",
  "quantity": 10,
  "tags": ["mobile", "phone"]
}
```

#### GET `/api/v1/products/{tenant}/{id}`
Get specific product by ID.

#### PUT `/api/v1/products/{tenant}/{id}`
Update product.

#### DELETE `/api/v1/products/{tenant}/{id}`
Delete product.

#### GET `/api/v1/products/{tenant}/search`
Search products with filters.

**Query Parameters:**
- `name` (string): Product name filter
- `category` (string): Category filter
- `in_stock` (boolean): Stock status filter
- `min_price` (number): Minimum price
- `max_price` (number): Maximum price
- `tags` (string): Comma-separated tags

#### GET `/api/v1/products/{tenant}/analytics`
Get product analytics by category.

**Response:**
```json
{
  "success": true,
  "data": {
    "total_products": 150,
    "in_stock_products": 120,
    "out_of_stock": 30,
    "category_breakdown": [
      {
        "_id": "electronics",
        "total_products": 80,
        "avg_price": 450.50,
        "min_price": 50.00,
        "max_price": 2000.00,
        "total_quantity": 500,
        "in_stock_count": 450
      }
    ]
  }
}
```

### Broadcast Service - `/api/v1/events`

#### GET `/api/v1/events/stream/{stream_id}`
Subscribe to event stream (Server-Sent Events).

#### POST `/api/v1/events/broadcast`
Broadcast an event to all subscribers.

**Request Body:**
```json
{
  "type": "user_action",
  "message": "User logged in",
  "data": {
    "user_id": "123"
  }
}
```

#### GET `/api/v1/events/streams`
Get active stream information.

### Cache Service - `/api/v1/cache`

#### GET `/api/v1/cache/{key}`
Get cached value.

#### POST `/api/v1/cache/{key}`
Set cached value.

**Request Body:**
```json
{
  "value": "cached data",
  "ttl": 3600
}
```

#### DELETE `/api/v1/cache/{key}`
Delete cached value.

### Grafana Service - `/api/v1/grafana`

#### POST `/api/v1/grafana/dashboards`
Create a Grafana dashboard.

#### GET `/api/v1/grafana/dashboards`
List dashboards.

#### GET `/api/v1/grafana/health`
Get Grafana health status.

### Health Endpoints

#### GET `/health`
Application health check.

**Response:**
```json
{
  "status": "ok",
  "server_ready": true,
  "infrastructure": {
    "postgres": {"initialized": true},
    "redis": {"initialized": false}
  },
  "initialization_progress": 0.8
}
```

#### GET `/health/infrastructure`
Detailed infrastructure health.

#### GET `/health/services`
Service-specific health status.

## Request Validation

### Built-in Validators

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field must not be empty | `validate:"required"` |
| `email` | Valid email format | `validate:"required,email"` |
| `min=X` | Minimum string length | `validate:"min=3"` |
| `max=X` | Maximum string length | `validate:"max=100"` |
| `gte=X` | Greater than or equal (numeric) | `validate:"gte=18"` |
| `lte=X` | Less than or equal (numeric) | `validate:"lte=120"` |
| `oneof=X Y Z` | Value must be one of listed | `validate:"oneof=low medium high"` |

### Custom Validators

#### Phone Number
```go
validate.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
    phone := fl.Field().String()
    matched, _ := regexp.MatchString(`^\+?[1-9]\d{1,14}$`, phone)
    return matched
})
```
Usage: `validate:"phone"`

#### Username
```go
validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
    username := fl.Field().String()
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9]{3,20}$`, username)
    return matched
})
```
Usage: `validate:"username"`

## Database Schemas

### PostgreSQL Tables

#### Users Table (GORM Auto-Migration)
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);
```

### MongoDB Collections

#### Products Collection
```javascript
{
  "_id": ObjectId("507f1f77bcf86cd799439011"),
  "name": "Laptop",
  "description": "Gaming laptop",
  "price": 1299.99,
  "category": "electronics",
  "in_stock": true,
  "quantity": 5,
  "tags": ["gaming", "laptop"],
  "created_at": ISODate("2024-01-01T00:00:00Z")
}
```

#### Analytics Aggregation Pipeline
```javascript
// Product analytics by category
[
  {
    "$group": {
      "_id": "$category",
      "total_products": {"$sum": 1},
      "avg_price": {"$avg": "$price"},
      "min_price": {"$min": "$price"},
      "max_price": {"$max": "$price"},
      "total_quantity": {"$sum": "$quantity"},
      "in_stock_count": {
        "$sum": {"$cond": ["$in_stock", 1, 0]}
      }
    }
  },
  {
    "$sort": {"total_products": -1}
  }
]
```

## Infrastructure Managers

### AsyncResult Types

All async operations return `AsyncResult[T]` types:

```go
type AsyncResult[T any] struct {
    Value T
    Error error
    Done  chan struct{}
}

// Methods
func (ar *AsyncResult[T]) Wait() (T, error)
func (ar *AsyncResult[T]) WaitWithTimeout(timeout time.Duration) (T, error)
func (ar *AsyncResult[T]) IsDone() bool
```

### Worker Pool Configuration

| Infrastructure | Default Workers | Purpose |
|----------------|-----------------|---------|
| Redis | 10 | Cache operations |
| Kafka | 5 | Message publishing |
| PostgreSQL | 15 | Database queries |
| MongoDB | 12 | Document operations |
| MinIO | 8 | File uploads |
| Cron | 5 | Scheduled jobs |

### Connection Pool Settings

#### PostgreSQL Multi-Connection
```yaml
postgres:
  connections:
    - name: "primary"
      max_open_conns: 10    # Maximum open connections
      max_idle_conns: 5     # Maximum idle connections
      conn_max_lifetime: "1h" # Connection max lifetime
    - name: "secondary"
      max_open_conns: 8     # Secondary connection pool
      max_idle_conns: 3
      conn_max_lifetime: "1h"
```

#### Redis
```yaml
redis:
  pool_size: 10           # Connection pool size
  min_idle_conns: 2       # Minimum idle connections
  conn_max_lifetime: "1h" # Connection max lifetime
  idle_timeout: "10m"     # Idle connection timeout
```

#### MongoDB
```yaml
mongo:
  connections:
    - name: "primary"
      max_pool_size: 100      # Maximum connection pool size
      min_pool_size: 10       # Minimum connection pool size
      max_idle_time: "10m"    # Maximum idle time
      connect_timeout: "30s"  # Connection timeout
    - name: "secondary"
      max_pool_size: 50
      min_pool_size: 5
      max_idle_time: "10m"
      connect_timeout: "30s"
```

### Infrastructure Connection Management

#### PostgreSQL Connection Manager
```go
// Get tenant-specific connection
conn, exists := postgresManager.GetConnection("tenant_a")
if exists {
    // Use tenant_a database
    result := conn.ORM.Where("tenant_id = ?", "tenant_a").Find(&data)
}

// List all available connections
connections := postgresManager.ListConnections()
for name, conn := range connections {
    fmt.Printf("Connection %s: %s\n", name, conn.Status())
}
```

#### MongoDB Connection Manager
```go
// Get tenant-specific database
db, exists := mongoManager.GetConnection("tenant_b")
if exists {
    // Use tenant_b database
    cursor, err := db.Collection("products").Find(context.Background(), bson.M{})
}

// List all available databases
databases := mongoManager.ListConnections()
for name, db := range databases {
    fmt.Printf("Database %s: %s\n", name, db.Name())
}
```

#### Redis Connection Manager
```go
// Basic operations
err := redisManager.Set(ctx, "key", "value", time.Hour)
value, err := redisManager.Get(ctx, "key")

// Hash operations
err = redisManager.HSet(ctx, "user:123", "name", "John")
name, err := redisManager.HGet(ctx, "user:123", "name")

// List operations
err = redisManager.LPush(ctx, "queue", "item1")
item, err := redisManager.LPop(ctx, "queue")
```

## Security Features

### API Obfuscation

**Configuration:**
```yaml
monitoring:
  obfuscate_api: true
```

**How it works:**
- Intercepts `/api/*` requests
- Base64 encodes JSON responses
- Sets `X-Obfuscated: true` header
- Excludes streaming endpoints

### API Encryption

**Configuration:**
```yaml
encryption:
  enabled: true
  algorithm: "aes-256-gcm"
  key: "32-byte-base64-encoded-key"
```

**Features:**
- AES-256-GCM authenticated encryption
- Automatic request/response encryption
- Key rotation support
- Client-side decryption utilities

### Authentication

**API Key Authentication:**
```yaml
auth:
  type: "apikey"
  secret: "your-secret-key"
```

**Usage:**
```bash
curl -H "X-API-Key: your-secret-key" http://localhost:8080/api/v1/users
```

## Monitoring & Observability

### Web Dashboard Endpoints

| Endpoint | Description |
|----------|-------------|
| `/` | Main dashboard |
| `/logs` | Real-time logs |
| `/postgres` | Database management |
| `/infrastructure` | Infrastructure status |
| `/config` | Configuration viewer |

### Terminal UI Controls

**Boot Sequence:**
- `q` - Skip countdown and continue
- `Ctrl+C` - Quit application

**Live Logs:**
- `↑/↓` - Scroll up/down
- `Page Up/Down` - Page navigation
- `Home/End` - Jump to top/bottom
- `/` - Open filter dialog
- `F1` - Toggle auto-scroll
- `F2` - Clear all logs
- `q/Esc` - Exit TUI

### Log Levels

- `DEBUG` - Detailed debugging information
- `INFO` - General information messages
- `WARN` - Warning messages
- `ERROR` - Error conditions
- `FATAL` - Critical errors that cause termination

## Build & Deployment

### Build Scripts

#### Unix/Linux/macOS (`scripts/build.sh`)
```bash
# Interactive build
./scripts/build.sh

# With obfuscation
echo "y" | ./scripts/build.sh

# Automated (no obfuscation)
echo "n" | ./scripts/build.sh
```

#### Windows (`scripts/build.bat`)
```batch
scripts\build.bat
```

### Docker Build Scripts

#### Build Options
```bash
# Build all stages
./scripts/docker_build.sh

# Build specific target
./scripts/docker_build.sh "myapp" "registry.com/myapp" "prod"

# Available targets: test, dev, prod, prod-slim, prod-minimal, ultra-prod
```

### Environment Variables

Override configuration at runtime:

```bash
# Application settings
export APP_DEBUG=true
export APP_ENABLE_TUI=false

# Server settings
export SERVER_PORT=3000

# Database settings
export POSTGRES_HOST=prod-db.example.com
export POSTGRES_PASSWORD=secure-password

# Monitoring
export MONITORING_PASSWORD=admin-password
```

## Advanced Features

### Multi-Tenant Architecture

**Database Switching:**
```go
// Get tenant-specific connection
conn, exists := postgresManager.GetConnection("tenant_a")
if exists {
    // Use tenant_a database
    result := conn.ORM.Where("tenant_id = ?", "tenant_a").Find(&data)
}
```

**Tenant Isolation:**
- Separate database connections per tenant
- Automatic tenant ID injection
- Isolated data access patterns

### Event Streaming

**Server-Sent Events:**
```javascript
const eventSource = new EventSource('/api/v1/events/stream/notifications');

eventSource.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Event:', data.type, data.message);
};
```

**Broadcasting:**
```go
// Broadcast to specific stream
broadcaster.Broadcast("notifications", "alert", "System alert", alertData)

// Broadcast to all streams
broadcaster.BroadcastToAll("global", "Global announcement", globalData)
```

### Cron Job Scheduling

**Job Definition:**
```yaml
cron:
  enabled: true
  jobs:
    cleanup: "0 0 * * *"        # Daily at midnight
    health_check: "*/5 * * * *"  # Every 5 minutes
    backup: "0 3 * * 1"         # Weekly backup (Monday 3 AM)
```

**Programmatic Jobs:**
```go
cronManager.AddJob("custom-job", "0 */2 * * *", func() {
    // Run every 2 hours
    performMaintenance()
})
```

## Performance Tuning

### Worker Pool Sizing

Adjust based on load:

```yaml
infrastructure:
  redis:
    workers: 20      # High cache load
  postgres:
    workers: 25      # High database load
  kafka:
    workers: 10      # Moderate message load
```

### Connection Pool Optimization

```yaml
postgres:
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime: "30m"

redis:
  pool_size: 15
  min_idle_conns: 5
  conn_max_lifetime: "1h"
```

### Memory Management

- **Log rotation**: Automatic cleanup prevents memory leaks
- **Connection pooling**: Reuses connections efficiently
- **Async operations**: Prevents blocking and resource exhaustion

## Troubleshooting

### Common Issues

**"Port already in use"**
```bash
# Find process using port
lsof -i :8080
# Kill process
kill -9 <PID>
```

**"Database connection refused"**
```bash
# Check if database is running
docker ps | grep postgres

# Test connection
psql -h localhost -U postgres -d stackyrd
```

**"Service not registering"**
- Verify service is enabled in `config.yaml`
- Check for compilation errors in service code
- Ensure service implements the `Service` interface correctly

**"Async operation timeout"**
- Increase timeout values in configuration
- Check worker pool sizing
- Monitor system resources

### Debug Mode

Enable detailed logging:

```yaml
app:
  debug: true
```

### Health Checks

Monitor system health:

```bash
# Application health
curl http://localhost:8080/health

# Infrastructure health
curl http://localhost:8080/health/infrastructure

# Service-specific health
curl http://localhost:8080/health/services
```

## Migration Guide

### From Single DB to Multi-Tenant

1. **Update Configuration:**
   ```yaml
   postgres:
     enabled: true
     connections:
       - name: "tenant_a"
         host: "db-tenant-a"
       - name: "tenant_b"
         host: "db-tenant-b"
   ```

2. **Update Services:**
   ```go
   // Before
   s.db.Find(&users)

   // After
   conn, _ := s.postgresManager.GetConnection(tenantID)
   conn.ORM.Find(&users)
   ```

3. **Add Tenant Context:**
   - Inject tenant ID into requests
   - Validate tenant access permissions
   - Update data models for tenant isolation

### From Monolithic to Microservices

1. **Extract Services:**
   - Identify service boundaries
   - Create separate service repositories
   - Implement inter-service communication

2. **Shared Infrastructure:**
   - Use shared databases with tenant isolation
   - Implement service discovery
   - Configure centralized logging

3. **Deployment Updates:**
   - Create separate Docker images
   - Update orchestration (Kubernetes, Docker Compose)
   - Configure load balancing

## API Versioning

### URL-based Versioning

```
GET /api/v1/users      # Version 1
GET /api/v2/users      # Version 2 (future)
```

### Header-based Versioning

```
Accept: application/vnd.stackyrd.v1+json
Accept: application/vnd.stackyrd.v2+json
```

### Deprecation Strategy

1. **Announce deprecation** in response headers
2. **Maintain backward compatibility** for N versions
3. **Provide migration guides** for breaking changes
4. **Sunset old versions** with clear timelines

## Integration Patterns

### External API Integration

```go
type ExternalAPIClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (c *ExternalAPIClient) MakeRequest(endpoint string, data interface{}) error {
    // Implement retry logic
    // Handle rate limiting
    // Parse responses
}
```

### Webhook Handling

```go
func (s *WebhookService) HandleWebhook(c *gin.Context) error {
    // Verify webhook signature
    signature := c.Request().Header.Get("X-Signature")
    if !s.verifySignature(c.Request().Body, signature) {
        return response.Forbidden(c, "Invalid signature")
    }

    // Process webhook payload
    var payload WebhookPayload
    if err := c.Bind(&payload); err != nil {
        return response.BadRequest(c, "Invalid payload")
    }

    // Queue for processing
    s.queue.ProcessAsync(payload)
    return response.Success(c, nil, "Webhook received")
}
```

### File Upload Handling

```go
func (s *UploadService) HandleUpload(c *gin.Context) error {
    file, err := c.FormFile("file")
    if err != nil {
        return response.BadRequest(c, "No file provided")
    }

    // Validate file size/type
    if file.Size > s.maxFileSize {
        return response.BadRequest(c, "File too large")
    }

    // Upload to storage
    result, err := s.storage.UploadFile(context.Background(),
        fmt.Sprintf("uploads/%s", file.Filename),
        file, file.Size, file.Header.Get("Content-Type"))

    return response.Created(c, map[string]interface{}{
        "filename": file.Filename,
        "url":      s.storage.GetFileUrl(result.Key),
    }, "File uploaded")
}
```

This technical reference provides comprehensive coverage of stackyrd's capabilities, configuration options, and implementation details. Use this document as your authoritative source for all technical aspects of the framework.
