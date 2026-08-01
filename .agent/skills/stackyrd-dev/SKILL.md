---
name: stackyrd-dev
description: Extend stackyrd at its three extension points: services, middleware, infrastructure components.
---

# stackyrd Dev Guide

Extend stackyrd at three extension points: **services** (API endpoints + business logic), **middleware** (HTTP filters), **infrastructure components** (external system clients). All follow: implement interface → register via `init()` → toggle in `config.yaml`.

```
Boot: main → config → Infra async init → Dependencies (sealed) → Middleware → Service discovery → Routes
```

| Ext Point | Dir | Interface | Factory Sig |
|-----------|-----|-----------|-------------|
| Service (plain) | `internal/services/modules/{name}_service.go` | `interfaces.Service` | `func(*config.Config, *logger.Logger) interfaces.Service` |
| Service (with deps) | `internal/services/modules/{name}_service.go` | `interfaces.Service` | `func(*config.Config, *logger.Logger, *registry.Dependencies) interfaces.Service` |
| Middleware | `internal/middleware/{name}.go` | `echo.MiddlewareFunc` | `MiddlewareFactory func(*config.Config, *logger.Logger) (echo.MiddlewareFunc, error)` |
| Infrastructure | `pkg/infrastructure/{name}.go` | `InfrastructureComponent` | `ComponentFactory func(*config.Config, *logger.Logger) (InfrastructureComponent, error)` |

Auto-registered via `init()`. Default: enabled unless `config.yaml` says `false`.

## Conventions

- **Files:** `{name}_service.go` / `{name}.go` (infra) / `{name}.go` (middleware)
- **Tests:** `tests/services/{name}_service_test.go` / `tests/infrastructure/{name}_test.go`
- **Config key:** underscore_case matching `WireName()`
- **Logger:** structured key-value pairs; log technical errors server-side, never echo raw `err.Error()` to clients — return generic messages
- **Responses:** `pkg/response.{Success,Created,BadRequest,NotFound,Error,ValidationError}`
- **Request binding:** `pkg/request.Bind(c, &target)` — returns typed `*ValidationError`; inspect with `errors.As`, not a bare type assertion
- **Dependencies:** services that need infra use `RegisterServiceWithDeps` and read **typed getters** on `*registry.Dependencies` (`deps.Redis()`, `deps.Postgres()`, `deps.Mongo()`, `deps.Kafka()`, `deps.Grafana()`, `deps.MinIO()`, `deps.Cron()`) — each returns `*T` or `nil`. The container is **sealed after boot**: `Set()` is a no-op once infrastructure registration completes.
- **Error wrapping:** use `fmt.Errorf("context: %w", err)`; match sentinels with `errors.Is` / typed chains with `errors.As`

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
- **Runtime switch:** the TUI command `:theme <name>` (or `theme <name>`, colon optional) switches the palette live and persists it back to `config.yaml` via `config.SaveTheme` (surgical line edit). `:themes` lists available themes. Baked-at-construction styles (spinner, command cursor) are re-applied by `applyTheme()`.

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

Themes are defined in `pkg/tui/themes.go` (~27 named palettes). When adding a theme, also append its name to the `app.theme` comment list in `config.yaml`.

## TUI Commands

The command bar (`:` or `ctrl+p`) accepts commands with or without a leading colon:

| Command | Purpose |
|---------|---------|
| `help` | List commands |
| `clear` | Clear the log view |
| `stats` | CPU / RAM / goroutines |
| `gc` | Force garbage collection, report heap before/after |
| `services` / `infra` | Service / infrastructure status counts |
| `list` / `ls` | List services, components, and endpoints |
| `themes` / `theme list` | List available themes in a styled overlay (active marked) |
| `theme <name>` | Switch theme live + persist to config.yaml |

`themes` and `list`/`ls` render their output as a full-screen styled overlay
(theme-colored headers, status bullets, grouped sections) rather than a log
line; press `y`/`n`/`esc` to dismiss it.

Log messages word-wrap to the panel width; messages containing a long
unbreakable token (serialized errors, URLs) are flattened to one line.
Mouse wheel scrolls the pane under the cursor; keyboard scroll targets the
focused pane (`tab` cycles sidebar/logs/command).
