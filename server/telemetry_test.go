package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

func TestHostContractParseFailureEmitsErrorSpan(t *testing.T) {
	provider, exporter := configureTestTracers(t)
	files := activationRequestFiles(map[string]string{
		"/index.ts": "export default null",
	})
	delete(files, hostContractPrefix+"index.js")
	body, contentType := buildMultipart(files)

	ctx, parentSpan := provider.Tracer("test").Start(context.Background(), "request")
	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body).WithContext(ctx)
	req.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	compileV3Handler(response, req)
	parentSpan.End()

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	hostSpan := requireRecordedSpan(t, exporter, "fly_tsgo.host_contract.parse")
	if hostSpan.Parent.SpanID() != parentSpan.SpanContext().SpanID() {
		t.Fatalf("host span parent = %s, want %s", hostSpan.Parent.SpanID(), parentSpan.SpanContext().SpanID())
	}
	if got := requireSpanAttribute(t, hostSpan, "fly_tsgo.host_contract.provided").AsBool(); !got {
		t.Fatal("expected host contract to be recorded as provided")
	}
	if got := requireSpanAttribute(t, hostSpan, "fly_tsgo.host_contract.success").AsBool(); got {
		t.Fatal("expected host contract parsing to be recorded as failed")
	}
	if got := requireSpanAttribute(t, hostSpan, "exception.slug").AsString(); got != "err-host-contract-parse" {
		t.Fatalf("exception.slug = %q, want %q", got, "err-host-contract-parse")
	}
	if hostSpan.Status.Code != codes.Error {
		t.Fatalf("status code = %v, want %v", hostSpan.Status.Code, codes.Error)
	}
}

func TestHostContractParseSuccessEmitsContractAttributes(t *testing.T) {
	provider, exporter := configureTestTracers(t)
	files := activationRequestFiles(map[string]string{
		"/index.ts": "export default null",
	})
	files["/package.json"] = []byte("{")
	body, contentType := buildMultipart(files)

	ctx, parentSpan := provider.Tracer("test").Start(context.Background(), "request")
	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body).WithContext(ctx)
	req.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	compileV3Handler(response, req)
	parentSpan.End()

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	hostSpan := requireRecordedSpan(t, exporter, "fly_tsgo.host_contract.parse")
	if got := requireSpanAttribute(t, hostSpan, "fly_tsgo.host_contract.success").AsBool(); !got {
		t.Fatal("expected host contract parsing to be recorded as successful")
	}
	if got := requireSpanAttribute(t, hostSpan, "fly_tsgo.host_contract.provider").AsString(); got != activationProvider {
		t.Fatalf("provider = %q, want %q", got, activationProvider)
	}
	if got := requireSpanAttribute(t, hostSpan, "fly_tsgo.host_contract.exports.count").AsInt64(); got != 7 {
		t.Fatalf("exports count = %d, want 7", got)
	}
}

func TestHostedCompilationEmitsActivationTrace(t *testing.T) {
	provider, exporter := configureTestTracers(t)
	files := activationRequestFiles(map[string]string{
		"/index.ts": `import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	return useContextState(Application.conversation.states.messages)
}`,
	})
	setupActivationDependencyCache(t, []byte("activation-test-lock"))
	hostContext, err := parseHostCompilationContext(files)
	if err != nil {
		t.Fatal(err)
	}

	ctx, parentSpan := provider.Tracer("test").Start(context.Background(), "request")
	var program *compiler.Program
	typecheckResponse := typecheckV3WithHost(
		ctx,
		files,
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		hostContext.contract,
		&program,
	)
	if len(typecheckResponse.Errors) > 0 {
		t.Fatalf("host typecheck failed: %v", typecheckResponse.Errors)
	}

	compilerTrace, ok := program.Tracing().(*otelCompilerTracer)
	if !ok {
		t.Fatalf("program tracer = %T, want *otelCompilerTracer", program.Tracing())
	}
	base := time.Now()
	compilerTrace.mu.Lock()
	compilerTrace.events = append(compilerTrace.events, compilerTraceEvent{
		end:   base.Add(20 * time.Millisecond),
		name:  "activationAnalysis",
		phase: tracing.PhaseCheck,
		start: base,
	})
	compilerTrace.mu.Unlock()

	activation, diagnostics := deriveActivation(ctx, program, "./index.ts", hostContext)
	if len(diagnostics) > 0 {
		t.Fatalf("activation failed: %v", diagnostics)
	}
	pkg, err := parsePackageJSON(files["/package.json"])
	if err != nil {
		t.Fatal(err)
	}
	compileResponse := compileV3WithHost(
		ctx,
		files,
		pkg,
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		hostContext.contract,
	)
	if len(compileResponse.Errors) > 0 {
		t.Fatalf("host compile failed: %v", compileResponse.Errors)
	}
	parentSpan.End()

	activationSpan := requireRecordedSpan(t, exporter, "fly_tsgo.activation.derive")
	if activationSpan.Parent.SpanID() != parentSpan.SpanContext().SpanID() {
		t.Fatalf("activation span parent = %s, want %s", activationSpan.Parent.SpanID(), parentSpan.SpanContext().SpanID())
	}
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.success").AsBool(); !got {
		t.Fatal("expected activation derivation to be recorded as successful")
	}
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.references.count").AsInt64(); got != 1 {
		t.Fatalf("reference count = %d, want 1", got)
	}
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.required_references.count").AsInt64(); got != 1 {
		t.Fatalf("required reference count = %d, want 1", got)
	}
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.scope").AsString(); got != activation.Explanation.ActivationScope {
		t.Fatalf("activation scope = %q, want %q", got, activation.Explanation.ActivationScope)
	}
	compilerSpan := requireRecordedSpan(t, exporter, "typescript.check.activationAnalysis")
	if compilerSpan.Parent.SpanID() != activationSpan.SpanContext.SpanID() {
		t.Fatalf("compiler span parent = %s, want activation span %s", compilerSpan.Parent.SpanID(), activationSpan.SpanContext.SpanID())
	}
	typecheckSpan := requireRecordedSpan(t, exporter, "fly_tsgo.v3.typecheck")
	if got := requireSpanAttribute(t, typecheckSpan, "fly_tsgo.host_contract.present").AsBool(); !got {
		t.Fatal("expected typecheck span to record host contract presence")
	}
	compileSpan := requireRecordedSpan(t, exporter, "fly_tsgo.v3.compile")
	if got := requireSpanAttribute(t, compileSpan, "fly_tsgo.host_contract.present").AsBool(); !got {
		t.Fatal("expected compile span to record host contract presence")
	}
}

func TestActivationFailureEmitsErrorSpan(t *testing.T) {
	provider, exporter := configureTestTracers(t)
	files := activationRequestFiles(map[string]string{
		"/index.ts": "export default null",
	})
	setupActivationDependencyCache(t, []byte("activation-test-lock"))
	hostContext, err := parseHostCompilationContext(files)
	if err != nil {
		t.Fatal(err)
	}

	ctx, parentSpan := provider.Tracer("test").Start(context.Background(), "request")
	var program *compiler.Program
	response := typecheckV3WithHost(
		ctx,
		files,
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		hostContext.contract,
		&program,
	)
	if len(response.Errors) > 0 {
		t.Fatalf("host typecheck failed: %v", response.Errors)
	}
	_, diagnostics := deriveActivation(ctx, program, "./missing.ts", hostContext)
	parentSpan.End()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one", diagnostics)
	}

	activationSpan := requireRecordedSpan(t, exporter, "fly_tsgo.activation.derive")
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.success").AsBool(); got {
		t.Fatal("expected activation derivation to be recorded as failed")
	}
	if got := requireSpanAttribute(t, activationSpan, "fly_tsgo.activation.errors.count").AsInt64(); got != 1 {
		t.Fatalf("error count = %d, want 1", got)
	}
	if got := requireSpanAttribute(t, activationSpan, "exception.slug").AsString(); got != "err-activation-entry-point" {
		t.Fatalf("exception.slug = %q, want %q", got, "err-activation-entry-point")
	}
	if activationSpan.Status.Code != codes.Error {
		t.Fatalf("status code = %v, want %v", activationSpan.Status.Code, codes.Error)
	}
}

func configureTestTracers(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousCompilerTracer := compilerTracer
	previousTSGoTracer := tsgoTracer
	compilerTracer = provider.Tracer("fly-tsgo/compiler")
	tsgoTracer = provider.Tracer("fly-tsgo/server")
	t.Cleanup(func() {
		compilerTracer = previousCompilerTracer
		tsgoTracer = previousTSGoTracer
		_ = provider.Shutdown(context.Background())
	})
	return provider, exporter
}

func requireRecordedSpan(t *testing.T, exporter *tracetest.InMemoryExporter, name string) tracetest.SpanStub {
	t.Helper()
	for _, span := range exporter.GetSpans() {
		if span.Name == name {
			return span
		}
	}
	t.Fatalf("missing span %q", name)
	return tracetest.SpanStub{}
}

func requireSpanAttribute(t *testing.T, span tracetest.SpanStub, key string) attribute.Value {
	t.Helper()
	for _, candidate := range span.Attributes {
		if string(candidate.Key) == key {
			return candidate.Value
		}
	}
	t.Fatalf("span %q missing attribute %q", span.Name, key)
	return attribute.Value{}
}
