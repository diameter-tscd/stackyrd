package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

func init() {
	// Register CORS middleware
	RegisterMiddleware("cors", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		return CORSAllowAll(), nil
	})
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// Default CORS configuration
var defaultCORSConfig = CORSConfig{
	AllowOrigins:     []string{"*"},
	AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
	AllowCredentials: true,
	MaxAge:           86400,
}

// CORSAllowAll enables CORS for all origins
func CORSAllowAll() gin.HandlerFunc {
	return CORS(defaultCORSConfig)
}

// CORSWithConfig enables CORS with specific origins
func CORSWithConfig(allowOrigins []string) gin.HandlerFunc {
	config := defaultCORSConfig
	config.AllowOrigins = allowOrigins
	return CORS(config)
}

// CORS enables CORS with full configuration
func CORS(config CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range config.AllowOrigins {
			if o == "*" || o == origin || matchSubdomain(o, origin) {
				allowed = true
				break
			}
		}

		if !allowed {
			c.Next()
			return
		}

		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Vary", "Origin")

		if config.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))

		if config.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}

		// Handle preflight request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// matchSubdomain checks if a wildcard subdomain pattern matches the origin
func matchSubdomain(pattern, origin string) bool {
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	suffix := pattern[1:] // Remove the *
	return strings.HasSuffix(origin, suffix)
}
