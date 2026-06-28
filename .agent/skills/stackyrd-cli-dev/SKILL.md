---
name: stackyrd-cli-dev
description: Develop and maintain Go CLI tools in scripts/. Primary skill for all scripts/ work — build, docker, pkg, plugin, swagger, service.
---

# stackyrd CLI Development

Every script is `package main`, single-file, run with `go run scripts/<name>/`.

## Scripts Quick Reference

| Script | File | Run | Purpose |
|--------|------|-----|---------|
| Build | `build/build.go` | `go run scripts/build/` | Compile, garble, UPX, backup |
| Docker | `docker/docker_build.go` | `go run scripts/docker/docker_build.go` | Multi-stage Docker build |
| Package | `pkg/pkg.go` | `go run scripts/pkg/pkg.go` | Install infra from GitHub index |
| Plugin | `plugin/pkg.go` | `go run scripts/plugin/pkg.go list` | Plugin lifecycle REST client |
| Swagger | `swagger/swagger.go` | `go run scripts/swagger/swagger.go` | Generate OpenAPI docs |
| Service | `service/service.go` | `go run scripts/service/service.go` | Scaffold service from templates |

Deep docs: `.agent/skills/scripts/{NAME}_SCRIPT.md`.

## Core Patterns

- **CLI flags:** `flag` package. No cobra/urfave — project has zero CLI framework deps.
- **Project root:** walk up from CWD looking for `go.mod` (`findProjectRoot()`, copy from any existing script — or extract to `pkg/scriptutil/` on 3rd copy)
- **Logger:** lightweight struct with `Info`/`Warn`/`Error`/`Success`/`Debug` + ANSI colors. Same pattern across all 6 scripts (teal-green info `\033[38;5;108m`, red errors `\033[38;5;167m`, yellow warnings `\033[38;5;186m`)
- **Multi-step workflows:** `[]struct{name string; fn func(*Logger) error}` + for-loop (see `build.go`)
- `go run scripts/<name>/` for single-file dirs; `go run scripts/<name>/file.go` also works

## Duplication Rule

2 copies → note it, leave both, add `ponytail:` comment. 3rd copy → extract to `pkg/scriptutil/` and update all callers. Current candidates at 3+ copies: `ClearScreen()` (4 copies), Logger (5 copies). `pkg/scriptutil/` doesn't exist yet.

## Testing

`go vet ./scripts/...` + `go build -o /dev/null ./scripts/<name>/`. Stdlib `testing.T` for non-trivial logic in `tests/scripts/`. No test frameworks.

## New Script Checklist

1. Fits in an existing script? Extend it.
2. Pick the most similar existing script as template.
3. `flag` for CLI args; `os.Args` for subcommands.
4. Same Logger, ANSI palette, project root pattern as existing scripts.
5. `ponytail:` comment for each deliberate shortcut.
6. `go vet ./scripts/<name>/` + `go build -o /dev/null ./scripts/<name>/`.
