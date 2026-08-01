# Security Guide

Security configuration, middleware, and best practices for stackyrd.

## Security Middleware

Enable/disable in `config.yaml` under `middleware:`.

### Security Headers

Adds HTTP security headers to all responses:

```yaml
middleware:
  security: true
```

Headers set:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `Content-Security-Policy: default-src 'self'`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: geolocation=(self), microphone=()`

### CORS

```yaml
middleware:
  cors: true
```

The default config allows all origins for development but **never combines a wildcard origin with credentials** — a credentialed request is only answered when the origin matches an explicit allow-list. Configure a specific allow-list for production:

```go
middleware.CORSWithConfig([]string{"https://app.example.com"})
```

### JWT Authentication

```yaml
middleware:
  jwt: true

auth:
  type: jwt
  secret: "${AUTH_SECRET}"   # required — must be non-empty
```

The JWT middleware **fails closed**: it refuses to start unless `auth.type` is `jwt` and `auth.secret` is non-empty (no hardcoded fallback secret). Tokens are validated with `golang-jwt/jwt/v5`:

- Signing method pinned to `HS256` (algorithm-confusion blocked)
- Tokens must carry the `iss=stackyrd` and `aud=stackyrd-api` claims
- Expiration enforced via registered claims

Issue tokens with `middleware.GenerateToken(userID, username, email, role, secret, ttl)`.

### API Key Authentication

```yaml
auth:
  type: apikey
```

> **Note:** `apikey` mode is currently declarative only — no API-key middleware is implemented. Use `jwt` for enforced authentication.

### Rate Limiting

```yaml
middleware:
  ratelimit: true
```

Prevents abuse by limiting requests per client. On a Redis outage the limiter **fails open** and logs a loud warning; review whether fail-open is acceptable for your deployment.

### Permission Check

```yaml
middleware:
  permission_check: true
```

Blocks `DELETE` requests by default as a safety measure. Can be extended for role-based access control.

## Authentication Modes

Configured via `auth.type`:

| Mode | Description | Use Case |
|------|-------------|----------|
| `none` | No authentication | Development, internal-only |
| `apikey` | Static API key | Service-to-service auth |
| `jwt` | JSON Web Tokens | User-facing APIs |

## Secrets Management

### Never Hardcode Secrets

```yaml
# BAD: secrets in config.yaml
auth:
  secret: "my-super-secret-key"

# GOOD: environment variable overrides
auth:
  secret: "${AUTH_SECRET}"
```

Use environment variables to override config values:

```bash
export AUTH_SECRET="<generate-a-long-random-secret>"
export POSTGRES_PASSWORD="<db-password>"
```

### Encryption Service

The `encryption_service` provides AES-256-GCM payload encryption. The configured `encryption.key` is stretched with SHA-256 to a full 32-byte key — it is **never** zero-padded or truncated. If no key is configured, a random in-memory key is generated (ciphertext becomes undecryptable after restart — configure a key in production).

```yaml
encryption:
  enabled: true
  algorithm: aes-256-gcm
  key: "${ENCRYPTION_KEY}"  # stretched via SHA-256; empty => random ephemeral key
```

`GET /encryption/status` and `POST /encryption/key-rotate` no longer expose any key material. Rotation requires a new key of at least 32 characters and replaces the in-memory key (existing ciphertext is not re-encrypted — rotate deliberately).

## Server Hardening

The Echo server applies these protections by default:

| Protection | Value |
|-----------|-------|
| Request body limit | 2 MB (`BodyLimit`) |
| Header read timeout | 5s |
| Read timeout | 15s |
| Write timeout | 30s |
| Idle timeout | 60s |

The server listens on plain HTTP by default — terminate TLS at a reverse proxy and never expose it directly to the internet.

## CORS Configuration

The default CORS allows all origins without credentials. For production, use an explicit allow-list and enable credentials only when the origin is trusted:

```go
cfg := middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
}
```

Credentials are only emitted when the request origin matches the allow-list exactly; wildcard (`*`) never accompanies `Access-Control-Allow-Credentials: true`.

## Input Validation

Use the `pkg/request` package for request binding and validation:

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func (s *UserService) create(c echo.Context) error {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        var validationErr *request.ValidationError
        if errors.As(err, &validationErr) {
            return response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
        }
        // Never echo raw bind errors to clients — they leak internal field/type details.
        return response.BadRequest(c, "Invalid request data")
    }
    // safe to use req
    return nil
}
```

## Production Checklist

- [ ] Set `auth.type` to `jwt` with a strong `auth.secret` from an environment variable (`none` and `apikey` do not enforce anything)
- [ ] Enable `security` middleware for HTTP headers
- [ ] Enable `ratelimit` middleware for abuse protection
- [ ] Set the `encryption.key` environment variable (otherwise a random ephemeral key is used)
- [ ] Use secrets from environment variables, not config.yaml
- [ ] Set CORS to specific origins (not `*`) whenever credentials are needed
- [ ] Enable `permission_check` middleware
- [ ] Enable `audit` middleware for request logging

- [ ] Use HTTPS in production (reverse proxy or TLS)
- [ ] Run as non-root user in Docker
- [ ] Restrict `/health` and `/health/dependencies` endpoints to internal networks — they expose infrastructure topology
