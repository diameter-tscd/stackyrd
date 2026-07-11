package middleware

import (
	"net/http"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/metrics"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("prometheus", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return Prometheus(), nil
	})
}

// Prometheus records HTTP request metrics (count, duration, sizes) with a
// bounded label set. The route template from c.Path() (e.g. "/users/:id")
// is used instead of the raw URL path, so cardinality stays bounded no matter
// how many distinct IDs flow through.
func Prometheus() echo.MiddlewareFunc {
	m := metrics.GetMetrics()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			status := c.Response().Status
			if status == 0 {
				status = http.StatusOK
			}

			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}

			var reqSize int64
			if c.Request().ContentLength > 0 {
				reqSize = c.Request().ContentLength
			}

			m.RecordHTTPRequest(c.Request().Method, path, status, latency, reqSize, c.Response().Size)
			return err
		}
	}
}
