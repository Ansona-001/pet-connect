// Package oidc verifies OpenID Connect ID tokens against a provider's
// published JSON Web Key Set. It's deliberately provider-agnostic: Google
// (Day 16) and Apple (Day 17, per ADR 0005's recorded provider defaults)
// both issue standard RS256 ID tokens verified the same three ways
// (signature via JWKS, issuer, audience) — only the JWKS URL, issuer
// string(s), and audience (the app's client id) differ between them.
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the subset of standard ID-token claims callers need for
// account linking — see auth.linkOrCreateOAuthAccount.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Verifier checks an ID token's signature against a cached JWKS, and
// validates its issuer, audience, and expiry.
type Verifier struct {
	jwksURL    string
	issuers    []string
	audience   string
	httpClient *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewVerifier builds a verifier for one provider. audience is normally
// that provider's OAuth client id for this app; issuers lists every
// exact `iss` value the provider is known to use (some providers have
// used more than one form historically, e.g. with and without a scheme).
func NewVerifier(jwksURL, audience string, issuers ...string) *Verifier {
	return &Verifier{
		jwksURL:    jwksURL,
		issuers:    issuers,
		audience:   audience,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keys:       map[string]*rsa.PublicKey{},
	}
}

// Verify parses and validates idToken, returning the claims callers need.
// A failure here (bad signature, wrong audience/issuer, expired) always
// means the token is rejected outright — there's no partial-trust path.
func (v *Verifier) Verify(ctx context.Context, idToken string) (Claims, error) {
	parsed, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("id token is missing a key id")
		}
		return v.key(ctx, kid)
	}, jwt.WithExpirationRequired(), jwt.WithAudience(v.audience))
	if err != nil {
		return Claims{}, fmt.Errorf("verify id token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("id token is invalid")
	}

	issuer, _ := claims["iss"].(string)
	if !v.issuerAllowed(issuer) {
		return Claims{}, fmt.Errorf("unexpected issuer %q", issuer)
	}

	subject, _ := claims["sub"].(string)
	if subject == "" {
		return Claims{}, errors.New("id token is missing a subject")
	}
	email, _ := claims["email"].(string)
	// Providers encode this as a JSON bool; tolerate a string form too
	// since not every provider is consistent about it.
	verified := claims["email_verified"]
	emailVerified := verified == true || verified == "true"
	name, _ := claims["name"].(string)

	return Claims{Subject: subject, Email: email, EmailVerified: emailVerified, Name: name}, nil
}

func (v *Verifier) issuerAllowed(issuer string) bool {
	for _, allowed := range v.issuers {
		if issuer == allowed {
			return true
		}
	}
	return false
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	key, ok := v.keys[kid]
	stale := time.Since(v.fetchedAt) > time.Hour
	v.mu.Unlock()
	if ok && !stale {
		return key, nil
	}

	if err := v.refreshKeys(ctx); err != nil {
		// A stale-but-present key is still usable if the refresh itself
		// failed (e.g. a transient network blip) — only error out if we
		// have nothing at all for this kid.
		v.mu.Lock()
		key, ok = v.keys[kid]
		v.mu.Unlock()
		if ok {
			return key, nil
		}
		return nil, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("no matching signing key for kid %q", kid)
	}
	return key, nil
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch JWKS: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read JWKS: %w", err)
	}

	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("parse JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("JWKS contained no usable RSA keys")
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func rsaPublicKeyFromJWK(nEncoded, eEncoded string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nEncoded)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eEncoded)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
