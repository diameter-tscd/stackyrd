package middleware

import (
	"fmt"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

func init() {
	// Register Security middleware
	RegisterMiddleware("security", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return Security(), nil
	})
}

// SecurityConfig holds security headers configuration
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

// Default security configuration
var defaultSecurityConfig = SecurityConfig{
	ContentSecurityPolicy:         "default-src 'self'",
	XContentTypeOptions:           "nosniff",
	XFrameOptions:                 "DENY",
	XXSSProtection:                "1; mode=block",
	ReferrerPolicy:                "strict-origin-when-cross-origin",
	PermissionsPolicy:             "camera=(), microphone=(), geolocation=()",
	StrictTransportSecurity:       "max-age=%d; includeSubDomains",
	StrictTransportSecurityMaxAge: 31536000, // 1 year
}

// Security middleware with default strict settings
func Security() gin.HandlerFunc {
	return SecurityWithConfig(defaultSecurityConfig)
}

// SecurityWithConfig middleware with custom settings
func SecurityWithConfig(config SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
		c.Writer.Header().Set("X-Content-Type-Options", config.XContentTypeOptions)
		c.Writer.Header().Set("X-Frame-Options", config.XFrameOptions)
		c.Writer.Header().Set("X-XSS-Protection", config.XXSSProtection)
		c.Writer.Header().Set("Referrer-Policy", config.ReferrerPolicy)
		c.Writer.Header().Set("Permissions-Policy", config.PermissionsPolicy)
		c.Writer.Header().Set("Strict-Transport-Security",
			fmt.Sprintf(config.StrictTransportSecurity, config.StrictTransportSecurityMaxAge))

		c.Next()
	}
}

// SecurityPermissive middleware for development environments
func SecurityPermissive() gin.HandlerFunc {
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
