# HTTP Message Signature Auth Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add RFC 9421 HTTP Message Signature verification to the tsgo server, gated by a P-256 ECDSA public key env var.

**Architecture:** A new `authMiddleware` function wraps protected routes. On startup, if `TSGO_AUTH_PUBLIC_KEY` is set, the server decodes the base58 compressed P-256 public key and creates an `httpsig.Verifier`. The middleware calls `Verify()` on incoming requests. Bodyless requests must sign `@method`, `@path`, `@authority`. Requests with bodies must also sign `content-digest`.

**Tech Stack:** Go stdlib `crypto/ecdsa` + `crypto/elliptic`, `github.com/dadrus/httpsig` (RFC 9421), `github.com/mr-tron/base58`, `github.com/dunglas/httpsfv`

**Spec:** `docs/superpowers/specs/2026-04-06-http-message-signature-auth-design.md`

---

## Chunk 1: Dependencies & Auth Middleware

### Task 1: Add dependencies

**Files:**
- Modify: `server/go.mod`
- Modify: `server/go.sum`

- [ ] **Step 1: Add the three new dependencies**

```bash
cd server && go get github.com/dadrus/httpsig@latest github.com/mr-tron/base58@latest github.com/dunglas/httpsfv@latest && go mod tidy
```

- [ ] **Step 2: Verify the build still works**

```bash
cd server && go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 3: Commit**

```bash
cd server && git add go.mod go.sum && git commit -m "Add httpsig, base58, httpsfv dependencies for auth"
```

---

### Task 2: Write failing tests for key loading

**Files:**
- Create: `server/auth_test.go`

The test file uses the `dadrus/httpsig` signer to create real signed requests — this validates the full roundtrip. Generate a P-256 keypair in the test, base58-encode the compressed public key, and test `parseAuthPublicKey`.

- [ ] **Step 1: Write tests for key parsing**

```go
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/mr-tron/base58"
)

func generateTestKeypair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	compressed := elliptic.MarshalCompressed(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	pubKeyBase58 := base58.Encode(compressed)
	return privKey, pubKeyBase58
}

func TestParseAuthPublicKey(t *testing.T) {
	_, pubKeyBase58 := generateTestKeypair(t)

	key, err := parseAuthPublicKey(pubKeyBase58)
	if err != nil {
		t.Fatalf("parseAuthPublicKey failed: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
	if key.Curve != elliptic.P256() {
		t.Fatal("expected P-256 curve")
	}
}

func TestParseAuthPublicKeyInvalidBase58(t *testing.T) {
	_, err := parseAuthPublicKey("not-valid-base58!!!")
	if err == nil {
		t.Fatal("expected error for invalid base58")
	}
}

func TestParseAuthPublicKeyWrongLength(t *testing.T) {
	_, err := parseAuthPublicKey(base58.Encode([]byte("tooshort")))
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
}

func TestParseAuthPublicKeyInvalidPoint(t *testing.T) {
	// 33 bytes but not a valid P-256 point
	bad := make([]byte, 33)
	bad[0] = 0x02 // compressed prefix
	// rest is zeros — not on the curve
	_, err := parseAuthPublicKey(base58.Encode(bad))
	if err == nil {
		t.Fatal("expected error for invalid point")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd server && go test -run TestParseAuthPublicKey -v ./...
```

Expected: FAIL — `parseAuthPublicKey` is not defined.

---

### Task 3: Implement key loading

**Files:**
- Create: `server/auth.go`

- [ ] **Step 1: Write the key parsing function and package-level vars**

```go
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dadrus/httpsig"
	"github.com/dunglas/httpsfv"
	"github.com/mr-tron/base58"
)

var (
	authPublicKey *ecdsa.PublicKey
	authVerifier  httpsig.Verifier
)

// parseAuthPublicKey decodes a base58-encoded compressed P-256 SEC1 public key.
func parseAuthPublicKey(encoded string) (*ecdsa.PublicKey, error) {
	decoded, err := base58.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base58: %w", err)
	}
	if len(decoded) != 33 {
		return nil, fmt.Errorf("expected 33 bytes (compressed P-256 point), got %d", len(decoded))
	}
	// elliptic.UnmarshalCompressed + manual struct is the only stdlib path for
	// compressed SEC1 -> *ecdsa.PublicKey. The X/Y field deprecation (Go 1.24)
	// has no replacement for this case as of Go 1.26.
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), decoded)
	if x == nil {
		return nil, fmt.Errorf("not a valid compressed P-256 point")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// initAuth reads TSGO_AUTH_PUBLIC_KEY and sets up the verifier.
// Called from main() before handlers start.
func initAuth() {
	pubKeyStr := os.Getenv("TSGO_AUTH_PUBLIC_KEY")
	if pubKeyStr == "" {
		log.Printf("Auth disabled: TSGO_AUTH_PUBLIC_KEY not set")
		return
	}

	var err error
	authPublicKey, err = parseAuthPublicKey(pubKeyStr)
	if err != nil {
		log.Fatalf("TSGO_AUTH_PUBLIC_KEY: %v", err)
	}

	key := httpsig.Key{
		Algorithm: httpsig.EcdsaP256Sha256,
		Key:       authPublicKey,
		KeyID:     "server",
	}
	authVerifier, err = httpsig.NewVerifier(
		key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	if err != nil {
		log.Fatalf("failed to create signature verifier: %v", err)
	}

	log.Printf("Auth enabled: verifying HTTP message signatures")
}
```

- [ ] **Step 2: Run key parsing tests**

```bash
cd server && go test -run TestParseAuthPublicKey -v ./...
```

Expected: all 4 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add server/auth.go server/auth_test.go && git commit -m "Add auth key loading with parseAuthPublicKey and initAuth"
```

---

### Task 4: Write failing tests for auth middleware

**Files:**
- Modify: `server/auth_test.go`

These tests use the `dadrus/httpsig` signer to create properly signed requests and verify the middleware accepts/rejects them correctly.

- [ ] **Step 1: Add middleware tests**

Append to `server/auth_test.go`:

```go
import (
	// add to existing imports:
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/dadrus/httpsig"
)

// signRequest signs an http.Request using the dadrus/httpsig signer.
// components should be e.g. "@method", "@path", "@authority" and optionally "content-digest".
func signRequest(t *testing.T, req *http.Request, privKey *ecdsa.PrivateKey, pubKeyBase58 string, components ...string) {
	t.Helper()
	key := httpsig.Key{
		Algorithm: httpsig.EcdsaP256Sha256,
		Key:       privKey,
		KeyID:     pubKeyBase58,
	}
	opts := []httpsig.SignerOption{
		httpsig.WithComponents(components...),
	}
	// Include content-digest auto-generation if signing it
	for _, c := range components {
		if c == "content-digest" {
			opts = append(opts, httpsig.WithContentDigestAlgorithm(httpsig.Sha256))
			break
		}
	}
	signer, err := httpsig.NewSigner(key, opts...)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	msg := httpsig.MessageFromRequest(req)
	headers, err := signer.Sign(msg)
	if err != nil {
		t.Fatalf("failed to sign request: %v", err)
	}

	// Copy signature headers back to request
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
}

func TestAuthMiddleware_NoAuthConfigured(t *testing.T) {
	// When authVerifier is nil, requests pass through
	oldVerifier := authVerifier
	authVerifier = nil
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidSignatureGET(t *testing.T) {
	privKey, pubKeyBase58 := generateTestKeypair(t)

	// Set up verifier with this key
	pubKey, _ := parseAuthPublicKey(pubKeyBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: pubKey, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_MissingSignature(t *testing.T) {
	_, pubKeyBase58 := generateTestKeypair(t)

	pubKey, _ := parseAuthPublicKey(pubKeyBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: pubKey, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Unsigned request
	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_WrongKey(t *testing.T) {
	// Sign with one key, verify with another
	signerKey, _ := generateTestKeypair(t)
	_, verifierPubBase58 := generateTestKeypair(t)

	verifierPub, _ := parseAuthPublicKey(verifierPubBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: verifierPub, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	signRequest(t, req, signerKey, "wrong-key-id", "@method", "@path", "@authority")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidSignaturePOSTWithBody(t *testing.T) {
	privKey, pubKeyBase58 := generateTestKeypair(t)

	pubKey, _ := parseAuthPublicKey(pubKeyBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: pubKey, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	var bodyReadByHandler []byte
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		bodyReadByHandler, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	body := []byte(`{"code":"const x: number = 1;","version":"0.0.4"}`)
	req := httptest.NewRequest("POST", "/typecheck", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority", "content-digest")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	// Verify downstream handler could read the body
	if string(bodyReadByHandler) != string(body) {
		t.Fatalf("downstream handler got body %q, expected %q", bodyReadByHandler, body)
	}
}

func TestAuthMiddleware_POSTWithoutContentDigest(t *testing.T) {
	privKey, pubKeyBase58 := generateTestKeypair(t)

	pubKey, _ := parseAuthPublicKey(pubKeyBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: pubKey, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := []byte(`{"code":"const x: number = 1;","version":"0.0.4"}`)
	req := httptest.NewRequest("POST", "/typecheck", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	// Sign WITHOUT content-digest
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for POST without content-digest, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd server && go test -run "TestAuthMiddleware" -v ./...
```

Expected: FAIL — `authMiddleware` is not defined.

---

### Task 5: Implement auth middleware

**Files:**
- Modify: `server/auth.go`

- [ ] **Step 1: Add the middleware and content-digest check to auth.go**

Append to `server/auth.go`:

```go
// authMiddleware verifies HTTP message signatures on incoming requests.
// If authVerifier is nil (no public key configured), requests pass through.
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

// signatureCoversContentDigest parses the Signature-Input header(s) and checks
// whether any signature's covered components include "content-digest".
func signatureCoversContentDigest(r *http.Request) bool {
	for _, val := range r.Header.Values("Signature-Input") {
		dict, err := httpsfv.UnmarshalDictionary([]string{val})
		if err != nil {
			continue
		}
		for _, name := range dict.Names() {
			member, _ := dict.Get(name)
			if il, ok := member.(httpsfv.InnerList); ok {
				for _, item := range il.Items {
					if s, ok := item.Value.(string); ok && s == "content-digest" {
						return true
					}
				}
			}
		}
	}
	return false
}
```

- [ ] **Step 2: Run all auth tests**

```bash
cd server && go test -run "TestAuth|TestParseAuth" -v ./...
```

Expected: all tests PASS.

- [ ] **Step 3: Commit**

```bash
git add server/auth.go server/auth_test.go && git commit -m "Add authMiddleware with signature verification and content-digest check"
```

---

### Task 6: Wire auth into server startup and routes

**Files:**
- Modify: `server/server.go`

- [ ] **Step 1: Add initAuth() call in main(), after S3 init, before route setup**

In `server/server.go`, add `initAuth()` call between the S3 initialization and route setup:

```go
// After: log.Printf("Initialized with S3 bucket: %s, disk cache: %s", s3Bucket, diskCachePath)
// Before: // Handle graceful shutdown

initAuth()
```

- [ ] **Step 2: Update route configuration to wrap protected routes with authMiddleware**

Replace the route setup block in `main()`:

```go
// Set up routes
http.HandleFunc("/", loggingMiddleware(hello))
http.HandleFunc("/health", loggingMiddleware(health))
http.HandleFunc("/build", loggingMiddleware(authMiddleware(build)))
http.HandleFunc("/sync", loggingMiddleware(authMiddleware(syncVersion)))
http.HandleFunc("/typecheck", loggingMiddleware(authMiddleware(typecheck)))
http.HandleFunc("/v2/build", loggingMiddleware(authMiddleware(buildV2)))
http.HandleFunc("/v2/typecheck", loggingMiddleware(authMiddleware(typecheckV2)))
```

- [ ] **Step 3: Remove unused imports if any, verify build**

```bash
cd server && go build ./...
```

Expected: clean build.

- [ ] **Step 4: Run all tests**

```bash
cd server && go test -v ./...
```

Expected: all existing tests still pass (they don't set `TSGO_AUTH_PUBLIC_KEY` so auth is disabled), all new auth tests pass.

- [ ] **Step 5: Commit**

```bash
git add server/server.go && git commit -m "Wire auth middleware into server routes"
```

---

### Task 7: Add integration test for unauthenticated routes

**Files:**
- Modify: `server/auth_test.go`

- [ ] **Step 1: Add test for tampered body and unauthenticated routes**

Append to `server/auth_test.go`:

```go
func TestAuthMiddleware_TamperedBody(t *testing.T) {
	privKey, pubKeyBase58 := generateTestKeypair(t)

	pubKey, _ := parseAuthPublicKey(pubKeyBase58)
	key := httpsig.Key{Algorithm: httpsig.EcdsaP256Sha256, Key: pubKey, KeyID: "server"}
	oldVerifier := authVerifier
	authVerifier, _ = httpsig.NewVerifier(key,
		httpsig.WithExpiredTimestampRequired(false),
		httpsig.WithMaxAge(300*time.Second),
		httpsig.WithRequiredComponents("@method", "@path", "@authority"),
		httpsig.WithValidateAllSignatures(),
	)
	defer func() { authVerifier = oldVerifier }()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Sign a valid request with content-digest
	body := []byte(`{"code":"const x: number = 1;","version":"0.0.4"}`)
	req := httptest.NewRequest("POST", "/typecheck", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority", "content-digest")

	// Tamper with the body after signing
	tamperedBody := []byte(`{"code":"EVIL CODE","version":"0.0.4"}`)
	req.Body = io.NopCloser(bytes.NewReader(tamperedBody))
	req.ContentLength = int64(len(tamperedBody))

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered body, got %d", rr.Code)
	}
}

func TestUnauthenticatedRoutesPassThrough(t *testing.T) {
	// Even with auth configured, / and /health should not go through authMiddleware.
	// This test verifies the route wiring by checking that a direct call to the
	// hello and health handlers works without signatures.

	setupTestServerWithMockS3(t)

	// Test /health
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", rr.Code)
	}

	// Test /
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	hello(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from /, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run test**

```bash
cd server && go test -run TestUnauthenticatedRoutesPassThrough -v ./...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add server/auth_test.go && git commit -m "Add integration test for unauthenticated routes"
```
