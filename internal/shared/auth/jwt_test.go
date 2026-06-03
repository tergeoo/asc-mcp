package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testKeyPEM(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), key
}

func TestTokenClaims(t *testing.T) {
	pemKey, key := testKeyPEM(t)
	p, err := NewProvider("issuer-123", "KEY123", pemKey)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	tok, err := p.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	parsed, err := jwt.Parse(tok, func(tk *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if kid, _ := parsed.Header["kid"].(string); kid != "KEY123" {
		t.Fatalf("kid header = %q, want KEY123", kid)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["iss"] != "issuer-123" {
		t.Fatalf("iss = %v", claims["iss"])
	}
	if claims["aud"] != audience {
		t.Fatalf("aud = %v, want %s", claims["aud"], audience)
	}
}

func TestTokenCachedAndRefreshed(t *testing.T) {
	pemKey, _ := testKeyPEM(t)
	p, err := NewProvider("issuer", "kid", pemKey)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return base }

	first, _ := p.Token()
	second, _ := p.Token()
	if first != second {
		t.Fatal("token should be cached when not near expiry")
	}

	// Advance beyond the refresh window; a new token must be minted.
	p.now = func() time.Time { return base.Add(tokenTTL) }
	third, _ := p.Token()
	if third == first {
		t.Fatal("token should refresh past the skew window")
	}
}

func TestNewProviderValidatesInput(t *testing.T) {
	pemKey, _ := testKeyPEM(t)
	if _, err := NewProvider("", "kid", pemKey); err == nil {
		t.Fatal("expected error for empty issuer")
	}
	if _, err := NewProvider("iss", "kid", []byte("not a key")); err == nil {
		t.Fatal("expected error for bad key")
	}
}
