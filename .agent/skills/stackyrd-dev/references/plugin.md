# Creating a Plugin

stackyrd supports four plugin runtimes: TypeScript (sandboxed goja VM), Lua (gopher-lua), Python (gRPC subprocess), and Go (compiled in-process). All plugins live in `pkg/plugin/builtin/{name}/` with a `plugin.yaml` manifest.

## Plugin Manifest (`plugin.yaml`)

Every plugin needs a manifest declaring its metadata and entrypoint:

```yaml
name: my-plugin
version: "1.0.0"
description: "What this plugin does"
entrypoint: "ts:scripts/handler.ts"   # or lua:, ext:, go:
author: "developer"
enabled: true
```

The entrypoint prefix determines the runtime:
- `ts:` → TypeScript (transpiled via esbuild → executed in goja)
- `lua:` → Lua (executed in gopher-lua)
- `ext:` → External (gRPC subprocess, e.g., Python)
- `go:` → Native Go (compiled into the binary)

## TypeScript Plugins

### File structure
```
pkg/plugin/builtin/{name}/
├── plugin.yaml
└── scripts/
    └── handler.ts
```

### `scripts/handler.ts`

```typescript
// Available globals (see sdk/plugin.d.ts):
//   $args: Record<string, any>      -- arguments passed from caller
//   $logger: { info, warn, error }  -- structured logger
//   $infra: { get(name: string): any }  -- access infrastructure components
//   $done(result?: any): void       -- complete the plugin execution
//   $limits: { timeout_ms, max_memory_bytes }

function main() {
    $logger.info("Plugin started", { args: $args });

    // Access infrastructure
    const redis = $infra.get("redis");
    if (redis) {
        // Use redis
    }

    $done({ status: "ok", data: $args });
}

main();
```

### Globals

| Global | Type | Description |
|--------|------|-------------|
| `$args` | `Record<string, any>` | Input arguments from `bridge.Execute(name, args)` |
| `$logger` | `{ info, warn, error }` | Structured key-value logger (`$logger.info("msg", { key: "val" })`) |
| `$infra` | `{ get(name): any }` | Access registered infrastructure components by name |
| `$done` | `(result?: any) => void` | Must be called to complete execution and return result |
| `$limits` | `{ timeout_ms, max_memory_bytes }` | Resource limits from config + per-plugin overrides |

### SDK reference

Read the full type declarations in `pkg/plugin/sdk/plugin.d.ts` for all available types and interfaces.

## Lua Plugins

### File structure
```
pkg/plugin/builtin/{name}/
├── plugin.yaml  (entrypoint: "lua:scripts/handler.lua")
└── scripts/
    └── handler.lua
```

### `scripts/handler.lua`

```lua
-- Available globals:
--   args (table)     -- input arguments
--   logger (table)   -- { info(msg, ...), warn(msg, ...), error(msg, ...) }
--   infra (table)    -- { get(name) } -- returns nil if not found
--   done(result)     -- must be called to complete

function main()
    logger:info("Plugin started", args)

    local redis = infra:get("redis")
    if redis then
        -- use redis
    end

    done({ status = "ok", data = args })
end

main()
```

## Python Plugins (External)

### File structure
```
pkg/plugin/builtin/{name}/
├── plugin.yaml  (entrypoint: "ext:scripts/handler.py")
└── scripts/
    └── handler.py
```

### `scripts/handler.py`

```python
# The external runtime loads this file, starts a gRPC server, and
# communicates with stackyrd via protobuf. The Plugin interface is:
class Handler:
    def execute(self, args: dict) -> dict:
        # args: dict of input arguments
        # return: dict to send back as result
        return {"status": "ok", "data": args}
```

See `pkg/plugin/python/sdk.py` for the base `Plugin` class.

## Go Plugins

Create a `.go` file in `pkg/plugin/` implementing the `Plugin` interface with `init()` registration:

```go
package plugin

import (
    "stackyrd/pkg/plugin"
)

func init() {
    p := &MyGoPlugin{}
    registry.Register("my-go-plugin", p)
}

type MyGoPlugin struct{}

func (p *MyGoPlugin) Execute(ctx *plugin.Context) *plugin.Result {
    // ctx.Args contains input arguments
    // ctx.Infra provides access to infrastructure
    // ctx.Logger for logging
    return &plugin.Result{
        Success: true,
        Data:    map[string]interface{}{"status": "ok"},
    }
}
```

Manifest entrypoint: `"go:MyGoPlugin"`

## Interacting with Plugins

### From a Service (via PluginBridge in Dependencies)

```go
func init() {
    registry.RegisterService("my_service", func(cfg *config.Config, l *logger.Logger, deps *registry.Dependencies) interfaces.Service {
        svc := NewMyService(cfg.Services.IsEnabled("my_service"), l)
        if b, ok := deps.Get("plugins"); ok {
            svc.bridge = b.(*plugin.PluginBridge)
        }
        return svc
    })
}

// At runtime:
func (s *MyService) handleRequest(c *gin.Context) {
    if s.bridge != nil && s.bridge.HasPlugin("inspector") {
        result, err := s.bridge.Execute("inspector", map[string]interface{}{
            "mode": "analyze",
        })
    }
}
```

### From an Infrastructure Component (via global registry)

```go
reg := infrastructure.GetGlobalRegistry()
if comp, ok := reg.Get("plugins"); ok {
    bridge := comp.(*plugin.PluginBridge)
    result, _ := bridge.Execute("inspector", args)
}
```

### Convenience accessor

```go
bridge := plugin.GetGlobalPluginBridge()
if bridge != nil && bridge.HasPlugin("inspector") {
    result, _ := bridge.Execute("inspector", args)
}
```

## Management API

A running stackyrd instance exposes plugin management endpoints under `/api/v1/plugins/`:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/plugins` | GET | List all plugins with status |
| `/api/v1/plugins/{name}` | GET | Get plugin metadata |
| `/api/v1/plugins/{name}/execute` | POST | Execute a plugin with JSON body |
| `/api/v1/plugins/upload` | POST | Upload a new plugin archive |
| `/api/v1/plugins/scripts` | GET | List all available scripts |
| `/api/v1/plugins/scripts/{name}` | GET | Get script content |
| `/api/v1/plugins/{name}/unload` | POST | Unload a plugin |
| `/api/v1/plugins/manager/status` | GET | Full system status |

## Key Points

- **Lua and Go plugins are the simplest** — Lua needs no transpilation and runs in an embedded VM; Go plugins compile directly into the binary
- **TypeScript plugins cost more** — esbuild transpilation is done once per file (SHA256-cached on disk), but each execution creates a fresh goja VM
- **Python plugins cost the most** — each execution starts a gRPC subprocess with timeout enforcement
- **Plugin sandboxing**: sandbox is enforced per runtime — goja has no filesystem/network access, gopher-lua has restricted libraries, gRPC subprocesses have OOM monitoring via gopsutil
- **Use the CLI tool**: `go run scripts/plugin/pkg.go` provides list/info/exec/upload/unload/status commands
