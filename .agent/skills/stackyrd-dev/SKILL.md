---
name: stackyrd-dev
description: Development guide for the stackyrd Go framework — Echo v4 modular service framework with auto-discovered services, middleware, infrastructure, DI, and TUI boot dashboard. Use this skill whenever the user works on this codebase: adding or modifying Go services, middleware, infrastructure components, config, or tests; running build/test/dev commands; touching config.yaml, cmd/app, config/, internal/, or pkg/ (except assets/embed); mentioning stackyrd, RegisterService, MiddlewareFactory, InfrastructureComponent, Dependencies, or TUI. Even if the user does not name the skill explicitly, apply it for any stackyrd framework task.
---

# stackyrd Dev Guide

Extend stackyrd at three extension points: **services** (API endpoints + business logic), **middleware** (HTTP filters), **infrastructure components** (external system clients). All follow: implement interface → register via `init()` → toggle in `config.yaml`. Carry the ponytail constraint: if the stdlib/native solution works, that IS the answer — one file before three, delete before add, no speculative factories or config flags.

```
Boot: main → config (Validate) → Infra async init → Dependencies (sealed) → Middleware → Service discovery → Routes
```

| Ext Point | Dir | Interface | Factory Sig |
|-----------|-----|-----------|-------------|
| Service (plain) | `internal/services/modules/{name}_service.go` | `interfaces.Service` | `func(*config.Config, *logger.Logger) interfaces.Service` |
| Service (with deps) | `internal/services/modules/{name}_service.go` | `interfaces.Service` | `func(*config.Config, *logger.Logger, *registry.Dependencies) interfaces.Service` |
| Middleware | `internal/middleware/{name}.go` | `echo.MiddlewareFunc` | `MiddlewareFactory func(*config.Config, *logger.Logger) (echo.MiddlewareFunc, error)` |
| Infrastructure | `pkg/infrastructure/{name}.go` | `InfrastructureComponent` | `ComponentFactory func(*config.Config, *logger.Logger) (InfrastructureComponent, error)` |

Auto-registered via `init()`. Default: enabled unless `config.yaml` says `false`. Duplicate `RegisterService`/`RegisterServiceWithDeps` names are ignored with a `stderr` warning — keep names unique.

## Conventions

- **Files:** `{name}_service.go` / `{name}.go` (infra) / `{name}.go` (middleware)
- **Tests:** `tests/services/{name}_service_test.go` / `tests/infrastructure/{name}_test.go` / `tests/*_test.go`
- **Goroutine leaks:** every `tests/**` package has a `goleak_testmain.go` with `goleak.VerifyTestMain` — leaked cron tickers, kafka pollers, redis reconnections, or websocket loops fail the package. Do not add bare `go func()` without `utils.GoSafe` or the infra `WorkerPool`; keep `Close()` idempotent.
- **Config key:** underscore_case matching `WireName()`
- **Config validation:** `(*Config).Validate()` runs automatically in `loadFromSource` — `server.port` must be 1–65535, `app.env` must be `development|production|staging`, `log.max_files >= 0`. Fail fast on bad config; do not silently default.
- **Log config:** `LogConfig` includes `max_age_hours` (default 168) and `max_size_mb` (default 100) alongside `max_files`/`compress` — use these for rotation, not ad-hoc env checks.
- **TUI config:** `AppConfig.TUI` (`SidebarMinWidth` default 135, `SidebarMinHeight` default 42) via `app.tui.sidebar_min_width` / `app.tui.sidebar_min_height` in `config.yaml`; `pkg/tui/terminal.go` reads `m.config.TUI` (not constants) and `cmd/app/application.go` passes `app.config.App.TUI` into `LiveConfig`. When adding TUI layout logic, read from `config.TUI`, not hardcoded thresholds.
- **Logger:** structured key-value pairs; log technical errors server-side, never echo raw `err.Error()` to clients — return generic messages
- **Responses:** `pkg/response.{Success,Created,BadRequest,NotFound,Error,ValidationError}`
- **Request binding:** `pkg/request.Bind(c, &target)` — returns typed `*ValidationError`; inspect with `errors.As`, not a bare type assertion
- **Dependencies:** services that need infra use `RegisterServiceWithDeps` and read **typed getters** on `*registry.Dependencies` (`deps.Redis()`, `deps.Postgres()`, `deps.Mongo()`, `deps.Kafka()`, `deps.Grafana()`, `deps.MinIO()`, `deps.Cron()`) — each returns `*T` or `nil`. The container is **sealed after boot**: `Set()` is a no-op once infrastructure registration completes.
- **Mocks:** `pkg/testing.MockService` is guarded by `var _ interfaces.Service = (*MockService)(nil)` and `RegisterRoutes` takes `*echo.Group` (not `any`). If a Service signature changes and the guard fails, fix the mock — do not silence the guard or widen to `any`.
- **Rate limiting:** `pkg/infrastructure/mcpserver.go` uses `golang.org/x/time/rate.NewLimiter(20, 50)` in `Handler()` — return `429` with JSON-RPC `-32003` when `!limiter.Allow()`. Follow this pattern for token-gated endpoints; do not hand-roll counters.
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
