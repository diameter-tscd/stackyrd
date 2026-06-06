# SWAGGER_SCRIPT — Swagger Generator Script (`scripts/swagger/swagger.go`)

## Overview

`scripts/swagger/swagger.go` is the **stackyrd Swagger documentation generator** — a standalone Go CLI tool for generating OpenAPI/Swagger documentation from service annotations.

The script scans all service files in `internal/services/modules/` for Swagger annotations, analyzes API endpoints and structs, generates `docs.go`, `swagger.json`, and `swagger.yaml` via the `swag` CLI, and verifies the output.

---

## Quick Start

```bash
# Generate Swagger docs (interactive)
go run scripts/swagger/swagger.go

# Dry-run (analyze only, no generation)
go run scripts/swagger/swagger.go -dry-run

# Verbose mode
go run scripts/swagger/swagger.go -verbose
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-verbose` | `false` | Enable verbose/debug logging |
| `-dry-run` | `false` | Analyze endpoints only, don't generate docs |

---

## Workflow

```
1. Find project root (walk up to go.mod)
2. Check swag CLI is installed (auto-install if missing)
3. Analyze all service files for Swagger annotations
4. Display analysis results (per-service + summary)
5. Prompt: Proceed with generation? (Y/n, 10s timeout)
6. Run: swag init -g ./cmd/app/main.go -o docs
7. Verify output: docs.go, swagger.json, swagger.yaml
8. Print success message with generated files list
```

---

## Analysis

The script scans all `.go` files in `internal/services/modules/` and extracts:

### Per-Service Info

| Field | Source |
|-------|--------|
| Service Name | Derived from filename (e.g. `orders_service.go` → `Orders Service`) |
| Swagger Annotations | Matches `// @Summary`, `@Description`, `@Tags`, `@Router`, etc. |
| Endpoints | Extracted from `// @Router /path [method]` annotations |
| Structs | Extracted from `type X struct` definitions |
| Annotation Status | ✓ Found or ✗ Not found |

### Endpoint Details

Each discovered endpoint includes:
- **Method** — HTTP method (GET, POST, PUT, DELETE)
- **Path** — URL path
- **Summary** — From `@Summary` annotation
- **Description** — From `@Description` annotation
- **Tags** — From `@Tags` annotation

### Analysis Summary

```
======================================================================
 SWAGGER ANALYSIS RESULTS
======================================================================

Orders Service
  File: orders_service.go
  Annotations: ✓ Found
  Endpoints: 5
    • GET /orders
      List all orders
    • POST /orders
      Create a new order
    ...

======================================================================
 Total Services:          12
 Services with Annotations: 9
 Total Endpoints:         47
 Total Structs:           23
======================================================================
```

---

## Swagger Generation

Runs the `swag` CLI with:

```
swag init \
  -g ./cmd/app/main.go \
  -o docs \
  --outputTypes go,json,yaml
```

If the `swag` CLI is not installed, it auto-installs via:

```
go install github.com/swaggo/swag/cmd/swag@latest
```

### Generated Files

| File | Format | Location |
|------|--------|----------|
| `docs.go` | Go source | `docs/docs.go` |
| `swagger.json` | JSON | `docs/swagger.json` |
| `swagger.yaml` | YAML | `docs/swagger.yaml` |

### Verification

After generation, the script verifies all three output files exist in `docs/`.

---

## Graceful Shutdown

Pressing `Ctrl+C` during generation prints a clean message and exits immediately. Signal handling captures `SIGINT` and `SIGTERM`.

---

## Architecture

### Key Functions

| Function | Role |
|----------|------|
| `findProjectRoot` | Walks up directories to find `go.mod` |
| `checkSwagInstalled` | Verifies/installs `swag` CLI |
| `installSwag` | Installs `swag` via `go install` |
| `analyzeAPIEndpoints` | Scans all service files for annotations and endpoints |
| `analyzeServiceFile` | Analyzes a single service file for swag annotations, routes, structs |
| `displayAnalysis` | Prints per-service analysis table |
| `generateSwagger` | Runs `swag init` with configured parameters |
| `verifyOutput` | Checks `docs.go`, `swagger.json`, `swagger.yaml` exist |

### Regex Patterns

| Pattern | Purpose |
|---------|---------|
| `//\s*@(Summary\|Description\|Tags\|Router\|Param\|Success\|Failure)\s+(.+)` | Detects all swagger annotations |
| `//\s*@Router\s+([^\s]+)\s+\[(\w+)\]` | Extracts route paths and methods |
| `//\s*@Summary\s+(.+)` | Extracts endpoint summary |
| `//\s*@Description\s+(.+)` | Extracts endpoint description |
| `//\s*@Tags\s+(.+)` | Extracts endpoint tags |
| `type\s+(\w+)\s+struct` | Detects struct definitions |

### Types

```go
type SwaggerConfig struct {
    GeneralInfo  bool
    ScanServices bool
    Verbose      bool
    DryRun       bool
}

type APIEndpoint struct {
    Method      string
    Path        string
    Summary     string
    Description string
    Tags        []string
    Service     string
}

type ServiceInfo struct {
    Name        string
    FileName    string
    Endpoints   []APIEndpoint
    Structs     []string
    HasSwagTags bool
}
```

---

## Configuration Constants

| Variable | Default | Description |
|----------|---------|-------------|
| `MAIN_PATH` | `./cmd/app/main.go` | Entry point for swag scan |
| `DOCS_DIR` | `docs` | Output directory |
| `SERVICES_DIR` | `internal/services/modules` | Service files to analyze |
| `OUTPUT_TYPES` | `go,json,yaml` | Output formats |

---

## Dependencies

- **External CLI**: `swag` (`github.com/swaggo/swag/cmd/swag`) — auto-installed if missing
- **Go standard library** — `flag`, `os/exec`, `regexp`, `strings`, `time`, `path/filepath`

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/swagger/

# Vet
go vet ./scripts/swagger/

# Run (from project root)
go run scripts/swagger/swagger.go
```
