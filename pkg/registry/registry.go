package registry

import (
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// ServiceFactory creates a service instance with dependencies
type ServiceFactory func(config *config.Config, logger *logger.Logger, deps *Dependencies) interfaces.Service

// Global registry of service factories — write-once, read-many after boot
var serviceFactories = &sync.Map{}

// Global read-only registry of discovered services — write-once, read-many
var (
	serviceDiscovered = &sync.Map{}
	// serviceDiscoveredMu removed: sync.Map is lock-free for reads
)

// RegisterService registers a service factory for automatic discovery
func RegisterService(name string, factory ServiceFactory) {
	// avoid duplicate register if service has same name
	if _, exist := serviceFactories.Load(name); !exist && factory != nil {
		serviceFactories.Store(name, factory)
	}
}

// AutoDiscoverServices automatically discovers and creates all enabled services
func AutoDiscoverServices(
	config *config.Config,
	logger *logger.Logger,
	deps *Dependencies,
) []interfaces.Service {
	var services []interfaces.Service

	serviceFactories.Range(func(nameObj, factoryObj interface{}) bool {
		name := nameObj.(string)
		factory := factoryObj.(ServiceFactory)
		logger.Debug("Creating service", "name", name)
		if config.Services.IsEnabled(name) {
			if service := factory(config, logger, deps); service != nil {
				services = append(services, service)
				logger.Info("Auto-registered service", "service", name)
				serviceDiscovered.Store(service.Name(), service.Get())
			} else {
				logger.Warn("Service factory returned nil", "service", name)
			}
		} else {
			logger.Debug("Service disabled via config", "service", name)
		}
		return true
	})

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
	result := make(map[string]ServiceFactory)
	serviceFactories.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(ServiceFactory)
		return true
	})
	return result
}

func GetService(name string) interface{} {
	val, _ := serviceDiscovered.Load(name)
	return val
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
	factoryObj, exists := serviceFactories.Load(serviceName)
	if !exists {
		return fmt.Errorf("service factory not found: %s", serviceName)
	}
	factory := factoryObj.(ServiceFactory)
	if !config.Services.IsEnabled(serviceName) {
		r.logger.Debug("Service disabled via config", "service", serviceName)
		return nil
	}
	if service := factory(config, logger, deps); service != nil {
		r.Register(service)
		r.logger.Info("Service registered with dependencies", "service", serviceName)
		return nil
	}
	return fmt.Errorf("failed to create service: %s", serviceName)
}

// GetServices returns the list of registered services
func (r *ServiceRegistry) GetServices() []interfaces.Service {
	return r.services
}

// Boot initializes enabled services and registers their routes
func (r *ServiceRegistry) Boot(engine *gin.Engine) {
	api := engine.Group(viper.GetString("server.services_endpoint"))

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
func (r *ServiceRegistry) BootService(engine *gin.Engine, s interfaces.Service) {
	if s.Enabled() {
		api := engine.Group(viper.GetString("server.services_endpoint"))
		r.logger.Info("Starting Service...", "service", s.Name())
		s.RegisterRoutes(api)
		r.logger.Info("Service Started", "service", s.Name())
	} else {
		r.logger.Warn("Service Skipped (Disabled via config)", "service", s.Name())
	}
}
