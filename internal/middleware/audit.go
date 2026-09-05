package middleware

import (
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("audit", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		ac := defaultAuditConfig
		ac.Logger = logger
		if len(cfg.Audit.SkipPaths) > 0 {
			ac.SkipPaths = cfg.Audit.SkipPaths
		}
		return Audit(ac, logger), nil
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
	skipSet := make(map[string]struct{}, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipSet[p] = struct{}{}
	}
	sensitiveSet := make(map[string]struct{}, len(config.SensitiveHeaders))
	for _, h := range config.SensitiveHeaders {
		sensitiveSet[h] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, ok := skipSet[c.Request().URL.Path]; ok {
				return next(c)
			}

			start := time.Now()
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery

			err := next(c)

			latency := time.Since(start)
			statusCode := c.Response().Status

			var kv [24]any
			n := 0
			kv[n] = "method"; kv[n+1] = c.Request().Method; n += 2
			kv[n] = "path"; kv[n+1] = path; n += 2
			kv[n] = "query"; kv[n+1] = query; n += 2
			kv[n] = "status"; kv[n+1] = statusCode; n += 2
			kv[n] = "latency"; kv[n+1] = latency.String(); n += 2
			kv[n] = "client_ip"; kv[n+1] = c.RealIP(); n += 2
			kv[n] = "user_agent"; kv[n+1] = c.Request().UserAgent(); n += 2
			kv[n] = "request_id"; kv[n+1] = c.Response().Header().Get("X-Request-ID"); n += 2
			if userID := c.Get("user_id"); userID != nil {
				kv[n] = "user_id"; kv[n+1] = userID; n += 2
			}
			if username := c.Get("username"); username != nil {
				kv[n] = "username"; kv[n+1] = username; n += 2
			}
			if config.LogHeaders {
				headers := make(map[string]string, len(c.Request().Header))
				for name, values := range c.Request().Header {
					if _, skip := sensitiveSet[name]; skip {
						continue
					}
					headers[name] = strings.Join(values, ",")
				}
				kv[n] = "headers"; kv[n+1] = headers; n += 2
			}

			if statusCode >= 500 {
				l.Error("API Request", nil, kv[:n]...)
			} else if statusCode >= 400 {
				l.Warn("API Request", kv[:n]...)
			} else {
				l.Info("API Request", kv[:n]...)
			}

			return err
		}
	}
}
