# TypeScript Compiler Server

A minimal HTTP server that compiles TypeScript code to JavaScript using the TypeScript-Go compiler, designed to run on Unikraft Cloud.

## Features

- **In-memory compilation**: No file system I/O required
- **TypeScript → ESNext/ESM**: Compiles to modern JavaScript modules
- **Error reporting**: Returns detailed TypeScript diagnostics with line/column information
- **Unikraft deployment**: Optimized for unikernel deployment with ~37ms boot time
- **High performance**: ~200ms average compilation time including network latency

## API

### `GET /`
Returns server information.

**Response:**
```
TypeScript Go Server
```

### `POST /compile`
Compiles TypeScript code to JavaScript.

**Request:**
```json
{
  "code": "export const hello: string = \"world\""
}
```

**Success Response:**
```json
{
  "code": "export const hello = \"world\";\n"
}
```

**Error Response:**
```json
{
  "errors": [
    {
      "message": "Type 'number' is not assignable to type 'string'.",
      "line": 1,
      "column": 7
    }
  ]
}
```

## Deployment

### Unikraft Cloud

```bash
# Deploy using 1Password CLI for secure environment variables
op run --env-file=".env" -- kraft cloud deploy -p 443:8080 . --name typescript-compiler
```

**Environment variables (`.env` file):**
```bash
UKC_METRO=dal0
UKC_TOKEN=your_unikraft_cloud_token_here
KRAFTKIT_BUILDKIT_HOST=docker-container://buildkitd
```

### Local Development

```bash
go run server.go
```

## Performance

### Benchmark Commands

**Unikraft Cloud (deployed):**
```bash
hyperfine --warmup 2 --runs 5 \
  'curl -X POST https://restless-mountain-fa2gk4yu.dal0.kraft.host/compile \
   -H "Content-Type: application/json" \
   -d "{\"code\": \"export const hello: number = \\\"hello!!!\\\"\"}"
```

**Local development:**
```bash
# Start server: go run server.go
hyperfine --warmup 2 --runs 5 \
  'curl -X POST http://localhost:8080/compile \
   -H "Content-Type: application/json" \
   -d "{\"code\": \"export const hello: number = \\\"hello!!!\\\"\"}"
```

### Results

**Unikraft Cloud:**
```
Benchmark 1: curl -X POST https://restless-mountain-fa2gk4yu.dal0.kraft.host/compile \
   -H "Content-Type: application/json" \
   -d "{\"code\": \"export const hello: number = \\\"hello!!!\\\"\"}"
  Time (mean ± σ):     218.2 ms ±  84.3 ms    [User: 7.9 ms, System: 4.3 ms]
  Range (min … max):   179.6 ms … 369.0 ms    5 runs
```

**Local:**
```
Benchmark 1: curl -X POST http://localhost:8080/compile \
   -H "Content-Type: application/json" \
   -d "{\"code\": \"export const hello: number = \\\"hello!!!\\\"\"}"
  Time (mean ± σ):      11.6 ms ±   2.1 ms    [User: 3.4 ms, System: 3.7 ms]
  Range (min … max):     8.6 ms …  14.4 ms    5 runs
```

### Performance Summary

- **Unikraft Cloud**: 218ms ± 84ms (including network latency to dal0)
- **Local development**: 11.6ms ± 2.1ms (localhost, no network latency)
- **Network overhead**: ~206ms (94% of total time for remote deployment)
- **Boot time**: 37ms on Unikraft Cloud
- **Memory usage**: 128 MiB
- **Compilation**: Full TypeScript parsing, type checking, and JavaScript emission

## Architecture

- **Go module**: `github.com/microsoft/typescript-go/serverexample`
- **Dependencies**: Uses vendored TypeScript-Go internal packages
- **File system**: Custom in-memory VFS implementation
- **Compiler**: Full TypeScript type checking and emission

## Files

- `server.go` - Main HTTP server implementation
- `Dockerfile` - Multi-stage build for static PIE binary
- `Kraftfile` - Unikraft Cloud deployment configuration
- `go.mod` - Go module definition with TypeScript-Go dependency
- `vendor/` - Vendored dependencies for offline builds
