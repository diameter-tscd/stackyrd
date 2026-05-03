package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

// MiddlewareFactory creates a middleware instance
type MiddlewareFactory func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error)

// MiddlewareRegistry manages middleware auto-registration
type MiddlewareRegistry struct {
	factories map[string]MiddlewareFactory
	enabled   map[string]bool
}

// Global registry instance
var (
	globalMiddlewareRegistry *MiddlewareRegistry
	registryOnce            sync.Once
)

// GetGlobalMiddlewareRegistry returns the singleton middleware registry
func GetGlobalMiddlewareRegistry() *MiddlewareRegistry {
	registryOnce.Do(func() {
		globalMiddlewareRegistry = &MiddlewareRegistry{
			factories: make(map[string]MiddlewareFactory),
			enabled:   make(map[string]bool),
		}
	})
	return globalMiddlewareRegistry
}

// RegisterMiddleware registers a middleware factory
func RegisterMiddleware(name string, factory MiddlewareFactory) {
	GetGlobalMiddlewareRegistry().Register(name, factory)
}

// Register registers a middleware factory with the registry
func (r *MiddlewareRegistry) Register(name string, factory MiddlewareFactory) {
	r.factories[name] = factory
	// Default to enabled
	r.enabled[name] = true
}

// SetEnabled sets whether a middleware is enabled
func (r *MiddlewareRegistry) SetEnabled(name string, enabled bool) {
	r.enabled[name] = enabled
}

// IsEnabled checks if a middleware is enabled
func (r *MiddlewareRegistry) IsEnabled(name string) bool {
	if enabled, exists := r.enabled[name]; exists {
		return enabled
	}
	return true // Default to enabled if not explicitly set
}

// ApplyConfig applies middleware configuration from config
func (r *MiddlewareRegistry) ApplyConfig(cfg *config.Config) {
	if cfg.Middleware == nil {
		return
	}
	
	// Update enabled status based on config
	for name := range r.factories {
		r.enabled[name] = cfg.Middleware.IsEnabled(name)
	}
}

// AutoDiscoverMiddlewares creates and returns all enabled middleware
func (r *MiddlewareRegistry) AutoDiscoverMiddlewares(cfg *config.Config, logger *logger.Logger) []gin.HandlerFunc {
	var middlewares []gin.HandlerFunc

	for name, factory := range r.factories {
		if r.IsEnabled(name) {
			logger.Debug("Creating middleware", "name", name)
			mw, err := factory(cfg, logger)
			if err != nil {
				logger.Error("Failed to create middleware", err, "name", name)
				continue
			}
			if mw != nil {
				middlewares = append(middlewares, mw)
				logger.Info("Auto-registered middleware", "middleware", name)
			}
		} else {
			logger.Debug("Middleware disabled via config", "middleware", name)
		}
	}

	return middlewares
}

// Config holds middleware configuration
type Config struct {
	AuthType string
	Logger   *logger.Logger
}

// InitMiddlewares registers global middlewares (legacy support)
func InitMiddlewares(r *gin.Engine, cfg Config) {
	// Request ID
	r.Use(RequestID())

	// Custom Logger Middleware
	r.Use(Logger(cfg.Logger))

	// Global Permission Middleware (Allow all except DELETE for demo purposes)
	// In a real app, this might be selective
	r.Use(PermissionCheck(cfg.Logger))
}

func init() {
	// Register core middlewares
	RegisterMiddleware("request_id", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return RequestID(), nil
	})

	RegisterMiddleware("logger", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return Logger(logger), nil
	})

	RegisterMiddleware("permission_check", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return PermissionCheck(logger), nil
	})
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID if not present
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		c.Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func Logger(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		msg := fmt.Sprintf("%d | %s | %s | %v", status, method, path, latency)

		if status >= 500 {
			l.Error(msg, nil)
		} else if status >= 400 {
			l.Warn(msg)
		} else {
			l.Info(msg)
		}
	}
}

// PermissionCheck enforces "allow accept permission except data deletion"
func PermissionCheck(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// This middleware intercepts all requests.
		// "Accept permission" implies we default to allow, but strictly block generic DELETE actions
		// if they are considered "delete data".

		if c.Request.Method == http.MethodDelete {
			l.Warn("Blocked DELETE attempt due to permission policy", "path", c.Request.URL.Path, "ip", c.ClientIP())
			c.JSON(http.StatusForbidden, map[string]string{
				"error": "Permission Denied: DELETE actions are restricted.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
