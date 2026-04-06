# tsgo-bench: Go CLI Benchmark Tool

## What it does

A Go CLI binary that benchmarks the v3 endpoints. Takes a URL and a private key. Runs all benchmarks. Prints results.

```bash
tsgo-bench --url https://server.fly.dev --key <base58-ecdsa-p256-private-key>
```

## How it works

1. Builds multipart/form-data request bodies from hardcoded fixtures (same fixtures as bench_fixtures_test.go: trivial, small component, medium component, multi-file)
2. For each fixture, sends authenticated requests to `/v3/typecheck` and `/v3/compile`
3. Signs each request with RFC 9421 HTTP Message Signatures (ECDSA P-256, content-digest for POST bodies)
4. Measures response time for each request across multiple iterations
5. Prints a summary table

## Authentication

- `--key` is a base58-encoded ECDSA P-256 private key
- Signs: `@method`, `@path`, `@authority`, `content-digest`
- Uses `dadrus/httpsig` (same library as the server)
- If `--key` is omitted, requests are sent unsigned (for servers without auth)

## Fixtures

Hardcoded in the binary. Same test cases as the existing Go benchmarks:
- Trivial: `export const x: number = 1;`
- Small component: React component with @crayonnow/core
- Medium component: Meditation timer
- Multi-file: 5 interconnected files

Each fixture includes a package.json (with deps + resolve-s3), bun.lock, tsconfig.json, and source files. The multipart body is built in Go.

## Output

```
tsgo-bench v3 benchmarks
Target: https://server.fly.dev
Auth:   enabled (ECDSA P-256)

Fixture              Endpoint       Avg       Min       Max       Runs
Trivial              /v3/typecheck  12.3ms    10.1ms    15.2ms    20
Trivial              /v3/compile    8.7ms     7.2ms     11.1ms    20
SmallComponent       /v3/typecheck  14.5ms    12.3ms    18.7ms    20
SmallComponent       /v3/compile    10.2ms    8.8ms     13.4ms    20
MediumComponent      /v3/typecheck  18.3ms    15.6ms    22.1ms    20
MediumComponent      /v3/compile    13.1ms    11.2ms    16.8ms    20
MultiFile            /v3/typecheck  22.7ms    19.4ms    27.3ms    20
MultiFile            /v3/compile    16.5ms    14.1ms    20.2ms    20
```

## CLI flags

| Flag | Required | Description |
|------|----------|-------------|
| `--url` | yes | Server base URL |
| `--key` | no | Base58-encoded P-256 private key. Unsigned if omitted. |
| `--runs` | no | Iterations per benchmark (default 20) |
| `--warmup` | no | Warmup iterations (default 3) |

## Location

`server/cmd/tsgo-bench/main.go`

Built with `go build ./cmd/tsgo-bench` from the server directory.

## Dependencies

Reuses existing server dependencies: `dadrus/httpsig`, `mr-tron/base58`. No new deps.
