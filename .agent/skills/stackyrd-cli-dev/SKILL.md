---
name: stackyrd-cli-dev
description: Guide for developing and maintaining Go CLI tools in the scripts/ directory. Use this whenever the user wants to create a new script, modify an existing one, add a flag or command to any script in scripts/, or understand how the build/service/swagger/docker/plugin/pkg CLI tools work. This is the primary skill for all scripts/ work — if the user mentions "scripts", "CLI", "build tool", "code generator", "scaffolder", or asks about any script in scripts/ (build, docker, pkg, plugin, service, swagger), load this skill. Covers patterns, conventions, and the ponytail/lazy approach to script development.
---

# stackyrd CLI Development Guide

Guide for developing Go CLI tools in `scripts/` — the laziest, most maintainable way.

## The Ponytail Way for Scripts

Every script in `scripts/` is `package main`, single-file (or a directory with one `.go`), run with `go run scripts/<name>/<file>.go`. The project already has 6 scripts and they share a lot of DNA. The lazy move is to follow the existing patterns instead of inventing new ones.

**Ladder for script decisions:**

1. Does a script already do this? Extend it. Don't create a new file.
2. Can `go run` + stdlib `flag` handle it? Yes. No cobra, no urfave/cli — the project has zero CLI framework deps.
3. Does another script already have the pattern you need (step pipeline, logger, project root discovery)? Copy from it — but better: notice what's duplicated and extract it.
4. One file per script. If the file exceeds ~600 lines, first think about extracting shared utilities to `pkg/` before splitting the script itself.
5. Only then: write new code.

## The Big Duplication Problem

Every script independently defines the same things. Before adding code, check if it already exists in 2+ scripts — if so, extract it:

| Duplicated pattern | Defined in | Lazy fix |
|---|---|---|
| `findProjectRoot()` — walks up to `go.mod` | build, docker, pkg, swagger, service (5x) | Extract to `pkg/scriptutil/` |
| `ClearScreen()` — platform-aware cls/clear | build, pkg, swagger, service (4x) | Extract to `pkg/scriptutil/` |
| Logger struct with `Info`/`Warn`/`Error`/`Success`/`Debug` + ANSI colors | All 6 scripts | Extract to `pkg/scriptutil/` |
| ANSI color constants (`P_PURPLE`=`#8daea5`, `P_CYAN`=`#87afff`, etc.) | All 6 scripts | Extract to `pkg/scriptutil/` |
| Step pipeline `[]struct{name string; fn func(*Logger) error}` | build, docker, swagger, service (4x) | Use the pattern but don't abstract until 3rd reuse |

When adding a new script: **do not copy-paste these**. Either extract them first (preferred) or reference the most similar existing script as a starting template.

## Existing Scripts Quick Reference

| Script | File | Run | Purpose |
|---|---|---|---|
| Build | `build/build.go` | `go run scripts/build/build.go` | Compile, obfuscate (garble), compress (UPX), backup |
| Docker | `docker/docker_build.go` | `go run scripts/docker/docker_build.go` | Multi-stage Docker image builder |
| Package | `pkg/pkg.go` | `go run scripts/pkg/pkg.go` | Install infra packages from GitHub index |
| Plugin | `plugin/pkg.go` | `go run scripts/plugin/pkg.go list` | Plugin lifecycle REST client |
| Swagger | `swagger/swagger.go` | `go run scripts/swagger/swagger.go` | Generate OpenAPI docs via `swag` |
| Service | `service/service.go` | `go run scripts/service/service.go` | Scaffold new service modules from templates |

Deep dives on each script are at `.kilo/skills/scripts/{NAME}_SCRIPT.md`.

## Patterns & Conventions

### Script anatomy

```go
package main

import (
    "flag"
    "fmt"
    "os"
    // stdlib only unless proven necessary
)

func main() {
    verbose := flag.Bool("verbose", false, "Enable verbose logging")
    flag.Parse()
    // ...
}
```

- `package main` — always. Scripts are run with `go run`, not imported.
- `flag` package for CLI flags — no exceptions. The project has zero CLI framework deps (cobra, urfave/cli, etc.) and doesn't need one.
- Single-file when possible. Use a directory package only if the script genuinely needs multiple files (e.g., embedded templates in `service/`).
- `go run scripts/<name>/` works when the directory has one `.go` file. `go run scripts/<name>/file.go` works for single-file scripts.

### Step pipeline pattern (for multi-step workflows)

```go
type step struct {
    name string
    fn   func(*Logger) error
}

steps := []step{
    {"Check project root", checkPath},
    {"Build application", buildApp},
    // ...
}

for i, s := range steps {
    log.Step(i+1, len(steps), s.name)
    if err := s.fn(log); err != nil {
        log.Error("%s failed: %v", s.name, err)
        os.Exit(1)
    }
}
```

This pattern is used by 4 scripts (build, docker, swagger, service). Follow it when your script has sequential steps.

### Project root discovery

Every script that touches project files needs to find the root (where `go.mod` lives). The pattern:

```go
func findProjectRoot() (string, error) {
    dir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    for {
        if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", fmt.Errorf("go.mod not found")
        }
        dir = parent
    }
}
```

This is duplicated across 5 scripts. If you need it in a new script, reference this doc instead of re-copying — or better, extract it.

### Logger pattern (for all scripts)

When a script needs formatted output, use this lightweight Logger:

```go
type Logger struct {
    verbose bool
}

func (l *Logger) Info(msg string, args ...any)  { fmt.Printf("  "+msg+"\n", args...) }
func (l *Logger) Success(msg string, args ...any) { fmt.Printf("  "+msg+"\n", args...) }
func (l *Logger) Warn(msg string, args ...any)   { fmt.Printf("  "+msg+"\n", args...) }
func (l *Logger) Error(msg string, args ...any)  { fmt.Printf("  "+msg+"\n", args...) }
func (l *Logger) Debug(msg string, args ...any) {
    if l.verbose {
        fmt.Printf("  "+msg+"\n", args...)
    }
}
```

Colors use ANSI sequences from this palette (shared by all scripts):

| Usage | ANSI code | Color |
|---|---|---|
| Info/Success | `\033[38;5;108m` | Teal-green (#8daea5) |
| Accent/headers | `\033[38;5;117m` | Blue (#87afff) |
| Success signals | `\033[38;5;114m` | Green |
| Warnings | `\033[38;5;186m` | Yellow |
| Errors | `\033[38;5;167m` | Red |
| Dim/muted | `\033[38;5;242m` | Gray |
| Bright | `\033[38;5;255m` | White |

## When to Extract Shared Code

If you're writing the same utility for the 2nd time, extract it to a shared package:

1. **2nd occurrence**: note it as duplication, leave both copies, add a `ponytail:` comment
2. **3rd occurrence**: extract to `pkg/scriptutil/` and update all callers

Good candidates for extraction (already at 3+ occurrences):
- `findProjectRoot()` — 5 scripts
- `ClearScreen()` — 4 scripts
- Logger + color constants — 6 scripts

## Test Philosophy

Scripts are run tools, not libraries. Tests for scripts should be minimal:
- `go vet ./scripts/...` catches most real issues
- Compile-check: `go build -o /dev/null ./scripts/<name>/`
- If the script has non-trivial logic (parsing, API client), put a smoke test in `tests/scripts/<name>_test.go`
- Integration tests for scripts belong in `tests/` mirroring the script structure

Don't add test frameworks. Stdlib `testing` and `go vet` cover 95% of script bugs.

## Adding a New Script

1. Check if the functionality fits in an existing script first
2. Pick the most similar existing script as a template (build for multi-step, swagger for analysis, plugin for API client)
3. Read the corresponding `.kilo/skills/scripts/{NAME}_SCRIPT.md` for its deep docs
4. Use `flag` for CLI args; only add `os.Args` parsing for subcommands (see `plugin/pkg.go` or `pkg/pkg.go`)
5. Use the same ANSI color palette, Logger pattern, and project root discovery
6. Add a `ponytail:` comment for each deliberate shortcut (global state, skipped edge case, known limitation)
7. Verify with `go vet ./scripts/<name>/` and `go build -o /dev/null ./scripts/<name>/`
