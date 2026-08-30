# Logging Guide

stackyrd uses `pkg/logger` – a zerolog wrapper.

## Creating a Logger

```go
import "stackyrd/pkg/logger"

cfg := logger.LoggerConfig{Debug: true}
log := logger.NewWithConfig(cfg)
```

## Logging

```go
log.Info("message", "key", "value")
log.Debug("debug message")
log.Error(err).Msg("operation failed")
```

## Output

Console is enabled by default. Switch format:

```go
cfg := logger.OutputConfig{
    ConsoleFormat: "json",  // "fancy" | "simple" | "json"
    Colors:        false,
}
log = logger.NewWithConfig(cfg)
```

## File Logging

File logging is handled by `pkg/infrastructure/logfile` (`logfile` component). Enabled via `config.yaml`:

```yaml
log:
  enabled: true
  path: "logs"
  filename: "stackyrd.log"
  max_files: 7
  compress: true
```

Files are written as `logs/stackyrd.log.YYYY-MM-DD` (JSON lines). The logger auto-attaches the file writer in `internal/server/server.go` right after infrastructure init and `Dependencies.Seal()` via `logger.AddWriter()`, so it works in both console and TUI (`quiet`) modes. Requires `log.enabled: true`; the file is created on first write and rotated daily.

Never use `Fatal` or `Panic` in production handlers.
