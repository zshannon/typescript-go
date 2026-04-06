# HTTP Message Signature Auth

Add RFC 9421 HTTP Message Signature verification to the tsgo server, using a P-256 ECDSA public key provided via environment variable.

## Requirements

- Server verifies requests are signed by a known P-256 private key
- Public key provided as `TSGO_AUTH_PUBLIC_KEY` env var (base58-encoded compressed SEC1 point)
- Auth is optional: if env var is unset, server runs without auth
- Health check (`/health`) and root (`/`) remain unauthenticated
- All other routes require valid signatures
- Requests with bodies must include `Content-Digest` as a signed component
- Signatures expire after 5 minutes, with 30 seconds of clock skew tolerance

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/dadrus/httpsig` | RFC 9421 signature verification (lightweight, depends only on `dunglas/httpsfv`) |
| `github.com/mr-tron/base58` v1.3.0 | Base58 decoding for the public key |
| `github.com/dunglas/httpsfv` | HTTP Structured Field Values parsing (transitive dep of httpsig, used directly for Signature-Input inspection) |

## Startup / Key Loading

On startup in `main()`:

1. Read `TSGO_AUTH_PUBLIC_KEY` from environment
2. If empty, log that auth is disabled. `authVerifier` remains nil.
3. If set:
   a. Base58-decode (plain Bitcoin alphabet) to raw bytes
   b. Validate length is exactly 33 bytes (compressed P-256 SEC1 point)
   c. Parse via `elliptic.UnmarshalCompressed(elliptic.P256(), bytes)` — check `x == nil` for invalid points
   d. Construct `*ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}`
   e. Create `httpsig.Verifier` via `httpsig.NewVerifier(key, ...opts)`
   f. Store both as package-level vars, set once before any handler goroutines start
   g. Log that auth is enabled (do not log the key)

Any failure in steps a-e is fatal (`log.Fatalf`) — the server should not start with a malformed key.

Note: `elliptic.UnmarshalCompressed` + manual struct construction is the only stdlib path for compressed SEC1 to `*ecdsa.PublicKey`. The X/Y field deprecation (Go 1.24) has no replacement for this case as of Go 1.26.

## Package-Level State

```go
var (
    authPublicKey *ecdsa.PublicKey  // nil if auth disabled
    authVerifier  httpsig.Verifier // nil if auth disabled; interface type
)
```

Both are write-once in `main()` before handlers start. No synchronization needed.

## Verifier Configuration

```go
key := httpsig.Key{
    Algorithm: httpsig.EcdsaP256Sha256,
    Key:       authPublicKey,
    KeyID:     "server",
}

authVerifier, err = httpsig.NewVerifier(
    key, // Key satisfies KeyResolver interface via value receiver
    httpsig.WithExpiredTimestampRequired(false),    // maxAge on `created` is sufficient
    httpsig.WithMaxAge(300 * time.Second),          // 5 minute signature lifetime
    httpsig.WithRequiredComponents("@method", "@path", "@authority"),
    httpsig.WithValidateAllSignatures(),
    httpsig.WithValidityTolerance(30 * time.Second), // tolerate 30s clock skew
)
```

Key decisions:
- Algorithm enforcement is inherent in `Key.Algorithm` — no separate option needed
- `WithExpiredTimestampRequired(false)` — clients only need `created`, not `expires`
- `WithValidateAllSignatures()` is required by the library (otherwise it errors on construction)
- `content-digest` is NOT in `WithRequiredComponents` because it can't be conditional on body presence

## Auth Middleware

```go
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if authVerifier == nil {
            next(w, r)
            return
        }

        msg := httpsig.MessageFromRequest(r)
        if err := authVerifier.Verify(msg); err != nil {
            log.Printf("auth: signature verification failed: %v", err)
            http.Error(w, "signature verification failed", http.StatusUnauthorized)
            return
        }

        // For requests with bodies, ensure content-digest was a covered component.
        // WithRequiredComponents can't conditionally require it, so check manually.
        if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
            if !signatureCoversContentDigest(r) {
                log.Printf("auth: request has body but signature does not cover content-digest")
                http.Error(w, "signature verification failed", http.StatusUnauthorized)
                return
            }
        }

        next(w, r)
    }
}
```

The `signatureCoversContentDigest` helper parses `Signature-Input` using `httpsfv.UnmarshalDictionary` and checks whether any signature's inner list items include the string `"content-digest"`. This is safe against false positives from keyid values or other parameters.

## Content-Digest Verification

When `content-digest` is a covered component in the signature, the library automatically:
1. Reads the body via `MessageFromRequest`'s lazy body snapshot
2. Hashes it with the algorithm specified in the `Content-Digest` header (SHA-256 or SHA-512, per RFC 9530)
3. Compares against the header value
4. Fails verification on mismatch

No manual digest code is needed. The library also handles body draining: after `Verify()`, `r.Body` is replaced with a re-readable copy, so downstream handlers can read it normally.

## Route Configuration

```go
// Unauthenticated
http.HandleFunc("/", loggingMiddleware(hello))
http.HandleFunc("/health", loggingMiddleware(health))

// Authenticated — logging wraps auth so failures are still logged
http.HandleFunc("/build", loggingMiddleware(authMiddleware(build)))
http.HandleFunc("/sync", loggingMiddleware(authMiddleware(syncVersion)))
http.HandleFunc("/typecheck", loggingMiddleware(authMiddleware(typecheck)))
http.HandleFunc("/v2/build", loggingMiddleware(authMiddleware(buildV2)))
http.HandleFunc("/v2/typecheck", loggingMiddleware(authMiddleware(typecheckV2)))
```

## Error Handling

All auth failures return `401 Unauthorized` with a generic message. Details are logged server-side for debugging. No information about the failure reason is exposed to the client.

## Behavior Matrix

| Scenario | Result |
|----------|--------|
| `TSGO_AUTH_PUBLIC_KEY` unset | All routes open |
| Valid signature | Request proceeds |
| Missing/invalid signature | 401 |
| POST without content-digest in covered components | 401 |
| Content-digest hash mismatch | 401 |
| Signature older than 5 minutes | 401 |
| Clock skew under 30 seconds | Tolerated |
| GET without content-digest | OK |
| `GET /health` or `GET /` | Always OK |

## Client Requirements

**Bodyless requests** (GET, DELETE, HEAD):
- Sign `@method`, `@path`, `@authority` with `ecdsa-p256-sha256`
- Include `Signature-Input` and `Signature` headers
- Include `created` timestamp parameter in signature params

**Requests with bodies** (POST, PUT, PATCH):
- Compute `Content-Digest: sha-256=:<base64>:` header (RFC 9530)
- Sign `@method`, `@path`, `@authority`, `content-digest` with `ecdsa-p256-sha256`
- Include `Content-Digest`, `Signature-Input`, and `Signature` headers
- Include `created` timestamp parameter in signature params
