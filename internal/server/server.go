package server

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	_ "stackyrd/internal/services/modules"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/plugin"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"
	"stackyrd/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Server struct {
	gin              *gin.Engine
	config           *config.Config
	logger           *logger.Logger
	dependencies     *registry.Dependencies
	infraInitManager *infrastructure.InfraInitManager
}

func New(cfg *config.Config, l *logger.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Custom error handler
	r.NoRoute(func(c *gin.Context) {
		l.Warn("Endpoint not found", "path", c.Request.URL.Path, "method", c.Request.Method)
		response.Error(c, http.StatusNotFound, "ENDPOINT_NOT_FOUND", "Endpoint not found. This incident will be reported.", map[string]interface{}{
			"path":   c.Request.URL.Path,
			"method": c.Request.Method,
		})
	})

	r.NoMethod(func(c *gin.Context) {
		l.Warn("Method not allowed")
		response.Error(c, http.StatusMethodNotAllowed, "HTTP_ERROR", "Method not allowed")
	})

	return &Server{
		gin:    r,
		config: cfg,
		logger: l,
	}
}

func (s *Server) Start() error {
	s.infraInitManager = infrastructure.NewInfraInitManager(s.logger)
	s.logger.Info("Starting async infrastructure initialization...")
	componentRegistry := s.infraInitManager.StartAsyncInitialization(s.config, s.logger)

	// Create dynamic dependencies container
	s.dependencies = registry.NewDependencies()

	// Dynamically load all components from registry
	for name, component := range componentRegistry.GetAll() {
		s.dependencies.Set(name, component)
		s.logger.Info("Registered infrastructure component", "name", name, "type", fmt.Sprintf("%T", component))
	}

	// Handle database connection defaults
	s.setConnectionDefaults()

	// Initialize plugin system — happens before services so they can
	// discover plugins via the PluginBridge.
	s.logger.Info("Initializing Plugin system...")
	pluginGroup := s.gin.Group("/api/v1")
	if err := plugin.Init(s.config, s.logger, pluginGroup); err != nil {
		s.logger.Error("Failed to initialize plugin system", err)
	}
	if bridge := plugin.GetGlobalPluginBridge(); bridge != nil {
		s.dependencies.Set("plugins", bridge)
		s.logger.Info("PluginBridge registered in service dependencies as 'plugins'")
	}

	s.logger.Info("Initializing Middleware...")

	// Apply middleware configuration from config
	middleware.GetGlobalMiddlewareRegistry().ApplyConfig(s.config)

	// Auto-discover and register all enabled middlewares
	middlewares := middleware.GetGlobalMiddlewareRegistry().AutoDiscoverMiddlewares(s.config, s.logger)
	for _, mw := range middlewares {
		if mw != nil {
			s.gin.Use(mw)
		}
	}

	s.logger.Info("Booting Services...")
	serviceRegistry := registry.NewServiceRegistry(s.logger)
	s.registerHealthEndpoints()

	services := registry.AutoDiscoverServices(s.config, s.logger, s.dependencies)
	for _, service := range services {
		serviceRegistry.Register(service)
	}

	if len(services) <= 0 {
		s.logger.Warn("No services registered!")
	}

	serviceRegistry.Boot(s.gin)
	s.logger.Info("All services boot successfully")

	// Register Swagger UI
	if s.config.Swagger.Enabled {
		s.logger.Info("Registering Swagger UI documentation...")
		middleware.RegisterSwaggerRoutes(s.gin, middleware.SwaggerConfig{
			Enabled:  s.config.Swagger.Enabled,
			BasePath: "/swagger",
		})
		s.logger.Info("Swagger UI available at /swagger/index.html")
	}

	port := s.config.Server.Port
	s.logger.Info("HTTP server starting immediately", "port", port, "env", s.config.App.Env)
	s.logger.Info("Infrastructure components initializing in background...")

	return s.gin.Run(":" + port)
}

func (s *Server) setConnectionDefaults() {
	// Handle PostgreSQL connection defaults
	if pg, ok := s.dependencies.Get("postgres"); ok {
		switch mgr := pg.(type) {
		case *infrastructure.PostgresConnectionManager:
			if defaultConn, exists := mgr.GetDefaultConnection(); exists {
				s.dependencies.Set("postgres.default", defaultConn)
				s.logger.Info("PostgreSQL single connection manager detected")
			}
		}
	}

	// Handle MongoDB connection defaults
	if mg, ok := s.dependencies.Get("mongo"); ok {
		switch mgr := mg.(type) {
		case *infrastructure.MongoConnectionManager:
			if defaultConn, exists := mgr.GetDefaultConnection(); exists {
				s.dependencies.Set("mongo.default", defaultConn)
				s.logger.Info("MongoDB single connection manager detected")
			}
		}
	}
}

func (s *Server) registerHealthEndpoints() {
	s.gin.GET("/health", func(c *gin.Context) {
		response.Success(c, map[string]interface{}{
			"status":                  "ok",
			"server_ready":            true,
			"infrastructure":          s.infraInitManager.GetStatus(),
			"initialization_progress": s.infraInitManager.GetInitializationProgress(),
		})
	})

	s.gin.GET("/health/infrastructure", func(c *gin.Context) {
		response.Success(c, s.infraInitManager.GetStatus())
	})

	s.gin.GET("/health/dependencies", func(c *gin.Context) {
		// Each GetAll() call is TTL-cached; snapshot once locally to avoid
		// repeated map copies during the same request.
		allComponents := s.dependencies.GetAll()
		allFactories := registry.GetServiceFactories()
		response.Success(c, map[string]interface{}{
			"total_infrastructure": len(allComponents),
			"list_infrastructure":  slices.Collect(maps.Keys(allComponents)),
			"total_service":        len(allFactories),
			"list_service":         slices.Collect(maps.Keys(allFactories)),
		})
	})

	s.gin.GET("/health/resources", func(c *gin.Context) {
		response.Success(c, map[string]interface{}{
			"memory_usage":    utils.GetMemSelf(),
			"routine_running": utils.GetRoutine(),
		})
	})
}

func (s *Server) Shutdown(ctx context.Context, logger *logger.Logger) error {
	utils.ClearScreen()
	logger.Info("Starting graceful shutdown of infrastructure...")

	if s.infraInitManager != nil {
		logger.Info("Stopping async infrastructure initialization manager...")
	}

	var shutdownErrors []error

	shutdownComponent := func(name string, closer interface{}) {
		if closer == nil {
			return
		}

		logger.Info("Shutting down " + name + "...")
		if c, ok := closer.(interface{ Close() error }); ok {
			done := make(chan struct{}, 1)
			go func() {
				err := c.Close()
				if err != nil {
					shutdownErrors = append(shutdownErrors, fmt.Errorf("%s shutdown error: %w", name, err))
					logger.Error("Error shutting down "+name, err)
				} else {
					logger.Info(name + " shut down successfully")
				}
				done <- struct{}{}
			}()
			select {
			case <-done:
				// completed normally
			case <-time.After(10 * time.Second):
				shutdownErrors = append(shutdownErrors, fmt.Errorf("%s: forced shutdown (timeout)", name))
				logger.Warn(name + " shutdown timed out after 10s, continuing")
			}
		}
	}

	// Dynamically shut down all registered components
	for name, component := range s.dependencies.GetAll() {
		shutdownComponent(name, component)
	}

	if len(shutdownErrors) > 0 {
		logger.Warn("Graceful shutdown completed with errors", "error_count", len(shutdownErrors))
		for _, err := range shutdownErrors {
			logger.Error("Shutdown error", err)
		}
		return fmt.Errorf("shutdown completed with %d errors", len(shutdownErrors))
	}

	logger.Info("Graceful shutdown completed successfully")
	return nil
}
