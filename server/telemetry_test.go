package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/tracing"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestDepResolveContextIgnoresCancellationAndPreservesTraceContext(t *testing.T) {
	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		TraceID:    traceID,
	})

	parent, cancel := context.WithCancel(context.Background())
	parent = trace.ContextWithSpanContext(parent, spanContext)
	cancel()

	ctx := depResolveContext(parent)
	if err := ctx.Err(); err != nil {
		t.Fatalf("expected dependency resolve context to ignore parent cancellation, got %v", err)
	}

	select {
	case <-ctx.Done():
		t.Fatal("expected dependency resolve context to have no cancellation signal")
	default:
	}

	got := trace.SpanContextFromContext(ctx)
	if got.TraceID() != traceID || got.SpanID() != spanID || !got.IsSampled() {
		t.Fatalf("expected trace context to be preserved, got trace=%s span=%s sampled=%t", got.TraceID(), got.SpanID(), got.IsSampled())
	}
}

func TestDefaultHoneycombEndpointTargetsTraceEndpoint(t *testing.T) {
	const want = "https://api.honeycomb.io/v1/traces"
	if defaultHoneycombEndpoint != want {
		t.Fatalf("defaultHoneycombEndpoint = %q, want %q", defaultHoneycombEndpoint, want)
	}
}

func TestHTTPSpanName(t *testing.T) {
	tests := []struct {
		method  string
		pattern string
		want    string
	}{
		{method: http.MethodGet, pattern: "", want: "GET"},
		{method: http.MethodPost, pattern: "/v3/compile", want: "POST /v3/compile"},
		{method: http.MethodGet, pattern: "/{$}", want: "GET /"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, "/", http.NoBody)
		req.Pattern = tt.pattern

		if got := httpSpanName("", req); got != tt.want {
			t.Fatalf("httpSpanName(%q, %q) = %q, want %q", tt.method, tt.pattern, got, tt.want)
		}
	}
}

func TestRouteSpanNameUsesServeMuxPattern(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
	})

	mux := http.NewServeMux()
	registerRoute(mux, "/v3/compile", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v3/compile", http.NoBody)
	res := httptest.NewRecorder()
	loggingMiddleware(mux.ServeHTTP)(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	for _, span := range exporter.GetSpans() {
		if span.SpanKind == trace.SpanKindServer && span.Name == "POST /v3/compile" {
			return
		}
	}

	t.Fatalf("expected server span named %q, got spans %#v", "POST /v3/compile", exporter.GetSpans())
}

func TestStandardOTLPHeadersConfigured(t *testing.T) {
	tests := []struct {
		name             string
		otelHeaders      string
		otelTraceHeaders string
		want             bool
	}{
		{name: "none", want: false},
		{name: "generic", otelHeaders: "x-honeycomb-team=key", want: true},
		{name: "traces", otelTraceHeaders: "x-honeycomb-team=key", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := standardOTLPHeadersConfigured(tt.otelHeaders, tt.otelTraceHeaders)
			if got != tt.want {
				t.Fatalf("standardOTLPHeadersConfigured(%q, %q) = %t, want %t", tt.otelHeaders, tt.otelTraceHeaders, got, tt.want)
			}
		})
	}
}

func TestTelemetryExportConfigured(t *testing.T) {
	tests := []struct {
		apiKey            string
		name              string
		otelEndpoint      string
		otelHeaders       string
		otelTraceEndpoint string
		otelTraceHeaders  string
		want              bool
	}{
		{name: "none", want: false},
		{name: "honeycomb api key", apiKey: "key", want: true},
		{name: "generic headers", otelHeaders: "x-honeycomb-team=key", want: true},
		{name: "traces headers", otelTraceHeaders: "x-honeycomb-team=key", want: true},
		{name: "generic endpoint", otelEndpoint: "http://collector:4318", want: true},
		{name: "traces endpoint", otelTraceEndpoint: "http://collector:4318/v1/traces", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := telemetryExportConfigured(tt.apiKey, tt.otelHeaders, tt.otelTraceHeaders, tt.otelEndpoint, tt.otelTraceEndpoint)
			if got != tt.want {
				t.Fatalf(
					"telemetryExportConfigured(%q, %q, %q, %q, %q) = %t, want %t",
					tt.apiKey,
					tt.otelHeaders,
					tt.otelTraceHeaders,
					tt.otelEndpoint,
					tt.otelTraceEndpoint,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestOTelCompilerTracerDisabledForNonRecordingSpan(t *testing.T) {
	tracer := newOTelCompilerTracer(context.Background())
	if tracer != nil {
		t.Fatal("expected compiler tracing to be disabled without a recording span")
	}
	if performanceTracer := asCompilerPerformanceTracer(tracer); performanceTracer != nil {
		t.Fatal("expected nil compiler tracer to remain nil when passed through the performance tracer interface")
	}
}

func TestOTelCompilerTracerParentsOverlappingEventsToPhaseSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousCompilerTracer := compilerTracer
	compilerTracer = provider.Tracer("fly-tsgo/compiler")
	t.Cleanup(func() {
		compilerTracer = previousCompilerTracer
		_ = provider.Shutdown(context.Background())
	})

	phaseCtx, phaseSpan := provider.Tracer("test").Start(context.Background(), "typescript.program.create")
	phaseSpanID := phaseSpan.SpanContext().SpanID()
	base := time.Now()
	tracer := &otelCompilerTracer{events: []compilerTraceEvent{
		{name: "findSourceFile", phase: tracing.PhaseProgram, start: base, end: base.Add(50 * time.Millisecond)},
		{name: "createSourceFile", phase: tracing.PhaseParse, start: base.Add(10 * time.Millisecond), end: base.Add(40 * time.Millisecond)},
	}}

	tracer.flush(phaseCtx)
	phaseSpan.End()

	compilerSpans := 0
	for _, span := range exporter.GetSpans() {
		if span.Name != "typescript.program.findSourceFile" && span.Name != "typescript.parse.createSourceFile" {
			continue
		}
		compilerSpans++
		if got := span.Parent.SpanID(); got != phaseSpanID {
			t.Fatalf("compiler span %q parent = %s, want phase span %s", span.Name, got, phaseSpanID)
		}
	}
	if compilerSpans != 2 {
		t.Fatalf("compiler spans = %d, want 2", compilerSpans)
	}
}
