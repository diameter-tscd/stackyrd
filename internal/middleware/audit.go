package middleware

import (
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

func init() {
	// Register Audit middleware
	RegisterMiddleware("audit", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return AuditWithConfig(logger), nil
	})
}

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
	skipPathsSet := make(map[string]struct{}, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipPathsSet[p] = struct{}{}
	}
	sensitiveHeadersSet := make(map[string]struct{}, len(config.SensitiveHeaders))
	for _, h := range config.SensitiveHeaders {
		sensitiveHeadersSet[h] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, skip := skipPathsSet[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		keyvals := make([]interface{}, 0, 20)
		keyvals = append(keyvals,
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"status", statusCode,
			"latency", latency,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"request_id", c.Writer.Header().Get("X-Request-ID"),
		)

		if userID, exists := c.Get("user_id"); exists {
			keyvals = append(keyvals, "user_id", userID)
		}
		if username, exists := c.Get("username"); exists {
			keyvals = append(keyvals, "username", username)
		}

		if config.LogHeaders {
			headers := make(map[string]string)
			for name, values := range c.Request.Header {
				if _, skip := sensitiveHeadersSet[name]; !skip {
					for _, v := range values {
						headers[name] = v
					}
				}
			}
			keyvals = append(keyvals, "headers", headers)
		}

		if statusCode >= 500 {
			l.Error("API Request", nil, keyvals...)
		} else if statusCode >= 400 {
			l.Warn("API Request", keyvals...)
		} else {
			l.Info("API Request", keyvals...)
		}
	}
}
