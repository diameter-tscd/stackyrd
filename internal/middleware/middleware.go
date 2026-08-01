package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"stackyrd/config"
	"stackyrd/pkg/logger"
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

func (r *MiddlewareRegistry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
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

var reqIDPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Request().Header.Get("X-Request-ID")
			// Only honor well-formed client-supplied IDs; anything else gets a
			// fresh server-generated ID so headers/logs can't be poisoned.
			if requestID != "" && !validRequestID(requestID) {
				requestID = ""
			}
			if requestID == "" {
				b := reqIDPool.Get().(*strings.Builder)
				b.Reset()
				b.WriteString("req-")
				b.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
				requestID = b.String()
				reqIDPool.Put(b)
			}
			c.Set("X-Request-ID", requestID)
			c.Response().Header().Set("X-Request-ID", requestID)
			return next(c)
		}
	}
}

// validRequestID restricts client-supplied request IDs to a safe charset and
// length, preventing header injection and unbounded log entries.
func validRequestID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
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

			if status >= 500 {
				l.Error("request failed", nil, "status", status, "method", method, "path", path, "latency", latency.String())
			} else if status >= 400 {
				l.Warn("request", "status", status, "method", method, "path", path, "latency", latency.String())
			} else {
				l.Info("request", "status", status, "method", method, "path", path, "latency", latency.String())
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
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	return path == pattern
}
