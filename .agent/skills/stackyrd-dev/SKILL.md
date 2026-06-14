---
name: stackyrd-dev
description: A comprehensive guide to extending the stackyrd Go/Gin framework. Use this whenever the user wants to add new features, API endpoints, HTTP middleware, database or external system integrations, or runtime-extensible plugin logic. This is the primary skill for all code generation and framework extension in stackyrd projects. Also use it when the user asks about the framework's architecture, auto-registration patterns, configuration conventions, boot order, or testing approach. If the user says anything about "adding", "creating", "scaffolding", "implementing", or "extending" in the context of this project, this skill should be loaded.
---

# stackyrd Development Guide

Use this guide to extend stackyrd at any of its four extension points: services, middleware, infrastructure components, and plugins. All four follow the same fundamental pattern: implement the interface, register via `init()`, and add a config toggle.

## Architecture Foundation

Understanding the boot order explains everything about how extensions wire themselves in:

```
main.go → config loading → Infrastructure async init → Dependencies populated →
Plugin init → Middleware auto-discovery → Service auto-discovery → Route registration → Server start
```

Every extension point uses Go's `init()` function for **auto-registration**. This means you never manually import or wire anything — you just create a file in the right directory with an `init()` that calls the package's registration function, and the framework picks it up.

Key convention: all auto-registered components default to **enabled** unless explicitly set to `false` in `config.yaml`.

## Decision Framework

Ask the user what they want to create:

| Goal | Create | Package | Interface | Reference |
|------|--------|---------|-----------|-----------|
| New API endpoints, business logic | Service | `internal/services/modules/` | `interfaces.Service` | `references/service.md` |
| HTTP request filtering, auth, logging | Middleware | `internal/middleware/` | `gin.HandlerFunc` (via factory) | `references/middleware.md` |
| External system integration (DB, queue, storage) | Infrastructure Component | `pkg/infrastructure/` | `InfrastructureComponent` | `references/infrastructure.md` |
| Runtime-extensible logic (TS, Lua, Python, Go) | Plugin | `pkg/plugin/builtin/{name}/` | Varies by runtime | `references/plugin.md` |

## Extension Point Templates

Each reference file contains:
- Interface definitions and requirements
- Complete code templates for the file structure
- Real examples from the existing codebase
- Registration pattern using `init()`
- Config toggle setup
- Testing guidance
- Edge cases and pitfalls

## Build Script (`scripts/build/build.go`)

The build script is a standalone single-file Go program (`package main`) that compiles the stackyrd application. Run it from anywhere in the project tree (it auto-discovers the project root via `go.mod`).

### Execution Modes

| Mode | How | When |
|------|-----|------|
| **TUI** (default) | `go run ./scripts/build/` | Interactive terminal with alt-screen bubbletea TUI |
| **CLI** | `go run ./scripts/build/ --no-tui` | CI/CD, piped stdout, non-interactive terminals |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout` | `10` | Seconds before user prompts auto-select default (0 = wait forever) |
| `--verbose` | `false` | Enable debug log output |
| `--garble` | `false` | Enable garble obfuscation (skips interactive prompt) |
| `--upx` | `false` | Enable UPX LZMA compression (skips interactive prompt) |
| `--archive-format` | `tar` | Backup archive format: `tar` (native LZMA2, default) or `7z` (requires `7z` binary). Unknown values warn and fall back to `tar`. Aliases: `tar.xz`/`txz` → `tar`, `sevenzip`/`7-zip` → `7z` |
| `--no-tui` | `false` | Force plain CLI output mode |

### Build Steps (TUI + CLI)

1. **Check Project Path** — find `go.mod` and `chdir` to project root
2. **Check Required Tools** — verify/install `goversioninfo` and `garble`
3. **Configure Garble** — interactive prompt (TUI: `y`/`n` keypress, CLI: stdin with countdown timer)
4. **Stop Running Process** — kill any running `stackyrd` instance via `pgrep`/`tasklist`
5. **Create Backup** — timestamped copy of existing dist files
6. **Archive Backup** — compress backup with LZMA2 (`tar.xz` natively or `7z` binary)
7. **Compile Plugins** — pre-compile Python plugin scripts in `pkg/plugin/builtin/`
8. **Build Application** — `go build` or `garble build` with trimmed ldflags
9. **Configure UPX** — interactive prompt (same pattern as garble)
10. **Compress with UPX** — apply `upx --lzma --best` to the binary
11. **Copy Assets** — copy `config.yaml` to dist

### TUI Architecture

The TUI (`BuildTuiModel`) uses `charmbracelet/bubbletea` with `tea.WithAltScreen()`:
- **Step list** — top section showing all 11 steps with status icons (pending/running/success/error/skipped)
- **Build Log** — bottom section showing captured log output in a fixed-height panel, auto-truncated to terminal size
- **Prompt handling** — inline prompts with live countdown timer and `y`/`n`/`q`/`ctrl+c` keybindings
- **Output capture** — `os.Stdout`/`os.Stderr` redirected to an OS pipe during step execution; logger writes to a `logCaptureWriter`; both feed a thread-safe `logState` buffer with ANSI stripping

### Critical Rules

- Always run with `go run ./scripts/build/` (package mode) — single-file `go run scripts/build/build.go` fails because Go compiles only that file
- The banner is read from `pkg/assets/banner.txt` at runtime; if missing, falls back to "  stackyrd" text
- `go build ./scripts/build/` and `go build ./...` both compile cleanly

## Service Generator Script (`scripts/service/service.go`)

The service generator is a standalone Go CLI tool for scaffolding new service modules. It shares the same bubbletea TUI architecture as the build script.

### Execution Modes

| Mode | How | When |
|------|-----|------|
| **TUI** (default) | `go run ./scripts/service/` | Interactive terminal with alt-screen bubbletea TUI |
| **CLI** | `go run ./scripts/service/ --no-tui` | CI/CD, piped stdout, non-interactive terminals |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-verbose` | `false` | Enable verbose/debug logging |
| `-dry-run` | `false` | Only analyze, don't generate files |
| `-no-tui` | `false` | Disable TUI, use plain CLI output |

### TUI Workflow

The TUI (`ServiceTuiModel`) follows the same step/status/prompt/log-capture pattern as the build script:

```
1. Find project root (walk up to go.mod)
2. Prompt: Service name (text input, duplicate check)
3. Prompt: Wire name (default: {name}-service)
4. Prompt: File name (default: {name}_service.go)
5. Prompt: Service pattern (1-6 select)
6. Prompt: Generate test file? (y/n)
7. Prompt: Generate database model? (y/n)
8. Prompt: Add custom routes? (y/n) → loop
9. Display configuration summary
10. Check method duplication
11. Prompt: Proceed with generation? (Y/n, 10s timeout)
12. Generate service file
13. Generate test file (if selected)
14. Display frozen summary (auto-closing in 15s)
```

### Prompts

| Type | Behavior |
|------|----------|
| **textinput** | `bubbles/textinput` with placeholder, Enter to submit |
| **y/n** | Keypress: `y`/`n`, 10s timeout, `q` cancels step |
| **select** | 1-6 + Enter, `q` cancels |
| **confirm** | Y/n with 10s timeout defaulting to Yes |

### Summary & Auto-Exit

After generation, a frozen summary shows service name, wire name, file, pattern, test/model flags, routes, and next steps. A live countdown "Auto-closing in 15s" runs; any key dismisses early. On exit, the console clears and a clean one-liner is printed.

### Rawterm Safety Net

Both the build and service scripts share the `ttyGuard` pattern:

```go
type ttyGuard struct {
    fd       int
    oldState *term.State
}
func (g *ttyGuard) Save() error    // term.GetState(fd)
func (g *ttyGuard) Restore()      // term.Restore(fd, oldState), nil-safe
```

- `Save()` at the top of the TUI entry function, `defer Restore()`
- Signal handler goroutine catches SIGINT/SIGTERM → Restore → exit(128+signo)
- Panic recovery defers Restore then re-panics
- `golang.org/x/term` is already in `go.mod` — no new dependency needed

### Critical Rules

- Run with `go run ./scripts/service/` (package mode) or `go run scripts/service/service.go` (single file works)
- Alignment in summary uses `fmt.Sprintf("     %12s  %s", label+":", value)` — NEVER `lipgloss.Width()` within styled strings (ANSI codes interfere)
- `ttyGuard.Restore()` is nil-safe and idempotent
- See `.kilo/skills/scripts/SERVICE_SCRIPT.md` for full documentation

## Shared TUI Pattern (Build + Service)

Both `scripts/build/build.go` and `scripts/service/service.go` share a common TUI architecture:

| Aspect | Pattern |
|--------|---------|
| Framework | `charmbracelet/bubbletea` with `tea.WithAltScreen()` |
| Step list | Slice of step structs: `name`, `status`, `action func` |
| Status icons | `○` pending, spinner running, `✓` success, `✗` error, `-` skipped |
| Prompt types | y/n (keypress), select (numbered + Enter), textinput (bubbles/textinput), confirm (Y/n + timeout) |
| Log capture | OS pipe → `io.Copy` goroutine → thread-safe `logState` buffer |
| Logger writer | Redirected to `logCaptureWriter` that strips ANSI and appends to `logState` |
| Auto-exit | `setDone()` captures `completedIn`, returns `tea.Tick(15s, doneTimeoutMsg)`; keypress exits early |
| Console clear | `ClearScreen()` on TUI exit → clean one-liner |
| Alignment | `fmt.Sprintf` width specifiers (never `lipgloss.Width()`) |
| Terminal safety | `ttyGuard.Save()` before TUI → `defer Restore()`, signal handler, panic recovery |
| Signal handling | Goroutine catches SIGINT/SIGTERM → `guard.Restore()` → `os.Exit(128+signo)` |
| Dependency | `golang.org/x/term` for `term.GetState`/`term.Restore` (already in `go.mod`) |

For a reusable reference of this pattern, see `TUI_INFO_PATTERN.md` at the project root.

## General Conventions

| Convention | Rule |
|---|---|
| **Package naming** | Services: `package modules`; Middleware: `package middleware`; Infrastructure: `package infrastructure` |
| **File naming** | Services: `{name}_service.go`; Middleware: `{name}.go`; Infrastructure: `{name}.go` |
| **Test files** | `tests/services/{name}_service_test.go`, `tests/infrastructure/{name}_test.go` |
| **Config naming** | Use underscore_case matching the WireName for config keys |
| **init() registration** | Always call the package's registration function (e.g., `registry.RegisterService`, `RegisterMiddleware`, `RegisterComponent`) |
| **Logger** | Always accept `*logger.Logger` and use structured key-value pairs for all log calls |
| **Response helpers** | Use `pkg/response` helpers: `Success`, `Created`, `BadRequest`, `NotFound`, `Error`, `ValidationError` |
| **Request binding** | Use `pkg/request.Bind()` for validation with field error support |
| **Error handling** | Return structured error responses using `response.Error` with machine-readable error codes |

## Common Abstractions

### The `response` package (`pkg/response`)

All HTTP responses go through standardized helpers. Key functions: `Success(c, data, msg)`, `Created(c, data, msg)`, `BadRequest(c, msg)`, `NotFound(c, msg)`, `Error(c, status, code, msg, details)`, `ValidationError(c, msg, fields)`, `SuccessWithMeta(c, data, meta, msg)`.

### The `request` package (`pkg/request`)

Use `request.Bind(c, &target)` instead of `c.ShouldBindJSON`. It returns typed `*request.ValidationError` with per-field error details that `response.ValidationError` can render.

### The `Dependencies` bag (`pkg/registry/dependencies.go`)

Services receive a `*registry.Dependencies` container populated with all infrastructure components and the PluginBridge. Access them in the service factory: `deps.Get("component_name")` returns `(interface{}, bool)`.
