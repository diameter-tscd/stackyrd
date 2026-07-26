---
name: stackyrd-dev
description: Extend stackyrd at its three extension points: services, middleware, infrastructure components.
---

# stackyrd Dev Guide

Extend stackyrd at three extension points: **services** (API endpoints + business logic), **middleware** (HTTP filters), **infrastructure components** (external system clients). All follow: implement interface → register via `init()` → toggle in `config.yaml`.

```
Boot: main → config → Infra async init → Dependencies → Middleware → Service discovery → Routes
```

| Ext Point | Dir | Interface | Factory Sig |
|-----------|-----|-----------|-------------|
| Service | `internal/services/modules/{name}_service.go` | `interfaces.Service` | `func(*config.Config, *logger.Logger, *registry.Dependencies) interfaces.Service` |
| Middleware | `internal/middleware/{name}.go` | `echo.MiddlewareFunc` | `MiddlewareFactory func(*config.Config, *logger.Logger) (echo.MiddlewareFunc, error)` |
| Infrastructure | `pkg/infrastructure/{name}.go` | `InfrastructureComponent` | `ComponentFactory func(*config.Config, *logger.Logger) (InfrastructureComponent, error)` |

Auto-registered via `init()`. Default: enabled unless `config.yaml` says `false`.

## Conventions

- **Files:** `{name}_service.go` / `{name}.go` (infra) / `{name}.go` (middleware)
- **Tests:** `tests/services/{name}_service_test.go` / `tests/infrastructure/{name}_test.go`
- **Config key:** underscore_case matching `WireName()`
- **Logger:** structured key-value pairs
- **Responses:** `pkg/response.{Success,Created,BadRequest,NotFound,Error,ValidationError}`
- **Request binding:** `pkg/request.Bind(c, &target)` — returns typed `*ValidationError`
- **Dependencies:** `deps.Get("name")` returns `(interface{}, bool)` for infra components

## References

- `references/service.md` — service file template + patterns
- `references/middleware.md` — middleware factory + skip patterns
- `references/infrastructure.md` — component structure + config setup

Existing services (`users_service.go`, `products_service.go`, `tasks_service.go`) and middleware (`audit.go`, `jwt.go`, `ratelimit.go`) are canonical reference implementations.
