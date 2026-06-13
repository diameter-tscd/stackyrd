# Adding Middleware

Middleware provides cross-cutting HTTP concerns like authentication, logging, rate limiting, CORS, and security headers. Middleware is **auto-registered** via `init()` in files under `internal/middleware/`.

## Architecture

Each middleware is wrapped in a **MiddlewareFactory** — a function that receives the full `*config.Config` and `*logger.Logger`, and returns a `gin.HandlerFunc`:

```go
type MiddlewareFactory func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error)
```

The factory pattern exists because middleware may need access to config values (rate limits, JWT secrets, CORS origins) and the logger.

## File Template

Create `internal/middleware/{name}.go`:

```go
package middleware

import (
    "net/http"
    "time"

    "stackyrd/config"
    "stackyrd/pkg/logger"

    "github.com/gin-gonic/gin"
)

func init() {
    RegisterMiddleware("{name}", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
        return {Name}Middleware(logger), nil
    })
}

func {Name}Middleware(logger *logger.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()

        c.Next()

        latency := time.Since(start)
        logger.Info("{Name} middleware", "latency", latency.String())
    }
}
```

## Config Toggle

Add to `middleware:` in `config.yaml`:

```yaml
middleware:
  {name}: true   # set to false to disable
```

Middleware defaults to **enabled** if not explicitly listed (see `MiddlewareConfig.IsEnabled`).

## When the Middleware Needs Config Values

Some middleware (CORS, JWT, rate limiting) needs config values. Pass them through the factory closure:

```go
func init() {
    RegisterMiddleware("cors", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
        return CORS(cfg), nil
    })
}

func CORS(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Use cfg values
    }
}
```

## Real Examples from the Codebase

| File | Purpose | Key Pattern |
|------|---------|-------------|
| `audit.go` | Request/response audit logging | Skip path filtering, config struct with defaults, field extraction |
| `cors.go` | CORS headers | Config-driven origins, preflight handling |
| `jwt.go` | JWT authentication | Middleware that reads auth header, validates token, sets user context |
| `ratelimit.go` | Rate limiting | Per-IP tracking, configurable limits |
| `security.go` | Security headers | Sets response headers (HSTS, CSP, X-Frame-Options) |
| `encryption.go` | Request/response encryption | Encrypts/decrypts payloads |

## Key Points

- Middleware **order matters** — the registry appends middleware in registration order. The global middleware chain is built in `server.go` after config is applied.
- Use `c.Next()` to pass control to the next handler, or `c.Abort()` to stop the chain.
- Set request-scoped values with `c.Set("key", value)` — they're accessible to downstream handlers and later middleware.
- Audit-logging style middleware should log after `c.Next()` to capture the response status.
- For middleware that needs to skip certain paths (health checks, etc.), use a skip list pattern as seen in `audit.go`.
- If the middleware returns an error from the factory, it's logged and the middleware is skipped (not added to the chain).
