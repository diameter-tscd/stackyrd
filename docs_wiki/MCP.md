# MCP Server

Model Context Protocol endpoint built into `pkg/infrastructure/mcpserver.go`.

## Enable

```yaml
mcp:
  enabled: true
  endpoint: /mcp
  token: ""  # empty = no auth
```

## Auth

Required headers when `token` set: `Authorization: Bearer <token>` or `X-MCP-Token: <token>`.

## Tools

| Tool | Returns |
|------|---------|
| `stackyrd_health` | Infra init status |
| `stackyrd_services` | Services + state |
| `stackyrd_infra` | Infra components |
| `stackyrd_endpoints` | Registered routes |
