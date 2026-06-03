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
    Close() error
    GetStatus() map[string]interface{}
}
```

---

## Testing

- The test framework is **testify** + `httptest` + Gin test mode.
- Test helpers live in `pkg/testing/helpers.go`: `NewTestContext`, `AssertStatus`, `AssertJSON`, `ParseResponse`.
- Place integration tests under `tests/services/` and unit tests under `tests/infrastructure/`.
- CI runs `go test -v ./...` on every push and PR.

---

## Code Quality

- Follow Go idioms and the style of existing code.
- Use `go vet ./...` and fix any warnings before submitting.
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
