package middleware

import (
	"fmt"
	"net/http"
	"time"

	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Config holds middleware configuration
type Config struct {
	AuthType string
	Logger   *logger.Logger
}

// InitMiddlewares registers global middlewares and returns specific ones for use
func InitMiddlewares(r *gin.Engine, cfg Config) {
	// Request ID
	r.Use(RequestID())

	// Custom Logger Middleware
	r.Use(Logger(cfg.Logger))

	// Global Permission Middleware (Allow all except DELETE for demo purposes)
	// In a real app, this might be selective
	r.Use(PermissionCheck(cfg.Logger))
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
