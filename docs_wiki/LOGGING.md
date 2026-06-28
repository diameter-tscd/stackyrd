# Logging Guide

Structured JSON logging powered by zerolog.

## Overview

stackyrd uses `pkg/logger/` — a zerolog wrapper — as its single logging subsystem across services, middleware, and infrastructure.

## Application Logger

```go
import "stackyrd/pkg/logger"

log := logger.NewLogger(cfg)
log.Info().Msg("service started")
log.Error().Err(err).Msg("operation failed")
log.Debug().Str("key", "val").Int("count", 42).Msg("debug info")
```

### Level-Specific Methods

```go
log.Debug().Msg("debug message")
log.Info().Msg("info message")
log.Warn().Msg("warning message")
log.Error().Err(err).Msg("error message")
log.Fatal().Msg("fatal message") // calls os.Exit(1)
log.Panic().Msg("panic message") // calls panic()
```

### Context Fields

```go
log.Info().
    Str("service", "users").
    Int("port", 8080).
    Str("env", "production").
    Msg("starting up")
```

### Output Configuration

```go
cfg := logger.OutputConfig{
    ConsoleEnabled:  true,
    ConsoleFormat:   "fancy",  // "fancy", "simple", "json"
    Colors:          true,
    TimestampFormat: "15:04:05",
}
```

## Log Entry Format

```json
{
  "level": "info",
  "service": "users",
  "time": "2026-06-03T15:10:00Z",
  "message": "starting up"
}
```

## Middleware Integration

```go
import "stackyrd/pkg/logger"

func LogMiddleware(log *logger.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()
            log.Info().Str("method", c.Request().Method).
                Str("path", c.Request().URL.Path).Msg("request started")

            err := next(c)

            log.Info().Str("method", c.Request().Method).
                Str("path", c.Request().URL.Path).
                Int("status", c.Response().Status).
                Int64("duration_ms", time.Since(start).Milliseconds()).
                Msg("request completed")
            return err
        }
    }
}
```

## Best Practices

- Use `pkg/logger` for all logging throughout the application
- Use structured fields instead of string formatting: `.Str("key", "val")` not `.Msgf("key=%s", val)`
- Set appropriate log levels: `Debug` for development, `Info` for production
- Avoid `Fatal` and `Panic` in middleware or request handlers
