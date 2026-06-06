# SERVICE_SCRIPT — Service Generator Script (`scripts/service/service.go`)

## Overview

`scripts/service/service.go` is the **stackyrd service code generator** — a standalone Go CLI tool for scaffolding new service modules with auto-registration, Swagger annotations, GORM models, and test files.

The script uses embedded Go templates (`//go:embed templates/*.tmpl`) and provides an interactive prompt-driven workflow with 6 service patterns, custom route support, method duplication detection, and configurable test/model generation.

---

## Quick Start

```bash
# Interactive service generation
go run scripts/service/service.go

# Dry-run (analyze only, no file generation)
go run scripts/service/service.go -dry-run

# Verbose mode
go run scripts/service/service.go -verbose
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-verbose` | `false` | Enable verbose/debug logging |
| `-dry-run` | `false` | Analyze and validate only, don't generate files |

---

## Interactive Workflow

```
1. Find project root (walk up to go.mod)
2. Prompt:  Service name (e.g. Orders, Inventory)
3. Prompt:  Wire name (default: {service-name}-service)
4. Prompt:  File name (default: {service-name}_service.go)
5. Prompt:  Service pattern (1-6)
6. Prompt:  Generate test file? (y/N)
7. Prompt:  Generate database model (GORM)? (y/N)
8. Prompt:  Add custom routes? (y/N)
   - Loop: path, method, summary, description
9. Display: Configuration summary
10. Check:  Method duplication detection
11. Prompt: Proceed with generation? (Y/n, 10s timeout)
12. Generate service file from template
13. Generate test file (if selected)
14. Display: Generation summary with next steps
```

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

### Key Functions

| Function | Role |
|----------|------|
| `findProjectRoot` | Walks up directories to find `go.mod` |
| `promptServiceName` | Interactive service name input with duplicate check |
| `promptWireName` | Wire name prompt with default generation |
| `promptFileName` | File name prompt with `.go` extension enforcement |
| `promptServicePattern` | Pattern selection menu (1-6) |
| `promptGenerateTests` | Test file generation toggle |
| `promptGenerateModel` | GORM model generation toggle |
| `promptCustomRoutes` | Custom route entry loop |
| `checkServiceExists` | Prevents overwriting existing services |
| `checkMethodDuplication` | Scans existing files for method conflicts |
| `readTemplate` | Reads embedded `.tmpl` file |
| `buildInitFunction` | Generates `init()` auto-registration code |
| `buildSwaggerAnnotations` | Generates Swagger doc annotations |
| `generateService` | Writes processed template to service file |
| `generateModelCode` | Generates GORM model struct |
| `generateTestFile` | Writes test file from template |
| `extractMethodNames` | Parses exported method names from Go source |
| `getPatternMethods` | Returns method names for current pattern |

### Types

```go
type ServicePattern struct {
    Name        string   // Display name
    Description string   // Short description
    Template    string   // Template filename (without .tmpl)
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

## Dependencies

- **Go standard library** — `flag`, `os`, `embed`, `os/exec`, `regexp`, `strings`, `time`
- **Embedded templates** — all template files via `//go:embed`

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/service/

# Vet
go vet ./scripts/service/

# Run (from project root)
go run scripts/service/service.go
```
