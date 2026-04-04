package server

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "stackyrd/internal/services/modules"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

type Server struct {
	echo             *echo.Echo
	config           *config.Config
	logger           *logger.Logger
	dependencies     *registry.Dependencies
	infraInitManager *infrastructure.InfraInitManager
}

func New(cfg *config.Config, l *logger.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(echoMiddleware.Gzip())

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		l.Error("HTTP Error", err)

		if he, ok := err.(*echo.HTTPError); ok {
			var message string
			code := he.Code

			if code == 404 {
				message = "Endpoint not found. This incident will be reported."
				response.Error(c, code, "ENDPOINT_NOT_FOUND", message, map[string]interface{}{
					"path":   c.Request().URL.Path,
					"method": c.Request().Method,
				})
				return
			}

			if msg, ok := he.Message.(string); ok {
				message = msg
			} else {
				message = "An unexpected error occurred"
			}
			response.Error(c, code, "HTTP_ERROR", message)
			return
		}

		response.InternalServerError(c, "An unexpected error occurred")
	}

	return &Server{
		echo:   e,
		config: cfg,
		logger: l,
	}
}

func (s *Server) Start() error {
	s.infraInitManager = infrastructure.NewInfraInitManager(s.logger)

	s.logger.Info("Starting async infrastructure initialization...")
	componentRegistry := s.infraInitManager.StartAsyncInitialization(s.config, s.logger)

	// Get components from registry
	redisManager, _ := componentRegistry.Get("redis")
	kafkaManager, _ := componentRegistry.Get("kafka")
	minioManager, _ := componentRegistry.Get("minio")
	postgresManager, _ := componentRegistry.Get("postgres")
	mongoManager, _ := componentRegistry.Get("mongo")
	grafanaManager, _ := componentRegistry.Get("grafana")
	cronManager, _ := componentRegistry.Get("cron")

	// Type assert to get the concrete types
	var redisMgr *infrastructure.RedisManager
	var kafkaMgr *infrastructure.KafkaManager
	var minioMgr *infrastructure.MinIOManager
	var postgresConnMgr *infrastructure.PostgresConnectionManager
	var mongoConnMgr *infrastructure.MongoConnectionManager
	var grafanaMgr *infrastructure.GrafanaManager
	var cronMgr *infrastructure.CronManager

	if rm, ok := redisManager.(*infrastructure.RedisManager); ok {
		redisMgr = rm
	}
	if km, ok := kafkaManager.(*infrastructure.KafkaManager); ok {
		kafkaMgr = km
	}
	if mm, ok := minioManager.(*infrastructure.MinIOManager); ok {
		minioMgr = mm
	}
	if pm, ok := postgresManager.(*infrastructure.PostgresConnectionManager); ok {
		postgresConnMgr = pm
	} else if _, ok := postgresManager.(*infrastructure.PostgresManager); ok {
		// Handle single connection case
		s.logger.Info("PostgreSQL single connection manager detected")
	}
	if mm, ok := mongoManager.(*infrastructure.MongoConnectionManager); ok {
		mongoConnMgr = mm
	} else if _, ok := mongoManager.(*infrastructure.MongoManager); ok {
		// Handle single connection case
		s.logger.Info("MongoDB single connection manager detected")
	}
	if gm, ok := grafanaManager.(*infrastructure.GrafanaManager); ok {
		grafanaMgr = gm
	}
	if cm, ok := cronManager.(*infrastructure.CronManager); ok {
		cronMgr = cm
	}

	s.dependencies = registry.NewDependencies(
		redisMgr,
		kafkaMgr,
		nil,
		postgresConnMgr,
		nil,
		mongoConnMgr,
		grafanaMgr,
		cronMgr,
		minioMgr,
	)

	s.setConnectionDefaults(postgresConnMgr, mongoConnMgr)

	s.logger.Info("Initializing Middleware...")
	middleware.InitMiddlewares(s.echo, middleware.Config{
		AuthType: s.config.Auth.Type,
		Logger:   s.logger,
	})

	if s.config.Encryption.Enabled {
		s.logger.Info("Initializing Encryption Middleware...")
		s.echo.Use(middleware.EncryptionMiddleware(s.config, s.logger))
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

	serviceRegistry.Boot(s.echo)
	s.logger.Info("All services boot successfully")

	port := s.config.Server.Port
	s.logger.Info("HTTP server starting immediately", "port", port, "env", s.config.App.Env)
	s.logger.Info("Infrastructure components initializing in background...")

	return s.echo.Start(":" + port)
}

func (s *Server) setConnectionDefaults(postgresConnectionManager *infrastructure.PostgresConnectionManager, mongoConnectionManager *infrastructure.MongoConnectionManager) {
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
}

func (s *Server) registerHealthEndpoints() {
	s.echo.GET("/health", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{
			"status":                  "ok",
			"server_ready":            true,
			"infrastructure":          s.infraInitManager.GetStatus(),
			"initialization_progress": s.infraInitManager.GetInitializationProgress(),
		})
	})

	s.echo.GET("/health/infrastructure", func(c echo.Context) error {
		return response.Success(c, s.infraInitManager.GetStatus())
	})

	s.echo.POST("/restart", func(c echo.Context) error {
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(1)
		}()
		return response.Success(c, map[string]string{"status": "restarting", "message": "Service is restarting..."})
	})
}

// Shutdown performs graceful shutdown of all infrastructure components
func (s *Server) Shutdown(ctx context.Context, logger *logger.Logger) error {
	logger.Info("Starting graceful shutdown of infrastructure...")

	go func() {
		time.Sleep(10 * time.Second)
		if logger != nil {
			logger.Warn("Maximum shutdown time is 20s, force shutdown when timeout.")
			logger.Fatal("Graceful shutdown timed out, force shutdown.", nil)
		}
		fmt.Println("Maximum shutdown time is 20s, force shutdown when timeout.")
		os.Exit(1)
	}()

	if s.infraInitManager != nil {
		logger.Info("Stopping async infrastructure initialization manager...")
	}

	var shutdownErrors []error

	shutdownComponent := func(name string, closer interface{}) {
		if closer == nil {
			return
		}
		logger.Info("Shutting down " + name + "...")
		if closerCloser, ok := closer.(interface{ Close() error }); ok {
			if err := closerCloser.Close(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("%s shutdown error: %w", name, err))
				logger.Error("Error shutting down "+name, err)
			} else {
				logger.Info(name + " shut down successfully")
			}
		}
	}

	shutdownComponent("Cron Manager", s.dependencies.CronManager)
	logger.Info("MongoDB connections will be closed by infrastructure manager")
	logger.Info("PostgreSQL connections will be closed by infrastructure manager")
	shutdownComponent("Kafka Manager", s.dependencies.KafkaManager)
	shutdownComponent("Redis Manager", s.dependencies.RedisManager)

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
