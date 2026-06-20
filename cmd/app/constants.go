package main

import (
	"time"
)

type Config struct{}
type Logger struct{}
type LogBroadcaster struct{}

const (
	AppName        = "stackyrd-nano"
	DefaultAppName = ""
	DefaultVersion = "1.0.0"
	DefaultEnv     = "development"

	DefaultServerPort   = "8080"
	DefaultStartupDelay = 15
	DefaultBannerPath   = "banner.txt"

	WebFolderPath = "web"

	ServiceConfigName     = "Configuration"
	ServiceMiddlewareName = "Middleware"
	ServiceMonitoringName = "Monitoring"
	ServicePostgreSQLName = "PostgreSQL"

	ColorPrimary = "\033[38;2;141;174;165m"
	ColorReset   = "\033[0m"
	ColorYellow  = "\033[33m"

	ErrInvalidConfigURLFormat = "invalid config URL format"
	ErrPortError              = "port error"
	ErrStepFailed             = "step failed"
)

type ServiceInit struct {
	Name     string
	Enabled  bool
	InitFunc func() error
}

type ServiceConfig struct {
	Name    string
	Enabled bool
}

type AppContext struct {
	Config      *Config
	Logger      *Logger
	Broadcaster *LogBroadcaster
	BannerText  string
	Timestamp   string
	ConfigURL   string
}

type AppStep struct {
	Name string
	Fn   func(*AppContext) error
}

type OutputMode int

const (
	OutputModeTUI OutputMode = iota
	OutputModeConsole
)

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

type ServiceStatus int

const (
	ServiceStatusEnabled ServiceStatus = iota
	ServiceStatusDisabled
	ServiceStatusSkipped
)

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

const (
	StartupDelay            = 500 * time.Millisecond
	ShutdownDelay           = 100 * time.Millisecond
	PortCheckTimeout        = 5 * time.Second
	GracefulShutdownTimeout = 30 * time.Second
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

const (
	ServiceTypeInfrastructure = "infrastructure"
	ServiceTypeApplication    = "application"
	ServiceTypeMonitoring     = "monitoring"
)

const (
	MinStartupDelay    = 0
	MaxStartupDelay    = 300
	MinPortNumber      = 1
	MaxPortNumber      = 65535
	MaxPhotoSizeMB     = 10
	DefaultPhotoSizeMB = 5
)
