# SERVICE_SCRIPT — Service Generator Script (`scripts/service/service.go`)

## Overview

`scripts/service/service.go` is the **stackyrd service code generator** — a standalone Go CLI tool for scaffolding new service modules with auto-registration, Swagger annotations, GORM models, and test files.

The script uses embedded Go templates (`//go:embed templates/*.tmpl`) and provides an interactive **bubbletea TUI** (default) with step list, prompts, log capture, and summary with auto-exit countdown. Falls back to plain CLI output via `--no-tui` or in non-interactive terminals.

---

## Quick Start

```bash
# Interactive TUI service generation (default)
go run scripts/service/service.go

# Dry-run (analyze only, no file generation)
go run scripts/service/service.go -dry-run

# Verbose mode (TUI or CLI)
go run scripts/service/service.go -verbose

# Plain CLI output (no TUI, suitable for CI/CD)
go run scripts/service/service.go --no-tui
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-verbose` | `false` | Enable verbose/debug logging |
| `-dry-run` | `false` | Analyze and validate only, don't generate files |
| `-no-tui` | `false` | Disable bubbletea TUI, use plain CLI output |

When `-no-tui` is set or the terminal is not a character device (`!isTerminal()`), the script runs in plain CLI mode with ANSI-colored output.

---

## Execution Modes

| Mode | How | When |
|------|-----|------|
| **TUI** (default) | `go run scripts/service/service.go` | Interactive terminal with alt-screen bubbletea TUI |
| **CLI** | `go run scripts/service/service.go --no-tui` | CI/CD, piped stdout, non-interactive terminals |

---

## TUI Workflow (default)

The TUI (`ServiceTuiModel`) follows the same step/status/prompt/log-capture pattern as the build script (`scripts/build/build.go`):

```
1. Find project root (walk up to go.mod)
2. Prompt:  Service name (text input, duplicate check via Enter key)
3. Prompt:  Wire name (text input, default: {name}-service)
4. Prompt:  File name (text input, default: {name}_service.go, .go enforced)
5. Prompt:  Service pattern (1-6 select, Enter to confirm)
6. Prompt:  Generate test file? (y/n keypress)
7. Prompt:  Generate database model (GORM)? (y/n keypress)
8. Prompt:  Add custom routes? (y/n)
   - Loop: path, method, summary, description via text input + y/n
9. Display configuration summary (spinner step)
10. Check method duplication against existing service files
11. Prompt: Proceed with generation? (Y/n, 10s timeout → default Yes)
12. Generate service file from template
13. Generate test file (if selected)
14. Display frozen detailed summary (auto-closing in 15s)
```

### Prompts

| Prompt Type | Behavior |
|-------------|----------|
| **textinput** | Text prompt using `bubbles/textinput` with placeholder, Enter to submit, `ctrl+c` to quit, `q` to cancel step |
| **y/n** | Yes/No with keypress: `y` or `n`; timeout after 10s with default; `q` / `ctrl+c` to quit |
| **select** | Pattern selection (1-6) with Enter to confirm; `q` / `ctrl+c` to quit |
| **confirm** | Y/n with 10s timeout defaulting to Yes; `n` for no, `q` / `ctrl+c` to quit |

### Step States

| Icon | State |
|------|-------|
| `○` | Pending |
| `◌` (spinner) | Running |
| `✓` | Success |
| `✗` | Error |
| `-` | Skipped |

### Summary & Auto-Exit

After generation completes:

1. A frozen detailed summary is displayed showing:
   - Service name, wire name, file path, pattern, tests/model flags, route counts
   - Next steps (config.yaml, implement logic, regenerate Swagger, test)
2. A live countdown "Auto-closing in 15s" is shown at the bottom
3. The user can press any key to dismiss early
4. On exit, the console is cleared via `RunServiceTUI` → `ClearScreen()` and a clean one-liner is printed:
   ```
   ✓ Service generated: orders_service.go
   ```
5. On error, the error message is printed to stderr and the process exits with code 1

### Log Capture

- A `logState` buffer (thread-safe, auto-truncated to terminal height) captures structured log lines
- `os.Stdout`/`os.Stderr` is **not** fully redirected (unlike build.go) — only the `Logger.writer` is replaced with a `logCaptureWriter`
- A dedicated OS pipe (`stepPipeR`/`stepPipeW`) feeds log lines into the TUI so the logger's output appears in the log panel below the step list
- The log panel shows the most recent lines, auto-sizing to available terminal height minus the step list and summary

### TUI Architecture Key Details

| Aspect | Implementation |
|--------|---------------|
| Framework | `charmbracelet/bubbletea` with `tea.WithAltScreen()` |
| Steps | Slice of `stepInfo` structs with `name`, `status`, `action` func |
| Prompts | Inline via `promptType` enum: `promptNone`, `promptYesNo`, `promptText`, `promptSelect`, `promptConfirm` |
| Timer | `startPromptTimeout` sends `promptTimeoutMsg` after 10s |
| Done | `setDone(success bool)` captures `completedIn` once, records `doneAt`, returns `tea.Tick(15s, doneTimeoutMsg)` |
| Alignment | `fmt.Sprintf("     %12s  %s", label+":", value)` — Go width specifiers, NOT lipgloss `Width()` |
| Console exit | `ClearScreen()` function clears the terminal before printing the one-liner |

### Rawterm Safety Net

A `ttyGuard` struct provides last-resort terminal restoration:

```go
type ttyGuard struct {
    fd       int
    oldState *term.State
}
```

- `guard.Save()` — captures terminal state via `term.GetState()` before TUI starts
- `guard.Restore()` — restores state via `term.Restore()`, nil-safe (second call is no-op), also sends `\033[?1049l` to leave alternate screen buffer
- Called in `RunServiceTUI` at the top-level defer, so it fires on normal return, panic (via recovery defer), or when the signal handler fires
- **Does NOT interfere with bubbletea's own terminal management** — bubbletea manages the terminal normally; the guard is only a safety net for abnormal exits

Signal handler flow in `setupTUISignalHandler`:
1. Catches `SIGINT`/`SIGTERM`
2. Restores terminal via `guard.Restore()`
3. Prints signal name to stderr
4. Exits with `128 + signo`
5. A `done` channel (closed when TUI completes normally) stops the signal handler goroutine

---

## Service Patterns

| # | Pattern | Template | Endpoints Generated |
|---|---------|----------|---------------------|
| 1 | **Basic CRUD** | `basic_crud` | List, Get, Create, Update, Delete |
| 2 | **Read-Only** | `read_only` | List, Get |
| 3 | **Write-Only** | `write_only` | Create, Update |
| 4 | **Event-Driven** | `event_driven` | Publish, Subscribe |
| 5 | **WebSocket** | `websocket` | WebSocket connection |
| 6 | **Batch Processing** | `batch_processing` | Batch process, Batch status |

Each pattern generates corresponding handler methods, route registrations, and Swagger annotations.

---

## Generated Output

### Service File

Written to `internal/services/modules/{name}_service.go`.

Includes:
- **Struct definition** with `ServiceName` struct
- **Constructor** `NewServiceName(enabled bool, logger *logger.Logger) *ServiceName`
- **Interface methods**: `Name()`, `WireName()`, `Enabled()`, `Endpoints()`, `RegisterRoutes()`, `Get()`
- **Handler methods** based on selected pattern
- **Auto-registration `init()` function** using `registry.RegisterService`
- **Swagger annotations** for all endpoints
- **Optional GORM model** (struct, `TableName()`)

### Template Placeholders

| Placeholder | Replacement |
|-------------|-------------|
| `{{SERVICE_NAME}}` | Capitalized service name (e.g. `Orders`) |
| `{{SERVICE_NAME_LOWER}}` | Lowercased service name (e.g. `orders`) |
| `{{WIRE_NAME}}` | Wire name (e.g. `orders-service`) |
| `{{IMPORTS}}` | Additional import statements |
| `{{FIELDS}}` | Struct fields |
| `{{PARAMS}}` | Constructor parameters |
| `{{ASSIGNMENTS}}` | Constructor field assignments |
| `{{INIT_FUNCTION}}` | Auto-registration init function |
| `{{SWAGGER_ANNOTATIONS}}` | Generated Swagger doc annotations |
| `{{MODEL_CODE}}` | GORM model (if enabled) |

### Auto-Registration Init Function

```go
func init() {
    registry.RegisterService("orders_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        helper := registry.NewServiceHelper(config, logger, deps)
        if !helper.IsServiceEnabled("orders_service") {
            return nil
        }
        return NewOrders(true, logger)
    })
}
```

### Test File

Written to `tests/services/{name}_service_test.go` (if `-dry-run` not set and user opts in).

### GORM Model (if selected)

```go
type Order struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:255;not null" json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Order) TableName() string {
    return "orders"
}
```

---

## Custom Routes

Users can add custom routes interactively:

```go
type CustomRoute struct {
    Method      string   // GET, POST, PUT, DELETE
    Path        string   // e.g. /search, /bulk
    HandlerName string   // Auto-generated from method + path
    Summary     string   // Swagger @Summary
    Description string   // Swagger @Description
}
```

Each custom route generates:
- A handler method on the service struct
- A route registration in `RegisterRoutes()`
- Swagger annotations (`@Summary`, `@Description`, `@Tags`, `@Router`)

---

## Duplicate Detection

Before generation, the script scans existing `_service.go` files in `internal/services/modules/` for method name conflicts. It:

1. Reads all existing service files
2. Extracts exported method names ( `func (s *X) MethodName(` )
3. Compares against methods that would be generated for the current pattern + custom routes
4. If conflicts are found, prints them and exits with an error

---

## Embedded Templates

Templates are embedded via `//go:embed templates/*.tmpl`:

| Template File | Purpose |
|---------------|---------|
| `templates/basic_crud.tmpl` | Basic CRUD service |
| `templates/read_only.tmpl` | Read-only service |
| `templates/write_only.tmpl` | Write-only service |
| `templates/event_driven.tmpl` | Event-driven service |
| `templates/websocket.tmpl` | WebSocket service |
| `templates/batch_processing.tmpl` | Batch processing service |
| `templates/test.tmpl` | Test file |

---

## Swagger Annotations

Generated annotations follow the pattern:

```go
// @Summary List orders
// @Description Get a list of all orders
// @Tags orders
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "Success"
// @Failure 400 {object} response.Response "Bad request"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /orders [get]
```

Pattern-specific annotations:
- **Basic CRUD**: List, Get/:id, Create, Update, Delete
- **Read-Only**: List, Get/:id
- **Write-Only**: Create, Update
- **Event-Driven**: Publish, Subscribe
- **WebSocket**: WebSocket (produces `text/plain`, success 101)
- **Batch**: Batch process, Batch status

---

## Next Steps (Post-Generation)

```
1. Add service to config.yaml:
   services:
     orders_service: true

2. Implement business logic in handler methods
3. Regenerate Swagger docs: go run scripts/swagger/swagger.go
4. Test the service endpoints
```

---

## Architecture

### Key TUI Functions

| Function | Role |
|----------|------|
| `NewServiceTuiModel` | Creates the TUI model with spinners, textinput, step list |
| `RunServiceTUI` | Entry point: terminal guard setup, pipe creation, `tea.NewProgram(m, tea.WithAltScreen())` |
| `ServiceTuiModel.Init` | Returns initial commands: `tea.EnterAltScreen`, spinner tick, `triggerCurrentStep` |
| `ServiceTuiModel.Update` | Message handler: step execution, prompt input, timeout, done |
| `ServiceTuiModel.View` | Renders: banner → step list → log panel → prompt → summary → footer |
| `triggerCurrentStep` | Executes step action or activates prompt with spinner |
| `setDone` | Captures `completedIn`, starts 15s auto-exit timer |
| `startPromptTimeout` | 10-second timeout for y/n, confirm, select prompts |

### Key Business Logic Functions

| Function | Role |
|----------|------|
| `findProjectRoot` | Walks up directories to find `go.mod` |
| `validateServiceName` | Validates name format and checks for duplicates |
| `extractMethodNames` | Parses exported method names from Go source |
| `getPatternMethods` | Returns method names for current pattern |
| `checkServiceExists` | Prevents overwriting existing services |
| `checkMethodDuplication` | Scans existing files for method conflicts |
| `readTemplate` | Reads embedded `.tmpl` file |
| `buildInitFunction` | Generates `init()` auto-registration code |
| `buildSwaggerAnnotations` | Generates Swagger doc annotations |
| `generateService` | Writes processed template to service file |
| `generateModelCode` | Generates GORM model struct |
| `generateTestFile` | Writes test file from template |

### Rawterm Safety Functions

| Function | Role |
|----------|------|
| `ttyGuard.Save()` | Captures terminal state via `term.GetState()` |
| `ttyGuard.Restore()` | Restores terminal state, leaves alt screen (`\033[?1049l`), nil-safe |
| `setupTUISignalHandler` | Goroutine catching SIGINT/SIGTERM → restore → exit with 128+signo |

### Types

```go
type stepStatus int          // statusPending, statusRunning, statusSuccess, statusError, statusSkipped
type promptType int          // promptNone, promptYesNo, promptText, promptSelect, promptConfirm

type stepInfo struct {
    name        string
    status      stepStatus
    message     string
    prompt      promptType
    promptLabel string
    promptDef   string
    defVal      bool
    action      func(*ServiceContext, *Logger) error
}

type ServicePattern struct {
    Name        string
    Description string
    Template    string
}

type ServiceConfig struct {
    ServiceName    string
    WireName       string
    FileName       string
    GenerateTests  bool
    GenerateModel  bool
    ServicePattern ServicePattern
    CustomRoutes   []CustomRoute
    Verbose        bool
    DryRun         bool
}
```

### Directory Layout

```
internal/services/modules/{name}_service.go   # Generated service
tests/services/{name}_service_test.go         # Generated tests (optional)
scripts/service/templates/*.tmpl              # Embedded templates
```

---

## Shared TUI Pattern

`scripts/service/service.go` and `scripts/build/build.go` share the same bubbletea TUI architecture:

| Aspect | Common Pattern |
|--------|---------------|
| **Framework** | `charmbracelet/bubbletea` with `tea.WithAltScreen()` |
| **Step list** | Slice of step structs with `name`, `status`, `action` func |
| **Status icons** | `○` pending, spinner running, `✓` success, `✗` error, `-` skipped |
| **Prompt types** | y/n (keypress), select (numbered + Enter), textinput, confirm (Y/n with timeout) |
| **Log capture** | OS pipe (`io.Copy` goroutine) feeding a thread-safe `logState` buffer |
| **Logger writer** | Redirected to `logCaptureWriter` that appends to `logState` without ANSI codes |
| **Auto-exit** | `setDone()` captures `completedIn` once → 15s `doneTimeoutMsg` timer → keypress dismisses early |
| **Console clear** | `ClearScreen()` on TUI exit; clean one-liner printed after clear |
| **Alignment** | `fmt.Sprintf` width specifiers (NOT lipgloss `Width()`) to avoid ANSI interference |
| **Term guard** | `ttyGuard` struct: `Save()` before TUI, `Restore()` in defer, signal handler, panic recovery |
| **Signal handling** | `setupTUISignalHandler` → goroutine catches SIGINT/SIGTERM → restore → exit(128+signo) |
| **Dependency** | `golang.org/x/term` for `term.GetState`/`term.Restore` |

### Critical Rules

- Always run with `go run ./scripts/service/` (package mode) or `go run scripts/service/service.go` (single file is fine since it has no package-level dependencies)
- Summary alignment uses `fmt.Sprintf("     %12s  %s", label+":", value)` — NEVER use `lipgloss.Width()` for alignment within styled strings, as ANSI escape codes interfere with width calculation
- The `ttyGuard` must be saved at the top of `RunServiceTUI` and deferred for restore; it is a last-resort safety net that does NOT conflict with bubbletea's own terminal driver
- `golang.org/x/term` is already in `go.mod` — no new dependency needed when adding the rawterm pattern to other scripts
- `ttyGuard.Restore()` is nil-safe and can be called multiple times — the second call is a no-op after `oldState` is set to nil

---

## Dependencies

- **Go standard library** — `flag`, `os`, `embed`, `os/exec`, `regexp`, `strings`, `time`
- **charmbracelet/bubbletea** — TUI framework (already in `go.mod`)
- **charmbracelet/bubbles** — `spinner` and `textinput` components (already in `go.mod`)
- **charmbracelet/lipgloss** — Style definitions (already in `go.mod`)
- **golang.org/x/term** — Terminal state save/restore for rawterm safety net (already in `go.mod`)

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/service/

# Vet
go vet ./scripts/service/

# Run (from project root)
go run scripts/service/service.go

# Run with no-tui for headless
go run scripts/service/service.go --no-tui -dry-run -verbose
```

---

## Related Skills

- **`BUILD_SCRIPT.md`** — Same TUI pattern for the build manager
- **`stackyrd-dev/SKILL.md`** — Development guide referencing both build and service scripts
- **`TUI_INFO_PATTERN.md`** (project root) — Reusable documentation of the info + auto-exit + console-clearing + rawterm pattern
