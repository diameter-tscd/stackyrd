package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

// LogLevel represents the log level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Caller      string                 `json:"caller,omitempty"`
	Stack       string                 `json:"stack,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	ServiceName string                 `json:"service_name"`
	Version     string                 `json:"version"`
	Environment string                 `json:"environment"`
}

// StructuredLogger provides structured JSON logging
type StructuredLogger struct {
	writer      io.Writer
	level       LogLevel
	serviceName string
	version     string
	environment string
	fields      map[string]interface{}
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(writer io.Writer, level LogLevel, serviceName, version, environment string) *StructuredLogger {
	if writer == nil {
		writer = os.Stdout
	}

	return &StructuredLogger{
		writer:      writer,
		level:       level,
		serviceName: serviceName,
		version:     version,
		environment: environment,
		fields:      make(map[string]interface{}),
	}
}

// WithFields creates a logger with additional fields
func (sl *StructuredLogger) WithFields(fields map[string]interface{}) *StructuredLogger {
	newLogger := &StructuredLogger{
		writer:      sl.writer,
		level:       sl.level,
		serviceName: sl.serviceName,
		version:     sl.version,
		environment: sl.environment,
		fields:      make(map[string]interface{}),
	}

	for k, v := range sl.fields {
		newLogger.fields[k] = v
	}

	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithContext creates a logger with context information
func (sl *StructuredLogger) WithContext(ctx context.Context) *StructuredLogger {
	fields := make(map[string]interface{})

	if requestID := ctx.Value("request_id"); requestID != nil {
		fields["request_id"] = requestID
	}

	if userID := ctx.Value("user_id"); userID != nil {
		fields["user_id"] = userID
	}

	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields["trace_id"] = traceID
	}

	if spanID := ctx.Value("span_id"); spanID != nil {
		fields["span_id"] = spanID
	}

	return sl.WithFields(fields)
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(msg string, fields ...map[string]interface{}) {
	if sl.level <= DEBUG {
		sl.log(DEBUG, msg, fields...)
	}
}

// Info logs an info message
func (sl *StructuredLogger) Info(msg string, fields ...map[string]interface{}) {
	if sl.level <= INFO {
		sl.log(INFO, msg, fields...)
	}
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(msg string, fields ...map[string]interface{}) {
	if sl.level <= WARN {
		sl.log(WARN, msg, fields...)
	}
}

// Error logs an error message
func (sl *StructuredLogger) Error(msg string, fields ...map[string]interface{}) {
	if sl.level <= ERROR {
		sl.log(ERROR, msg, fields...)
	}
}

// Fatal logs a fatal message
func (sl *StructuredLogger) Fatal(msg string, fields ...map[string]interface{}) {
	if sl.level <= FATAL {
		sl.log(FATAL, msg, fields...)
		os.Exit(1)
	}
}

// log writes a log entry
func (sl *StructuredLogger) log(level LogLevel, msg string, fields ...map[string]interface{}) {
	entry := LogEntry{
		Timestamp:   time.Now().UTC(),
		Level:       level.String(),
		Message:     msg,
		ServiceName: sl.serviceName,
		Version:     sl.version,
		Environment: sl.environment,
		Fields:      make(map[string]interface{}),
	}

	// Add caller information
	_, file, line, ok := runtime.Caller(3)
	if ok {
		entry.Caller = fmt.Sprintf("%s:%d", file, line)
	}

	// Add fields
	for k, v := range sl.fields {
		entry.Fields[k] = v
	}

	for _, f := range fields {
		for k, v := range f {
			entry.Fields[k] = v
		}
	}

	// Add context fields
	if requestID, ok := entry.Fields["request_id"]; ok {
		entry.RequestID = fmt.Sprint(requestID)
		delete(entry.Fields, "request_id")
	}

	if userID, ok := entry.Fields["user_id"]; ok {
		entry.UserID = fmt.Sprint(userID)
		delete(entry.Fields, "user_id")
	}

	if traceID, ok := entry.Fields["trace_id"]; ok {
		entry.TraceID = fmt.Sprint(traceID)
		delete(entry.Fields, "trace_id")
	}

	if spanID, ok := entry.Fields["span_id"]; ok {
		entry.SpanID = fmt.Sprint(spanID)
		delete(entry.Fields, "span_id")
	}

	// Add stack trace for errors
	if level >= ERROR {
		entry.Stack = getStackTrace()
	}

	// Write the log entry
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	data = append(data, '\n')
	_, _ = sl.writer.Write(data)
}

// getStackTrace returns a stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// LogMiddleware creates a middleware that logs requests
func LogMiddleware(logger *StructuredLogger) func(next func()) func() {
	return func(next func()) func() {
		return func() {
			start := time.Now()
			logger.Info("Request started", map[string]interface{}{
				"timestamp": start,
			})

			next()

			duration := time.Since(start)
			logger.Info("Request completed", map[string]interface{}{
				"duration": duration.String(),
			})
		}
	}
}
