# BUILD_SCRIPT — Build Manager Script (`scripts/build/build.go`)

## Overview

`scripts/build/build.go` is the **stackyrd build manager** — a standalone Go CLI tool for building the project binary with optional obfuscation (garble), compression (UPX), version info embedding (goversioninfo), backup archiving, and asset copying.

The script handles the full build pipeline: process management, backup, compilation, compression, and asset deployment — all with color-coded output and interactive prompts with timeout.

---

## Quick Start

```bash
# Interactive build (prompts for garble/UPX)
go run scripts/build/build.go

# Non-interactive with garble
go run scripts/build/build.go -garble

# Non-interactive with garble + UPX
go run scripts/build/build.go -garble -upx

# Verbose build
go run scripts/build/build.go -verbose

# Custom timeout for prompts
go run scripts/build/build.go -timeout 5
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-timeout` | `10` | Timeout for interactive prompts (seconds) |
| `-verbose` | `false` | Enable verbose/debug logging |
| `-garble` | `false` | Enable garble obfuscation (skips interactive prompt) |
| `-upx` | `false` | Enable UPX LZMA compression (skips interactive prompt) |

When `-garble` or `-upx` flags are set, the corresponding interactive prompt is skipped and the feature is applied directly. Without flags, the script prompts the user with a configurable timeout (default 10s) for each decision.

---

## Build Pipeline

The script executes these steps in order:

```
1. checkPath          → Verify project root (find go.mod, chdir, create dist/)
2. checkRequiredTools → Check/install goversioninfo and garble
3. askUserAboutGarble → Prompt (or skip via -garble) for garble obfuscation
4. stopRunningProcess → Kill any running stackyrd instance (via pgrep)
5. createBackup       → Timestamped copy of dist/ files to dist/backups/
6. archiveBackup      → ZIP the backup directory, remove uncompressed copy
7. buildApplication   → go build or garble build with ldflags
8. askUserAboutUPX    → Prompt (or skip via -upx) for UPX LZMA compression
9. compressWithUPX    → Run upx --lzma on the binary
10. copyAssets        → Copy config.yaml and banner.txt to dist/
```

---

## Step Details

### 1. Project Root Detection (`checkPath`)

Walks up the directory tree looking for `go.mod`. Changes working directory to the project root. Creates `dist/` directory if missing.

### 2. Tool Checks (`checkRequiredTools`)

- **goversioninfo** — checks if available; if not, skips version info generation (non-fatal)
- **garble** — checks if available; if not, auto-installs via `go install mvdan.cc/garble@latest`

### 3. Garble Prompt (`askUserAboutGarble`)

Prompts the user: "Use garble build for obfuscation? (y/N)". Waits for input or times out after configured duration. Timed-out input defaults to "no" (regular go build).

### 4. Process Killer (`stopRunningProcess`)

Uses `pgrep -x stackyrd` (or `tasklist` on Windows) to find running instances and kills them with `process.Kill()`. Waits 1 second after killing.

### 5. Backup (`createBackup`)

Copies existing `dist/` files (`stackyrd`, `stackyrd.exe`, `config.yaml`, `banner.txt`) to `dist/backups/{timestamp}/`. Skips non-existent files.

### 6. Backup Archiving (`archiveBackup`)

ZIP-compresses the timestamped backup directory and removes the uncompressed directory. Produces `dist/backups/{timestamp}.zip`.

### 7. Build (`buildApplication`)

Build command and flags:

| Mode | Command |
|------|---------|
| Regular | `go build -ldflags=-s -w -buildid= -trimpath -o dist/stackyrd ./cmd/app` |
| Garble | `garble build -ldflags=-s -w -buildid= -trimpath -o dist/stackyrd ./cmd/app` |

If `goversioninfo` is available, runs `goversioninfo -platform-specific` before the build. Output goes to `dist/stackyrd` (or `dist/stackyrd.exe` on Windows).

### 8. UPX Prompt (`askUserAboutUPX`)

Prompts the user: "Apply UPX LZMA compression to the binary? (y/N)". Same timeout behavior as garble prompt.

### 9. UPX Compression (`compressWithUPX`)

Runs `upx --lzma --best` on the output binary. On macOS, adds `--force-macos` flag. If UPX is not installed, auto-installs via:
- **macOS**: `brew install upx`
- **Linux**: `apt-get install -y upx` or `apk add upx`

Compression failure is non-fatal (build continues without compression).

### 10. Asset Copying (`copyAssets`)

Copies `config.yaml` and `banner.txt` from project root to `dist/`. Missing files are skipped.

---

## Output

```
dist/
├── stackyrd              # Compiled binary
├── config.yaml           # Copied from project root
├── banner.txt            # Copied from project root (if exists)
└── backups/
    └── 20260528_143020.zip  # Timestamped backup archive
```

---

## Graceful Shutdown

Pressing `Ctrl+C` during the build prints a clean message and exits immediately. Signal handling captures `SIGINT` and `SIGTERM`.

---

## Architecture

### Key Functions

| Function | Role |
|----------|------|
| `findProjectRoot` | Walks up directories to find `go.mod` |
| `checkRequiredTools` | Verifies/installs goversioninfo and garble |
| `stopRunningProcess` | Finds and kills running application instances |
| `createBackup` | Copies existing dist files to timestamped backup |
| `archiveBackup` | ZIP-compresses backup directory |
| `buildApplication` | Runs `go build` or `garble build` with ldflags |
| `compressWithUPX` | Applies UPX LZMA compression to binary |
| `copyAssets` | Copies config and assets to output directory |
| `installGarble` | Installs garble via `go install` |
| `installUPX` | Installs UPX via system package manager |
| `findRunningProcesses` | Uses `pgrep`/`tasklist` to find process PIDs |
| `parsePID` | Extracts numeric PID from command output |
| `moveFile` / `copyDir` | File and directory copy utilities |
| `createZipArchive` | Creates ZIP from directory contents |

### Build Configuration

```go
type BuildConfig struct {
    UseGarble        bool
    UseGoversioninfo bool
    UseUPX           bool
    Timeout          time.Duration
    Verbose          bool
}

type BuildContext struct {
    Config     BuildConfig
    Timestamp  string
    BackupPath string
    DistPath   string
    ProjectDir string
}
```

### Constants

| Variable | Default | Description |
|----------|---------|-------------|
| `DIST_DIR` | `dist` | Output directory |
| `APP_NAME` | `stackyrd` | Binary name |
| `MAIN_PATH` | `./cmd/app` | Main package path |
| `CONFIG_YML` | `config.yaml` | Config file to copy |
| `BANNER_TXT` | `banner.txt` | Banner file to copy |

---

## Dependencies

- **External tools**: `go`, `garble`, `upx`, `goversioninfo` (all optional)
- **Go standard library** — `flag`, `os/exec`, `archive/zip`, `runtime`, `strconv`, `time`
- **No external Go dependencies** — standalone script

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/build/

# Vet
go vet ./scripts/build/

# Run interactive build (from project root)
go run scripts/build/build.go

# Run headless build
go run scripts/build/build.go -garble -upx -timeout 0
```
