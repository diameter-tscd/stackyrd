package main

import "time"

const (
	AppName = "stackyrd"

	DefaultBannerPath = "banner.txt"

	DefaultAppName      = "stackyrd"
	DefaultVersion      = "1.0.0"
	DefaultEnv          = "development"
	DefaultServerPort   = "8080"
	DefaultStartupDelay = 3

	ServiceGrafanaName    = "Grafana"
	ServiceMinIOName      = "MinIO"
	ServiceRedisCacheName = "Redis Cache"
	ServiceKafkaName      = "Kafka Messaging"
	ServicePostgreSQLName = "PostgreSQL"
	ServiceMongoDBName    = "MongoDB"
	ServiceCronName       = "Cron Scheduler"

	ServiceConfigName     = "Configuration"
	ServiceMiddlewareName = "Middleware"
	ServiceMonitoringName = "Monitoring"

	MinStartupDelay = 1
	MaxStartupDelay = 30

	StartupDelay  = time.Second * 2
	ShutdownDelay = time.Second

	ColorPrimary = "\033[38;2;141;174;165m"
	ColorReset   = "\033[0m"
	ColorYellow  = "\033[33m"

	ErrStepFailed             = "step failed"
	ErrInvalidConfigURLFormat = "invalid config URL format"
	ErrPortError              = "port error"
)

type AppStep struct {
	Name string
	Fn   func(*AppContext) error
}

type AppContext struct {
	Timestamp string
	ConfigURL string
}

type ServiceConfig struct {
	Name    string
	Enabled bool
}

type ServiceInit struct {
	Name     string
	Enabled  bool
	InitFunc func() error
}

type ServiceStatus int

const (
	ServiceStatusEnabled ServiceStatus = iota
	ServiceStatusDisabled
)

func (s ServiceStatus) String() string {
	switch s {
	case ServiceStatusEnabled:
		return "enabled"
	case ServiceStatusDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)
