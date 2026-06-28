package middleware

import (
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("audit", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return AuditWithConfig(logger), nil
	})
}

type AuditConfig struct {
	Logger           *logger.Logger
	LogRequestBody   bool
	LogHeaders       bool
	SensitiveHeaders []string
	SkipPaths        []string
}

var defaultAuditConfig = AuditConfig{
	LogRequestBody:   false,
	LogHeaders:       false,
	SensitiveHeaders: []string{"Authorization", "Cookie", "Set-Cookie"},
	SkipPaths:        []string{"/health", "/health/infrastructure"},
}

func AuditWithConfig(l *logger.Logger) echo.MiddlewareFunc {
	return Audit(defaultAuditConfig, l)
}

func AuditSkipHealthCheck(l *logger.Logger) echo.MiddlewareFunc {
	config := defaultAuditConfig
	config.Logger = l
	return Audit(config, l)
}

func Audit(config AuditConfig, l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			for _, path := range config.SkipPaths {
				if c.Request().URL.Path == path {
					return next(c)
				}
			}

			start := time.Now()
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery

			err := next(c)

			latency := time.Since(start)
			statusCode := c.Response().Status

			fields := map[string]interface{}{
				"method":     c.Request().Method,
				"path":       path,
				"query":      query,
				"status":     statusCode,
				"latency":    latency.String(),
				"client_ip":  c.RealIP(),
				"user_agent": c.Request().UserAgent(),
				"request_id": c.Response().Header().Get("X-Request-ID"),
			}

			if userID := c.Get("user_id"); userID != nil {
				fields["user_id"] = userID
			}
			if username := c.Get("username"); username != nil {
				fields["username"] = username
			}

			if config.LogHeaders {
				headers := make(map[string]string)
				for name, values := range c.Request().Header {
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

			keyvals := make([]interface{}, 0, len(fields)*2)
			for k, v := range fields {
				keyvals = append(keyvals, k, v)
			}

			if statusCode >= 500 {
				l.Error("API Request", nil, keyvals...)
			} else if statusCode >= 400 {
				l.Warn("API Request", keyvals...)
			} else {
				l.Info("API Request", keyvals...)
			}

			return err
		}
	}
}
