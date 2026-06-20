package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
