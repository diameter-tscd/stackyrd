# Getting Started with Stackyard

This guide will get you up and running with Stackyard in minutes. Stackyard is a production-ready Go application framework with modular services, real-time monitoring, and extensive infrastructure integrations.

## Quick Start

### 1. Prerequisites

- **Go 1.21+** - [Download here](https://golang.org/dl/)
- **Git** - For cloning the repository

### 2. Installation

```bash
# Clone the repository
git clone https://github.com/diameter-tscd/stackyard.git
cd stackyard

# Install Go dependencies
go mod download

# Run the application
go run cmd/app/main.go
```

### 3. First Access

Open your browser and visit:
- **Monitoring Dashboard**: http://localhost:9090
- **Login**: `admin` / `admin` (Change this immediately)

## Basic Configuration

### Essential Settings

Edit `config.yaml` to configure your application:

```yaml
app:
  name: "My Application"     # Your app name
  debug: true                # Enable for development

server:
  port: "8080"              # API server port

monitoring:
  enabled: true             # Web monitoring dashboard
  port: "9090"              # Dashboard port
  password: "your-secure-password"  # ⚠️ Change from default!
```

### Service Configuration

Enable/disable built-in services:

```yaml
services:
  service_a: true   # Basic CRUD API example
  service_b: false  # Disable if not needed
```

## Hello World Example

Let's create a simple API endpoint:

### 1. Create a Service

Add to `internal/services/modules/service_hello.go`:

```go
package modules

import (
    "stackyard/pkg/response"
    "github.com/labstack/echo/v4"
)

type HelloService struct {
    enabled bool
}

func NewHelloService(enabled bool) *HelloService {
    return &HelloService{enabled: enabled}
}

func (s *HelloService) Name() string        { return "Hello Service" }
func (s *HelloService) Enabled() bool       { return s.enabled }
func (s *HelloService) Endpoints() []string { return []string{"/hello"} }

func (s *HelloService) RegisterRoutes(g *echo.Group) {
    g.GET("/hello", s.hello)
}

func (s *HelloService) hello(c echo.Context) error {
    return response.Success(c, map[string]string{
        "message": "Hello, World!",
        "status":  "running",
    }, "Hello endpoint")
}
```

### 2. Register the Service

Add to `internal/server/server.go`:

```go
registry.Register(modules.NewHelloService(s.config.Services.IsEnabled("hello")))
```

### 3. Enable in Config

Add to `config.yaml`:

```yaml
services:
  hello: true
```

### 4. Test Your API

```bash
# Test the endpoint
curl http://localhost:8080/api/v1/hello

# Response:
{
  "success": true,
  "message": "Hello endpoint",
  "data": {
    "message": "Hello, World!",
    "status": "running"
  },
  "timestamp": 1642598400
}
```

## Database Setup (Optional)

### PostgreSQL Quick Setup

Using Docker for development:

```bash
# Start PostgreSQL
docker run -d \
  --name postgres \
  -e POSTGRES_PASSWORD=mypassword \
  -p 5432:5432 \
  postgres:15

# Configure in config.yaml
postgres:
  enabled: true
  host: "localhost"
  user: "postgres"
  password: "mypassword"
  dbname: "postgres"
```

### Redis Quick Setup

```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:7

# Configure in config.yaml
redis:
  enabled: true
  address: "localhost:6379"
```

## Monitoring & Debugging

### Web Dashboard

Access the monitoring dashboard at http://localhost:9090 to:
- View real-time system metrics
- Monitor API endpoints
- Check service health
- View application logs

### Terminal UI

For interactive monitoring, enable the TUI:

```yaml
app:
  enable_tui: true
```

The TUI provides:
- Real-time log viewing
- Service initialization status
- System resource monitoring
- Interactive controls

### Common Issues

**"Port already in use"**
```bash
# Find what's using the port
lsof -i :8080

# Kill the process
kill -9 <PID>
```

**"Connection refused to database"**
- Check if Docker containers are running: `docker ps`
- Verify database credentials in `config.yaml`
- Wait for database to fully start up

**"Module not found"**
```bash
# Clean and reinstall dependencies
go clean -modcache
go mod download
```


### What to Explore Next:

1. **[Development Guide](DEVELOPMENT.md)** - Learn to add features and extend the app
2. **[Architecture Overview](ARCHITECTURE.md)** - Understand the technical design
3. **[API Reference](REFERENCE.md)** - Complete technical documentation

### Useful Commands:

```bash
# Development
go run cmd/app/main.go              # Run in development mode
go build -o app cmd/app/main.go     # Build binary

# Docker
./scripts/docker_build.sh           # Build Docker images
docker-compose up                   # Run with full stack

# Testing
go test ./...                       # Run all tests
```

