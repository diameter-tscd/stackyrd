package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type MiddlewareFactory func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error)

type MiddlewareRegistry struct {
	mu        sync.RWMutex
	factories map[string]MiddlewareFactory
	enabled   map[string]bool
}

var (
	globalMiddlewareRegistry *MiddlewareRegistry
	registryOnce             sync.Once
)

func GetGlobalMiddlewareRegistry() *MiddlewareRegistry {
	registryOnce.Do(func() {
		globalMiddlewareRegistry = &MiddlewareRegistry{
			factories: make(map[string]MiddlewareFactory),
			enabled:   make(map[string]bool),
		}
	})
	return globalMiddlewareRegistry
}

func RegisterMiddleware(name string, factory MiddlewareFactory) {
	GetGlobalMiddlewareRegistry().Register(name, factory)
}

func (r *MiddlewareRegistry) Register(name string, factory MiddlewareFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
	r.enabled[name] = true
}

func (r *MiddlewareRegistry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[name] = enabled
}

func (r *MiddlewareRegistry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if enabled, exists := r.enabled[name]; exists {
		return enabled
	}
	return true
}

func (r *MiddlewareRegistry) ApplyConfig(cfg *config.Config) {
	if cfg.Middleware == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for name := range r.factories {
		r.enabled[name] = cfg.Middleware.IsEnabled(name)
	}
}

func (r *MiddlewareRegistry) AutoDiscoverMiddlewares(cfg *config.Config, logger *logger.Logger) []echo.MiddlewareFunc {
	var middlewares []echo.MiddlewareFunc

	r.mu.RLock()
	defer r.mu.RUnlock()

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

type Config struct {
	AuthType string
	Logger   *logger.Logger
}

func InitMiddlewares(e *echo.Echo, cfg Config) {
	e.Use(RequestID())
	e.Use(Logger(cfg.Logger))
	e.Use(PermissionCheck(cfg.Logger))
}

func init() {
	RegisterMiddleware("request_id", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return RequestID(), nil
	})

	RegisterMiddleware("logger", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return Logger(logger), nil
	})

	RegisterMiddleware("permission_check", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		blockedMethods := viper.GetStringSlice("middleware.permission_check.blocked_methods")
		blockedPaths := viper.GetStringSlice("middleware.permission_check.blocked_paths")
		if len(blockedMethods) == 0 {
			blockedMethods = []string{http.MethodDelete}
		}
		logger.Debug("PermissionCheck configured", "blocked_methods", blockedMethods, "blocked_paths", blockedPaths)
		return PermissionCheckWithConfig(logger, blockedMethods, blockedPaths), nil
	})
}

func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Request().Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = "req-" + strconv.FormatInt(time.Now().UnixNano(), 10)
			}
			c.Set("X-Request-ID", requestID)
			c.Response().Header().Set("X-Request-ID", requestID)
			return next(c)
		}
	}
}

func Logger(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			latency := time.Since(start)
			status := c.Response().Status
			method := c.Request().Method
			path := c.Request().URL.Path

			msg := strconv.Itoa(status) + " | " + method + " | " + path + " | " + latency.String()

			if status >= 500 {
				l.Error(msg, nil)
			} else if status >= 400 {
				l.Warn(msg)
			} else {
				l.Info(msg)
			}

			return err
		}
	}
}

func PermissionCheck(l *logger.Logger) echo.MiddlewareFunc {
	return PermissionCheckWithConfig(l, []string{http.MethodDelete}, nil)
}

func PermissionCheckWithConfig(l *logger.Logger, blockedMethods []string, blockedPaths []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			path := c.Request().URL.Path

			for _, m := range blockedMethods {
				if method != m {
					continue
				}

				if len(blockedPaths) == 0 {
					l.Warn("Blocked request due to permission policy", "method", method, "path", path, "ip", c.RealIP())
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": "Permission Denied: " + method + " actions are restricted.",
					})
				}

				for _, p := range blockedPaths {
					if matchPath(path, p) {
						l.Warn("Blocked request due to permission policy", "method", method, "path", path, "ip", c.RealIP())
						return c.JSON(http.StatusForbidden, map[string]string{
							"error": "Permission Denied: " + method + " on " + p + " is restricted.",
						})
					}
				}
			}

			return next(c)
		}
	}
}

func matchPath(path, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	n := len(pattern)
	if n > 0 && pattern[n-1] == '*' {
		return len(path) >= n-1 && path[:n-1] == pattern[:n-1]
	}
	return path == pattern
}
