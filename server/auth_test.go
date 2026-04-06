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
