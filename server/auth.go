package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dadrus/httpsig"
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
	if x == nil || x.Sign() == 0 {
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
