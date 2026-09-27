package middleware

import (
	"fmt"
	"net/http"
	"runtime"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("recovery", func(cfg *config.Config, log *logger.Logger) (echo.MiddlewareFunc, error) {
		response.SetTraceDebug(cfg.App.Debug)
		return Recovery(log), nil
	})
}

// Recovery converts panics into traced JSON errors (same envelope as
// response.TraceError) instead of Echo's default empty 500.
func Recovery(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := make([]byte, 4096)
					n := runtime.Stack(stack, false)
					stack = stack[:n]

					traceID := response.GetCorrelationID(c)
					if traceID == "" {
						traceID = c.Response().Header().Get("X-Request-ID")
					}
					endpoint := c.Path()
					if endpoint == "" {
						endpoint = c.Request().URL.Path
					}
					l.Error("panic recovered",
						fmt.Errorf("%v", rec),
						"trace_id", traceID,
						"endpoint", endpoint,
						"method", c.Request().Method,
						"path", c.Request().URL.Path,
						"stack", string(stack),
					)
					err = response.TracePanic(c, rec, stack, endpoint, "")
				}
			}()
			return next(c)
		}
	}
}

// HTTPErrorHandler renders Echo errors (404/405/bind failures returned as
// *echo.HTTPError) through the traced envelope so every failure carries
// endpoint + trace_id + kind.
func HTTPErrorHandler(l *logger.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := http.StatusInternalServerError
		msg := "Internal server error"
		if he, ok := err.(*echo.HTTPError); ok {
			if he.Code != 0 {
				code = he.Code
			}
			if m, ok := he.Message.(string); ok && m != "" {
				msg = m
			} else if he.Message != nil {
				msg = fmt.Sprintf("%v", he.Message)
			}
		} else if err != nil {
			msg = err.Error()
		}
		endpoint := c.Path()
		if endpoint == "" && c.Request() != nil && c.Request().URL != nil {
			endpoint = c.Request().URL.Path
		}
		method, path := "", ""
		if c.Request() != nil {
			method = c.Request().Method
			if c.Request().URL != nil {
				path = c.Request().URL.Path
			}
		}
		traceID := response.GetCorrelationID(c)
		if traceID == "" {
			traceID = c.Response().Header().Get("X-Request-ID")
		}
		if code >= 500 {
			l.Error("http error", err, "trace_id", traceID, "endpoint", endpoint, "method", method, "path", path, "status", code)
		} else {
			l.Warn("http error", "trace_id", traceID, "endpoint", endpoint, "method", method, "path", path, "status", code)
		}
		_ = response.TraceError(c, code, err, endpoint, "")
		_ = msg
	}
}
