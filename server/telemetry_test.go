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
