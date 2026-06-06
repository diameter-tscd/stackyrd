# PLUGIN_SCRIPT — Plugin Manager Script (`scripts/plugin/pkg.go`)

## Overview

`scripts/plugin/pkg.go` is the **stackyrd plugin manager** — a standalone Go CLI tool for listing, inspecting, executing, and managing plugins via the plugin REST API at `/api/v1/plugins`.

It wraps all plugin management endpoints (list, detail, execute, upload, unload, manager status) into a convenient command-line interface with structured output, color-coded types, and per-plugin execution stats.

---

## Quick Start

```bash
# List all loaded plugins with status and manager metrics
go run scripts/plugin/pkg.go list

# Show detailed info for a specific plugin
go run scripts/plugin/pkg.go info inspector

# Execute a plugin with mode argument
go run scripts/plugin/pkg.go exec inspector -mode ping

# Execute a plugin with key=value arguments
go run scripts/plugin/pkg.go exec aggregator mode=dashboard

# Execute a Lua plugin
go run scripts/plugin/pkg.go exec lua_demo name=world

# Upload a new script to a plugin
go run scripts/plugin/pkg.go upload inspector -file ./handler.ts

# Upload inline script content
go run scripts/plugin/pkg.go upload inspector -content "function handler() { $done({success:true}); } handler();"

# Unload a plugin
go run scripts/plugin/pkg.go unload inspector

# Show plugin manager health metrics
go run scripts/plugin/pkg.go status
```

---

## Subcommands

### `list`

Lists all loaded plugins with their type, status, version, execution count, and manager-level metrics.

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

Output columns per plugin:
- **Type** — color-coded label: `[typescript]`, `[lua]`, `[python]`, `[go]`
- **Name** — plugin name
- **Description** — truncated to 42 chars
- **Version** — semver version
- **Exec** — total execution count
- **Status** — `✓` (loaded) or `✗` (error)

Manager footer shows: active executions, goroutine count, memory usage/limit with percentage, uptime.

```bash
go run scripts/plugin/pkg.go list
go run scripts/plugin/pkg.go list -url http://192.168.1.100:8080/api/v1/plugins
go run scripts/plugin/pkg.go list -verbose
```

---

### `info`

Shows detailed information for a single plugin, including type, status, entrypoint, execution stats, and embedded file size.

| Argument | Description |
|----------|-------------|
| `<name>` | Plugin name (required) |

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

```bash
go run scripts/plugin/pkg.go info inspector
go run scripts/plugin/pkg.go info lua_demo
```

Output includes: plugin name, status (with ✓/✗ icon), type (color-coded), entrypoint, version, author, description, execution count, last/total execution time (ms), load time (ms), embedded file size.

---

### `exec`

Executes a plugin with optional arguments and displays the result.

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `""` | Execution mode (plugin-specific, e.g. `ping`, `status`, `dashboard`) |
| `-raw` | `false` | Print raw JSON response instead of formatted output |
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

| Argument | Description |
|----------|-------------|
| `<name>` | Plugin name (required) |
| `key=val ...` | Optional `key=value` pairs passed as execution args |

**Argument merging:**
- If `-mode` is set, it is added to the args map as `"mode": "<value>"`
- Additional `key=value` positional args are parsed from `=` delimiters and merged into the args map
- `-mode` takes precedence if the same key is also provided as a positional arg

```bash
# Execute with mode flag
go run scripts/plugin/pkg.go exec inspector -mode ping

# Execute with key=value positional args
go run scripts/plugin/pkg.go exec lua_demo name=developer

# Execute with both mode and additional args
go run scripts/plugin/pkg.go exec aggregator mode=transform input='{"name":"hello"}' rule=uppercase

# Raw JSON output
go run scripts/plugin/pkg.go exec inspector -mode ping -raw
```

On success, the result data is pretty-printed as JSON. On failure, the error message is displayed and the script exits with code 1.

---

### `upload`

Uploads or replaces a plugin script at runtime to the on-disk overlay, shadowing the embedded version.

| Flag | Default | Description |
|------|---------|-------------|
| `-file` | `""` | Path to a local script file to upload |
| `-content` | `""` | Inline script content (alternative to `-file`) |
| `-script-path` | `scripts/handler.ts` | Remote script path within the plugin |
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

| Argument | Description |
|----------|-------------|
| `<name>` | Plugin name (required) |

**Content sources (mutually exclusive):**
1. `-file path/to/script.ts` — reads from a local file
2. `-content "source code"` — inline string

If neither `-file` nor `-content` is provided, the command exits with an error.

```bash
# Upload from file
go run scripts/plugin/pkg.go upload inspector -file ./handler.ts

# Upload inline content
go run scripts/plugin/pkg.go upload inspector -script-path scripts/handler.ts \
  -content "function handler() { $done({success: true, data: {msg: \"hello\"}}); } handler();"

# Upload a Lua script
go run scripts/plugin/pkg.go upload lua_demo -script-path scripts/handler.lua \
  -file ./custom_lua_handler.lua
```

The file is written to `store/plugins/{name}/scripts/{file}` on disk. The next execution will load the new source.

---

### `unload`

Unloads a plugin from the registry. The plugin will be re-discovered on next app restart.

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

| Argument | Description |
|----------|-------------|
| `<name>` | Plugin name (required) |

```bash
go run scripts/plugin/pkg.go unload inspector
go run scripts/plugin/pkg.go unload lua_transformer
```

On success, confirms the unloaded plugin name. On failure, displays the API error response.

---

### `status`

Shows plugin manager health metrics: total/loaded plugins, execution counts, goroutines, memory usage, and uptime.

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost:8080/api/v1/plugins` | Base URL of the plugin API |
| `-verbose` | `false` | Enable verbose/debug logging |

```bash
go run scripts/plugin/pkg.go status
```

Output sections:
1. **Manager Status** — total plugins, loaded plugins, total executions, active executions, goroutine count, memory usage/limit/percent, uptime
2. **Plugin Details** — per-plugin summary with name, version, type, execution count, and status icon

---

## Help

```
go run scripts/plugin/pkg.go -h           # show global help
go run scripts/plugin/pkg.go --help       # show global help
go run scripts/plugin/pkg.go help         # show global help
go run scripts/plugin/pkg.go list -h      # show list subcommand help
go run scripts/plugin/pkg.go info -h      # show info subcommand help
```

---

## Verbose / Debug Mode

All subcommands support `-verbose` (or a global `-V` / `--verbose` in any position):

```bash
go run scripts/plugin/pkg.go -V
go run scripts/plugin/pkg.go list -verbose
go run scripts/plugin/pkg.go exec inspector -mode ping -verbose
```

---

## Architecture

### Client Layer

The script communicates with the plugin REST API via a `Client` struct that wraps `net/http`:

```
Client.doRequest(method, path, body) → *http.Response
    │
    ├── getJSON(path, target)       → GET request, JSON response
    ├── postJSON(path, body, target)→ POST request with JSON body
    ├── putJSON(path, body, target) → PUT request with JSON body
    └── delete(path)                → DELETE request
```

All requests use a 30-second timeout. The base URL defaults to `http://localhost:8080/api/v1/plugins` and is overridable via the `-url` flag.

### API Response Types

| Struct | Endpoint | Fields |
|--------|----------|--------|
| `PluginListResponse` | `GET /api/v1/plugins` | `plugins[]`, `total`, `loaded`, `active_execs`, `goroutines`, `memory_bytes`, `memory_limit`, `memory_percent`, `uptime_seconds` |
| `PluginDetail` | `GET /api/v1/plugins/:name` | `name`, `version`, `description`, `author`, `entrypoint`, `type`, `status`, `load_time_ms`, `embedded_file_size`, `execute_count`, `last_execution_ms`, `total_execution_ms` |
| `ExecuteResponse` | `POST /api/v1/plugins/:name/execute` | `success`, `data`, `error` |
| `UploadResponse` | `PUT /api/v1/plugins/:name/scripts/:file` | `message`, `path` |
| `DeleteResponse` | `DELETE /api/v1/plugins/:name` | `message`, `name` |
| `ManagerStatus` | `GET /api/v1/plugins/manager/status` | `total_plugins`, `loaded_plugins`, `total_executions`, `active_executions`, `goroutine_count`, `memory_usage_bytes`, `memory_limit_bytes`, `memory_percent`, `uptime_seconds`, `plugins[]` |

### Key Functions

| Function | Role |
|----------|------|
| `cmdList` | Fetches `GET /api/v1/plugins` and displays color-coded plugin table with manager footer |
| `cmdInfo` | Fetches `GET /api/v1/plugins/:name` and displays detailed plugin information |
| `cmdExec` | Builds args map from `-mode` + positional `key=val` pairs, posts to `POST /api/v1/plugins/:name/execute`, pretty-prints result |
| `cmdUpload` | Reads file content or uses inline string, puts to `PUT /api/v1/plugins/:name/scripts/:path` |
| `cmdUnload` | Sends `DELETE /api/v1/plugins/:name` and confirms unload |
| `cmdStatus` | Fetches `GET /api/v1/plugins/manager/status` and displays manager health snapshot |

### Output Formatting

- **Type colors**: TypeScript → cyan, Lua → green, Python (external) → yellow, Go → purple
- **Status icons**: loaded → green `✓`, error → red `✗`
- **Byte formatting**: `formatBytes()` renders human-readable sizes (`B`, `KiB`, `MiB`, `GiB`)
- **Description truncation**: `truncate()` cuts long descriptions at 42 chars with `...`
- **Raw mode**: `exec -raw` bypasses formatting and prints the raw JSON response

### Dependencies

- **Go standard library** — `flag`, `net/http`, `encoding/json`, `os`, `strings`, `time`
- **No external dependencies** — communicates via HTTP to the running stackyrd instance

---

## Build & Development

```bash
# Build (compile check)
go build -o /dev/null ./scripts/plugin/

# Vet
go vet ./scripts/plugin/

# Run (from project root, with stackyrd running)
go run scripts/plugin/pkg.go list

# Run against a remote instance
go run scripts/plugin/pkg.go list -url http://192.168.1.100:8080/api/v1/plugins
```
