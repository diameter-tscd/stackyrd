package registry

import (
	"stackyrd/config"
	"stackyrd/pkg/logger"
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
