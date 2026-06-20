# DOCKER_SCRIPT — Docker Builder Script (`scripts/docker/docker_build.go`)

## Overview

`scripts/docker/docker_build.go` is the **stackyrd-nano Docker builder** — a standalone Go CLI tool for building multi-stage Docker images with target selection (test, dev, prod, slim, minimal, ultra variants).

The script presents an interactive menu for target selection, validates the Docker environment, builds the appropriate stages, and cleans up dangling images.

---

## Quick Start

```bash
# Interactive mode (prompts for target, app name, image name)
go run scripts/docker/docker_build.go
```

---

## Build Targets

The script presents an interactive numbered menu:

| # | Target | Description |
|---|--------|-------------|
| 1 | `all` | Build all images (test, dev, prod) |
| 2 | `test` | Build and run tests only |
| 3 | `dev` | Build development image |
| 4 | `prod` | Build production image |
| 5 | `prod-slim` | Build slim production image |
| 6 | `prod-minimal` | Build minimal production image |
| 7 | `ultra-prod` | Build ultra production image |
| 8 | `ultra-all` | Build all ultra images |
| 9 | `ultra-dev` | Build ultra development image |
| 10 | `ultra-test` | Build and run ultra tests |

---

## Interactive Flow

```
1. Print banner
2. Select build target (1-10)
3. Enter app name  [stackyrd-nano]
4. Enter image name [myapp]
5. Enable verbose logging? (y/N)
6. Validate target
7. Ensure project root (find go.mod)
8. Check Dockerfile exists
9. Check Docker is available
10. Execute build steps for selected target
11. Print success message with image names and usage examples
```

---

## Stage Details

### Test Stage

```
docker build --target test -t <image>:test .
docker run --rm <image>:test
```

Builds the `test` stage from the Dockerfile and runs tests inside a temporary container.

### Dev Stage

```
docker build --target dev -t <image>:dev .
```

### Ultra Dev Stage

```
docker build --target ultra-dev -t <image>:dev .
```

### Prod Stage

```
docker build --target prod -t <image>:latest .
```

### Slim Prod Stage

```
docker build --target prod-slim -t <image>:slim .
```

### Minimal Prod Stage

```
docker build --target prod-minimal -t <image>:minimal .
```

### Ultra Prod Stage

```
docker build --target ultra-prod -t <image>:ultra .
```

### Cleanup

```
docker image prune -f
```

Always runs after build steps, regardless of target. Cleanup failure is non-fatal.

---

## Image Tags by Target

| Target | Tags |
|--------|------|
| `test` | `<image>:test` |
| `dev` | `<image>:dev` |
| `prod` | `<image>:latest` |
| `prod-slim` | `<image>:slim` |
| `prod-minimal` | `<image>:minimal` |
| `ultra-prod` | `<image>:ultra` |
| `all` | `<image>:test`, `<image>:dev`, `<image>:latest` |
| `ultra-all` | `<image>:test`, `<image>:dev`, `<image>:ultra` |
| `ultra-dev` | `<image>:dev` |
| `ultra-test` | `<image>:test` |

---

## Success Output

After completion, the script displays:
- Built image tags and their purpose
- Usage examples (e.g., `docker run -p 8080:8080 <image>:dev`)

---

## Prerequisites

- **Docker** installed and running
- **Dockerfile** in project root with matching stage targets
- **go.mod** in project root (for project root detection)

---

## Graceful Shutdown

Pressing `Ctrl+C` during the build prints a clean message and exits immediately. Signal handling captures `SIGINT` and `SIGTERM`.

---

## Architecture

### Key Functions

| Function | Role |
|----------|------|
| `findProjectRoot` | Walks up directories to find `go.mod` |
| `validateTarget` | Validates target against allowed list |
| `calculateTotalSteps` | Returns step count based on target |
| `checkDockerfile` | Verifies Dockerfile exists in project root |
| `checkDocker` | Verifies Docker is available via `docker version` |
| `buildTestStage` | Builds test image and runs tests |
| `buildDevStage` | Builds development image |
| `buildUltraDevStage` | Builds ultra development image |
| `buildProdStage` | Builds production image (`:latest`) |
| `buildSlimProdStage` | Builds slim production image (`:slim`) |
| `buildMinimalProdStage` | Builds minimal production image (`:minimal`) |
| `buildUltraProdStage` | Builds ultra production image (`:ultra`) |
| `cleanupDanglingImages` | Runs `docker image prune -f` |
| `selectTarget` | Interactive numbered menu for target selection |
| `promptWithDefault` | Prompts with a default value |
| `askVerbose` | Asks for verbose logging toggle |

### Config

```go
type DockerBuildConfig struct {
    AppName   string
    ImageName string
    Target    string
    Verbose   bool
}

type DockerBuildContext struct {
    Config     DockerBuildConfig
    ProjectDir string
    Step       int
    TotalSteps int
}
```

### Valid Targets

```
all, test, dev, prod, prod-slim, prod-minimal,
ultra-prod, ultra-all, ultra-dev, ultra-test
```

---

## Dependencies

- **External**: `docker` CLI
- **Go standard library** — `flag`, `os/exec`, `bufio`, `strconv`, `path/filepath`

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/docker/

# Vet
go vet ./scripts/docker/

# Run (from project root)
go run scripts/docker/docker_build.go
```
