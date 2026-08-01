# Adding an Infrastructure Component

Create `pkg/infrastructure/{name}.go` (package `infrastructure`). Implement `InfrastructureComponent`, register via `init()`.

## Interface

```go
type InfrastructureComponent interface {
    Name() string
    Close() error
    GetStatus() map[string]interface{}
}

type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
```

Return `nil, nil` when disabled (silently skipped by registry).

## Skeleton

```go
package infrastructure

import (
    "stackyrd/config"
    "stackyrd/pkg/logger"
)

type ThingManager struct {
    client    interface{}
    connected bool
}

func NewThing(cfg config.ThingConfig, l *logger.Logger) (*ThingManager, error) {
    if !cfg.Enabled { return nil, nil }
    return &ThingManager{connected: true}, nil
}

func (m *ThingManager) Name() string                         { return "Thing" }
func (m *ThingManager) Close() error                         { return nil }
func (m *ThingManager) GetStatus() map[string]interface{}    { return map[string]interface{}{"connected": m.connected} }

func init() {
    RegisterComponent("thing", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
        return NewThing(cfg.Thing, log)
    })
}
```

## Config

1. Add `ThingConfig` struct to `config/config.go` + field in `Config`.
2. Add `viper.SetDefault("thing.enabled", false)` in `setupViperDefaults()`.
3. Add to `config.yaml`:
```yaml
thing:
  enabled: true
  endpoint: "localhost:9999"
```

## Access from Services

Services resolve components through the typed getters on `*registry.Dependencies`
(each returns `*T` or `nil`) — never `deps.Get("name")` + manual type assertion:

```go
svc.thing = deps.Thing()  // *infrastructure.ThingManager or nil
```

Get the dependencies from a `RegisterServiceWithDeps` factory (see `service.md`).

## Patterns

| File | Pattern |
|------|---------|
| `mongo.go` | Multi-connection, TTL-cached health, async ops, worker pool, batch |
| `redis.go` | Sync/async/batch wrappers |
| `kafka.go` | Producer/consumer lifecycle |
| `postgres.go` | Multi-connection with GORM |
| `minio.go` | S3-compatible storage with progress |
| `grafana.go` | HTTP API client wrapper |

- Return `nil, nil` when disabled
- TTL-cached `GetStatus()` avoids hammering health endpoints (2s TTL, see `MongoManager`)
- Async init is handled by `InfraInitManager` — just register the component
- Thread safety: use `sync.Mutex` for shared state in `GetStatus()`/`Close()`
- `Close()` is called during server shutdown (10s timeout per component)
- Constructors that need a logger can accept it variadically: `func NewThing(cfg config.ThingConfig, log ...*logger.Logger)` — see `NewPostgresConnectionManager`
- Panic safety: route async jobs through the worker pool (`WorkerPool`) or `utils.GoSafe(logger, fn)` — never spawn a bare `go func()` without a recover
- Avoid log-and-return: log a failure OR return the wrapped error (`fmt.Errorf("...: %w", err)`), not both — the registry/shutdown path logs propagated errors
