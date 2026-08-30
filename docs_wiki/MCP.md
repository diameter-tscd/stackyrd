# MCP Server

Model Context Protocol endpoint built into `pkg/infrastructure/mcpserver.go`. Implements `InfrastructureComponent` + `RouteRegistrar` (auto-mounted on `mcp.endpoint`), with all state in the singleton `*MCPServer`.

Protocol `2026-07-28` (current) with backward compat for `2025-11-25` and `2025-03-26`.

## Enable

```yaml
mcp:
  enabled: true
  endpoint: /mcp
  token: "f76fda0080884d4ab36b93abf6421d49" # empty = auto-generates 32-byte hex temp token (WARN logged)
  allowed_origins: []  # [] = allow all; e.g. ["https://app.example.com","https://*.example.org","*"]
  rate_limit_enabled: false
  rate_limit_ip: 100            # max requests per window
  rate_limit_time: 60           # window seconds
  rate_limit_cooldowntime: 300  # block seconds after breach
  rate_limit_excludeip: ["127.0.0.1","::1","localhost"]
```

Defaults match `config/config.go` (`60s` window, `300s` cooldown, `100` req). Config file example uses `1000 / 36000 / 3200`. Empty `token` generates `hex(32 rand bytes)` at boot and logs `MCP temporary token generated — set mcp.token in config.yaml for persistence`.

Identity env (K8s Downward API): `POD_NAME`/`POD_IP`/`POD_NAMESPACE`/`NODE_NAME`/`INSTANCE_ID`; local dev synthesizes `<hex>-<hostname>-<pid>` via `resolveIdentity()`.

## Transport

Streamable HTTP — single `POST /mcp` plus `OPTIONS` preflight. `GET`/`DELETE` return `405` (`-32601`). Batch JSON-RPC (`[ {...}, {...} ]`) is supported; notifications (no `id`) return `202 Accepted` with no body. Body size capped at `1MiB` (`io.LimitReader`). Every response sets `MCP-Protocol-Version` and `X-MCP-Instance-ID` / `X-MCP-Pod-Name` headers.

SSE: when `Accept: text/event-stream`, responses are `text/event-stream` with `event: message` frames (`writeSSE` / `writeSSEBatch`); `GET` with SSE returns `405` event stream.

## Auth

Required on every `POST` when `mcp.token != ""` (always protected — no open mode). Accepts:

- `Authorization: Bearer <token>`
- `X-MCP-Token: <token>`

Missing/invalid → `401` (`-32603 Unauthorized`).

## Protocol Negotiation

Header `MCP-Protocol-Version` (case-insensitive fallback `Mcp-Protocol-Version`) and/or body `_meta["io.modelcontextprotocol/protocolVersion"]`. Rules:

- Both present but differ → `400` (`-32020 Header mismatch`)
- Either present but not in `["2026-07-28","2025-11-25","2025-03-26"]` → `400` (`-32022 Unsupported protocol version`, `data: {supported, requested}`)
- Neither present → defaults to `2025-03-26` (legacy compat)
- Accepted version is echoed as `MCP-Protocol-Version` response header
- Batch requests validate the header version once before dispatch

Modern (`2026-07-28` / `2025-11-25`) unknown methods return `404` with `-32601`; legacy returns `200` with `-32601` envelope.

## CORS / Origin

Global CORS (`middleware.cors` allow-all) is bypassed for the MCP endpoint. MCP handles its own CORS:

- `allowed_origins: []` (default) — any `Origin` allowed, echoed as `Access-Control-Allow-Origin` (no block)
- Otherwise whitelist `["https://app.example.com","https://*.example.org","*"]` with `*.` wildcard via `isWildcardOrigin`; non-listed → `403 Forbidden` (`-32000`)
- `OPTIONS` → `204` with `Access-Control-Allow-Methods: POST, GET, DELETE, OPTIONS`, `Access-Control-Allow-Headers` (echoed from `Access-Control-Request-Headers` or default `Content-Type, Accept, Authorization, MCP-Protocol-Version, X-MCP-Token, X-Requested-With`), `Max-Age: 86400`

## Rate Limiting

Disabled by default (`rate_limit_enabled: false`). When enabled (`mcpserver.allowIP`):

- Per-IP sliding window: `rate_limit_ip` requests per `rate_limit_time` seconds; breach → `429 Too Many Requests` (`-32003`, `Retry-After` header, `data: {retry_after, cooldown}`)
- Block lasts `rate_limit_cooldowntime` seconds
- Excluded IPs (`rate_limit_excludeip` plus implicit `127.0.0.1`/`::1`/`localhost`) bypass the limiter; host-only part of `ip:port` is checked
- `GetStatus()` exposes `rate_limit_tracked_ips` / `rate_limit_blocked_ips`

## Instance Identity

`InstanceIdentity{instance_id, pod_name, pod_ip, namespace, node_name, hostname, pid, started_at}` from `resolveIdentity()`. Exposed via:

- Response headers `X-MCP-Instance-ID`, `X-MCP-Pod-Name`
- `GetStatus().instance` / `instance_id`
- Tools `stackyrd_identity`, `stackyrd_cluster`, `stackyrd_app`, `stackyrd_health`, etc.

## Methods

| Method | Notes |
|--------|-------|
| `initialize` | Returns `protocolVersion: 2026-07-28`, `capabilities{tools,resources,prompts}`, `serverInfo{name: stackyrd, version, instanceId}`, `_meta{io.stackyrd/instance}` |
| `server/discover` | Modern discovery — `supportedVersions`, `capabilities`, `_meta{io.modelcontextprotocol/serverInfo, io.stackyrd/instance}`, `instructions` |
| `tools/list` | Lists all 14 tools (cached via `toolDefsCache` + `sync.RWMutex`) |
| `tools/call` | Dispatches `stackyrd_*` tools (`callParams{name, arguments}`) → `{content:[{type:text, text: json}], isError}` |
| `resources/list`, `resources/templates/list` | Lists 11 `stackyrd://` resources |
| `resources/read` | Reads one resource by `uri` → `{contents:[{uri, mimeType: application/json, text}]}` |
| `prompts/list` | `{prompts: []}` (no prompts) |
| `prompts/get` | `-32602 Prompt not found` |
| `ping` | `{}` |
| `notifications/initialized`, `notifications/cancelled`, `notifications/progress` | No-op → `202` (notification path) |

Batch: each element routed independently; invalid `jsonrpc`/`method` → `-32600`; notifications skipped; empty batch → `-32600`; `202` if all notifications.

## Tools

| Tool | Description | Input |
|------|-------------|-------|
| `stackyrd_health` | Infra init status & overall progress | `{}` |
| `stackyrd_services` | Registered services with run state, wire name, endpoints | `{}` |
| `stackyrd_infra` | Infra components + `GetStatus()` | `{}` |
| `stackyrd_infra_detail` | Single component status by `name` (e.g. `redis`) | `{name: string}` |
| `stackyrd_endpoints` | Registered routes (deduped, sorted) | `{}` |
| `stackyrd_uptime` | `uptime`, `started_at`, `uptime_seconds` | `{}` |
| `stackyrd_resources` | CPU %, RAM %, cores, goroutines, `app_mem_mib`, hostname, cpu_model | `{}` |
| `stackyrd_middleware` | Middleware enabled/disabled states | `{}` |
| `stackyrd_dashboard` | Full snapshot mirroring TUI sidebar: `app, instance, resources, services, infra, infra_disabled, middleware, endpoints` | `{}` |
| `stackyrd_app` | App name/version/env/port/uptime/pid | `{}` |
| `stackyrd_identity` | Pod identity (`InstanceIdentity`) | `{}` |
| `stackyrd_cluster` | Cluster members (`{members:[identity], count:1, self}`) — phase 1 local only | `{}` |
| `stackyrd_memory` | Detailed memory with visualization thresholds/scale (`system, app, visualization{thresholds, scale, gauge, status}`) | `{}` |
| `stackyrd_goroutines` | Goroutine dump (`count, returned, truncated, filter, states, goroutines[{id, function, state, stack}]`) | `{filter?: string, limit?: int}` default 500 |

All tool results are JSON-stringified and wrapped as `toolResult(text, isError)`.

## Resources

| URI | Name | Description |
|-----|------|-------------|
| `stackyrd://dashboard` | Stackyrd Dashboard | Full TUI sidebar snapshot |
| `stackyrd://resources` | System Resources | CPU/RAM/goroutines/host |
| `stackyrd://services` | Services | Service states |
| `stackyrd://infra` | Infrastructure | Components status |
| `stackyrd://middleware` | Middleware | Enabled flags |
| `stackyrd://endpoints` | Endpoints | All routes |
| `stackyrd://app` | App Info | Name/version/env/port/uptime |
| `stackyrd://identity` | Instance Identity | Pod identity |
| `stackyrd://cluster` | Cluster | Members (local only) |
| `stackyrd://memory` | Memory Details | Thresholds/scale/gauge for frontend |
| `stackyrd://goroutines` | Goroutine Dump | Stacks for leak detection |

`resources/read` with unknown URI → `-32602 Resource not found`.

## Status

`GetStatus()` (`connected: true` when operational) returns:

```json
{
  "enabled": true, "endpoint": "/mcp", "connected": true,
  "protocol_version": "2026-07-28", "supported_versions": ["2026-07-28","2025-11-25","2025-03-26"],
  "tools": ["stackyrd_health", ...], "tools_count": 14,
  "allowed_origins": [], "auth_required": true, "token_set": true,
  "uptime": "12s", "uptime_seconds": 12, "started_at": "2026-08-30T...",
  "rate_limit_enabled": false, "rate_limit_ip": 100, "rate_limit_time": 60,
  "rate_limit_cooldowntime": 300, "rate_limit_excludeip": ["::1","127.0.0.1","localhost"],
  "rate_limit_tracked_ips": 3, "rate_limit_blocked_ips": 0,
  "instance_id": "a1b2-...-hostname-123", "instance": {"instance_id": "...", "pod_name": "...", "pod_ip": "", "namespace": "default", "node_name": "", "hostname": "...", "pid": 123, "started_at": "..."}
}
```

Boot wiring: `server.go` calls `infrastructure.SetInitManager()` and `infrastructure.SetServices()` once during `Start()` so `toolHealth`/`toolServices` reflect live state; `resolveEffective()` falls back to the singleton if the handler receiver is zero-valued.
