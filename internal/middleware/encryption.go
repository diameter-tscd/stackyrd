package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

func init() {
	RegisterMiddleware("encryption", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return EncryptionMiddleware(cfg, logger), nil
	})

	RegisterMiddleware("gzip", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		return GzipMiddleware(), nil
	})
}

func EncryptionMiddleware(cfg *config.Config, l *logger.Logger) echo.MiddlewareFunc {
	if !cfg.Encryption.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return next(c)
			}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			w := &encryptionResponseWriter{
				ResponseWriter: c.Response().Writer,
				body:           &bytes.Buffer{},
				config:         cfg,
				logger:         l,
			}
			c.Response().Writer = w

			err := next(c)

			if w.body.Len() > 0 {
				contentType := c.Response().Header().Get("Content-Type")
				if strings.Contains(contentType, "application/json") {
					encoded := base64.StdEncoding.EncodeToString(w.body.Bytes())
					c.Response().Header().Set("X-Obfuscated", "true")
					c.Response().Header().Set("Content-Length", strconv.Itoa(len(encoded)))
					c.Response().WriteHeader(w.statusCode)
					_, _ = c.Response().Writer.Write([]byte(encoded))
				} else {
					c.Response().WriteHeader(w.statusCode)
					_, _ = c.Response().Writer.Write(w.body.Bytes())
				}
			}

			return err
		}
	}
}

type encryptionResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	config     *config.Config
	logger     *logger.Logger
	once       sync.Once
	statusCode int
}

func (w *encryptionResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *encryptionResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
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
