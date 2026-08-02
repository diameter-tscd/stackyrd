---
name: stackyrd-cli-dev
description: Develop and maintain the standalone CLI in scripts/. Single binary with subcommands: build, service, pkg, swagger, docker.
---

# stackyrd CLI Development

The scripts folder is a standalone Go module (`scripts/go.mod`) producing a single binary `scripts/yrd` via `cd scripts && go build -o yrd .`.

## Subcommands

| Command | Package | Purpose |
|---------|---------|---------|
| `build` | `internal/build` | Compile, garble, UPX, backup (output: `dist/`) |
| `docker` | `internal/docker` | Multi-stage Docker build (10 targets) |
| `pkg` | `internal/pkg` | Install infra from GitHub index |
| `swagger` | `internal/swagger` | Generate OpenAPI docs |
| `service` | `internal/service` | Scaffold service from templates |

Deep docs: `.agent/skills/scripts/{NAME}_SCRIPT.md`.

## Architecture

Each subcommand lives in `scripts/internal/<name>/` as its own Go package with a single exported entry point:

```go
func Run(args []string)
```

`scripts/main.go` is the dispatcher. It strips a global `-path <dir>` flag (anywhere in args) and chdirs to that project root; otherwise it uses the current directory. It then calls the subcommand with `os.Args = append([]string{"yrd"}, args...)` so each subcommand's existing `flag.Parse()` / `os.Args` logic continues to work unchanged.

## Core Patterns

- **CLI flags:** stdlib `flag` package. No cobra/urfave — zero CLI framework deps.
- **Project root:** walk up from CWD looking for `cmd/app/main.go` (`findProjectRoot()`). NOT `go.mod` — `scripts/` has its own go.mod and must not be mistaken for the stackyrd root. `-path <dir>` overrides auto-detection.
- **Logger:** lightweight struct with `Info`/`Warn`/`Error`/`Success`/`Debug` + ANSI colors. Same pattern across all 5 scripts (teal-green info `\033[38;5;108m`, red errors `\033[38;5;167m`, yellow warnings `\033[38;5;186m`).
- **Multi-step workflows:** `[]struct{name string; fn func(*Logger) error}` + for-loop (see `build.go`)
- `./scripts/yrd <command> [flags]` for all tooling
- `./scripts/yrd -path <dir> <command>` for a specific project
- `cd scripts && go run . <command>` for development

## Adding a Subcommand

1. Create `scripts/internal/<name>/<name>.go` as its own package (e.g. `package build`).
2. Rename `func main()` → `func Run(args []string)` and prepend `os.Args = append([]string{"yrd"}, args...)`.
3. Add a case to `scripts/main.go` dispatcher: `<name>.<name>.Run(args)`.
4. Add new dep to `scripts/go.mod` if needed. `go mod tidy` in `scripts/`.
5. Update `.agent/skills/stackyrd-cli-dev/SKILL.md` and doc files.

## Script Bundling

- `scripts/` is its own Go module (independent `go.mod`).
- Import path: `github.com/diameter-tscd/stackyrd/scripts`
- The main `stackyrd` server does NOT import anything from `scripts/`.
- Build: `cd scripts && go build -o yrd .`
- Vet/test: `cd scripts && go vet ./... && go test ./...`

## Testing

`cd scripts && go vet ./...` + `go test ./...`. Stdlib `testing.T` only. No test frameworks.
