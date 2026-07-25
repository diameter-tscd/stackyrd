package main

const (
	AppName = "stackyrd"

	DefaultBannerPath = "banner.txt"

	ServiceGrafanaName    = "Grafana"
	ServiceMinIOName      = "MinIO"
	ServiceRedisCacheName = "Redis Cache"
	ServiceKafkaName      = "Kafka Messaging"
	ServicePostgreSQLName = "PostgreSQL"
	ServiceMongoDBName    = "MongoDB"
	ServiceCronName       = "Cron Scheduler"

	ColorPrimary = "\033[38;2;141;174;165m"
	ColorReset   = "\033[0m"
	ColorYellow  = "\033[33m"

	ErrStepFailed = "step failed"
)

type ServiceConfig struct {
	Name    string
	Enabled bool
}

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)
