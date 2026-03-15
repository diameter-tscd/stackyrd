package server

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	_ "stackyard/internal/services/modules"

	"stackyard/config"
	"stackyard/internal/middleware"
	"stackyard/internal/monitoring"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/response"
	"stackyard/pkg/utils"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

type Server struct {
	echo             *echo.Echo
	config           *config.Config
	logger           *logger.Logger
	dependencies     *registry.Dependencies
	broadcaster      *monitoring.LogBroadcaster
	infraInitManager *infrastructure.InfraInitManager
}

func New(cfg *config.Config, l *logger.Logger, b *monitoring.LogBroadcaster) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Enable GZIP compression for all responses
	e.Use(echoMiddleware.Gzip())

	// Custom HTTP Error Handler for JSON responses
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		l.Error("HTTP Error", err)

		// Handle HTTP errors with JSON response
		if he, ok := err.(*echo.HTTPError); ok {
			var message string
			code := he.Code

			// Custom message for 404 Not Found
			if code == 404 {
				message = "Endpoint not found. This incident will be reported."
				response.Error(c, code, "ENDPOINT_NOT_FOUND", message, map[string]interface{}{
					"path":   c.Request().URL.Path,
					"method": c.Request().Method,
				})
				return
			}

			// For other HTTP errors, use the original message if it's a string
			if msg, ok := he.Message.(string); ok {
				message = msg
			} else {
				message = "An unexpected error occurred"
			}
			response.Error(c, code, "HTTP_ERROR", message)
			return
		}

		// For non-HTTP errors, return internal server error
		response.InternalServerError(c, "An unexpected error occurred")
	}

	return &Server{
		echo:        e,
		config:      cfg,
		logger:      l,
		broadcaster: b,
	}
}

func (s *Server) Start() error {
	// Initialize async infrastructure manager
	s.infraInitManager = infrastructure.NewInfraInitManager(s.logger)

	// 1. Start Async Infrastructure Initialization (doesn't block)
	s.logger.Info("Starting async infrastructure initialization...")
	redisManager, kafkaManager, _, postgresConnectionManager, mongoConnectionManager, grafanaManager, cronManager :=
		s.infraInitManager.StartAsyncInitialization(s.config, s.logger)

	// Create dependencies container
	s.dependencies = registry.NewDependencies(
		redisManager,
		kafkaManager,
		nil, // Will be set from connection manager
		postgresConnectionManager,
		nil, // Will be set from connection manager
		mongoConnectionManager,
		grafanaManager,
		cronManager,
	)

	// Set default connections for backward compatibility
	if postgresConnectionManager != nil {
		if defaultConn, exists := postgresConnectionManager.GetDefaultConnection(); exists {
			s.dependencies.PostgresManager = defaultConn
		}
	}
	if mongoConnectionManager != nil {
		if defaultConn, exists := mongoConnectionManager.GetDefaultConnection(); exists {
			s.dependencies.MongoManager = defaultConn
		}
	}

	// 2. Init Middleware (synchronous, lightweight)
	s.logger.Info("Initializing Middleware...")
	middleware.InitMiddlewares(s.echo, middleware.Config{
		AuthType: s.config.Auth.Type,
		Logger:   s.logger,
	})

	// Add encryption middleware if enabled
	if s.config.Encryption.Enabled {
		s.logger.Info("Initializing Encryption Middleware...")
		s.echo.Use(middleware.EncryptionMiddleware(s.config, s.logger))
	}

	// 3. Init Services (phased: independent first, then infrastructure-dependent)
	s.logger.Info("Booting Services...")
	serviceRegistry := registry.NewServiceRegistry(s.logger)

	// Health Check Endpoint with infrastructure status
	s.echo.GET("/health", func(c echo.Context) error {
		health := map[string]interface{}{
			"status":                  "ok",
			"server_ready":            true,
			"infrastructure":          s.infraInitManager.GetStatus(),
			"initialization_progress": s.infraInitManager.GetInitializationProgress(),
		}
		return response.Success(c, health)
	})

	// Infrastructure status endpoint
	s.echo.GET("/health/infrastructure", func(c echo.Context) error {
		status := s.infraInitManager.GetStatus()
		return response.Success(c, status)
	})

	// Restart Endpoint (Maintenance)
	s.echo.POST("/restart", func(c echo.Context) error {
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(1)
		}()
		return response.Success(c, map[string]string{"status": "restarting", "message": "Service is restarting..."})
	})

	// Auto-discover and register all services
	s.logger.Info("Auto-discovering services...")
	services := registry.AutoDiscoverServices(s.config, s.logger, s.dependencies)

	// Register services with the registry
	for _, service := range services {
		serviceRegistry.Register(service)
	}

	if len(services) <= 0 {
		s.logger.Warn("No services registered!")
	}

	// Boot all services
	serviceRegistry.Boot(s.echo)
	s.logger.Info("All services boot successfully, ready to start monitoring")

	// 4. Start Monitoring (if enabled) - after all services are registered
	if s.config.Monitoring.Enabled {
		// Dynamic Service List Generation
		var servicesList []monitoring.ServiceInfo
		for _, srv := range serviceRegistry.GetServices() {
			// Prepend /api/v1 to endpoints
			var fullEndpoints []string
			for _, endp := range srv.Endpoints() {
				fullEndpoints = append(fullEndpoints, "/api/v1"+endp)
			}

			servicesList = append(servicesList, monitoring.ServiceInfo{
				Name:       srv.Name(),
				StructName: reflect.TypeOf(srv).Elem().String(),
				Active:     srv.Enabled(),
				Endpoints:  fullEndpoints,
			})
		}
		go monitoring.Start(s.config.Monitoring, s.config, s, s.broadcaster, redisManager, s.dependencies.PostgresManager, postgresConnectionManager, s.dependencies.MongoManager, mongoConnectionManager, kafkaManager, cronManager, servicesList, s.logger)
		s.logger.Info("Monitoring interface started", "port", s.config.Monitoring.Port, "services_count", len(servicesList))
	}

	// 5. Start HTTP Server immediately (doesn't wait for infrastructure)
	port := s.config.Server.Port
	s.logger.Info("HTTP server starting immediately", "port", port, "env", s.config.App.Env)
	s.logger.Info("Infrastructure components initializing in background...")

	return s.echo.Start(":" + port)
}

// GetStatus satisfies monitoring.StatusProvider
func (s *Server) GetStatus() map[string]interface{} {
	diskStats, _ := utils.GetDiskUsage()
	netStats, _ := utils.GetNetworkInfo()

	infra := map[string]bool{
		"redis":    s.config.Redis.Enabled && s.dependencies != nil && s.dependencies.RedisManager != nil,
		"kafka":    s.config.Kafka.Enabled && s.dependencies != nil && s.dependencies.KafkaManager != nil,
		"postgres": (s.config.Postgres.Enabled || s.config.PostgresMultiConfig.Enabled) && (s.dependencies != nil && s.dependencies.PostgresManager != nil),
		"mongo":    (s.config.Mongo.Enabled || s.config.MongoMultiConfig.Enabled) && (s.dependencies != nil && s.dependencies.MongoManager != nil),
		"grafana":  s.config.Grafana.Enabled && s.dependencies != nil && s.dependencies.GrafanaManager != nil,
		"cron":     s.config.Cron.Enabled && s.dependencies != nil && s.dependencies.CronManager != nil,
	}

	return map[string]interface{}{
		"version":        "1.0.0",
		"services":       s.config.Services, // Dynamic map from config
		"infrastructure": infra,
		"system": map[string]interface{}{
			"disk":    diskStats,
			"network": netStats,
		},
	}
}

// Shutdown performs graceful shutdown of all infrastructure components
func (s *Server) Shutdown(ctx context.Context, logger *logger.Logger) error {
	logger.Info("Starting graceful shutdown of infrastructure...")

	// Force shutdown when more 10s
	go func() {
		warnTimeout := "Maximum shutdown time is 20s, force shutdown when timeout."
		warnForce := "Graceful shutdown timed out, force shutdown."
		duration := 10 * time.Second

		if logger != nil {
			logger.Warn(warnTimeout)
			time.Sleep(duration)
			logger.Fatal(warnForce, nil)
		}

		fmt.Println(warnTimeout)
		time.Sleep(duration)
		os.Exit(1)

	}()

	// Stop async initialization manager
	if s.infraInitManager != nil {
		logger.Info("Stopping async infrastructure initialization manager...")
		// Note: InfraInitManager doesn't have a Close method, but we can signal completion
	}

	// Shutdown infrastructure components in reverse order of initialization
	var shutdownErrors []error

	// 1. Cron Manager
	if s.dependencies != nil && s.dependencies.CronManager != nil {
		logger.Info("Shutting down Cron Manager...")
		if err := s.dependencies.CronManager.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("cron manager shutdown error: %w", err))
			logger.Error("Error shutting down Cron Manager", err)
		} else {
			logger.Info("Cron Manager shut down successfully")
		}
	}

	// 2. MongoDB connections - need to get from connection manager
	// Note: We don't have direct access to connection managers anymore,
	// but they should be closed by the infra init manager
	logger.Info("MongoDB connections will be closed by infrastructure manager")

	// 3. PostgreSQL connections - need to get from connection manager
	// Note: We don't have direct access to connection managers anymore,
	// but they should be closed by the infra init manager
	logger.Info("PostgreSQL connections will be closed by infrastructure manager")

	// 4. Kafka Manager
	if s.dependencies != nil && s.dependencies.KafkaManager != nil {
		logger.Info("Shutting down Kafka Manager...")
		if err := s.dependencies.KafkaManager.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("kafka manager shutdown error: %w", err))
			logger.Error("Error shutting down Kafka Manager", err)
		} else {
			logger.Info("Kafka Manager shut down successfully")
		}
	}

	// 5. Redis Manager
	if s.dependencies != nil && s.dependencies.RedisManager != nil {
		logger.Info("Shutting down Redis Manager...")
		if err := s.dependencies.RedisManager.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("redis manager shutdown error: %w", err))
			logger.Error("Error shutting down Redis Manager", err)
		} else {
			logger.Info("Redis Manager shut down successfully")
		}
	}

	// Log shutdown summary
	if len(shutdownErrors) > 0 {
		logger.Warn("Graceful shutdown completed with errors", "error_count", len(shutdownErrors))
		for _, err := range shutdownErrors {
			logger.Error("Shutdown error", err)
		}
		return fmt.Errorf("shutdown completed with %d errors", len(shutdownErrors))
	} else {
		logger.Info("Graceful shutdown completed successfully")
		return nil
	}
}
