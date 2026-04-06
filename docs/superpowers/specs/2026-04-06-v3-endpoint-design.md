# V3 API: Self-Contained TypeScript Typecheck & Compile

## Principle

V3 assumes nothing. The caller provides the complete project definition: source files, dependencies (package.json + bun.lock), TypeScript config (tsconfig.json), and esbuild options (via package.json). The server is a pure execution environment.

## Endpoints

### POST /v3/typecheck

Typechecks all TypeScript files using compiler options from the caller's tsconfig.json.

### POST /v3/compile

Compiles/bundles the project using esbuild. Entry point read from `package.json#main`. Typechecks first by default.

Query parameters:
- `skip_typecheck=true` -- skip the typecheck step, compile only

## Request Format

`Content-Type: multipart/form-data`

Each part's field name is the file path (absolute, rooted at `/`). The server parses all parts into a `map[string][]byte`, converting to `string` when populating `diskFS.userFiles` (which remains `map[string]string` to match the existing v2 interface).

### Required files

- `/package.json` -- must contain:
  - `main` -- entry point for compile (e.g., `"./src/index.ts"`)
  - `dependencies` and/or `devDependencies` -- packages to install
  - `esbuild` (optional) -- esbuild build options (see below)
- `/bun.lock` -- lockfile pinning exact dependency versions

### Optional files

- `/tsconfig.json` -- TypeScript compiler options. If absent, the server uses sensible defaults (strict, ES2022 target, bundler module resolution).

### Source files

All other parts are treated as source files. Paths must not start with `/node_modules/` (reserved for installed deps).

### Limits

Same as v2:
- Max 100 files per request
- Max 1 MB per file
- Max 10 MB total payload

## esbuild Configuration

The `esbuild` field in package.json maps directly to esbuild Go API options. Only the following fields are supported:

```json
{
  "esbuild": {
    "bundle": true,
    "external": ["some-runtime-dep"],
    "format": "cjs",
    "minify": false,
    "minifyIdentifiers": false,
    "minifySyntax": true,
    "minifyWhitespace": true,
    "platform": "browser",
    "target": "es2022"
  }
}
```

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `bundle` | bool | `true` | Bundle imports into output |
| `external` | string[] | `[]` | Packages to exclude from bundle (e.g., `["*"]` for all, `["zod"]` for specific) |
| `format` | `"cjs"` \| `"esm"` \| `"iife"` | `"cjs"` | Output format |
| `minify` | bool | -- | Shorthand for all three minify flags |
| `minifyIdentifiers` | bool | `false` | Minify variable names |
| `minifySyntax` | bool | `true` | Minify syntax constructs |
| `minifyWhitespace` | bool | `true` | Minify whitespace |
| `platform` | `"browser"` \| `"node"` \| `"neutral"` | `"browser"` | Target platform |
| `target` | string | `"es2022"` | JS target version |

JSX settings (factory, fragment, import source) come from tsconfig.json, not from the esbuild field. esbuild will respect the tsconfig.json JSX configuration.

**Note on externals:** Because the esbuild virtual-fs plugin intercepts all import resolution (overriding esbuild's built-in `External` option), the plugin's OnResolve handler must explicitly check the externals list and return `External: true` for matching imports. Setting `"external": ["*"]` in package.json means "externalize all bare imports" — the output will contain `require('zod')` calls instead of inlining the package code. The default `[]` bundles everything, matching the current v1/v2 behavior where the platform does not support `require`.

## tsconfig.json Handling

If `/tsconfig.json` is provided, the server parses it and uses the `compilerOptions` for both the TypeScript-Go compiler and esbuild's TypeScript/JSX handling.

If absent, defaults are:

```json
{
  "compilerOptions": {
    "allowJs": true,
    "declaration": true,
    "esModuleInterop": true,
    "forceConsistentCasingInFileNames": true,
    "isolatedModules": true,
    "jsx": "react-jsx",
    "module": "commonjs",
    "moduleResolution": "bundler",
    "noEmit": true,
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "strict": true,
    "strictNullChecks": true,
    "target": "ES2022",
    "lib": ["ES2022"]
  }
}
```

These match the current v2 hardcoded options (including the explicit `lib` field), providing backward compatibility for callers that don't need custom config.

## Response Format

Same JSON structure as v2. Both TypeScript-Go diagnostics and esbuild errors are mapped into the same `DiagnosticErrorV2` shape:

```json
// typecheck success
{"pass": true}

// typecheck errors
{"errors": [{"file": "/src/index.ts", "message": "...", "line": 1, "column": 5}]}

// compile success
{"code": "...bundled JS..."}

// compile errors (esbuild errors mapped to same shape)
{"errors": [{"file": "/src/index.ts", "message": "...", "line": 1, "column": 5}]}
```

## Dependency Caching

### Cache key

SHA256 hash of the raw `/bun.lock` file contents. Fully deterministic since the lockfile pins exact versions.

### Lookup order

1. **Local disk** -- check `{DISK_CACHE_PATH}/deps/{hash}/node_modules/` exists. If yes, use it.
2. **S3** -- check `deps/{hash}.tar.gz` in the S3 bucket. If found, download and extract to local disk path.
3. **Install** -- write `package.json` + `bun.lock` to a temp directory, run `bun install --frozen-lockfile --ignore-scripts`, tar.gz the resulting `node_modules`, upload to S3 via `PutObject`, move to local disk cache path. If `bun install` fails, clean up the temp directory and return a 502 error with `{"errors": [{"message": "dependency installation failed: <stderr>"}]}`. If S3 upload fails, log the error but proceed (local cache is sufficient; S3 upload can be retried on next miss).

### S3 interface change

The existing `S3ClientInterface` must be extended with `PutObject` to support uploading cached dep tarballs:

```go
type S3ClientInterface interface {
    GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
    ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
    PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}
```

### S3 layout

Dep caches live under the `deps/` prefix: `deps/{sha256hash}.tar.gz`. This avoids collision with the existing `{version}/*` layout used by v1/v2.

### Local disk eviction

Same LRU strategy the server already uses for versioned caches. Eviction covers both `{version}/` directories (v1/v2) and `deps/{hash}/` directories (v3). When disk is full, evict the oldest entry by mtime regardless of type.

### Concurrency

If multiple requests arrive simultaneously with the same bun.lock hash and all miss cache, only one installs. A `sync.Map` of in-flight installs keyed by hash coordinates this -- the second request waits on the first via a channel. The install result (success or error) is broadcast to all waiters; on error, each waiter returns the same 502 to its caller.

## Request Processing Flow

### Typecheck

1. Parse multipart fields into `map[string][]byte`
2. Validate `/package.json` and `/bun.lock` are present
3. Hash `/bun.lock` -> resolve deps (local -> S3 -> install)
4. If `/tsconfig.json` is present, parse compiler options from it; otherwise use defaults
5. Build `diskFS` blending user source files with resolved `node_modules` at `{DISK_CACHE_PATH}/deps/{hash}`
6. Set up TypeScript-Go compiler with parsed options
7. Collect syntactic then semantic diagnostics
8. Return `{"pass": true}` or `{"errors": [...]}`

### Compile

1. Parse multipart fields into `map[string][]byte`
2. Validate `/package.json` and `/bun.lock` are present
3. Hash `/bun.lock` -> resolve deps (local -> S3 -> install)
4. Parse `main` from `/package.json` -> entry point
5. If `skip_typecheck` is not set, run typecheck first; return errors if any
6. Parse esbuild options from `package.json#esbuild` (or use defaults)
7. Build `diskFS` blending user source files with resolved `node_modules`
8. Run esbuild with parsed options, entry point, and virtual-fs plugin
9. Return `{"code": "..."}` or `{"errors": [...]}`

## diskFS Changes

The existing `diskFS` struct currently takes a `version` string and reads from `{DISK_CACHE_PATH}/{version}/`. For v3, a new constructor creates a diskFS backed by the dep cache path instead:

```go
func newDiskFSFromDeps(depCachePath string) *diskFS
```

This points `basePath` at `{DISK_CACHE_PATH}/deps/{hash}` where bun installed the node_modules. User files are overlaid on top via `userFiles` map, same as v2.

The `/node_modules/` path validation in `normalizeAndValidatePath` still applies -- user-submitted files cannot override installed packages.

## Authentication

Same HTTP Message Signature middleware as v1/v2 endpoints. Both `/v3/typecheck` and `/v3/compile` require authentication when `TSGO_AUTH_PUBLIC_KEY` is set.

## Metrics

New metric labels for v3 endpoints, following the existing pattern:
- `http_request_duration_seconds{endpoint="/v3/typecheck"}` and `{endpoint="/v3/compile"}`
- `http_requests_total{endpoint="/v3/typecheck"}` and `{endpoint="/v3/compile"}`
- Existing `typecheck_duration_seconds`, `typecheck_results_total`, `compile_duration_seconds`, `compile_results_total` counters are shared across all API versions

New metrics:
- `dep_cache_lookups_total{result="local_hit|s3_hit|miss"}` -- tracks cache effectiveness
- `dep_install_duration_seconds` -- histogram for bun install time on cache miss

## Docker Changes

The Dockerfile needs bun installed in the runtime image. Bun provides an Alpine-compatible binary (~50MB). Add to the final stage:

```dockerfile
RUN apk add --no-cache curl unzip && \
    curl -fsSL https://bun.sh/install | bash && \
    ln -s /root/.bun/bin/bun /usr/local/bin/bun
```

## Environment Variables

No new required env vars. `S3_BUCKET` and `DISK_CACHE_PATH` are reused. Bun respects standard `HOME` for its cache.

## What V3 Does NOT Do

- No `version` field or versioned S3 sync -- the caller owns all deps via package.json
- No hardcoded JSX factory/fragment -- comes from tsconfig.json
- No hardcoded compiler options -- comes from tsconfig.json
- No special-case react handling -- if the caller wants react as an external global, they configure it via esbuild externals
- No `/sync` endpoint equivalent -- deps are cached automatically on first use
