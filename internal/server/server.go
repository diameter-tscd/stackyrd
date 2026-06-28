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
	"stackyrd/pkg/metrics"
	"stackyrd/pkg/plugin"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"
	"stackyrd/pkg/utils"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

type Server struct {
	e                *echo.Echo
	config           *config.Config
	logger           *logger.Logger
	dependencies     *registry.Dependencies
	infraInitManager *infrastructure.InfraInitManager
}

func New(cfg *config.Config, l *logger.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomiddleware.Recover())

	e.RouteNotFound("/*", func(c echo.Context) error {
		l.Warn("Endpoint not found", "path", c.Request().URL.Path, "method", c.Request().Method)
		return response.Error(c, http.StatusNotFound, "ENDPOINT_NOT_FOUND", "Endpoint not found. This incident will be reported.", map[string]interface{}{
			"path":   c.Request().URL.Path,
			"method": c.Request().Method,
		})
	})

	echo.MethodNotAllowedHandler = func(c echo.Context) error {
		l.Warn("Method not allowed", "path", c.Request().URL.Path, "method", c.Request().Method)
		return response.Error(c, http.StatusMethodNotAllowed, "HTTP_ERROR", "Method not allowed")
	}

	return &Server{
		e:      e,
		config: cfg,
		logger: l,
	}
}

func (s *Server) Start() error {
	s.infraInitManager = infrastructure.NewInfraInitManager(s.logger)
	s.logger.Info("Starting async infrastructure initialization...")
	componentRegistry := s.infraInitManager.StartAsyncInitialization(s.config, s.logger)

	s.dependencies = registry.NewDependencies()

	for name, component := range componentRegistry.GetAll() {
		s.dependencies.Set(name, component)
		s.logger.Info("Registered infrastructure component", "name", name, "type", fmt.Sprintf("%T", component))
	}

	s.setConnectionDefaults()

	s.logger.Info("Initializing Plugin system...")
	pluginGroup := s.e.Group("/api/v1")
	if err := plugin.Init(s.config, s.logger, pluginGroup); err != nil {
		s.logger.Error("Failed to initialize plugin system", err)
	}
	if bridge := plugin.GetGlobalPluginBridge(); bridge != nil {
		s.dependencies.Set("plugins", bridge)
		s.logger.Info("PluginBridge registered in service dependencies as 'plugins'")
	}

	s.logger.Info("Initializing Middleware...")

	middleware.GetGlobalMiddlewareRegistry().ApplyConfig(s.config)

	middlewares := middleware.GetGlobalMiddlewareRegistry().AutoDiscoverMiddlewares(s.config, s.logger)
	for _, mw := range middlewares {
		if mw != nil {
			s.e.Use(mw)
		}
	}

	s.logger.Info("Registering infrastructure component routes...")
	for name, comp := range componentRegistry.GetAll() {
		if rr, ok := comp.(infrastructure.RouteRegistrar); ok {
			for _, rh := range rr.RouteHandlers() {
				rg := s.e.Group(rh.Path)
				if rh.Mode == infrastructure.RouterCustom && len(rh.Handlers) > 0 {
					rg.Use(rh.Handlers...)
				}
				rh.Handler(rg)
				s.logger.Info("Mounted component routes",
					"component", name, "path", rh.Path, "mode", rh.Mode)
			}
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

	serviceRegistry.Boot(s.e)

	s.logger.Info("All services boot successfully")

	if s.config.Metrics.Enabled {
		s.logger.Info("Registering Prometheus metrics endpoint", "path", s.config.Metrics.Path)
		s.e.GET(s.config.Metrics.Path, echo.WrapHandler(metrics.GetMetrics().Handler()))
	}

	if s.config.Swagger.Enabled {
		s.logger.Info("Registering Swagger UI documentation...")
		middleware.RegisterSwaggerRoutes(s.e, middleware.SwaggerConfig{
			Enabled:  s.config.Swagger.Enabled,
			BasePath: "/swagger",
		})
		s.logger.Info("Swagger UI available at /swagger/index.html")
	}

	port := s.config.Server.Port
	s.logger.Info("HTTP server starting immediately", "port", port, "env", s.config.App.Env)
	s.logger.Info("Infrastructure components initializing in background...")

	return s.e.Start(":" + port)
}

func (s *Server) setConnectionDefaults() {
	if pg, ok := s.dependencies.Get("postgres"); ok {
		switch mgr := pg.(type) {
		case *infrastructure.PostgresConnectionManager:
			if defaultConn, exists := mgr.GetDefaultConnection(); exists {
				s.dependencies.Set("postgres.default", defaultConn)
				s.logger.Info("PostgreSQL single connection manager detected")
			}
		}
	}

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
	s.e.GET("/health", func(c echo.Context) error {
		ready := s.infraInitManager.IsReady()
		status := "ok"
		if !ready {
			status = "initializing"
		}
		return response.Success(c, map[string]interface{}{
			"status":                  status,
			"server_ready":            ready,
			"infrastructure":          s.infraInitManager.GetStatus(),
			"initialization_progress": s.infraInitManager.GetInitializationProgress(),
		})
	})

	s.e.GET("/health/infrastructure", func(c echo.Context) error {
		return response.Success(c, s.infraInitManager.GetStatus())
	})

	s.e.GET("/health/dependencies", func(c echo.Context) error {
		allComponents := s.dependencies.GetAll()
		allFactories := registry.GetServiceFactories()
		return response.Success(c, map[string]interface{}{
			"total_infrastructure": len(allComponents),
			"list_infrastructure":  slices.Collect(maps.Keys(allComponents)),
			"total_service":        len(allFactories),
			"list_service":         slices.Collect(maps.Keys(allFactories)),
		})
	})

	s.e.GET("/health/resources", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{
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
			case <-time.After(10 * time.Second):
				shutdownErrors = append(shutdownErrors, fmt.Errorf("%s: forced shutdown (timeout)", name))
				logger.Warn(name + " shutdown timed out after 10s, continuing")
			}
		}
	}

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
