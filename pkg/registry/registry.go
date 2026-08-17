package registry

import (
	"fmt"
	"slices"
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
	if factory == nil {
		return
	}
	if _, loaded := serviceFactories.LoadOrStore(name, factory); loaded {
		return
	}
}

func RegisterServiceWithDeps(name string, factory ServiceFactoryWithDeps) {
	if factory == nil {
		return
	}
	if _, loaded := serviceFactoriesWithDeps.LoadOrStore(name, factory); loaded {
		return
	}
}

func AutoDiscoverServices(
	config *config.Config,
	logger *logger.Logger,
	deps *Dependencies,
) []interfaces.Service {
	var services []interfaces.Service

	serviceFactories.Range(func(nameObj, factoryObj any) bool {
		name, nameOK := nameObj.(string)
		factory, factoryOK := factoryObj.(ServiceFactory)
		if !nameOK || !factoryOK {
			return true
		}
		logger.Debug("Creating service", "name", name)
		if config.Services.IsEnabled(name) {
			if service := factory(config, logger); service != nil {
				services = append(services, service)
				logger.Info("Auto-registered service", "service", name)
				storeDiscovered(name, service)
				setServiceState(name, "running")
			} else {
				logger.Warn("Service factory returned nil", "service", name)
				setServiceState(name, "failed")
			}
		} else {
			logger.Debug("Service disabled via config", "service", name)
			setServiceState(name, "disabled")
		}
		return true
	})

	serviceFactoriesWithDeps.Range(func(nameObj, factoryObj any) bool {
		name, nameOK := nameObj.(string)
		factory, factoryOK := factoryObj.(ServiceFactoryWithDeps)
		if !nameOK || !factoryOK {
			return true
		}
		logger.Debug("Creating service with dependencies", "name", name)
		if config.Services.IsEnabled(name) {
			if service := factory(config, logger, deps); service != nil {
				services = append(services, service)
				logger.Info("Auto-registered service", "service", name)
				storeDiscovered(name, service)
				setServiceState(name, "running")
			} else {
				logger.Warn("Service factory returned nil", "service", name)
				setServiceState(name, "failed")
			}
		} else {
			logger.Debug("Service disabled via config", "service", name)
			setServiceState(name, "disabled")
		}
		return true
	})

	return services
}

// storeDiscovered indexes a service instance under both its registration key
// (e.g. "broadcast_service") and its display name ("Broadcast Service") so
// lookups by either identifier succeed.
func storeDiscovered(name string, service interfaces.Service) {
	if get := service.Get(); get != nil {
		serviceDiscovered.Store(name, get)
		serviceDiscovered.Store(service.Name(), get)
	}
}

type ServiceRegistry struct {
	mu       sync.RWMutex
	services []interfaces.Service
	logger   *logger.Logger
}

func NewServiceRegistry(logger *logger.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		services: make([]interfaces.Service, 0),
		logger:   logger,
	}
}

func GetServiceFactories() map[string]any {
	result := make(map[string]any)
	serviceFactories.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			result[k] = value
		}
		return true
	})
	serviceFactoriesWithDeps.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			result[k] = value
		}
		return true
	})
	return result
}

func GetService(name string) any {
	val, _ := serviceDiscovered.Load(name)
	return val
}

// Service states recorded during discovery. Values: "running" (instance
// created), "failed" (enabled by config but factory returned nil — typically a
// missing infrastructure dependency), "disabled" (disabled by config).
var serviceStates = &sync.Map{}

func setServiceState(name, state string) {
	serviceStates.Store(name, state)
}

// GetServiceState returns the recorded state of a service:
// "running", "failed", "disabled", or "" if the service is unknown.
func GetServiceState(name string) string {
	state, _ := serviceStates.Load(name)
	s, _ := state.(string)
	return s
}

// GetServiceStates returns a copy of all recorded service states.
func GetServiceStates() map[string]string {
	result := make(map[string]string)
	serviceStates.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			s, _ := value.(string)
			result[k] = s
		}
		return true
	})
	return result
}

func (r *ServiceRegistry) Register(s interfaces.Service) {
	if s == nil {
		return
	}
	r.mu.Lock()
	r.services = append(r.services, s)
	r.mu.Unlock()
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
		factory, ok := factoryObj.(ServiceFactory)
		if !ok {
			return fmt.Errorf("invalid factory for service: %s", serviceName)
		}
		service := factory(config, logger)
		if service == nil {
			setServiceState(serviceName, "failed")
			return fmt.Errorf("failed to create service: %s", serviceName)
		}
		r.Register(service)
		storeDiscovered(serviceName, service)
		setServiceState(serviceName, "running")
		r.logger.Info("Service registered", "service", serviceName)
		return nil
	}
	if factoryObj, exists := serviceFactoriesWithDeps.Load(serviceName); exists {
		if !config.Services.IsEnabled(serviceName) {
			r.logger.Debug("Service disabled via config", "service", serviceName)
			return nil
		}
		factory, ok := factoryObj.(ServiceFactoryWithDeps)
		if !ok {
			return fmt.Errorf("invalid factory for service: %s", serviceName)
		}
		service := factory(config, logger, deps)
		if service == nil {
			setServiceState(serviceName, "failed")
			return fmt.Errorf("failed to create service: %s", serviceName)
		}
		r.Register(service)
		storeDiscovered(serviceName, service)
		setServiceState(serviceName, "running")
		r.logger.Info("Service registered with dependencies", "service", serviceName)
		return nil
	}
	return fmt.Errorf("service factory not found: %s", serviceName)
}

func (r *ServiceRegistry) GetServices() []interfaces.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.services)
}

func (r *ServiceRegistry) Boot(e *echo.Echo) {
	api := e.Group(viper.GetString("server.services_endpoint"))

	for _, s := range r.GetServices() {
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
