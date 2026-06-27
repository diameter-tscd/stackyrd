# Logging Guide

Structured JSON logging with sampling, rotation, and context enrichment.

## Overview

stackyrd provides two logging subsystems:

| Package | Purpose |
|---------|---------|
| `pkg/logger/` | Zerolog-based structured logger used by services and infrastructure |
| `pkg/logger/` | Enhanced logging with rotation, sampling, structured helpers |

## Application Logger (pkg/logger)

The primary logger used throughout services and middleware:

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

## Structured Logger (pkg/logging)

For applications needing log rotation, sampling, and richer structured output:

### Basic Setup

```go
import "stackyrd/pkg/logging"

// With rotation
writer, err := logging.NewRotatingWriter(
    "/var/log/stackyrd/app.log",
    logging.DefaultRotationConfig(),
)
logger := logging.NewStructuredLogger(
    writer,
    logging.INFO,
    "my-service",
    "1.0.0",
    "production",
)

// Default writer (stdout)
logger := logging.NewStructuredLogger(
    nil, // defaults to os.Stdout
    logging.DEBUG,
    "my-service", "1.0", "development",
)
```

### Structured Logging API

```go
// With context fields
logger = logger.WithFields(map[string]interface{}{
    "env": "production",
    "region": "us-east-1",
})

// With request context (extracts request_id, user_id, trace_id, span_id)
logger = logger.WithContext(ctx)

// Log at different levels
logger.Info("starting up")
logger.Debug("connecting to database")
logger.Warn("high memory usage", "threshold", 90)
logger.Error("request failed", "error", err)
```

### Log Entry Format

```json
{
  "timestamp": "2026-06-03T15:10:00Z",
  "level": "INFO",
  "message": "starting up",
  "caller": "cmd/app/main.go:42",
  "service_name": "my-service",
  "version": "1.0.0",
  "environment": "production",
  "request_id": "req-abc123",
  "fields": {
    "env": "production"
  }
}
```

## Log Rotation

Rotate log files by size with configurable retention:

```go
writer, err := logging.NewRotatingWriter("app.log", logging.RotationConfig{
    MaxSize:    100 * 1024 * 1024, // 100MB per file
    MaxAge:     7 * 24 * time.Hour, // keep 7 days
    MaxBackups: 10,                  // max rotated files
    Compress:   true,                // gzip old files
})
```

Default config: 100MB, 7 days, 10 backups, compressed.

## Log Sampling

### Rate-Based Sampling

Log a percentage of entries:

```go
sampler := logging.NewLogSampler(logging.SampleByRate, 0.1, 0, 0)
// Logs ~10% of entries

samplingLogger := logging.NewSamplingLogger(logger, sampler)
samplingLogger.Info("high-volume event")
```

### Count-Based Sampling

Log every Nth entry:

```go
sampler := logging.NewLogSampler(logging.SampleByCount, 0, 100, 0)
// Logs every 100th entry
```

### Time-Window Sampling

Log at most N entries per time window:

```go
sampler := logging.NewLogSampler(logging.SampleByTime, 0, 100, time.Minute)
// Logs at most 100 entries per minute
```

### Adaptive Sampling

Automatically adjusts rate based on volume:

```go
adaptive := logging.NewAdaptiveSampler(
    0.5,           // base rate (50%)
    0.1,           // minimum rate
    1.0,           // maximum rate
    time.Minute,   // adjustment window
)
```

## Middleware Integration

```go
import (
    "stackyrd/pkg/logger"
    "stackyrd/pkg/logging"
)

// In your middleware:
func LogMiddleware(logger *logging.StructuredLogger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        logger.Info("request started",
            "method", c.Request.Method,
            "path", path,
        )

        c.Next()

        logger.Info("request completed",
            "method", c.Request.Method,
            "path", path,
            "status", c.Writer.Status(),
            "duration_ms", time.Since(start).Milliseconds(),
        )
    }
}
```

## Best Practices

- Use `pkg/logger` (zerolog) for standard service logging
- Use `pkg/logging` when you need file rotation or sampling
- Always include `request_id` in logs via `WithContext(ctx)`
- Set appropriate log levels: `DEBUG` for development, `INFO` for production
- Use structured fields instead of string formatting: `.Str("key", "val")` not `.Msgf("key=%s", val)`
- Enable compression for rotated logs in production
- Use adaptive sampling for high-volume endpoints
