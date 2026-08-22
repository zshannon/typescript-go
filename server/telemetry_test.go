package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/tracing"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
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

func TestTelemetryExportsW3CChildToConfiguredEndpoint(t *testing.T) {
	type capturedExport struct {
		apiKey  string
		err     error
		path    string
		request collectortracepb.ExportTraceServiceRequest
	}

	exports := make(chan capturedExport, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		captured := capturedExport{
			apiKey: r.Header.Get("X-Honeycomb-Team"),
			err:    err,
			path:   r.URL.Path,
		}
		if err == nil {
			captured.err = proto.Unmarshal(body, &captured.request)
		}
		exports <- captured

		response, err := proto.Marshal(&collectortracepb.ExportTraceServiceResponse{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(response)
	}))
	t.Cleanup(receiver.Close)

	t.Setenv("HONEYCOMB_API_KEY", "test-api-key")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", receiver.URL+"/v1/traces")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_SERVICE_NAME", "fly-tsgo-test")

	previousCompilerTracer := compilerTracer
	previousPropagator := otel.GetTextMapPropagator()
	previousProvider := otel.GetTracerProvider()
	previousTSGoTracer := tsgoTracer
	t.Cleanup(func() {
		compilerTracer = previousCompilerTracer
		otel.SetTextMapPropagator(previousPropagator)
		otel.SetTracerProvider(previousProvider)
		tsgoTracer = previousTSGoTracer
	})

	shutdown := initTelemetry(context.Background())

	const traceIDHex = "0102030405060708090a0b0c0d0e0f10"
	const parentSpanIDHex = "0102030405060708"
	mux := http.NewServeMux()
	registerRoute(mux, "/v3/compile", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/v3/compile", http.NoBody)
	req.Header.Set("traceparent", "00-"+traceIDHex+"-"+parentSpanIDHex+"-01")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown telemetry: %v", err)
	}

	var captured capturedExport
	select {
	case captured = <-exports:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OTLP trace export")
	}
	if captured.err != nil {
		t.Fatalf("decode OTLP trace export: %v", captured.err)
	}
	if captured.path != "/v1/traces" {
		t.Fatalf("OTLP export path = %q, want %q", captured.path, "/v1/traces")
	}
	if captured.apiKey != "test-api-key" {
		t.Fatalf("OTLP x-honeycomb-team header = %q, want %q", captured.apiKey, "test-api-key")
	}

	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		t.Fatal(err)
	}
	parentSpanID, err := trace.SpanIDFromHex(parentSpanIDHex)
	if err != nil {
		t.Fatal(err)
	}
	for _, resourceSpans := range captured.request.ResourceSpans {
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				if span.Kind != tracepb.Span_SPAN_KIND_SERVER || span.Name != "POST /v3/compile" {
					continue
				}
				if !bytes.Equal(span.TraceId, traceID[:]) {
					t.Fatalf("server span trace ID = %x, want %s", span.TraceId, traceIDHex)
				}
				if !bytes.Equal(span.ParentSpanId, parentSpanID[:]) {
					t.Fatalf("server span parent ID = %x, want %s", span.ParentSpanId, parentSpanIDHex)
				}
				return
			}
		}
	}

	t.Fatalf("expected exported server span %q", "POST /v3/compile")
}

func TestCompilerSpanRecordingThreshold(t *testing.T) {
	tests := []struct {
		duration time.Duration
		name     string
		want     bool
	}{
		{duration: 9 * time.Millisecond, name: "findSourceFile", want: false},
		{duration: 10 * time.Millisecond, name: "findSourceFile", want: true},
		{duration: 50 * time.Millisecond, name: "createProgram", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.duration.String(), func(t *testing.T) {
			if got := shouldRecordCompilerSpan(tt.name, tt.duration); got != tt.want {
				t.Fatalf("shouldRecordCompilerSpan(%q, %s) = %t, want %t", tt.name, tt.duration, got, tt.want)
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
