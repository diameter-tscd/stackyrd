# Security Guide

## Middleware Flags

Enable in `config.yaml`:

| Flag | Purpose |
|------|---------|
| `security` | HTTP security headers |
| `cors` | Cross-origin requests |
| `jwt` | JWT auth (`golang-jwt/jwt/v5`, `HS256`) |
| `ratelimit` | Request rate limiting (Redis) |
| `permission_check` | Block DELETE by default |

Enable auth:

```yaml
auth:
  type: jwt
  secret: "${AUTH_SECRET}"
```

## CORS

Default allows all origins without credentials. Production example:

```go
cfg := middleware.CORSConfig{
    AllowOrigins: []string{"https://app.example.com"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
}
```

Wildcard `*` never accompanies `Access-Control-Allow-Credentials: true`.

## Input Validation

Use `request.Bind(c, &req)` to bind and validate. Map fields to structs instead of raw map access.

## Secrets

Never hardcode. Use env vars:

```bash
export AUTH_SECRET="..."
```

## Encryption

`encryption.enabled: true` with AES-256-GCM. `encryption.key` is stretched via SHA-256; empty = random in-memory key (ephemeral).

## Rate Limiting

Fails open on Redis outage; tune for your deployment.

## Production Checklist

- Use HTTPS, non-root Docker user
- Restrict `/health` and `/health/dependencies` to internal networks
- Enable `security`, `ratelimit`, `permission_check`
