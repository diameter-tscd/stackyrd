package tracer

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string  // e.g. "otel-collector:4318"
	Enabled        bool
	SampleRate     float64 // 0.0–1.0
}

// Tracer wraps OpenTelemetry TracerProvider
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   Config
	enabled  bool
}

// New creates a tracer with OTLP HTTP exporter
func New(cfg Config) (*Tracer, error) {
	if !cfg.Enabled {
		return &Tracer{enabled: false, config: cfg}, nil
	}

	sr := cfg.SampleRate
	if sr <= 0 || sr > 1 {
		sr = 1.0
	}
	ep := cfg.OTLPEndpoint
	if ep == "" {
		ep = "localhost:4318"
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		attribute.String("deployment.environment", cfg.Environment),
		attribute.String("go.version", runtime.Version()),
	)

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(ep),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	var sampler sdktrace.Sampler
	if sr < 1.0 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sr))
	} else {
		sampler = sdktrace.AlwaysSample()
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		provider: provider,
		tracer:   provider.Tracer(cfg.ServiceName),
		config:   cfg,
		enabled:  true,
	}, nil
}

// Shutdown flushes and shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if !t.enabled || t.provider == nil {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return t.provider.Shutdown(c)
}

// Start creates a new span
func (t *Tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, name, opts...)
}

// Enabled returns whether tracing is active
func (t *Tracer) Enabled() bool { return t.enabled }

// Config returns the tracer config
func (t *Tracer) Config() Config { return t.config }

var global *Tracer

// Get returns the global tracer
func Get() *Tracer { return global }

// Set sets the global tracer
func Set(tr *Tracer) { global = tr }
