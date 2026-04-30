package main

import (
	"time"
)

// Forward declarations to avoid circular imports
type Config struct{}
type Logger struct{}
type LogBroadcaster struct{}

// Application constants
const (
	AppName        = "stackyrd"
	DefaultAppName = ""
	DefaultVersion = "1.0.0"
	DefaultEnv     = "development"

	// Default configuration values
	DefaultServerPort   = "8080"
	DefaultStartupDelay = 15 // seconds
	DefaultBannerPath   = "banner.txt"

	// File paths
	WebFolderPath = "web"

	// Service names for logging and initialization
	ServiceConfigName     = "Configuration"
	ServiceMiddlewareName = "Middleware"
	ServiceMonitoringName = "Monitoring"
	ServiceGrafanaName    = "Grafana"
	ServiceMinIOName      = "MinIO"
	ServiceRedisCacheName = "Redis Cache"
	ServiceKafkaName      = "Kafka Messaging"
	ServicePostgreSQLName = "PostgreSQL"
	ServiceMongoDBName    = "MongoDB"
	ServiceCronName       = "Cron Scheduler"
	ServiceExternalName   = "External Services"

	// Color codes for TUI output
	ColorPurple = "\033[35m"
	ColorReset  = "\033[0m"
	ColorYellow = "\033[33m"

	// Error messages
	ErrInvalidConfigURLFormat = "invalid config URL format"
	ErrPortError              = "port error"
	ErrStepFailed             = "step failed"
)

// ServiceInit represents a service in the initialization queue
type ServiceInit struct {
	Name     string
	Enabled  bool
	InitFunc func() error
}

// ServiceConfig represents a service with its name and enabled status
type ServiceConfig struct {
	Name    string
	Enabled bool
}

// AppContext holds the application state throughout initialization
type AppContext struct {
	Config      *Config
	Logger      *Logger
	Broadcaster *LogBroadcaster
	BannerText  string
	Timestamp   string
	ConfigURL   string
}

// AppStep represents a single step in the application initialization process
type AppStep struct {
	Name string
	Fn   func(*AppContext) error
}

// OutputMode represents the output mode for the application
type OutputMode int

const (
	OutputModeTUI OutputMode = iota
	OutputModeConsole
)

// String returns the string representation of the output mode
func (m OutputMode) String() string {
	switch m {
	case OutputModeTUI:
		return "TUI"
	case OutputModeConsole:
		return "Console"
	default:
		return "Unknown"
	}
}

// ServiceStatus represents the status of a service
type ServiceStatus int

const (
	ServiceStatusEnabled ServiceStatus = iota
	ServiceStatusDisabled
	ServiceStatusSkipped
)

// String returns the string representation of the service status
func (s ServiceStatus) String() string {
	switch s {
	case ServiceStatusEnabled:
		return "enabled"
	case ServiceStatusDisabled:
		return "disabled"
	case ServiceStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Duration constants for timeouts and delays
const (
	StartupDelay            = 500 * time.Millisecond
	ShutdownDelay           = 100 * time.Millisecond
	PortCheckTimeout        = 5 * time.Second
	GracefulShutdownTimeout = 30 * time.Second
)

// Log levels for structured logging
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

// Service types for categorization
const (
	ServiceTypeInfrastructure = "infrastructure"
	ServiceTypeApplication    = "application"
	ServiceTypeMonitoring     = "monitoring"
)

// Configuration validation constants
const (
	MinStartupDelay    = 0
	MaxStartupDelay    = 300 // 5 minutes
	MinPortNumber      = 1
	MaxPortNumber      = 65535
	MaxPhotoSizeMB     = 10
	DefaultPhotoSizeMB = 5
)
