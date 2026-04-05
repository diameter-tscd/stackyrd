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

	"github.com/gin-gonic/gin"
)

// EncryptionMiddleware provides API encryption/obfuscation
func EncryptionMiddleware(cfg *config.Config, l *logger.Logger) gin.HandlerFunc {
	if !cfg.Encryption.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// Wrap the response writer to intercept the response
		w := &encryptionResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			config:         cfg,
			logger:         l,
		}
		c.Writer = w

		c.Next()

		// The response has been written to w.body
		// We need to write it back through the original writer
		if w.body.Len() > 0 {
			// Check if the response is JSON
			contentType := c.Writer.Header().Get("Content-Type")
			if strings.Contains(contentType, "application/json") {
				// Apply obfuscation (base64 encoding for demo)
				encoded := base64.StdEncoding.EncodeToString(w.body.Bytes())
				c.Writer.Header().Set("X-Obfuscated", "true")
				c.Writer.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
				c.Writer.WriteHeaderNow()
				c.Writer.Write([]byte(encoded))
			} else {
				// Pass through non-JSON responses
				c.Writer.WriteHeaderNow()
				c.Writer.Write(w.body.Bytes())
			}
		}
	}
}

type encryptionResponseWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	config *config.Config
	logger *logger.Logger
	once   sync.Once
}

func (w *encryptionResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *encryptionResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *encryptionResponseWriter) WriteHeaderNow() {
	w.ResponseWriter.WriteHeaderNow()
}

func (w *encryptionResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *encryptionResponseWriter) Status() int {
	return w.ResponseWriter.Status()
}

// GzipMiddleware provides GZIP compression for responses
func GzipMiddleware() gin.HandlerFunc {
	var gzPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}

	return func(c *gin.Context) {
		// Check if client accepts gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		w := c.Writer

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		gz := gzPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			gz.Close()
			gzPool.Put(gz)
		}()

		// Wrap the writer
		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			Writer:         gz,
		}
		c.Writer = gzw

		c.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) WriteHeaderNow() {
	w.ResponseWriter.WriteHeaderNow()
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *gzipResponseWriter) Status() int {
	return w.ResponseWriter.Status()
}
