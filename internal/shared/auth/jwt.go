// Package auth provides a thread-safe ES256 JWT provider for App Store Connect.
//
// ASC requires a short-lived (<=20 min) ES256 token with aud=appstoreconnect-v1
// and the key id in the header. We generate tokens lazily, cache them, and
// refresh slightly before expiry. The private key and tokens are never logged.
package auth

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	audience = "appstoreconnect-v1"
	// tokenTTL is kept under Apple's 20-minute ceiling.
	tokenTTL = 19 * time.Minute
	// refreshSkew refreshes the token before it actually expires.
	refreshSkew = 1 * time.Minute
)

// Clock is injectable for deterministic tests.
type Clock func() time.Time

// Provider mints and caches ASC bearer tokens.
type Provider struct {
	issuerID string
	keyID    string
	key      *ecdsa.PrivateKey
	now      Clock

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// NewProvider parses the PEM-encoded .p8 private key and returns a provider.
func NewProvider(issuerID, keyID string, pemKey []byte) (*Provider, error) {
	if issuerID == "" || keyID == "" {
		return nil, errors.New("auth: issuerID and keyID are required")
	}
	key, err := jwt.ParseECPrivateKeyFromPEM(pemKey)
	if err != nil {
		return nil, fmt.Errorf("auth: parse ES256 private key: %w", err)
	}
	return &Provider{
		issuerID: issuerID,
		keyID:    keyID,
		key:      key,
		now:      time.Now,
	}, nil
}

// Token returns a valid bearer token, minting a new one if the cached token is
// missing or near expiry. Safe for concurrent use.
func (p *Provider) Token() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if p.token != "" && now.Before(p.expiresAt.Add(-refreshSkew)) {
		return p.token, nil
	}

	tok, exp, err := p.mint(now)
	if err != nil {
		return "", err
	}
	p.token = tok
	p.expiresAt = exp
	return tok, nil
}

func (p *Provider) mint(now time.Time) (string, time.Time, error) {
	exp := now.Add(tokenTTL)
	claims := jwt.MapClaims{
		"iss": p.issuerID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"aud": audience,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	t.Header["kid"] = p.keyID
	signed, err := t.SignedString(p.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, exp, nil
}
