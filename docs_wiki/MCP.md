# MCP Server

Model Context Protocol (MCP) server built into stackyrd as an infrastructure component. Exposes internal state to LLM clients over streamable HTTP, with optional token-based authentication.

## Overview

The MCP server wraps stackyrd's own registries — no network round-trips to the HTTP API. All state is read from the in-memory `ComponentRegistry` and `InfraInitManager` directly. The server is zero-dependency: hand-rolled JSON-RPC 2.0 over HTTP, no external MCP SDK.

**Transport:** Stateless streamable HTTP (POST `/mcp`). Every request returns a single `application/json` response. Notifications return 202 Accepted with no body.

## Activation

```yaml
mcp:
  enabled: true
  endpoint: "/mcp"        # default
  token: "my-secret-token"  # empty = no auth
```

## Authentication

When `mcp.token` is set (non-empty), every request must include it via one of:
- `Authorization: Bearer <token>` header
- `X-MCP-Token: <token>` header

Requests without a valid token receive `401 Unauthorized`.

## Available Tools

| Tool | Description | Input |
|------|-------------|-------|
| `stackyrd_health` | Infrastructure initialization status and per-component progress | — |
| `stackyrd_services` | Registered services with state, wire name, endpoints | — |
| `stackyrd_infra` | All infrastructure components and their status | — |
| `stackyrd_infra_detail` | Full status map of one component by name | `name` (string, required) |
| `stackyrd_endpoints` | All registered service endpoints | — |

## JSON-RPC Protocol

Standard MCP JSON-RPC 2.0 over HTTP POST.

### Initialize

```bash
curl -X POST localhost:8080/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize"}'
```

Response includes `protocolVersion`, `capabilities`, and `serverInfo`.

### List Tools

```bash
curl -X POST localhost:8080/mcp -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer my-secret-token' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

### Call a Tool

```bash
curl -X POST localhost:8080/mcp -H 'Content-Type: application/json' \
  -H 'X-MCP-Token: my-secret-token' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"stackyrd_infra_detail","arguments":{"name":"redis"}}}'
```

### Notifications (no response body)

Notifications have no `id` field and return `202 Accepted`.

```json
{"jsonrpc":"2.0","method":"notifications/initialized"}
```

## Architecture

The MCP server follows the standard stackyrd patterns:

- **Single file:** `pkg/infrastructure/mcpserver.go`
- **InfrastructureComponent:** registered via `init()` → `RegisterComponent("mcp", factory)`
- **RouteRegistrar:** auto-mounts the MCP endpoint via the existing component route loop in `server.go`
- **Config toggle:** `mcp.enabled` in `config.yaml`

### Wiring

`server.go` calls two package-level functions during boot:

1. `infrastructure.SetInitManager(m)` — gives MCP access to health data
2. `infrastructure.SetServices(svcs)` — gives MCP access to service metadata

Both are injected once during startup. MCP-specific state is encapsulated in `mcpserver.go` (package-level `mcpState` struct with mutex).

## Regenerating Swagger Docs

After any API change, regenerate:

```bash
go run scripts/swagger/swagger.go
```
