# MCP Server

Model Context Protocol endpoint built into `pkg/infrastructure/mcpserver.go`.

Protocol `2026-07-28` (current) with dual-era backward compat for `2025-11-25` and `2025-03-26`.

## Enable

```yaml
mcp:
  enabled: true
  endpoint: /mcp
  token: ""  # empty = auto-generates 32-byte hex temp token (logged as WARN)
  allowed_origins: []  # [] = allow all (CORS disabled); restrict e.g. ["https://app.example.com","https://*.example.org"]
```

## Transport

Streamable HTTP — single POST endpoint. `GET`/`DELETE` to the endpoint return `405`. Notifications (no `id`) return `202`.

## Auth

Required headers: `Authorization: Bearer <token>` or `X-MCP-Token: <token>`. When `mcp.token` is empty, the server generates a random 32-byte hex temporary token at startup and logs `MCP temporary token generated — set mcp.token in config.yaml for persistence` with `token` and `endpoint` fields; the endpoint is always protected (no open mode).

## Protocol Negotiation

Every modern request carries `MCP-Protocol-Version` header and `_meta.io.modelcontextprotocol/protocolVersion` — they must match or the server returns `400` (`-32020 HeaderMismatch`). Unknown version returns `400` with `-32022 UnsupportedProtocolVersionError` listing `supported: ["2026-07-28","2025-11-25","2025-03-26"]`. Legacy `2025-03-26` clients that omit the header are accepted as `2025-03-26`. Modern `2026-07-28` requests must also send `Mcp-Method` (and `Mcp-Name` for `tools/call`) matching the body.

Rate limited at 20 req/s (burst 50): `429` with `-32003`.

## CORS / Origin

Global CORS (`middleware.cors` allow-all) is bypassed for the MCP endpoint. MCP handles its own CORS: `allowed_origins: []` (default) disables the check — any `Origin` is allowed and echoed as `Access-Control-Allow-Origin` (no browser block). Set a whitelist to restrict, e.g. `["https://app.example.com","https://*.example.org","*"]`; non-listed origins get `403 Forbidden` (`-32000`). `OPTIONS` preflight returns `204` with CORS headers.

## Methods

| Method | Notes |
|--------|-------|
| `initialize` | Legacy handshake — returns `protocolVersion`, `capabilities.tools`, `serverInfo` |
| `server/discover` | Modern discovery — returns `supportedVersions`, `capabilities`, `_meta.serverInfo` |
| `tools/list` | Lists all 5 tools |
| `tools/call` | Dispatches `stackyrd_*` tools |
| `notifications/initialized`, `notifications/cancelled` | No-op → `202` |

Unknown method: `404` with `-32601` on modern, `200` with `-32601` on legacy.

## Tools

| Tool | Returns |
|------|---------|
| `stackyrd_health` | Infra init status |
| `stackyrd_services` | Services + state |
| `stackyrd_infra` | Infra components |
| `stackyrd_infra_detail` | Single component `GetStatus()` by `name` |
| `stackyrd_endpoints` | Registered routes (deduped, sorted) |
