# Server Benchmark Suite Design

## Goal

Layered Go benchmarks for the v2 server endpoints that isolate each phase of a request so we can tell exactly where time is spent: diskFS setup, TypeScript compilation, esbuild bundling, and HTTP overhead.

## Test Fixtures

All fixtures use React JSX with `@crayonnow/core` imports, matching production usage. Defined as Go constants in a `bench_fixtures_test.go` file.

### Trivial (~1 line)
```typescript
export const x: number = 1;
```

### Small Component (~10 lines)
Simple JSX component with one `@crayonnow/core` import and basic props.

### Medium Component (~60 lines)
The meditation timer: multiple `@crayonnow/core` imports, `useState`, `useEffect`, conditional rendering, computed values.

### Multi-File Project (~5 files)
V2 multi-file scenario:
- `types.ts` — shared interfaces
- `hooks.ts` — custom hooks using `useState`/`useEffect`
- `Button.tsx` — component importing from types + `@crayonnow/core`
- `Card.tsx` — component importing Button + types
- `index.tsx` — entry point composing everything

## Benchmark Layers

All benchmarks live in `bench_v2_test.go`. All use `b.ReportAllocs()`.

### Layer 1: Pure Compiler (`BenchmarkV2Typecheck`)

Calls `typecheckTypeScriptV2` directly. S3 is pre-synced in setup (outside the timer). Measures: diskFS creation + compiler program creation + type checking.

Sub-benchmarks:
- `BenchmarkV2Typecheck/Trivial`
- `BenchmarkV2Typecheck/SmallComponent`
- `BenchmarkV2Typecheck/MediumComponent`
- `BenchmarkV2Typecheck/MultiFile`

### Layer 2: Full Pipeline (`BenchmarkV2Build`)

Calls `buildTypeScriptV2` directly. Measures: diskFS + compiler + esbuild bundling. Same sub-benchmarks as Layer 1.

### Layer 3: HTTP Handler (`BenchmarkV2HTTP`)

Uses `httptest.NewRequest` + `httptest.NewRecorder` to call the `typecheckV2` and `buildV2` HTTP handlers. Measures the full request path: JSON decode, validation, compilation, JSON encode.

Sub-benchmarks:
- `BenchmarkV2HTTP/Typecheck/MediumComponent`
- `BenchmarkV2HTTP/Build/MediumComponent`
- `BenchmarkV2HTTP/Typecheck/MultiFile`
- `BenchmarkV2HTTP/Build/MultiFile`

(Only medium and multi-file for HTTP layer — trivial/small add noise without signal at this layer.)

## Setup

A shared `setupBenchServer(b *testing.B)` function that:
1. Creates a `MockS3Client` with pre-populated packages
2. Sets up a temp disk cache directory
3. Pre-syncs the version so diskFS creation doesn't hit the mock S3 on every iteration
4. Silences `log` output (sets `log.SetOutput(io.Discard)`, restores in cleanup)
5. Registers `b.Cleanup` to tear down temp dirs

## Custom Metrics

Each benchmark reports:
- `ns/op`, `B/op`, `allocs/op` (standard)
- `output_bytes` via `b.ReportMetric` — size of compiled JS output (build benchmarks)
- `diagnostics` via `b.ReportMetric` — number of diagnostics returned (typecheck benchmarks)

## Files

| File | Purpose |
|------|---------|
| `bench_fixtures_test.go` | Test fixture constants (code strings) |
| `bench_v2_test.go` | All v2 benchmark functions + `setupBenchServer` |

## Out of Scope

- v1 endpoints (deprecated)
- Production latency benchmarks (`benchmark_prod.sh` stays as-is)
- Replacing existing test code
