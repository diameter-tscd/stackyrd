package logger

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var themeColorFunc func(string) string
var themeColorMu sync.RWMutex

func SetThemeColorFunc(fn func(string) string) {
	themeColorMu.Lock()
	themeColorFunc = fn
	themeColorMu.Unlock()
}

func getThemeColor(key string) string {
	themeColorMu.RLock()
	fn := themeColorFunc
	themeColorMu.RUnlock()
	if fn != nil {
		if c := fn(key); c != "" {
			return c
		}
	}
	return ""
}

func hexToANSI(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 0)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 0)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 0)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// OutputConfig defines the output formatting configuration
type OutputConfig struct {
	ConsoleEnabled  bool
	ConsoleFormat   string // "fancy", "simple", "json"
	Colors          bool
	TimestampFormat string
	NoColor         bool
}

// DefaultOutputConfig returns a default output configuration
func DefaultOutputConfig() OutputConfig {
	return OutputConfig{
		ConsoleEnabled:  true,
		ConsoleFormat:   "fancy",
		Colors:          true,
		TimestampFormat: "15:04:05",
		NoColor:         false,
	}
}

// LoggerConfig contains configuration for the logger
type LoggerConfig struct {
	Debug       bool
	Quiet       bool // suppress console output (logs still go to broadcaster)
	Broadcaster io.Writer
	Output      OutputConfig
}

// DefaultLoggerConfig returns a default logger configuration
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Debug:       false,
		Quiet:       false,
		Broadcaster: nil,
		Output:      DefaultOutputConfig(),
	}
}

// Logger wraps the zerolog logger with modular configuration
type Logger struct {
	mu     sync.RWMutex
	z      zerolog.Logger
	quiet  bool
	config LoggerConfig
	extra  []io.Writer
}

// setTimeFormatOnce guards the process-wide zerolog.TimeFieldFormat write.
var setTimeFormatOnce sync.Once

// New creates a new fancy logger
func New(debug bool, broadcaster io.Writer) *Logger {
	cfg := DefaultLoggerConfig()
	cfg.Debug = debug
	cfg.Broadcaster = broadcaster
	cfg.Quiet = false
	return NewWithConfig(cfg)
}

// NewQuiet creates a new logger with console output suppressed
func NewQuiet(debug bool, broadcaster io.Writer) *Logger {
	cfg := DefaultLoggerConfig()
	cfg.Debug = debug
	cfg.Broadcaster = broadcaster
	cfg.Quiet = true
	return NewWithConfig(cfg)
}

func buildWriters(cfg LoggerConfig, extra []io.Writer) zerolog.LevelWriter {
	var consoleOutput zerolog.ConsoleWriter
	if cfg.Output.ConsoleEnabled {
		consoleOutput = zerolog.ConsoleWriter{
			Out:           os.Stdout,
			TimeFormat:    cfg.Output.TimestampFormat,
			FormatLevel:   getLevelFormatter(cfg.Output),
			FormatMessage: getMessageFormatter(cfg.Output),
			NoColor:       !cfg.Output.Colors || cfg.Output.NoColor,
		}
	} else {
		consoleOutput = zerolog.ConsoleWriter{Out: io.Discard}
	}
	var writers []io.Writer
	if cfg.Quiet {
		if cfg.Broadcaster != nil {
			broadcasterOutput := zerolog.ConsoleWriter{
				Out:        cfg.Broadcaster,
				TimeFormat: cfg.Output.TimestampFormat,
				NoColor:    true,
			}
			writers = append(writers, broadcasterOutput)
		} else if len(extra) == 0 {
			writers = append(writers, zerolog.ConsoleWriter{Out: io.Discard})
		}
	} else {
		writers = append(writers, consoleOutput)
		if cfg.Broadcaster != nil {
			broadcasterOutput := zerolog.ConsoleWriter{
				Out:        cfg.Broadcaster,
				TimeFormat: cfg.Output.TimestampFormat,
				NoColor:    true,
			}
			writers = append(writers, broadcasterOutput)
		}
	}
	writers = append(writers, extra...)
	if len(writers) == 0 {
		writers = append(writers, zerolog.ConsoleWriter{Out: io.Discard})
	}
	return zerolog.MultiLevelWriter(writers...)
}

func newZerolog(cfg LoggerConfig, extra []io.Writer) zerolog.Logger {
	setTimeFormatOnce.Do(func() {
		zerolog.TimeFieldFormat = time.RFC3339
	})
	multi := buildWriters(cfg, extra)
	lvl := zerolog.InfoLevel
	if cfg.Debug {
		lvl = zerolog.DebugLevel
	}
	return zerolog.New(multi).Level(lvl).With().Timestamp().Logger()
}

// NewWithConfig creates a new logger with full configuration
func NewWithConfig(cfg LoggerConfig) *Logger {
	return &Logger{z: newZerolog(cfg, nil), quiet: cfg.Quiet, config: cfg}
}

func (l *Logger) AddWriter(w io.Writer) {
	if w == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.extra = append(l.extra, w)
	l.z = newZerolog(l.config, l.extra)
}

func getLevelFormatter(output OutputConfig) func(any) string {
	if !output.Colors || output.NoColor {
		return func(i any) string {
			if ll, ok := i.(string); ok {
				return strings.ToUpper(ll)
			}
			return strings.ToUpper(fmt.Sprintf("%s", i))
		}
	}

	return func(i any) string {
		var ll string
		if s, ok := i.(string); ok {
			ll = s
		} else {
			return strings.ToUpper(fmt.Sprintf("%s", i))
		}
		var hex, label string
		switch ll {
		case "debug":
			hex = getThemeColor("secondary")
			if hex == "" {
				hex = "#8BE9FD"
			}
			label = "[ DEBUG ]"
		case "info":
			hex = getThemeColor("success")
			if hex == "" {
				hex = "#50FA7B"
			}
			label = "[ INFO  ]"
		case "warn":
			hex = getThemeColor("warning")
			if hex == "" {
				hex = "#F1FA8C"
			}
			label = "[ WARN  ]"
		case "error":
			hex = getThemeColor("error")
			if hex == "" {
				hex = "#FF5555"
			}
			label = "[ ERROR ]"
		case "fatal", "panic":
			hex = getThemeColor("error")
			if hex == "" {
				hex = "#FF5555"
			}
			label = "[ " + strings.ToUpper(ll) + " ]"
		default:
			return strings.ToUpper(ll)
		}
		if ansi := hexToANSI(hex); ansi != "" {
			return ansi + label + "\x1b[0m"
		}
		return label
	}
}

// getMessageFormatter returns the appropriate message formatter based on output configuration
func getMessageFormatter(output OutputConfig) func(any) string {
	if !output.Colors || output.NoColor {
		return func(i any) string {
			return fmt.Sprintf("%s", i)
		}
	}

	return func(i any) string {
		return fmt.Sprintf("\x1b[1m%s\x1b[0m", i)
	}
}

// New creates a new logger with the same configuration as the current logger but with different debug and broadcaster settings
func (l *Logger) New(debug bool, broadcaster io.Writer) *Logger {
	cfg := l.config
	cfg.Debug = debug
	cfg.Broadcaster = broadcaster
	cfg.Quiet = false
	return NewWithConfig(cfg)
}

// WithOutput returns a new logger with modified output configuration
func (l *Logger) WithOutput(output OutputConfig) *Logger {
	cfg := l.config
	cfg.Output = output
	return NewWithConfig(cfg)
}

// WithQuiet returns a new logger with quiet mode enabled/disabled
func (l *Logger) WithQuiet(quiet bool) *Logger {
	cfg := l.config
	cfg.Quiet = quiet
	return NewWithConfig(cfg)
}

// Config returns the current logger configuration
func (l *Logger) Config() LoggerConfig {
	return l.config
}

// IsQuiet returns whether the logger is in quiet mode
func (l *Logger) IsQuiet() bool {
	return l.quiet
}

// Info logs an info message
func (l *Logger) Info(msg string, keyvals ...any) {
	l.mu.RLock()
	z := l.z
	l.mu.RUnlock()
	l.log(z.Info(), msg, keyvals...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, keyvals ...any) {
	l.mu.RLock()
	z := l.z
	l.mu.RUnlock()
	if err != nil {
		l.log(z.Error().Err(err), msg, keyvals...)
	} else {
		l.log(z.Error(), msg, keyvals...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, keyvals ...any) {
	l.mu.RLock()
	z := l.z
	l.mu.RUnlock()
	l.log(z.Debug(), msg, keyvals...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, keyvals ...any) {
	l.mu.RLock()
	z := l.z
	l.mu.RUnlock()
	l.log(z.Warn(), msg, keyvals...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error) {
	l.mu.RLock()
	z := l.z
	l.mu.RUnlock()
	if err != nil {
		z.Fatal().Err(err).Msg(msg)
	} else {
		z.Fatal().Msg(msg)
	}
}

func (l *Logger) log(e *zerolog.Event, msg string, keyvals ...any) {
	if len(keyvals)%2 != 0 {
		e.Msg(msg + " (odd number of keyvals caused metadata drop)")
		return
	}
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}
		// Marshal errors as their message, not as {} (empty struct JSON).
		if errVal, ok := keyvals[i+1].(error); ok {
			e.AnErr(key, errVal)
		} else {
			e.Interface(key, keyvals[i+1])
		}
	}
	e.Msg(msg)
}
