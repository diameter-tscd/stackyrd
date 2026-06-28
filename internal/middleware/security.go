package middleware

import (
	"fmt"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("security", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return Security(), nil
	})
}

type SecurityConfig struct {
	ContentSecurityPolicy         string
	XContentTypeOptions           string
	XFrameOptions                 string
	XXSSProtection                string
	ReferrerPolicy                string
	PermissionsPolicy             string
	StrictTransportSecurity       string
	StrictTransportSecurityMaxAge int
}

var defaultSecurityConfig = SecurityConfig{
	ContentSecurityPolicy:         "default-src 'self'",
	XContentTypeOptions:           "nosniff",
	XFrameOptions:                 "DENY",
	XXSSProtection:                "1; mode=block",
	ReferrerPolicy:                "strict-origin-when-cross-origin",
	PermissionsPolicy:             "camera=(), microphone=(), geolocation=()",
	StrictTransportSecurity:       "max-age=%d; includeSubDomains",
	StrictTransportSecurityMaxAge: 31536000,
}

func Security() echo.MiddlewareFunc {
	return SecurityWithConfig(defaultSecurityConfig)
}

func SecurityWithConfig(config SecurityConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			c.Response().Header().Set("X-Content-Type-Options", config.XContentTypeOptions)
			c.Response().Header().Set("X-Frame-Options", config.XFrameOptions)
			c.Response().Header().Set("X-XSS-Protection", config.XXSSProtection)
			c.Response().Header().Set("Referrer-Policy", config.ReferrerPolicy)
			c.Response().Header().Set("Permissions-Policy", config.PermissionsPolicy)
			c.Response().Header().Set("Strict-Transport-Security",
				fmt.Sprintf(config.StrictTransportSecurity, config.StrictTransportSecurityMaxAge))

			return next(c)
		}
	}
}

func SecurityPermissive() echo.MiddlewareFunc {
	return SecurityWithConfig(SecurityConfig{
		ContentSecurityPolicy:         "default-src 'self' 'unsafe-inline' 'unsafe-eval'",
		XContentTypeOptions:           "nosniff",
		XFrameOptions:                 "SAMEORIGIN",
		XXSSProtection:                "0",
		ReferrerPolicy:                "no-referrer",
		PermissionsPolicy:             "",
		StrictTransportSecurity:       "",
		StrictTransportSecurityMaxAge: 0,
	})
}
