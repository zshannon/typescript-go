# Server Benchmark Suite Design

## Goal

Layered Go benchmarks for the v2 server endpoints that isolate each phase of a request so we can tell exactly where time is spent: diskFS setup, TypeScript compilation, esbuild bundling, and HTTP overhead.

## Test Fixtures

All fixtures use React JSX with `@crayonnow/core` imports, matching production usage. Defined as Go constants in a `bench_fixtures_test.go` file.

Single-file fixtures are wrapped for the v2 API as:
```go
files := map[string]string{"/index.tsx": code}
entryPoints := []string{"/index.tsx"}  // for typecheck
entryPoint := "/index.tsx"             // for build
```

All fixtures use `benchVersion = "0.0.4"` to match the mock S3 pre-populated packages.

### Trivial (~1 line)
```typescript
export const x: number = 1;
```

### Small Component (~10 lines)
Simple JSX component with one `@crayonnow/core` import and basic props.

### Medium Component (~60 lines)
The meditation timer from the existing test fixtures — verbatim copy. Multiple `@crayonnow/core` imports (`Flex`, `Text`, `Button`, `Picker`), `useState`, `useEffect`, conditional rendering, computed display values.

### Multi-File Project (~5 files)
V2 multi-file scenario passed as `map[string]string`:
- `/types.ts` — shared interfaces (e.g., `TimerState`, `TimerConfig`)
- `/hooks.ts` — custom hooks using `useState`/`useEffect`, imports from `./types`
- `/Button.tsx` — component importing from `./types` + `@crayonnow/core`
- `/Card.tsx` — component importing `./Button` + `./types`
- `/index.tsx` — entry point composing everything (this is the `entryPoint` for build)

## Benchmark Layers

All benchmarks live in `bench_v2_test.go`. All use `b.ReportAllocs()`. Sub-benchmarks must NOT run in parallel — the server uses package-level globals (`s3Client`, `s3Bucket`, `diskCachePath`) that are not safe for concurrent writes.

### Layer 1: Pure Compiler (`BenchmarkV2Typecheck`)

Calls `typecheckTypeScriptV2(files, entryPoints, benchVersion)` directly. S3 is pre-synced in setup (outside the timer). Measures: diskFS creation (`os.Stat` check + struct alloc) + compiler program creation + type checking.

The `os.Stat` on every iteration is acceptable — it's the real production path and takes ~5us vs ~ms for compilation.

Sub-benchmarks:
- `BenchmarkV2Typecheck/Trivial`
- `BenchmarkV2Typecheck/SmallComponent`
- `BenchmarkV2Typecheck/MediumComponent`
- `BenchmarkV2Typecheck/MultiFile`

### Layer 2: Full Pipeline (`BenchmarkV2Build`)

Calls `buildTypeScriptV2(files, entryPoint, benchVersion)` directly. Measures: diskFS + compiler + esbuild bundling. Same sub-benchmarks as Layer 1. `entryPoint` is `"/index.tsx"` for all fixtures.

### Layer 3: HTTP Handler (`BenchmarkV2HTTP`)

Uses `httptest.NewRequest` + `httptest.NewRecorder` to call `typecheckV2` and `buildV2` handler functions directly (not through `loggingMiddleware` — we want to isolate compilation, not logging overhead). Measures: JSON decode + validation + compilation + JSON encode.

Pre-serializes the JSON request body once before the benchmark loop so we're not measuring `json.Marshal` in the hot path.

Sub-benchmarks:
- `BenchmarkV2HTTP/Typecheck/MediumComponent`
- `BenchmarkV2HTTP/Build/MediumComponent`
- `BenchmarkV2HTTP/Typecheck/MultiFile`
- `BenchmarkV2HTTP/Build/MultiFile`

(Only medium and multi-file for HTTP layer — trivial/small add noise without signal at this layer.)

## Setup

A shared `setupBenchServer(b *testing.B)` function that:
1. Creates a `MockS3Client` with pre-populated packages (version `"0.0.4"`)
2. Sets globals: `s3Client`, `s3Bucket = "test-bucket"`, `serverVersion = "1.0.0"`, `startTime = time.Now()`
3. Creates a temp disk cache directory, sets `diskCachePath`
4. Calls `newDiskFS(context.Background(), "0.0.4")` once to trigger the mock S3 sync and create the on-disk directory — subsequent iterations just see the directory exists and skip sync
5. Silences `log` output: saves `log.Writer()`, sets `log.SetOutput(io.Discard)`, restores in `b.Cleanup`
6. Registers `b.Cleanup` to restore log output and remove temp dirs

## Custom Metrics

Each benchmark reports:
- `ns/op`, `B/op`, `allocs/op` (standard Go bench output)
- `output_bytes/op` via `b.ReportMetric` — size of compiled JS output (build benchmarks only)

## Files

| File | Purpose |
|------|---------|
| `bench_fixtures_test.go` | Test fixture constants (code strings, multi-file maps) |
| `bench_v2_test.go` | All v2 benchmark functions + `setupBenchServer` |

## Out of Scope

- v1 endpoints
- Production latency benchmarks (`benchmark_prod.sh` stays as-is)
- Replacing existing test code
- `?validate_types=true` query parameter variant (can be added later)
- `b.RunParallel` (globals are not thread-safe)
