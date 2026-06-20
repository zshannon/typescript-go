package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	defaultHoneycombEndpoint = "https://api.honeycomb.io/v1/traces"
	defaultServiceName       = "fly-tsgo"
)

var tsgoTracer = otel.Tracer("fly-tsgo/server")

func initTelemetry(ctx context.Context) func(context.Context) error {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	apiKey := strings.TrimSpace(os.Getenv("HONEYCOMB_API_KEY"))
	otelHeaders := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	otelTraceHeaders := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS"))
	otelEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	otelTraceEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	if !telemetryExportConfigured(apiKey, otelHeaders, otelTraceHeaders, otelEndpoint, otelTraceEndpoint) {
		log.Printf("OpenTelemetry export disabled: no Honeycomb API key or OTLP export config is set")
		return func(context.Context) error { return nil }
	}

	options := []otlptracehttp.Option{}
	if apiKey != "" && !standardOTLPHeadersConfigured(otelHeaders, otelTraceHeaders) {
		options = append(options, otlptracehttp.WithHeaders(map[string]string{
			"x-honeycomb-team": apiKey,
		}))
	}
	if otelEndpoint == "" && otelTraceEndpoint == "" {
		options = append(options, otlptracehttp.WithEndpointURL(defaultHoneycombEndpoint))
	}

	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		log.Printf("OpenTelemetry export disabled: failed to initialize OTLP exporter: %v", err)
		return func(context.Context) error { return nil }
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	resource, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.namespace", "flick"),
			attribute.String("service.version", serverVersion),
		),
	)
	if err != nil {
		log.Printf("OpenTelemetry export disabled: failed to initialize resource: %v", err)
		return func(context.Context) error { return nil }
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)
	otel.SetTracerProvider(provider)
	tsgoTracer = provider.Tracer("fly-tsgo/server")
	log.Printf("OpenTelemetry export enabled for service %q", serviceName)

	return provider.Shutdown
}

func standardOTLPHeadersConfigured(otelHeaders string, otelTraceHeaders string) bool {
	return otelHeaders != "" || otelTraceHeaders != ""
}

func telemetryExportConfigured(apiKey string, otelHeaders string, otelTraceHeaders string, otelEndpoint string, otelTraceEndpoint string) bool {
	return apiKey != "" || otelHeaders != "" || otelTraceHeaders != "" || otelEndpoint != "" || otelTraceEndpoint != ""
}

func depResolveContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

func httpSpanName(_ string, r *http.Request) string {
	method := r.Method
	if method == "" {
		method = "HTTP"
	}

	route := strings.TrimSuffix(r.Pattern, "{$}")
	if route != "" {
		return method + " " + route
	}

	return method
}

func otelHandler(handler http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(
		http.HandlerFunc(handler),
		"HTTP",
		otelhttp.WithSpanNameFormatter(httpSpanName),
	)
}

func registerRoute(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mux.Handle(pattern, otelHandler(handler))
}

func recordSpanError(span oteltrace.Span, slug string, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(
		attribute.Bool("error", true),
		attribute.String("exception.slug", slug),
	)
}

func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	return tsgoTracer.Start(ctx, name, oteltrace.WithAttributes(attrs...))
}

func spanDurationMS(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
