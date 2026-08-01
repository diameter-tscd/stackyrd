package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("cors", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return CORSAllowAll(), nil
	})
}

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

var defaultCORSConfig = CORSConfig{
	AllowOrigins:     []string{"*"},
	AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
	AllowCredentials: false,
	MaxAge:           86400,
}

func CORSAllowAll() echo.MiddlewareFunc {
	return CORS(defaultCORSConfig)
}

func CORSWithConfig(allowOrigins []string) echo.MiddlewareFunc {
	config := defaultCORSConfig
	config.AllowOrigins = allowOrigins
	return CORS(config)
}

func originAllowed(allowOrigins []string, origin string) bool {
	for _, o := range allowOrigins {
		if o == "*" || o == origin || matchSubdomain(o, origin) {
			return true
		}
	}
	return false
}

func hasWildcard(allowOrigins []string) bool {
	for _, o := range allowOrigins {
		if o == "*" {
			return true
		}
	}
	return false
}

func CORS(config CORSConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")

			if !originAllowed(config.AllowOrigins, origin) {
				return next(c)
			}

			// Credentialed CORS must never combine with the "*" origin per the
			// Fetch spec: an explicit, allow-listed origin is required instead.
			if config.AllowCredentials {
				if origin == "" || hasWildcard(config.AllowOrigins) {
					return next(c)
				}
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
				c.Response().Header().Set("Vary", "Origin")
				c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			} else if hasWildcard(config.AllowOrigins) {
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
				c.Response().Header().Set("Vary", "Origin")
			}

			c.Response().Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
			c.Response().Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))

			if config.MaxAge > 0 {
				c.Response().Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
			}

			if c.Request().Method == "OPTIONS" {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

func matchSubdomain(pattern, origin string) bool {
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	suffix := pattern[1:]
	return strings.HasSuffix(origin, suffix)
}
