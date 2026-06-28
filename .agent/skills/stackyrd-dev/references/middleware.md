# Adding Middleware

Create `internal/middleware/{name}.go` (package `middleware`). Register a `MiddlewareFactory` via `init()`.

## Factory Signature

```go
type MiddlewareFactory func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error)
```

`echo.MiddlewareFunc` is `func(next echo.HandlerFunc) echo.HandlerFunc`.

## Skeleton

```go
package middleware

import (
    "time"
    "github.com/labstack/echo/v4"
    "stackyrd/config"
    "stackyrd/pkg/logger"
)

func init() {
    RegisterMiddleware("{name}", func(cfg *config.Config, log *logger.Logger) (echo.MiddlewareFunc, error) {
        return MyMiddleware(log), nil
    })
}

func MyMiddleware(log *logger.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()
            err := next(c)
            log.Info("MyMiddleware", "latency", time.Since(start).String())
            return err
        }
    }
}
```

## Config

```yaml
middleware:
  {name}: true
```

Defaults to enabled if not listed (see `MiddlewareConfig.IsEnabled`).

## With Config Values

```go
func init() {
    RegisterMiddleware("cors", func(cfg *config.Config, log *logger.Logger) (echo.MiddlewareFunc, error) {
        return CORS(cfg), nil
    })
}
```

## Patterns

| Middleware | Key Pattern |
|------------|-------------|
| `audit.go` | Skip path filtering, config struct with defaults, field extraction |
| `cors.go` | Config-driven origins, preflight |
| `jwt.go` | Header validation, token parse, context set |
| `ratelimit.go` | Per-IP tracking, configurable limits |
| `security.go` | Response headers (HSTS, CSP) |
| `encryption.go` | Payload encrypt/decrypt |

- **Order matters** — registry appends in registration order
- `next(c)` passes control; returning an error from middleware short-circuits the chain
- Set request-scoped values with `c.Set("key", value)` — downstream handlers/middleware read with `c.Get("key")`
- For path-skipping middleware, use a skip-list pattern (see `audit.go`)
- Factory returning `nil, nil` is logged and skipped
