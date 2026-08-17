package middleware

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
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
		New: func() any {
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
	// A bodyless response must not carry Content-Encoding: gzip.
	if statusCode >= 100 && statusCode < 200 ||
		statusCode == http.StatusNoContent || statusCode == http.StatusNotModified {
		w.ResponseWriter.Header().Del("Content-Encoding")
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Flush keeps streaming/SSE responses from stalling: flush the gzip buffer,
// then the underlying writer.
func (w *gzipResponseWriter) Flush() {
	if f, ok := w.Writer.(http.Flusher); ok {
		f.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets WebSocket upgrades pass through the gzip wrapper.
func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Unwrap restores the underlying writer for http.ResponseController.
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
