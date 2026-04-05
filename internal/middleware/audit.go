package middleware

import (
	"time"

	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	Logger           *logger.Logger
	LogRequestBody   bool
	LogHeaders       bool
	SensitiveHeaders []string
	SkipPaths        []string
}

// Default audit configuration
var defaultAuditConfig = AuditConfig{
	LogRequestBody:   false,
	LogHeaders:       false,
	SensitiveHeaders: []string{"Authorization", "Cookie", "Set-Cookie"},
	SkipPaths:        []string{"/health", "/health/infrastructure"},
}

// AuditWithConfig creates audit logging middleware with custom configuration
func AuditWithConfig(l *logger.Logger) gin.HandlerFunc {
	return Audit(defaultAuditConfig, l)
}

// AuditSkipHealthCheck creates audit logging middleware that skips health check endpoints
func AuditSkipHealthCheck(l *logger.Logger) gin.HandlerFunc {
	config := defaultAuditConfig
	config.Logger = l
	return Audit(config, l)
}

// Audit creates audit logging middleware
func Audit(config AuditConfig, l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip configured paths
		for _, path := range config.SkipPaths {
			if c.Request.URL.Path == path {
				c.Next()
				return
			}
		}

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		// Build log fields
		fields := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"status":     statusCode,
			"latency":    latency.String(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
			"request_id": c.Writer.Header().Get("X-Request-ID"),
		}

		// Add user info if available
		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}
		if username, exists := c.Get("username"); exists {
			fields["username"] = username
		}

		// Log headers if configured
		if config.LogHeaders {
			headers := make(map[string]string)
			for name, values := range c.Request.Header {
				// Skip sensitive headers
				skip := false
				for _, sensitive := range config.SensitiveHeaders {
					if name == sensitive {
						skip = true
						break
					}
				}
				if !skip {
					for _, v := range values {
						headers[name] = v
					}
				}
			}
			fields["headers"] = headers
		}

		// Convert fields map to keyvals slice
		keyvals := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			keyvals = append(keyvals, k, v)
		}

		// Log with appropriate level based on status code
		if statusCode >= 500 {
			l.Error("API Request", nil, keyvals...)
		} else if statusCode >= 400 {
			l.Warn("API Request", keyvals...)
		} else {
			l.Info("API Request", keyvals...)
		}
	}
}
