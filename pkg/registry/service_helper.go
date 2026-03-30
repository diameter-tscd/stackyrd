package registry

import (
	"stackyard/config"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/logger"
)

// ServiceHelper helps services with dependency validation
type ServiceHelper struct {
	config *config.Config
	logger *logger.Logger
	deps   *Dependencies
}

// NewServiceHelper creates a new service helper
func NewServiceHelper(config *config.Config, logger *logger.Logger, deps *Dependencies) *ServiceHelper {
	return &ServiceHelper{
		config: config,
		logger: logger,
		deps:   deps,
	}
}

// RequireDependency validates dependency is available
func (h *ServiceHelper) RequireDependency(name string, available bool) bool {
	if !available {
		h.logger.Warn(name + " not available, skipping service")
		return false
	}
	return true
}

// IsServiceEnabled checks if service is enabled in config
func (h *ServiceHelper) IsServiceEnabled(serviceName string) bool {
	return h.config.Services.IsEnabled(serviceName)
}

// GetRedis returns Redis manager or error if not available
func (h *ServiceHelper) GetRedis() (*infrastructure.RedisManager, bool) {
	if h.deps.RedisManager == nil {
		return nil, false
	}
	return h.deps.RedisManager, true
}

// GetKafka returns Kafka manager or error if not available
func (h *ServiceHelper) GetKafka() (*infrastructure.KafkaManager, bool) {
	if h.deps.KafkaManager == nil {
		return nil, false
	}
	return h.deps.KafkaManager, true
}

// GetPostgres returns PostgreSQL manager (single connection) or error
func (h *ServiceHelper) GetPostgres() (*infrastructure.PostgresManager, bool) {
	if h.deps.PostgresManager == nil {
		return nil, false
	}
	return h.deps.PostgresManager, true
}

// GetPostgresConnection returns PostgreSQL connection manager (multi-tenant) or error
func (h *ServiceHelper) GetPostgresConnection() (*infrastructure.PostgresConnectionManager, bool) {
	if h.deps.PostgresConnectionManager == nil {
		return nil, false
	}
	return h.deps.PostgresConnectionManager, true
}

// GetMongo returns MongoDB manager (single connection) or error
func (h *ServiceHelper) GetMongo() (*infrastructure.MongoManager, bool) {
	if h.deps.MongoManager == nil {
		return nil, false
	}
	return h.deps.MongoManager, true
}

// GetMongoConnection returns MongoDB connection manager (multi-tenant) or error
func (h *ServiceHelper) GetMongoConnection() (*infrastructure.MongoConnectionManager, bool) {
	if h.deps.MongoConnectionManager == nil {
		return nil, false
	}
	return h.deps.MongoConnectionManager, true
}

// GetGrafana returns Grafana manager or error if not available
func (h *ServiceHelper) GetGrafana() (*infrastructure.GrafanaManager, bool) {
	if h.deps.GrafanaManager == nil {
		return nil, false
	}
	return h.deps.GrafanaManager, true
}

// GetCron returns Cron manager or error if not available
func (h *ServiceHelper) GetCron() (*infrastructure.CronManager, bool) {
	if h.deps.CronManager == nil {
		return nil, false
	}
	return h.deps.CronManager, true
}

// GetMinIO returns MinIO manager or error if not available
func (h *ServiceHelper) GetMinIO() (*infrastructure.MinIOManager, bool) {
	if h.deps.MinIOManager == nil {
		return nil, false
	}
	return h.deps.MinIOManager, true
}
