# TypeScript Compiler Server

A high-performance HTTP server that provides TypeScript type checking and JavaScript compilation using the TypeScript-Go compiler and esbuild.

## Features

- **TypeScript type checking**: Full TypeScript type checking with detailed diagnostics
- **JavaScript bundling**: Uses esbuild for fast, optimized JavaScript output
- **React support**: Built-in React global transform for JSX
- **In-memory compilation**: No file system I/O required
- **Module caching**: Node modules loaded once at startup for fast performance
- **High performance**: ~155ms average build time, ~208ms typecheck (including network latency)

## API

### `GET /`
Returns server information.

**Response:**
```
TypeScript Go Server
```

### `GET /health`
Returns server health and statistics.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "26s",
  "modules": {
    "TotalFiles": 2557,
    "TypeDefinitions": 482,
    "JavaScriptFiles": 2056,
    "PackageFiles": 19,
    "LoadErrors": 0
  }
}
```

### `POST /typecheck`
Type checks TypeScript code without compilation.

**Request:**
```json
{
  "code": "export const hello: string = 123"
}
```

**Success Response:**
```json
{
  "pass": true
}
```

**Error Response:**
```json
{
  "errors": [
    {
      "message": "Type 'number' is not assignable to type 'string'.",
      "line": 1,
      "column": 30
    }
  ]
}
```

### `POST /build`
Compiles and bundles TypeScript code to JavaScript.

**Query Parameters:**
- `validate_types` (optional): Set to `true` to run type checking before building. If type errors are found, the build will fail and return the type errors instead of building.

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

**Error Response (build errors):**
```json
{
  "errors": [
    {
      "message": "Module not found",
      "line": 1,
      "column": 7
    }
  ]
}
```

**With Type Validation (`/build?validate_types=true`):**

If type errors are found, returns them without attempting to build:
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

### Fly.io

```bash
# Deploy using 1Password CLI for secure environment variables
op run --env-file=".env.op" -- sh -c 'cd .. && fly deploy --config server/fly.toml --dockerfile server/Dockerfile --build-secret GITHUB_TOKEN="$GITHUB_TOKEN"'

# Or using npm scripts
npm run deploy:server
```

### Docker

```bash
# Build Docker image
npm run build:docker

# Run locally
docker run -p 8080:8080 typescript-server
```

### Local Development

```bash
go run server.go
```

## Performance

### Benchmark Commands

**Fly.io (deployed):**
```bash
# Create test file
cat > test-fortune-cookie.json << 'EOF'
{
  "code": "import { Button, Flex, Text } from '@crayonnow/core';\nimport { useState } from 'react';\n\nconst fortunes = [\n  \"A beautiful, smart, and loving person will be coming into your life.\",\n  \"Believe it can be done.\",\n  \"Your ability to overcome challenges will set you up for success.\"\n];\n\nexport default () => {\n  const [currentFortune, setCurrentFortune] = useState(fortunes[0]);\n\n  return (\n    <Flex style={{ padding: '20px' }}>\n      <Text>{currentFortune}</Text>\n      <Button onClick={() => setCurrentFortune(fortunes[Math.floor(Math.random() * fortunes.length)])}>\n        Get New Fortune\n      </Button>\n    </Flex>\n  );\n};"
}
EOF

# Benchmark both endpoints
hyperfine --warmup 3 --min-runs 10 \
  'curl -s -X POST https://server-wild-sea-9370.fly.dev/typecheck -H "Content-Type: application/json" -d @test-fortune-cookie.json' \
  'curl -s -X POST https://server-wild-sea-9370.fly.dev/build -H "Content-Type: application/json" -d @test-fortune-cookie.json'
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

**Fly.io:**
```
Benchmark 1: curl -s -X POST https://server-wild-sea-9370.fly.dev/typecheck -H "Content-Type: application/json" -d @test-fortune-cookie.json
  Time (mean ± σ):     212.1 ms ±  15.8 ms    [User: 10.4 ms, System: 7.1 ms]
  Range (min … max):   190.3 ms … 246.7 ms    11 runs

Benchmark 2: curl -s -X POST https://server-wild-sea-9370.fly.dev/build -H "Content-Type: application/json" -d @test-fortune-cookie.json
  Time (mean ± σ):     146.6 ms ±  15.9 ms    [User: 8.8 ms, System: 5.5 ms]
  Range (min … max):   125.5 ms … 187.3 ms    22 runs
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

- **Local development**: 11.6ms ± 2.1ms (localhost, no network latency)
- **Fly.io**: 64.4ms ± 6.2ms (including network latency to sjc)
- **Unikraft Cloud**: 218ms ± 84ms (including network latency to dal0)
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
