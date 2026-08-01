package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("gzip", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return GzipMiddleware(), nil
	})
}

func GzipMiddleware() echo.MiddlewareFunc {
	var gzPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !strings.Contains(c.Request().Header.Get("Accept-Encoding"), "gzip") {
				return next(c)
			}

			w := c.Response().Writer
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			gz := gzPool.Get().(*gzip.Writer)
			gz.Reset(w)
			defer func() {
				_ = gz.Close()
				gzPool.Put(gz)
			}()

			gzw := &gzipResponseWriter{
				ResponseWriter: w,
				Writer:         gz,
			}
			c.Response().Writer = gzw

			return next(c)
		}
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}
