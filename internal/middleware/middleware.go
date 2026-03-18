package middleware

import (
	"fmt"
	"net/http"
	"time"

	"stackyard/pkg/logger"

	"github.com/labstack/echo/v4"
)

// Config holds middleware configuration
type Config struct {
	AuthType string
	Logger   *logger.Logger
}

// InitMiddlewares registers global middlewares and returns specific ones for use
func InitMiddlewares(e *echo.Echo, cfg Config) {
	// Request ID
	e.Use(RequestID())

	// Custom Logger Middleware
	e.Use(Logger(cfg.Logger))

	// Global Permission Middleware (Allow all except DELETE for demo purposes)
	// In a real app, this might be selective
	e.Use(PermissionCheck(cfg.Logger))
}

func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Basic implementation, Echo has its own middleware.RequestID() too
			return next(c)
		}
	}
}

func Logger(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			status := res.Status
			method := req.Method
			path := req.URL.Path
			latency := time.Since(start)

			msg := fmt.Sprintf("%d | %s | %s | %v", status, method, path, latency)

			if status >= 500 {
				l.Error(msg, err)
			} else if status >= 400 {
				l.Warn(msg)
			} else {
				l.Info(msg)
			}
			return err
		}
	}
}

// PermissionCheck enforces "allow accept permission except data deletion"
func PermissionCheck(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// This middleware intercepts all requests.
			// "Accept permission" implies we default to allow, but strictly block generic DELETE actions
			// if they are considered "delete data".

			if c.Request().Method == http.MethodDelete {
				l.Warn("Blocked DELETE attempt due to permission policy", "path", c.Request().URL.Path, "ip", c.RealIP())
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "Permission Denied: DELETE actions are restricted.",
				})
			}

			// For other methods (GET, POST, PUT, PATCH), we "accept permission" (proceed).
			return next(c)
		}
	}
}
