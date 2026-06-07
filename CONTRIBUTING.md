# Contributing to stackyrd

Thank you for taking the time to contribute! This document sets out the ground rules for contributing to this project.

---

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Branching Strategy](#branching-strategy)
- [Contributing Workflow](#contributing-workflow)
- [Commit Message Convention](#commit-message-convention)
- [Pull Request Guidelines](#pull-request-guidelines)
- [Adding a New Service](#adding-a-new-service)
- [Adding New Middleware](#adding-new-middleware)
- [Adding a New Plugin](#adding-a-new-plugin)
- [Adding Infrastructure Components](#adding-infrastructure-components)
- [Testing](#testing)
- [Code Quality](#code-quality)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)
- [License](#license)

---

## Development Setup

### Prerequisites

- **Go 1.25.3** — [download](https://go.dev/dl/)
- **Docker + Docker Compose** — for the dev environment stack
- **Git** — for version control
- Optional: `garble` (`go install mvdan.cc/garble@latest`) — for obfuscated builds
- Optional: `pip3 install grpcio protobuf` — for Python (`ext:`) plugins

### Clone the Repository

```bash
git clone https://github.com/diameter-tscd/stackyard.git
cd stackyard
```

### Install Dependencies

```bash
go mod download
go mod tidy
```

### Run Locally

```bash
go run cmd/app/main.go
```

The application reads configuration from `config.yaml` in the working directory.

### Run Tests

```bash
go test ./...                    # All packages
go test -v ./tests/...           # Verbose integration tests
go test -v ./pkg/testing/...     # Test helpers
```

### Build

```bash
go run scripts/build/build.go
```

### Docker Compose (Full Dev Environment)

```bash
docker-compose up
```

This starts Redis, PostgreSQL, Kafka, MongoDB, Grafana, MinIO, and the stackyrd app.

---

## Project Structure

```
stackyrd/
├── cmd/app/                          # Entry point, CLI flags, bootstrap
├── config/                           # Config structs & Viper setup
├── internal/
│   ├── middleware/                    # HTTP middleware (auto-registered via init())
│   └── server/                        # Gin server, health endpoints, graceful shutdown
├── internal/services/modules/         # Business logic services (auto-discovered)
├── pkg/
│   ├── interfaces/                    # Core interfaces (Service, etc.)
│   ├── registry/                      # Service registry & DI container
│   ├── plugin/                        # Plugin system (TS/Lua/Python/Go runtimes)
│   │   └── builtin/                   # Built-in plugin manifests + scripts
│   ├── infrastructure/                # Infrastructure components (auto-registered)
│   ├── logger/                        # Structured logger (zerolog)
│   ├── response/                      # API response helpers
│   ├── request/                       # Request binding & validation
│   ├── tui/                           # Terminal UI (bubbletea + lipgloss)
│   ├── metrics/                       # Prometheus metrics
│   ├── pagination/                    # Cursor-based pagination
│   ├── caching/                       # Redis-backed cache abstraction
│   ├── batch/                         # Batch processing utilities
│   ├── resilience/                    # Circuit breaker, retry, timeout, health checks
│   ├── testing/                       # Test helpers and mocks
│   ├── utils/                         # General utilities
│   ├── webhook/                       # Webhook handler
│   └── websocket/                     # WebSocket handler
├── scripts/                           # Build, Docker, packaging, code generators
├── tests/                             # Integration & unit tests
├── config.yaml                        # Main YAML configuration
├── docs/                              # Auto-generated Swagger docs
└── docs_wiki/                         # Full project documentation
```

---

## Branching Strategy

- `main` — production-ready, stable releases.
- Feature work branches off `main`.
- Name branches by type and scope, for example:

| Type      | Example                                         |
|-----------|-------------------------------------------------|
| Feature   | `feature/add-email-service`                     |
| Fix       | `fix/criticalperf-01`                           |
| Chore     | `chore/update-dependencies`                     |
| Docs      | `docs/api-reference`                            |
| Test      | `test/coverage-users-service`                   |

---

## Contributing Workflow

1. **Fork** the repository (or create a local branch if you have write access).
2. **Create a branch** from `main` for your work.
3. **Make your changes**, following the conventions below.
4. **Run the test suite** and ensure everything passes.
5. **Run linters / formatter** where applicable.
6. **Commit** with a message following the convention below.
7. **Push** your branch and open a Pull Request against `main`.
8. **Respond to review feedback** and iterate until approval.
9. A maintainer will **squash-merge** your PR.

---

## Commit Message Convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Examples:**

```
feat(users): add forgot-password endpoint
fix(middleware): prevent panic on nil auth config
chore(deps): update gin to v1.10.1
docs(services): add multi-tenant service docs
test(cache): cover expired key eviction path
```

| Type        | When to use                                   |
|-------------|-----------------------------------------------|
| `feat`      | A new feature                                 |
| `fix`       | A bug fix                                     |
| `chore`     | Maintenance / non-functional changes          |
| `docs`      | Documentation changes                         |
| `test`      | Adding or fixing tests                        |
| `refactor`  | Code change that neither fixes a bug nor adds a feature |
| `perf`      | A performance improvement                     |
| `ci`        | CI/CD configuration changes                   |

---

## Pull Request Guidelines

- **One concern per PR.** Keep PRs focused and small.
- **Describe the change.** Fill out the PR template: what changed, why, and how it was tested.
- **Keep diffs minimal.** Avoid reformatting unrelated code.
- **Pass all CI checks.** Your PR must pass lint, build, and test jobs.
- **Request a review.** Tag an appropriate maintainer.

---

## Adding a New Service

1. Create `internal/services/modules/{name}_service.go`.
2. Implement the `pkg/interfaces.Service` interface.
3. Add `{name}_service: true/false` to the `services:` section in `config.yaml`.
4. Optionally write integration tests at `tests/services/{name}_service_test.go`.

The service registry (`pkg/registry/registry.go`) will auto-discover it via `AutoDiscoverServices`.

```go
type Service interface {
    Name() string
    WireName() string
    Enabled() bool
    Endpoints() []string
    RegisterRoutes(g *gin.RouterGroup)
    Get() interface{}
}
```

---

## Adding New Middleware

1. Create `internal/middleware/{name}.go`.
2. Export a factory: `func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error)`.
3. Register via `init()`:

```go
func init() {
    RegisterMiddleware("name", factory)
}
```

4. Add `{name}: true/false` to the `middleware:` section in `config.yaml`.

---

## Adding Infrastructure Components

1. Create `pkg/infrastructure/{name}.go`.
2. Implement the `InfrastructureComponent` interface.
3. Register via `init()`:

```go
func init() {
    RegisterComponent("name", factory)
}
```

Components are initialized asynchronously and health-check-polled; results appear in the TUI dashboard.

```go
type InfrastructureComponent interface {
    Name() string
    Close(ctx context.Context) error
    GetStatus(ctx context.Context) map[string]interface{}
}
```

---

## Adding a New Plugin

Four plugin types are supported: **TypeScript**, **Lua**, **Python** (gRPC subprocess), and **Go**. Each follows a different creation pattern but shares the same auto-discovery mechanism via `//go:embed builtin`.

### TypeScript Plugin

1. Create `pkg/plugin/builtin/{name}/plugin.yaml`:
    ```yaml
    name: my_plugin
    version: 1.0.0
    description: My TypeScript plugin
    author: you
    entrypoint: "ts:scripts/handler.ts"
    limits:
      max_timeout_ms: 5000
      max_memory_bytes: 26214400
    ```
2. Create `pkg/plugin/builtin/{name}/scripts/handler.ts` using the injected globals `$args`, `$logger`, `$infra`, `$limits`, and `$done()`.
3. At startup, the `.ts` is transpiled to JS via esbuild (SHA256-cached) and executed in a sandboxed goja VM.
4. No Go code needed. See `pkg/plugin/sdk/plugin.d.ts` for type declarations.

### Lua Plugin

1. Create `pkg/plugin/builtin/{name}/plugin.yaml`:
    ```yaml
    name: my_lua_plugin
    version: 1.0.0
    description: My Lua plugin
    author: you
    entrypoint: "lua:scripts/handler.lua"
    limits:
      max_timeout_ms: 10000
      max_memory_bytes: 33554432
    ```
2. Create `pkg/plugin/builtin/{name}/scripts/handler.lua` with a `handle(args)` function using the injected globals `args`, `logger`, `limits`, `infra`, `plugin_name`, and `done()`.
3. No transpilation step — Lua runs directly in the embedded gopher-lua VM (pure Go, no CGo).
4. The sandbox blocks `io`, `os` (except `os.time`), `debug`, `loadfile`, `dofile`, and `require` with file paths.

### Python / External Language Plugin

1. Create `pkg/plugin/builtin/{name}/plugin.yaml`:
    ```yaml
    name: my_python_plugin
    version: 1.0.0
    description: My Python plugin
    author: you
    entrypoint: "ext:scripts/handler.py"
    limits:
      max_timeout_ms: 15000
      max_memory_bytes: 33554432
    ```
2. Create `pkg/plugin/builtin/{name}/scripts/handler.py` with a class extending `Plugin` from `sdk`:
    ```python
    from sdk import Plugin

    class MyPlugin(Plugin):
        def execute(self, args):
            name = args.get("name", "world")
            return {"success": True, "data": {"message": f"Hello, {name}!"}}
    ```
3. The Python script runs as a subprocess communicating via gRPC over a Unix socket.
4. Requires `pip3 install grpcio protobuf` on the host.

### Go Plugin

1. Create a flat `.go` file in `pkg/plugin/` (e.g., `pkg/plugin/plugin_myplugin.go`):
    ```go
    package plugin

    import "github.com/spf13/afero"

    func init() {
        RegisterPlugin("myplugin", func(meta PluginMeta, fs afero.Fs) (Plugin, error) {
            return &MyPlugin{fs: fs, name: meta.Name}, nil
        })
    }

    type MyPlugin struct {
        fs   afero.Fs
        name string
    }

    func (p *MyPlugin) Meta() PluginMeta { return PluginMeta{Name: p.name} }
    func (p *MyPlugin) Execute(ctx Context, args map[string]interface{}) (*Result, error) {
        return &Result{Success: true, Data: map[string]interface{}{"message": "hello"}}, nil
    }
    func (p *MyPlugin) Validate() error { return nil }
    func (p *MyPlugin) Close() error    { return nil }
    ```
2. Create `pkg/plugin/builtin/{name}/plugin.yaml` with `entrypoint: "go:MyPlugin"`.
3. **Important:** Go registration files must be placed directly in `pkg/plugin/` (not inside `builtin/`), because Go requires all files with the same `package` declaration to be in a single directory.

### All Plugin Types

- The plugin is auto-discovered at startup via `//go:embed builtin`.
- Runtime script overrides can be uploaded via `PUT /api/v1/plugins/:name/scripts/:file`.
- Config overrides can be set in `config.yaml` → `plugins.overrides`.
- Plugin execution is tracked with per-plugin stats (count, duration, memory).
- See [PLUGIN_GUIDE.md](PLUGIN_GUIDE.md) for complete documentation, and `.agent/skills/PLUGIN_PKG.md` for package internals.

---

## Testing

- The test framework is **testify** + `httptest` + Gin test mode.
- Test helpers live in `pkg/testing/helpers.go`: `NewTestContext`, `AssertStatus`, `AssertJSON`, `ParseResponse`.
- Place integration tests under `tests/services/` and unit tests under `tests/infrastructure/`.
- CI runs `go test -v ./...` on every push and PR.

---

## Code Quality

- Follow Go idioms and the style of existing code.
- Run `golangci-lint run ./...` or `staticcheck ./...` before submitting.
- Use `go vet ./...` and fix any warnings before submitting.
- Use `go build ./...` to verify compilation across all packages.
- Use typed constants (`response.ErrorBadRequest`, etc.) instead of raw strings for error codes.
- Keep packages cohesive; avoid circular dependencies.
- Do **not** commit secrets or credentials (never hardcode secrets in `config.yaml`).
- Never commit `config.yaml` with real secrets, the `dist/` directory, or `.env` files.

---

## Reporting Bugs

1. Open an issue at https://github.com/diameter-tscd/stackyard/issues.
2. Use the **Bug Report** template.
3. Include:
   - A clear description of the problem.
   - Steps to reproduce (commands, config snippets, environment details).
   - Expected vs. actual behaviour.
   - Logs or error output.
   - Go version, OS, and any relevant dependency versions.

---

## Feature Requests

1. Open an issue at https://github.com/diameter-tscd/stackyard/issues.
2. Use the **Feature Request** template.
3. Describe the problem the feature solves, the proposed solution, and any alternatives you have considered.

---

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see the `LICENSE` file for details).
