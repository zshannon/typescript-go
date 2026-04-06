package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dadrus/httpsig"
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

func TestParseAuthPublicKeyWrongLength(t *testing.T) {
	_, err := parseAuthPublicKey(base58.Encode([]byte("tooshort")))
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
}

// signRequest signs an http.Request using the dadrus/httpsig signer.
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

	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
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

	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_NoAuthConfigured(t *testing.T) {
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
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for POST without content-digest, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidSignatureGET(t *testing.T) {
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

	req := httptest.NewRequest("GET", "/build", nil)
	req.Host = "localhost"
	signRequest(t, req, privKey, pubKeyBase58, "@method", "@path", "@authority")

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
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
	if string(bodyReadByHandler) != string(body) {
		t.Fatalf("downstream handler got body %q, expected %q", bodyReadByHandler, body)
	}
}

func TestAuthMiddleware_WrongKey(t *testing.T) {
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
	setupTestServerWithMockS3(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	hello(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from /, got %d", rr.Code)
	}
}
