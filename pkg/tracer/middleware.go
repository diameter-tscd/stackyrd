package tracer

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

// TraceMiddleware returns a middleware that creates a span for each request
func TraceMiddleware(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if tracer := GetTracer(); tracer == nil || !tracer.Enabled() {
				return next(c)
			}

			// Extract trace context from headers (for distributed tracing)
			var ctx context.Context
			if traceparent := c.Request().Header.Get("traceparent"); traceparent != "" {
				prop := otel.GetTextMapPropagator()
				ctx = prop.Extract(context.Background(), propagation.HeaderCarrier{
					"traceparent": []string{traceparent},
				})
			} else {
				ctx = context.Background()
			}

			// Add request attributes to context
			ctx = trace.ContextWithSpan(ctx, trace.SpanFromContext(ctx))

			// Start server span
			spanName := c.Request().Method + " " + c.Request().URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// Add attributes
			span.SetAttributes(
				attribute.String("http.method", c.Request().Method),
				attribute.String("http.url", c.Request().URL.String()),
				attribute.String("http.path", c.Request().URL.Path),
				attribute.String("http.host", c.Request().Host),
				attribute.String("http.scheme", c.Request().URL.Scheme),
				attribute.Int("http.status_code", 0), // Will be updated after request
				attribute.String("net.peer.ip", c.RealIP()),
				attribute.String("user_agent", c.Request().UserAgent()),
			)

			// Store span in context for handlers to use
			c.Set("trace_span", span)
			c.SetRequest(c.Request().WithContext(ctx))

			// Execute handler
			err := next(c)

			// Update status code
			span.SetAttributes(attribute.Int("http.status_code", c.Response().Status))

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		}
	}
}

// TraceHandler wraps an http.Handler with tracing
func TraceHandler(handler http.Handler, spanName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer := GetTracer()
		if tracer == nil || !tracer.Enabled() {
			handler.ServeHTTP(w, r)
			return
		}

		ctx, span := tracer.Start(r.Context(), spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.path", r.URL.Path),
		)

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AddTraceHeaders adds trace context to response for client-side propagation
func AddTraceHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)

		// Propagate traceparent to client
		if span := trace.SpanFromContext(c.Request().Context()); span != nil && span.SpanContext().IsValid() {
			c.Response().Header().Set("traceparent", span.SpanContext().String())
		}

		return err
	}
}

// SpanFromContext retrieves the span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// Attribute helper
var attribute = struct {
	String  func(string, string) attribute.KeyValue
	Int     func(string, int) attribute.KeyValue
	Float64 func(string, float64) attribute.KeyValue
 Bool   func(string, bool) attribute.KeyValue
}{
	String:  attribute.String,
	Int:     attribute.Int,
	Float64: attribute.Float64,
	Bool:    attribute.Bool,
}

// Codes for span status
var codes = struct {
	Ok    int
	Error int
}{
	Ok:    int(codes.Ok),
	Error: int(codes.Error),
}

// SpanContextFromHeader parses a traceparent header
func SpanContextFromHeader(traceparent string) (string, error) {
	// Parse W3C traceparent format: version-traceid-parentid-flags
	parts := strings.Split(traceparent, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid traceparent format")
	}
	// Return the full traceparent for propagation
	return traceparent, nil
}