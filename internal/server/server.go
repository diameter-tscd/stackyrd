package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	_ "stackyrd/internal/services/modules"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/metrics"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"

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
	// Cap request bodies (memory DoS) and slowloris-style idle connections.
	e.Use(echomiddleware.BodyLimit("2M"))
	e.Server.ReadHeaderTimeout = 5 * time.Second
	e.Server.ReadTimeout = 15 * time.Second
	e.Server.WriteTimeout = 30 * time.Second
	e.Server.IdleTimeout = 60 * time.Second

	e.RouteNotFound("/*", func(c echo.Context) error {
		l.Warn("Endpoint not found", "path", c.Request().URL.Path, "method", c.Request().Method)
		return response.Error(c, http.StatusNotFound, "ENDPOINT_NOT_FOUND", "Endpoint not found. This incident will be reported.", map[string]any{
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
	infrastructure.SetInitManager(s.infraInitManager)

	s.dependencies = registry.NewDependencies()

	for name, component := range componentRegistry.GetAll() {
		s.dependencies.Set(name, component)
		s.logger.Info("Registered infrastructure component", "name", name, "type", fmt.Sprintf("%T", component))
	}

	s.dependencies.Seal()
	s.logger.Info("Dependencies sealed — no further infrastructure registration allowed")

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

	svcs := make([]infrastructure.ServiceMeta, 0, len(services))
	for _, svc := range services {
		meta := infrastructure.ServiceMeta{Name: svc.Name(), State: registry.GetServiceState(svc.Name())}
		if wne, ok := svc.Get().(interface{ WireName() string; Endpoints() []string }); ok {
			meta.WireName = wne.WireName()
			meta.Endpoints = wne.Endpoints()
		}
		svcs = append(svcs, meta)
	}
	infrastructure.SetServices(svcs)

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
			BasePath: s.config.Swagger.BasePath,
		})
		s.logger.Info("Swagger UI available at " + s.config.Swagger.BasePath + "/index.html")
	}

	port := s.config.Server.Port
	s.logger.Info("HTTP server starting immediately", "port", port, "env", s.config.App.Env)
	s.logger.Info("Infrastructure components initializing in background...")

	return s.e.Start(":" + port)
}

func (s *Server) registerHealthEndpoints() {
	s.e.GET("/health", func(c echo.Context) error {
		ready := s.infraInitManager.IsReady()
		status := "ok"
		if !ready {
			status = "initializing"
		}
		return response.Success(c, map[string]any{
			"status":                  status,
			"server_ready":            ready,
			"infrastructure":          s.infraInitManager.GetStatus(),
			"initialization_progress": s.infraInitManager.GetInitializationProgress(),
		})
	})

	s.e.GET("/health/dependencies", func(c echo.Context) error {
		allComponents := s.dependencies.GetAll()
		allFactories := registry.GetServiceFactories()
		infraKeys := make([]string, 0, len(allComponents))
		for k := range allComponents {
			infraKeys = append(infraKeys, k)
		}
		svcKeys := make([]string, 0, len(allFactories))
		for k := range allFactories {
			svcKeys = append(svcKeys, k)
		}
		return response.Success(c, map[string]any{
			"total_infrastructure": len(allComponents),
			"list_infrastructure":  infraKeys,
			"total_service":        len(allFactories),
			"list_service":         svcKeys,
		})
	})
}

func (s *Server) Shutdown(ctx context.Context, logger *logger.Logger) error {
	if s.dependencies == nil {
		return nil
	}
	logger.Info("Starting graceful shutdown of infrastructure...")

	var mu sync.Mutex
	var shutdownErrors []error
	var wg sync.WaitGroup

	for name, component := range s.dependencies.GetAll() {
		if component == nil {
			continue
		}
		c, ok := component.(interface{ Close() error })
		if !ok {
			continue
		}
		logger.Info("Shutting down component", "component", name)
		wg.Add(1)
		go func(name string, closeFn func() error) {
			defer wg.Done()
			if err := closeFn(); err != nil {
				mu.Lock()
				shutdownErrors = append(shutdownErrors, fmt.Errorf("%s shutdown error: %w", name, err))
				mu.Unlock()
			} else {
				logger.Info("Component shut down", "component", name)
			}
		}(name, c.Close)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-ctx.Done():
		logger.Warn("Shutdown deadline exceeded; some components may not have closed")
	case <-time.After(30 * time.Second):
		logger.Warn("Shutdown timed out after 30s; some components may not have closed")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(shutdownErrors) > 0 {
		logger.Warn("Graceful shutdown completed with errors", "error_count", len(shutdownErrors))
		return errors.Join(shutdownErrors...)
	}

	logger.Info("Graceful shutdown completed successfully")
	return nil
}
