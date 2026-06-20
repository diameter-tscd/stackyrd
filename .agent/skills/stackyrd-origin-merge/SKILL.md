---
name: stackyrd-origin-merge
description: Merge the latest upstream stackyrd (full version) into this lightweight nano fork while preserving all nano-specific stripping (plugins, heavy infra, pre-built services, metrics, swagger, image utils). Use this whenever the user says they want to sync with upstream, merge origin changes, apply upstream updates, rebase on upstream, pull from origin stackyrd, update from the main repo, catch up with upstream, or "get the latest from stackyrd" — even if they don't explicitly mention merging. Also trigger when the user reports a feature in upstream stackyrd that they want in nano but needs lightweight adaptation, or when they mention "development branch is behind", "need to bring in upstream changes", or "merge the original repo". Do NOT trigger for general development tasks on nano itself (use stackyrd-dev for that).
---

# stackyrd-origin-merge: Merge Upstream into Nano

Use this skill to merge the latest upstream [stackyrd](https://github.com/diameter-tscd/stackyrd) code into this lightweight nano fork while preserving the nano identity: stripped of plugins, heavy infrastructure, pre-built services, metrics, swagger, and image utilities.

## When to use this skill

- "Sync with upstream" — fetch the latest from the original stackyrd repo
- "Merge upstream changes" — incorporate new commits from upstream
- "Update from origin" — pull the latest from the main stackyrd repo
- "Catch up development branch" — bring our fork up to date
- Any request about merging, rebasing, or syncing with the original stackyrd repository

## Repository Setup

| Remote | URL | Purpose |
|--------|-----|---------|
| `origin` | `https://github.com/diameter-tscd/stackyrd-nano.git` | This lightweight nano fork |
| `upstream` | `https://github.com/diameter-tscd/stackyrd.git` | Original full stackyrd repo |

## Strategy

The merge uses a **re-apply approach**:

1. Start from `upstream/development` (latest full stackyrd code)
2. Delete everything that was stripped in the nanofy commit
3. Apply nano-specific modifications on top of remaining files
4. For new upstream files that match strip patterns, also delete them
5. Verify the build compiles

This is more robust than cherry-picking the nanofy commit because upstream may have changed files in conflicting ways. The re-apply approach is deterministic and handles new upstream additions.

## Automated Merge Script

The bundled script `scripts/merge_upstream.go` automates the entire process.

```bash
# Run from project root:
go run .kilo/skills/stackyrd-origin-merge/scripts/merge_upstream.go
```

### What the script does

| Step | Description |
|------|-------------|
| 1. Fetch upstream | `git fetch upstream` |
| 2. Check branch | Creates `development` from `upstream/development` |
| 3. Strip deletions | Removes all files/directories from strip list |
| 4. Apply nano patches | For files that were modified in nano (not deleted), checks out the nano version from `origin/base/nano` |
| 5. Strip new upstream files | Checks for new upstream files matching strip patterns |
| 6. Update go.mod | Runs `go mod tidy` to prune deps |
| 7. Commit | Creates a commit with the merge description |
| 8. Print status | Shows what was done |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-dry-run` | `false` | Show what would be done without making changes |
| `-branch` | `"development"` | Target branch name |
| `-upstream-branch` | `"upstream/development"` | Upstream branch to merge from |
| `-nano-ref` | `"origin/base/nano"` | Nano reference branch for patches |
| `-verbose` | `false` | Verbose output |

### Dry run

Always run with `-dry-run` first to preview changes:

```bash
go run .kilo/skills/stackyrd-origin-merge/scripts/merge_upstream.go -dry-run
```

## What Gets Stripped

This section documents every file and directory pattern removed during the merge. The script uses this list.

### Deleted directories (entire subtrees)

| Directory | Reason |
|-----------|--------|
| `internal/services/modules/` | All 9 pre-built services (cache, broadcast, encryption, grafana, mongodb, multi_tenant, products, tasks, users) |
| `pkg/plugin/` | Entire plugin system (goja, esbuild, gRPC, Lua, WASM, Python runtimes + 10 built-in plugins) |
| `pkg/metrics/` | Prometheus metrics |
| `docs/` | Swagger generated docs (docs.go, swagger.json, swagger.yaml) |
| `scripts/plugin/` | Plugin packaging and build scripts |
| `scripts/swagger/` | Swagger generation script |
| `tests/services/` | Pre-built service tests |
| `tests/infrastructure/` | Infrastructure tests (only afero_test.go and testdata/) |
| `pkg/testing/` | Testing helpers/mocks (moved to `tests/testutil/`) |
| `deployments/` | Kubernetes deployment manifests |

### Deleted individual files

| File | Reason |
|------|--------|
| `internal/middleware/swagger.go` | Swagger middleware |
| `pkg/infrastructure/afero.go` | Afero virtual filesystem |
| `pkg/infrastructure/cron_manager.go` | Cron scheduler |
| `pkg/infrastructure/grafana.go` | Grafana integration |
| `pkg/infrastructure/kafka.go` | Kafka message queue |
| `pkg/infrastructure/minio.go` | MinIO object storage |
| `pkg/infrastructure/mongo.go` | MongoDB database |
| `pkg/infrastructure/redis.go` | Redis cache/queue |
| `pkg/utils/image.go` | Image processing utilities |
| `PLUGIN_GUIDE.md` | Plugin documentation |
| `versioninfo.json` | Windows version info |

### Modified files (nano version replaces upstream)

These files had modifications in the nano branch. The script checks out the nano version for these files. For each, understand WHY it was modified to properly resolve conflicts:

| File | Nano change reason |
|------|--------------------|
| `config.yaml` | Postgres-only config, no service toggles, stripped middleware list |
| `config/config.go` | Stripped heavy infra config structs, postgres-only connections |
| `go.mod` | Direct deps reduced from ~43 to 17, removed Echo/plugin/infra deps |
| `cmd/app/main.go` | Removed swagger init, plugin loading, metrics bootstrap |
| `cmd/app/application.go` | Simplified TUI to console mode fallback |
| `cmd/app/config_manager.go` | Removed plugin/service/metric config loading |
| `cmd/app/constants.go` | Reduced app constants to nano subset |
| `cmd/app/embed.go` | Removed plugin embeds |
| `internal/server/server.go` | Removed metrics routes, swagger, Echo→Gin conversion |
| `internal/middleware/*.go` | Reference changes, no Echo-specific code |
| `pkg/infrastructure/postgres.go` | GORM-based multi-connection, unchanged from upstream |
| `pkg/infrastructure/async_init.go` | Async init manager, minor changes |
| `pkg/infrastructure/registry.go` | Component registry, minor changes |
| `pkg/infrastructure/component.go` | Interface definition, minor changes |
| `AGENTS.md` | Updated for nano project structure |
| `README.md` | Nano branding and usage |

### Startup config check

New upstream files land in `pkg/infrastructure/` — they're kept if they're part of the core (like `postgres.go`), but stripped if they match deleted infrastructure patterns (kafka, mongo, redis, minio, grafana, cron, afero). The script handles this by checking new files against a prefix list.

The script also checks for new pre-built services in `internal/services/modules/` and new plugin directories in `pkg/plugin/builtin/`.

## Manual Merge (if script fails)

If the automated script encounters issues, follow these steps:

1. **Fetch upstream**: `git fetch upstream`
2. **Create merge branch**: `git checkout -b development upstream/development`
3. **Delete stripped files** — remove all directories and files listed in "What Gets Stripped" above
4. **Apply nano patches** — for each modified file, check it out from nano ref:
   `git checkout origin/base/nano -- <path>`
5. **Strip new upstream additions** — check if upstream added new files in directories that should be stripped
6. **Tidy modules**: `go mod tidy`
7. **Verify build**: `go build ./cmd/app`
8. **Commit**: `git add -A && git commit -m "Merge upstream into nano"`
9. **Push**: `git push origin development`

## Conflict Resolution

If `git checkout origin/base/nano -- <path>` fails because upstream renamed or substantially changed a file:

1. Identify the upstream change: `git diff upstream/development -- <path>`
2. Apply the nano intent manually — for example, if upstream added a new feature flag, add it to the nano config in a disabled state
3. If the file was substantially restructured upstream, you may need to port the nano changes as a patch:
   - `git diff origin/base/nano -- <path> > /tmp/nano-patch.patch`
   - `git checkout upstream/development -- <path>`
   - Apply the patch: `git apply /tmp/nano-patch.patch` (may need manual fixup)

## Verification

After merging, always verify:

```bash
# Build check
go build ./cmd/app

# Run tests
go test ./... 2>&1 | tail -20

# Check binary size (should be smaller than full stackyrd)
go build -o /tmp/stackyrd-nano ./cmd/app && ls -lh /tmp/stackyrd-nano

# Check core deps are gone
go list -m all | grep -E 'kafka|mongo|redis|minio|grafana|prometheus|plugin'
# → should produce no output
```

## go.mod Dependency Baseline

The nano version's `go.mod` should have these **direct** dependencies (17):

| Dependency | Purpose |
|---|---|
| `gin-gonic/gin` | HTTP router |
| `charmbracelet/bubbletea` | TUI |
| `charmbracelet/bubbles` | TUI components |
| `charmbracelet/lipgloss` | TUI styling |
| `rs/zerolog` | Logging |
| `spf13/viper` | Config |
| `jackc/pgx/v5` | Postgres driver |
| `gorm.io/gorm` | ORM |
| `gorm.io/driver/postgres` | Postgres ORM driver |
| `golang-jwt/jwt/v5` | JWT auth |
| `gorilla/websocket` | WebSocket |
| `go-playground/validator/v10` | Validation |
| `google/uuid` | UUID generation |
| `stretchr/testify` | Testing |
| `shirou/gopsutil/v3` | System stats |
| `ulikunitz/xz` | XZ compression |
| `spf13/afero` | Filesystem abstraction (only for embed/banner) |

## Post-Merge Cleanup

After a successful merge:

1. Update `config.yaml` version: `app.version` should reflect the merge
2. Update `README.md` if upstream changed the template
3. Update skill docs if new skills were added upstream in `.agent/skills/`
4. Push: `git push origin development`
