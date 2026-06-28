---
name: stackyrd-dev
description: Extend stackyrd at its four extension points: services, middleware, infrastructure components, and plugins.
---

# stackyrd Dev Guide

Extend stackyrd at any of four extension points: **services** (API endpoints + business logic), **middleware** (HTTP filters), **infrastructure components** (external system clients), **plugins** (runtime logic). All follow: implement interface → register via `init()` → toggle in `config.yaml`.

```
Boot: main → config → Infra async init → Dependencies → Plugin init → Middleware → Service discovery → Routes
```

| Ext Point | Dir | Interface | Factory Sig |
|-----------|-----|-----------|-------------|
| Service | `internal/services/modules/` | `interfaces.Service` | `func(*config.Config, *logger.Logger, *registry.Dependencies) interfaces.Service` |
| Middleware | `internal/middleware/` | `echo.MiddlewareFunc` | `MiddlewareFactory func(*config.Config, *logger.Logger) (echo.MiddlewareFunc, error)` |
| Infrastructure | `pkg/infrastructure/` | `InfrastructureComponent` | `ComponentFactory func(*config.Config, *logger.Logger) (InfrastructureComponent, error)` |
| Plugin | `pkg/plugin/builtin/{name}/` | Varies by runtime | See `pkg/plugin/` |

Auto-registered via `init()`. Default: enabled unless `config.yaml` says `false`.

## Conventions

- **Files:** `{name}_service.go` / `{name}.go` (infra/plugin) / `{name}.go` (middleware)
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
