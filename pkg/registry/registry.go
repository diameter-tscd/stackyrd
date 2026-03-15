package registry

import (
	"fmt"
	"stackyard/config"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"

	"github.com/labstack/echo/v4"
)

// ServiceFactory creates a service instance with dependencies
type ServiceFactory func(config *config.Config, logger *logger.Logger, deps *Dependencies) interfaces.Service

// Global registry of service factories
var serviceFactories = make(map[string]ServiceFactory)

// RegisterService registers a service factory for automatic discovery
func RegisterService(name string, factory ServiceFactory) {
	serviceFactories[name] = factory
}

// AutoDiscoverServices automatically discovers and creates all enabled services
func AutoDiscoverServices(
	config *config.Config,
	logger *logger.Logger,
	deps *Dependencies,
) []interfaces.Service {
	var services []interfaces.Service

	for name, factory := range serviceFactories {
		logger.Debug("Creating service", "name", name)
		if config.Services.IsEnabled(name) {
			if service := factory(config, logger, deps); service != nil {
				services = append(services, service)
				logger.Info("Auto-registered service", "service", name)
			} else {
				logger.Warn("Service factory returned nil", "service", name)
			}
		} else {
			logger.Debug("Service disabled via config", "service", name)
		}
	}

	return services
}

// ServiceRegistry holds discovered services and manages their lifecycle
type ServiceRegistry struct {
	services []interfaces.Service
	logger   *logger.Logger
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(logger *logger.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		services: make([]interfaces.Service, 0),
		logger:   logger,
	}
}

// GetServiceFactories returns the global service factories map for testing/debugging
func GetServiceFactories() map[string]ServiceFactory {
	return serviceFactories
}

// Register adds a service to the registry
func (r *ServiceRegistry) Register(s interfaces.Service) {
	r.services = append(r.services, s)
}

// RegisterServiceWithDependencies creates and registers a service with dependencies
func (r *ServiceRegistry) RegisterServiceWithDependencies(
	config *config.Config,
	logger *logger.Logger,
	deps *Dependencies,
	serviceName string,
) error {
	if factory, exists := serviceFactories[serviceName]; exists {
		if config.Services.IsEnabled(serviceName) {
			service := factory(config, logger, deps)
			if service != nil {
				r.Register(service)
				r.logger.Info("Service registered with dependencies", "service", serviceName)
				return nil
			}
			return fmt.Errorf("failed to create service: %s", serviceName)
		} else {
			r.logger.Debug("Service disabled via config", "service", serviceName)
			return nil
		}
	}
	return fmt.Errorf("service factory not found: %s", serviceName)
}

// GetServices returns the list of registered services
func (r *ServiceRegistry) GetServices() []interfaces.Service {
	return r.services
}

// Boot initializes enabled services and registers their routes
func (r *ServiceRegistry) Boot(e *echo.Echo) {
	api := e.Group("/api/v1")

	for _, s := range r.services {
		if s.Enabled() {
			r.logger.Info("Starting Service...", "service", s.Name())
			s.RegisterRoutes(api)
			r.logger.Info("Service Started", "service", s.Name())
		} else {
			r.logger.Warn("Service Skipped (Disabled via config)", "service", s.Name())
		}
	}
}

// BootService boots a single service (for dynamic registration)
func (r *ServiceRegistry) BootService(e *echo.Echo, s interfaces.Service) {
	if s.Enabled() {
		api := e.Group("/api/v1")
		r.logger.Info("Starting Service...", "service", s.Name())
		s.RegisterRoutes(api)
		r.logger.Info("Service Started", "service", s.Name())
	} else {
		r.logger.Warn("Service Skipped (Disabled via config)", "service", s.Name())
	}
}
