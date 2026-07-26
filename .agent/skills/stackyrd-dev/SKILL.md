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

## TUI Color Theme System

TUI styles use 7 semantic color keys defined per theme in `pkg/tui/themes.go`:

| Key | Usage |
|-----|-------|
| `primary` | Headers, banners, sidebar titles |
| `secondary` | Info badges, subheaders |
| `success` | Connected/good status dots, log info |
| `warning` | Warning badges, log warnings |
| `error` | Error badges, disconnected status, log errors, log fatal |
| `dim` | Muted text, dividers, disabled status |
| `text` | Main body text |

### How it works

- `styles.go` exports functions that call `TC(key)` at call time (not `var` init time)
- `TC(key)` reads the current theme's color map via `themeMu.RWMutex`
- `SetThemeName(name)` changes `currentThemeName` then all subsequent renders use the new palette
- Config key: `app.theme` in `config.yaml` (set in `application.go:runWithTUI()`)

### Adding a theme

One entry in `pkg/tui/themes.go`:

```go
"my_theme": {
    Name: "my_theme",
    Colors: map[string]string{
        "primary":   "#hex",
        "secondary": "#hex",
        "success":   "#hex",
        "warning":   "#hex",
        "error":     "#hex",
        "dim":       "#hex",
        "text":      "#hex",
    },
},
```

Available themes: default, vintage_purple, vintage_dark, vintage_pinky, blue_ish, slate, charcoal_tea, pastel_light, sunflower_gold, muted_teal.
