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

Never use `Fatal` or `Panic` in production handlers.
