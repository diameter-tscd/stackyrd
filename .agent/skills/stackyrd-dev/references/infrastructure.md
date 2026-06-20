# Adding an Infrastructure Component

Infrastructure components wrap external system clients and provide a consistent lifecycle. They're **auto-registered** via `init()` in files under `pkg/infrastructure/`.

## Interface Requirements

```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}
```

Components use a `ComponentFactory`:

```go
type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
```

Return `nil, nil` when the component is disabled.

## File Template

Create `pkg/infrastructure/{name}.go`:

```go
package infrastructure

import (
    "sync"
    "time"

    "stackyrd-nano/config"
    "stackyrd-nano/pkg/logger"
)

type {Name}Manager struct {
    client       interface{}
    connected    bool
    statusTTL    time.Duration
    statusExpiry time.Time
    statusCache  map[string]interface{}
    statusMu     sync.Mutex
}

func (m *{Name}Manager) Name() string { return "{Name}" }

func New{Name}(cfg config.{Name}Config, l *logger.Logger) (*{Name}Manager, error) {
    if !cfg.Enabled {
        return nil, nil
    }
    return &{Name}Manager{
        connected: true,
        statusTTL: 2 * time.Second,
    }, nil
}

func (m *{Name}Manager) GetStatus() map[string]interface{} {
    if m == nil || !m.connected {
        return map[string]interface{}{"connected": false}
    }
    return map[string]interface{}{"connected": true}
}

func (m *{Name}Manager) Close() error { return nil }

func init() {
    RegisterComponent("{name}", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
        return New{Name}(cfg.{ConfigSection}, log)
    })
}
```

## Config Setup

Add to `config/config.go`:

```go
type {Name}Config struct {
    Enabled  bool   `mapstructure:"enabled"`
    Endpoint string `mapstructure:"endpoint"`
}
```

Add to `Config` struct and `setupViperDefaults()`.

Add to `config.yaml`:

```yaml
{name_in_yaml}:
  enabled: true
  endpoint: "localhost:9999"
```

## Accessing from Services

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

## Existing Components

| File | Description |
|------|-------------|
| `postgres.go` | Multi-connection PostgreSQL with GORM + raw SQL |

## Key Points

- **Return `nil, nil` when disabled** — the registry silently skips nil components
- **TTL-cached status** — ping the service, cache for 2 seconds
- **Async initialization** — handled by `InfraInitManager`
- **Thread safety** — use `sync.Mutex` for shared state in `GetStatus()` / `Close()`
- **Graceful shutdown** — `Close()` is called with a 10-second timeout per component
