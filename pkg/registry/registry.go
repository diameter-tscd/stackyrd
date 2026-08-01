package registry

import (
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type ServiceFactory func(config *config.Config, logger *logger.Logger) interfaces.Service

type ServiceFactoryWithDeps func(config *config.Config, logger *logger.Logger, deps *Dependencies) interfaces.Service

var serviceFactories = &sync.Map{}
var serviceFactoriesWithDeps = &sync.Map{}

var (
	serviceDiscovered = &sync.Map{}
)

func RegisterService(name string, factory ServiceFactory) {
	if _, exist := serviceFactories.Load(name); !exist && factory != nil {
		serviceFactories.Store(name, factory)
	}
}

func RegisterServiceWithDeps(name string, factory ServiceFactoryWithDeps) {
	if _, exist := serviceFactoriesWithDeps.Load(name); !exist && factory != nil {
		serviceFactoriesWithDeps.Store(name, factory)
	}
}

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
			if service := factory(config, logger); service != nil {
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

	serviceFactoriesWithDeps.Range(func(nameObj, factoryObj interface{}) bool {
		name := nameObj.(string)
		factory := factoryObj.(ServiceFactoryWithDeps)
		logger.Debug("Creating service with dependencies", "name", name)
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

type ServiceRegistry struct {
	services []interfaces.Service
	logger   *logger.Logger
}

func NewServiceRegistry(logger *logger.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		services: make([]interfaces.Service, 0),
		logger:   logger,
	}
}

func GetServiceFactories() map[string]interface{} {
	result := make(map[string]interface{})
	serviceFactories.Range(func(key, value interface{}) bool {
		result[key.(string)] = value
		return true
	})
	serviceFactoriesWithDeps.Range(func(key, value interface{}) bool {
		result[key.(string)] = value
		return true
	})
	return result
}

func GetService(name string) interface{} {
	val, _ := serviceDiscovered.Load(name)
	return val
}

func (r *ServiceRegistry) Register(s interfaces.Service) {
	r.services = append(r.services, s)
}

func (r *ServiceRegistry) RegisterServiceWithDependencies(
	config *config.Config,
	logger *logger.Logger,
	deps *Dependencies,
	serviceName string,
) error {
	if factoryObj, exists := serviceFactories.Load(serviceName); exists {
		if !config.Services.IsEnabled(serviceName) {
			r.logger.Debug("Service disabled via config", "service", serviceName)
			return nil
		}
		service := factoryObj.(ServiceFactory)(config, logger)
		if service == nil {
			return fmt.Errorf("failed to create service: %s", serviceName)
		}
		r.Register(service)
		r.logger.Info("Service registered", "service", serviceName)
		return nil
	}
	if factoryObj, exists := serviceFactoriesWithDeps.Load(serviceName); exists {
		if !config.Services.IsEnabled(serviceName) {
			r.logger.Debug("Service disabled via config", "service", serviceName)
			return nil
		}
		service := factoryObj.(ServiceFactoryWithDeps)(config, logger, deps)
		if service == nil {
			return fmt.Errorf("failed to create service: %s", serviceName)
		}
		r.Register(service)
		r.logger.Info("Service registered with dependencies", "service", serviceName)
		return nil
	}
	return fmt.Errorf("service factory not found: %s", serviceName)
}

func (r *ServiceRegistry) GetServices() []interfaces.Service {
	return r.services
}

func (r *ServiceRegistry) Boot(e *echo.Echo) {
	api := e.Group(viper.GetString("server.services_endpoint"))

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

func (r *ServiceRegistry) BootService(e *echo.Echo, s interfaces.Service) {
	if s.Enabled() {
		api := e.Group(viper.GetString("server.services_endpoint"))
		r.logger.Info("Starting Service...", "service", s.Name())
		s.RegisterRoutes(api)
		r.logger.Info("Service Started", "service", s.Name())
	} else {
		r.logger.Warn("Service Skipped (Disabled via config)", "service", s.Name())
	}
}
