# INIT_SCRIPT — `yrd init` (rename + onboarding)

Package: `scripts/internal/init` (`package init`, imported aliased as `initcmd` in `scripts/main.go`). One file: `init.go`. Tests: `init_test.go` (stdlib `testing` only).

## What it does

1. **Rename** the project (`stackyrd` → new name): module paths, Go imports, `app.name`, log filename, binary/docker names, service templates, k8s/compose manifests.
2. **Onboarding menu** (numbered loop): app build, docker build, pkg install, config-derived package recommendations, swagger, rename-again, exit. Menu actions re-exec the `yrd` binary (`os.Executable` + stdio wired) so each tool keeps its own `flag` set — never call other `Run()` in-process (global `flag.CommandLine` would panic on redefined flags).

## Flags

`-name` (skip prompt), `-module` (default `<name>`), `-yes` (needs `-name`), `-dry-run` (preview, no writes), `-skip-rename` (menu only), `-verbose`.

## Rename rules (ordered, longest first)

Built by `buildReplacements(name, module)` from fragment-assembled `legacyName` (`"stack"+"yrd"` — never a literal, so the tool survives renaming its own tree):

1. `diameter-tscd/stackyrd-pkg` → placeholder (remote index repo — external, never touch)
2. `stackyrd_` → placeholder (MCP tool names — API contract, covered by tests)
3. `stackyrd://` → placeholder (MCP URI scheme — API contract)
4. `github.com/diameter-tscd/stackyrd/scripts` → new scripts module (`<module>/scripts`, or `github.com/diameter-tscd/<name>/scripts` for bare modules)
5. `diameter-tscd/stackyrd` → `diameter-tscd/<name>`
6. bare `stackyrd` → `<name>` (go.mod, imports, `APP_NAME`, `MODULE_NAME`, `app.name`, `*.log`, banners, error strings, templates)

Skipped: `*.md`, `postman/`, `*.json`, `*.html`, `*.sarif`, banner art, build artifacts (`dist/`, `scripts/dist/`, `store/`, `logs/`), `.git`, `graphify-out`, `.kilo`. `Stackyrd` (display case) left as-is.

## Safety

- Name validated `^[a-z][a-z0-9_-]*$`; dirty git tree requires explicit confirm (no timeout-proceed — destructive op).
- Preview lists file/hit counts before confirm; `planRename`/`execRename` are pure enough for `t.TempDir` tests.
- Tests must build all legacy literals from `legacyName`, or the rename corrupts the test file itself.

## Concurrency + progress

- `parallelFiles` runs scan/rewrite over a bounded worker pool (`NumCPU`, clamped 2–8); errors joined via `errors.Join`, progress callback fires once per file (completion order is non-deterministic — assert multisets, not sequences).
- `progressMilestones` logs `[INFO] Renaming N% (done/total files)` every 10% (always 100%); final `[SUCCESS]` includes elapsed time.
- `Logger` is mutex-guarded — safe for concurrent worker output.
- Preview counts come from `applyReplacementsCounted`, the same function `execRename` uses, so preview totals always equal actual writes.
