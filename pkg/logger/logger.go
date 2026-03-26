package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

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
	z      zerolog.Logger
	quiet  bool
	config LoggerConfig
}

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

// NewWithConfig creates a new logger with full configuration
func NewWithConfig(cfg LoggerConfig) *Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	// Create console output based on configuration
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
		// Console disabled, use discard writer
		consoleOutput = zerolog.ConsoleWriter{Out: io.Discard}
	}

	var multi zerolog.LevelWriter

	if cfg.Quiet {
		// Quiet mode: only write to broadcaster (if available), not to console
		if cfg.Broadcaster != nil {
			// Create a simple console writer for the broadcaster (without stdout)
			broadcasterOutput := zerolog.ConsoleWriter{
				Out:        cfg.Broadcaster,
				TimeFormat: cfg.Output.TimestampFormat,
				NoColor:    true,
			}
			multi = zerolog.MultiLevelWriter(broadcasterOutput)
		} else {
			// No broadcaster and quiet mode = discard all logs
			multi = zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: io.Discard})
		}
	} else {
		// Normal mode: write to console and broadcaster
		if cfg.Broadcaster != nil {
			multi = zerolog.MultiLevelWriter(consoleOutput, cfg.Broadcaster)
		} else {
			multi = zerolog.MultiLevelWriter(consoleOutput)
		}
	}

	logLevel := zerolog.InfoLevel
	if cfg.Debug {
		logLevel = zerolog.DebugLevel
	}

	z := zerolog.New(multi).Level(logLevel).With().Timestamp().Logger()

	return &Logger{z: z, quiet: cfg.Quiet, config: cfg}
}

// getLevelFormatter returns the appropriate level formatter based on output configuration
func getLevelFormatter(output OutputConfig) func(interface{}) string {
	if !output.Colors || output.NoColor {
		return func(i interface{}) string {
			if ll, ok := i.(string); ok {
				return strings.ToUpper(ll)
			}
			return strings.ToUpper(fmt.Sprintf("%s", i))
		}
	}

	// Pastel color formatter
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case "debug":
				l = "\x1b[38;2;139;233;253m[ DEBUG ]\x1b[0m" // Pastel Cyan
			case "info":
				l = "\x1b[38;2;189;147;249m[ INFO  ]\x1b[0m" // Pastel Purple
			case "warn":
				l = "\x1b[38;2;241;250;140m[ WARN  ]\x1b[0m" // Pastel Yellow
			case "error":
				l = "\x1b[38;2;255;121;198m[ ERROR ]\x1b[0m" // Pastel Pink
			case "fatal":
				l = "\x1b[38;2;255;85;85m[ FATAL ]\x1b[0m" // Pastel Red
			case "panic":
				l = "\x1b[38;2;255;85;85m[ PANIC ]\x1b[0m" // Pastel Red
			default:
				l = strings.ToUpper(ll)
			}
		} else {
			if i == nil {
				l = strings.ToUpper(fmt.Sprintf("%s", i))
			} else {
				l = strings.ToUpper(fmt.Sprintf("%s", i))
			}
		}
		return l
	}
}

// getMessageFormatter returns the appropriate message formatter based on output configuration
func getMessageFormatter(output OutputConfig) func(interface{}) string {
	if !output.Colors || output.NoColor {
		return func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		}
	}

	return func(i interface{}) string {
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

// GetConfig returns the current logger configuration
func (l *Logger) GetConfig() LoggerConfig {
	return l.config
}

// IsQuiet returns whether the logger is in quiet mode
func (l *Logger) IsQuiet() bool {
	return l.quiet
}

// Info logs an info message
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	l.log(l.z.Info(), msg, keyvals...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, keyvals ...interface{}) {
	if err != nil {
		l.z.Error().Err(err).Fields(keyvals).Msg(msg)
	} else {
		l.log(l.z.Error(), msg, keyvals...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	l.log(l.z.Debug(), msg, keyvals...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	l.log(l.z.Warn(), msg, keyvals...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error) {
	if err != nil {
		l.z.Fatal().Err(err).Msg(msg)
	} else {
		l.z.Fatal().Msg(msg)
	}
}

func (l *Logger) log(e *zerolog.Event, msg string, keyvals ...interface{}) {
	if len(keyvals)%2 != 0 {
		e.Msg(msg + " (odd number of keyvals caused metadata drop)")
		return
	}
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}
		e.Interface(key, keyvals[i+1])
	}
	e.Msg(msg)
}
