# PKG_SCRIPT — Package Manager (`stackyrd pkg`)

## Overview

`stackyrd pkg` (implemented in `scripts/internal/pkg/pkg.go`) is the **stackyrd package manager** — part of the standalone `scripts/` CLI module for installing, tracking, and managing infrastructure packages from the [stackyrd-pkg](https://github.com/diameter-tscd/stackyrd-pkg) repository.

Packages are Go/yrd source files installed into `pkg/infrastructure/`. The command handles downloading, converting (`.yrd` → `.go`), and metadata tracking via a YAML manifest.

---

## Quick Start

```bash
# Interactive install (search packages by name)
./scripts/yrd pkg

# Direct install by name@version
./scripts/yrd pkg install -pkg cloud/aws/ec2@1.0.0

# Reinstall an existing package (fuzzy name or -pkg flag)
./scripts/yrd pkg reinstall ec2
./scripts/yrd pkg reinstall -pkg cloud/aws/ec2@1.0.0

# Manual install by full repo path
./scripts/yrd pkg install -path /pkg/infrastructure/cloud/aws/ec2/1.0.0/ec2.go

# List installed packages
./scripts/yrd pkg list

# Show package details
./scripts/yrd pkg info cloud/aws/ec2

# Remove a package
./scripts/yrd pkg remove cloud/aws/ec2

# Upgrade all packages to latest
./scripts/yrd pkg upgrade

# Refresh local index cache
./scripts/yrd pkg update
```

---

## Subcommands

### `install`

Installs a package from the index or via a manual path.

| Flag | Default | Description |
|------|---------|-------------|
| `-pkg` | `""` | Package to install in `name@version` format (e.g. `cloud/aws/ec2@1.0.0`) |
| `-path` | `""` | Manual package path on the remote repository (e.g. `/pkg/infrastructure/cloud/aws/ec2/1.0.0/ec2.go`) |
| `-timeout` | `30` | Timeout for user prompts (seconds) |
| `-verbose` | `false` | Enable verbose/debug logging |
| `-yes` | `false` | Auto-confirm prompts (skip confirmation) |
| `-dry-run` | `false` | Preview what would be installed without making changes |

**Usage patterns:**

```bash
# Interactive mode — prompts for package name search and version selection
./scripts/yrd pkg install

# Non-interactive with explicit package
./scripts/yrd pkg install -pkg cloud/aws/ec2@1.0.0

# Manual path install — auto-detects .go vs .yrd via HEAD request
./scripts/yrd pkg install -path /pkg/infrastructure/cloud/aws/ec2/1.0.0/ec2.go
```

When `-path` is provided, the script:
1. Parses the path to extract `{pkgName}/{version}/{file}`
2. Issues a HEAD request to check whether `.go` or `.yrd` extension exists on the remote
3. Downloads the resolved file into `pkg/infrastructure/`
4. If `.yrd`, runs `yrdconv` to convert to `.go`
5. Records the installation in `package.yml`

**"Already installed" check:** The script checks the **manifest** (`package.yml`) to determine if a package is already installed, not the filesystem. If a `.go` file exists on disk but the manifest has no entry (e.g. from a previous buggy version, manual placement, or a cleared manifest), the install proceeds normally — it overwrites the file and creates a fresh manifest entry.

---

### `reinstall`

Re-downloads and re-installs an existing package, overwriting files and refreshing the manifest timestamp.

| Flag | Default | Description |
|------|---------|-------------|
| `-pkg` | `""` | Explicit package to reinstall in `name@version` format |
| `-yes` | `false` | Skip confirmation prompt |
| `-dry-run` | `false` | Preview without reinstalling |
| `-verbose` | `false` | Enable verbose logging |

| Argument | Description |
|----------|-------------|
| `<name>` | Package name — full path or short last-segment name (e.g. `cloud/aws/ec2` or just `ec2`). Used only when `-pkg` is not specified. |

**Two modes:**
1. **Fuzzy from manifest** — `reinstall ec2` finds the package in the manifest via `resolvePackageName`, then re-downloads its currently-installed version from the index.
2. **Explicit `-pkg`** — `reinstall -pkg cloud/aws/ec2@1.0.0` forces reinstall of a specific package+version regardless of manifest state (useful for orphaned files without a manifest entry).

```bash
./scripts/yrd pkg reinstall ec2                        # fuzzy match, same version
./scripts/yrd pkg reinstall -pkg cloud/aws/ec2@1.0.0   # explicit package+version
./scripts/yrd pkg reinstall -yes cloud/aws/ec2          # skip confirmation
./scripts/yrd pkg reinstall -dry-run ec2                # preview only
```

---

### `list`

Lists all installed packages from the manifest with version, install date, and upgrade status.

| Flag | Default | Description |
|------|---------|-------------|
| *(none)* | | No flags; reads `package.yml` and cached index |

Output columns:
- **Name** — package name
- **Version** — installed version
- **Date** — install date (YYYY-MM-DD)
- **Status** — "up to date", "X.Y.Z available" (if newer exists), or "not in index"

```bash
./scripts/yrd pkg list
```

---

### `info`

Shows detailed information for a single installed package.

| Argument | Description |
|----------|-------------|
| `<name>` | Package name (required) |

```bash
./scripts/yrd pkg info cloud/aws/ec2
```

Output includes: name, version, source (index/manual), manual path (if manual), files, install date, update date, status.

---

### `remove`

Removes an installed package, deleting its files and updating the manifest.

| Flag | Default | Description |
|------|---------|-------------|
| `-yes` | `false` | Skip confirmation prompt |
| `-dry-run` | `false` | Preview files that would be removed without deleting |

| Argument | Description |
|----------|-------------|
| `<name>` | Package name — full path or short last-segment name (e.g. `cloud/aws/ec2` or just `ec2`) |

**Name resolution:**
1. Exact match against full package name is tried first
2. If no exact match, searches for packages whose last path segment matches the input (e.g. `oauth` matches `pkg/infrastructure/auth/oauth`)
3. If exactly one match is found, it is selected automatically
4. If multiple packages share the same short name (e.g. `pkg/infrastructure/auth/oauth` and `pkg/infrastructure/other/oauth`), an interactive prompt lets you choose which to remove

```bash
./scripts/yrd pkg remove cloud/aws/ec2             # full path
./scripts/yrd pkg remove ec2                       # short last-segment name
./scripts/yrd pkg remove -yes ec2                  # skip confirmation
./scripts/yrd pkg remove -dry-run ec2              # preview only
```

Safety: the script verifies that files are inside `pkg/infrastructure/` before deleting (path traversal protection). Errors are reported accurately — "File already removed" if the file is missing, "Failed to remove" with the specific error for other failures. The project root is resolved via `findProjectRoot` (go.mod walk) so remove works correctly from any subdirectory.

---

### `upgrade`

Upgrades installed packages to their latest available versions.

| Flag | Default | Description |
|------|---------|-------------|
| `-yes` | `false` | Skip per-package confirmation prompts |
| `-dry-run` | `false` | Preview upgrades without making changes |

Upgrade a single package by name:
```bash
./scripts/yrd pkg upgrade cloud/aws/ec2
```

Upgrade all installed packages (interactive per-package):
```bash
./scripts/yrd pkg upgrade
```

Upgrade all without confirmation:
```bash
./scripts/yrd pkg upgrade -yes
```

**Backup and rollback:** Before downloading new files, existing files are renamed with a `.bak.{timestamp}` suffix. If download or conversion fails, the backup is restored. Successful upgrades clean up backup files.

**Skipped packages:**
- Packages installed from `source: manual` are skipped (they are not in the index)
- Packages already at the latest version are skipped
- Packages not found in the index are skipped

---

### `update`

Refreshes the local package index cache and checks for available updates.

```bash
./scripts/yrd pkg update
```

What it does:
1. Downloads the fresh index from the remote repository
2. Saves it to `store/pkg-index.cache`
3. Updates `package.yml` metadata (`last_updated`, `index_url`)
4. Compares installed package versions against the index and reports any newer versions

---

## Legacy Mode (Backward Compatible)

When invoked without a subcommand, the script falls back to the original install-only mode:

```bash
./scripts/yrd pkg                          # interactive install
./scripts/yrd pkg -pkg cloud/aws/ec2@1.0.0 # direct install
./scripts/yrd pkg -verbose                 # with debug output
```

| Flag | Default | Description |
|------|---------|-------------|
| `-pkg` | `""` | Package to install directly (format: `name@version`) |
| `-timeout` | `30` | Timeout for user prompts (seconds) |
| `-verbose` | `false` | Enable verbose logging |

---

## Help

```
./scripts/yrd pkg -h          # show global help
./scripts/yrd pkg --help      # show global help  
./scripts/yrd pkg help        # show global help
./scripts/yrd pkg install -h  # show install subcommand help (via flag.ExitOnError)
```

---

## Manifest (`package.yml`)

The manifest at the project root tracks all installed packages. It is written atomically (temp file + `os.Rename`).

### Structure

```yaml
meta:
  last_updated: "2026-05-28T22:07:59+07:00"
  index_url: "https://raw.githubusercontent.com/diameter-tscd/stackyrd-pkg/master/index"

packages:
  cloud/aws/ec2:
    name: cloud/aws/ec2
    version: 1.0.0
    installed_at: "2026-05-28T22:07:59+07:00"
    updated_at: "2026-05-28T22:07:59+07:00"
    files:
      - ec2.go
    install_root: pkg/infrastructure
    source: index
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Package name (matches index path) |
| `version` | string | Installed semver version |
| `installed_at` | string | ISO 8601 install timestamp |
| `updated_at` | string | ISO 8601 last-update timestamp |
| `files` | []string | Files installed for this package (after .yrd → .go conversion) |
| `install_root` | string | Installation directory (always `pkg/infrastructure`) |
| `source` | string | `"index"` for index installs, `"manual"` for `-path` installs |
| `manual_path` | string | Populated only for `source: manual` (the original `-path` value) |

**Important:** The `files` list contains only the files that were actually downloaded for the package (with `.yrd` → `.go` mapping), not all files in the `pkg/infrastructure/` directory.

---

## Index Cache (`store/pkg-index.cache`)

The index downloaded from the remote repository is cached locally to speed up `list` and `update` operations.

- **Path:** `store/pkg-index.cache`
- **Staleness warning:** Displays a warning if cache is older than 1 hour
- **Refresh:** Run `./scripts/yrd pkg update` to refresh

---

## Verbose / Debug Mode

All subcommands and legacy mode support `-verbose` (or a global `-V` / `--verbose` in any position):

```bash
./scripts/yrd pkg -V
./scripts/yrd pkg install -verbose
./scripts/yrd pkg upgrade --verbose
```

---

## Architecture

### File Flow

```
Index (remote) → fetchIndex() → parseIndexLines() → PackageInfo[]
                        │
                        ├──→ downloadFiles() → .yrd/.go files to pkg/infrastructure/
                        │       │
                        │       └──→ convertAndInstall() → yrdconv extracts .go from .yrd
                        │
                        └──→ trackedFiles() → filters whitelist, maps .yrd→.go
                              │
                              └──→ saveManifest() → package.yml (atomic write)
```

### Key Functions

| Function | Role |
|----------|------|
| `fetchIndex` | Downloads the remote index file |
| `parseIndexLines` | Parses flat index into structured `PackageInfo` entries with versions and file paths |
| `loadCachedIndex` | Reads local `store/pkg-index.cache` for offline operations |
| `loadManifest` / `saveManifest` | Read/write `package.yml` with atomic tempfile+rename |
| `downloadFiles` | Downloads whitelisted files (`.go` / `.yrd`) to install root |
| `downloadFile` | Single-file download (used by manual install) |
| `convertAndInstall` | Runs `yrdconv` to decode `.yrd` files into `.go`, cleans up originals |
| `trackedFiles` | Maps original file list to final installed files (`.yrd` → `.go` removal) |
| `parseManualPath` | Validates and deconstructs a `-path` argument into pkg/version/filename |
| `resolveManualFilename` | HEAD-requests remote to auto-detect `.go` vs `.yrd` for manual paths |
| `ensureYrdconv` | Downloads the `yrdconv` binary if not present in `scripts/pkg/` |
| `promptUserByName` | Interactive package search with substring matching |
| `promptVersion` | Interactive version selection from available versions |
| `confirmPrompt` | Generic Y/N prompt with input validation |

### Package Resolution for Manual Paths (`-path`)

When a user provides a manual path like `/pkg/infrastructure/cloud/aws/ec2/1.0.0/ec2`:

1. `parseManualPath` parses the path, identifying the semver version segment
2. `resolveManualFilename` issues HEAD requests to check for `ec2.go` and `ec2.yrd` on the remote
3. The first URL that returns 200 is used for download

### Upgrade Rollback Safety

During `upgrade`, each existing file is backed up as `{file}.bak.{unix_timestamp}` before new files are downloaded. If any failure occurs:
- Download failures → backups are restored
- Conversion failures → backups are restored
- Successful completion → backup files are cleaned up

### File Whitelist

Only files matching `\.yrd$` or `\.go$` are downloaded or tracked. Non-whitelisted files (e.g. `README.md`) listed in the index are silently skipped during download and never appear in the manifest `files` list.

### Dependencies

- **`gopkg.in/yaml.v3`** — manifest serialization
- **`yrdconv`** — external Go binary downloaded from GitHub releases for `.yrd` → `.go` conversion
- **Go standard library** — `flag`, `net/http`, `os/exec`, `regexp`, `sort`, etc.

---

## Build & Development

```bash
# Build the CLI once
cd scripts && go build -o /dev/null .

# Vet
go vet ./...

# Run (from project root)
./scripts/yrd pkg
```
