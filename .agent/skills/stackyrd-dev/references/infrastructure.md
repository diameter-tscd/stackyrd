# Adding an Infrastructure Component

Infrastructure components wrap external system clients (databases, message queues, storage, monitoring) and provide a consistent lifecycle: connect, health-check, close. They're **auto-registered** via `init()` in files under `pkg/infrastructure/`.

## Interface Requirements

Every component implements `InfrastructureComponent` (`pkg/infrastructure/component.go`):

```go
type InfrastructureComponent interface {
    Name() string                    // Display name for logging and TUI
    Close() error                    // Graceful shutdown
    GetStatus() map[string]interface{}  // Health check data (returned by /health endpoints)
}
```

Components are registered using a `ComponentFactory`:

```go
type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
```

Return `nil, nil` when the component is disabled (factory is still called but shouldn't initialize).

## File Template

Create `pkg/infrastructure/{name}.go`:

```go
package infrastructure

import (
    "fmt"
    "sync"
    "time"

    "stackyrd/config"
    "stackyrd/pkg/logger"
)

type {Name}Manager struct {
    client      interface{}   // Actual client (e.g., *somepkg.Client)
    connected   bool
    statusTTL    time.Duration
    statusExpiry time.Time
    statusCache  map[string]interface{}
    statusMu     sync.Mutex
}

func (m *{Name}Manager) Name() string {
    return "{Name}"
}

func New{Name}(cfg config.{Name}Config, l *logger.Logger) (*{Name}Manager, error) {
    if !cfg.Enabled {
        return nil, nil
    }

    l.Info("Connecting to {Name}", "endpoint", cfg.Endpoint)

    // Create client connection with timeout
    // ...

    return &{Name}Manager{
        // client: client,
        connected: true,
        statusTTL: 2 * time.Second,
    }, nil
}

func (m *{Name}Manager) GetStatus() map[string]interface{} {
    stats := make(map[string]interface{})
    if m == nil || !m.connected {
        stats["connected"] = false
        return stats
    }

    // Use TTL-cached result when possible (same pattern as MongoManager)
    m.statusMu.Lock()
    if time.Now().Before(m.statusExpiry) && m.statusCache != nil {
        cached := m.statusCache
        m.statusMu.Unlock()
        return cached
    }
    m.statusMu.Unlock()

    // Slow path: actually check health
    err := m.ping()
    stats["connected"] = err == nil
    if err != nil {
        stats["error"] = err.Error()
    }

    m.statusMu.Lock()
    m.statusCache = stats
    m.statusExpiry = time.Now().Add(m.statusTTL)
    m.statusMu.Unlock()

    return stats
}

func (m *{Name}Manager) Close() error {
    if m.client != nil {
        // return m.client.Close()
    }
    return nil
}

func (m *{Name}Manager) ping() error {
    // Return nil if healthy, error if not
    return nil
}

func init() {
    RegisterComponent("{name}", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
        return New{Name}(cfg.{ConfigSection}, log)
    })
}
```

## Config Setup

### 1. Add config struct in `config/config.go`

```go
type {Name}Config struct {
    Enabled  bool   `mapstructure:"enabled"`
    Endpoint string `mapstructure:"endpoint"`
    // Add fields as needed
}
```

Add the field to the main `Config` struct:

```go
type Config struct {
    // ... existing fields
    {Name} {Name}Config `mapstructure:"{name_in_yaml}"`
}
```

### 2. Add defaults in `setupViperDefaults()`:

```go
viper.SetDefault("{name_in_yaml}.enabled", false)
```

### 3. Add to `config.yaml`:

```yaml
{name_in_yaml}:
  enabled: true
  endpoint: "localhost:9999"
```

## Accessing the Component from Services

Components are automatically populated into the `*registry.Dependencies` bag. Services access them in their factory:

```go
func init() {
    registry.RegisterService("my_service", func(cfg *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := NewMyService(cfg.Services.IsEnabled("my_service"), logger)
        if comp, ok := deps.Get("{name}"); ok {
            svc.{Name} = comp.(*infrastructure.{Name}Manager)
        }
        return svc
    })
}
```

## Real Examples from the Codebase

| File | Pattern | Key Details |
|------|---------|-------------|
| `mongo.go` | Multi-connection manager with health caching, async ops, worker pool, batch operations | Demonstrates TTL-cached status, AsyncResult pattern, multi-connection support |
| `redis.go` | Sync/async/batch client wrapper | Multiple access patterns (direct, async, batch) |
| `kafka.go` | Producer/consumer lifecycle | Async message handling |
| `postgres.go` | Multi-connection with GORM | GORM + raw SQL, connection manager pattern |
| `minio.go` | S3-compatible storage client | File upload/download operations with progress tracking |
| `grafana.go` | HTTP API client | REST API wrapper pattern |

## Key Points

- **Return `nil, nil` when disabled** — the registry expects `nil` components to be silently skipped
- **TTL-cached status** follows the same pattern as `MongoManager.GetStatus()` — ping the service, cache for 2 seconds to avoid hammering health endpoints
- **Async initialization** is handled automatically by `InfraInitManager` — your component just needs to be registered
- **Multi-connection pattern**: if the external system supports multiple connections (like Postgres and Mongo), implement a `{Name}ConnectionManager` wrapper with named connections
- **Thread safety**: use `sync.Mutex` or `sync.RWMutex` for any shared state used in `GetStatus()` / `Close()`
- **Graceful shutdown**: `Close()` will be called during server shutdown with a 10-second timeout per component
