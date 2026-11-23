// Package authn verifies identity-service access tokens without a per-request
// network call. It has no dependency on an HTTP framework or application roles.
package authn

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Identity struct {
	UserID    string
	SessionID string
	DeviceID  string
}

var ErrJWKSUnavailable = errors.New("JWKS is unavailable")

type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

type Config struct {
	JWKSURL  string
	Issuer   string
	Audience string
	TTL      time.Duration
	Client   *http.Client
}

type Verifier struct {
	url      string
	issuer   string
	audience string
	ttl      time.Duration
	client   *http.Client
	mu       sync.RWMutex // refresh uses the write lock to coalesce requests
	keys     map[string]ed25519.PublicKey
	expires  time.Time
	lastMiss time.Time
}

type claims struct {
	SessionID string `json:"sid"`
	DeviceID  string `json:"device_id"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []struct {
		ID        string `json:"kid"`
		Type      string `json:"kty"`
		Curve     string `json:"crv"`
		Algorithm string `json:"alg"`
		X         string `json:"x"`
	} `json:"keys"`
}

func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.JWKSURL == "" || cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("JWKS URL, issuer and audience are required")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 5 * time.Minute
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 2 * time.Second}
	}
	return &Verifier{url: cfg.JWKSURL, issuer: cfg.Issuer, audience: cfg.Audience, ttl: cfg.TTL, client: cfg.Client}, nil
}

func Bearer(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("bearer token is required")
	}
	return parts[1], nil
}

func (v *Verifier) Verify(ctx context.Context, raw string) (Identity, error) {
	parsedClaims := new(claims)
	parsed, err := jwt.ParseWithClaims(raw, parsedClaims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, errors.New("EdDSA is required")
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || strings.TrimSpace(kid) == "" {
			return nil, errors.New("kid is required")
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}), jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return Identity{}, fmt.Errorf("invalid access token: %w", err)
	}
	if !parsed.Valid {
		return Identity{}, errors.New("invalid access token")
	}
	if parsedClaims.Subject == "" || parsedClaims.SessionID == "" || parsedClaims.DeviceID == "" || parsedClaims.ID == "" || parsedClaims.IssuedAt == nil {
		return Identity{}, errors.New("required identity claim is missing")
	}
	return Identity{UserID: parsedClaims.Subject, SessionID: parsedClaims.SessionID, DeviceID: parsedClaims.DeviceID}, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	v.mu.RLock()
	key, found := v.keys[kid]
	fresh := time.Now().Before(v.expires)
	v.mu.RUnlock()
	if found && fresh {
		return key, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	key, found = v.keys[kid]
	if found && time.Now().Before(v.expires) {
		return key, nil
	}
	if !found && time.Now().Before(v.expires) && time.Since(v.lastMiss) < 10*time.Second {
		return nil, errors.New("unknown signing key")
	}
	if !found || !time.Now().Before(v.expires) {
		if !found {
			v.lastMiss = time.Now()
		}
		if err := v.refresh(ctx); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrJWKSUnavailable, err) // fail closed, including when a cached key is stale
		}
		key, found = v.keys[kid]
		if found {
			v.lastMiss = time.Time{}
		}
	}
	if !found {
		return nil, errors.New("unknown signing key")
	}
	return key, nil
}

func (v *Verifier) refresh(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return err
	}
	response, err := v.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS returned HTTP %d", response.StatusCode)
	}
	var document jwksDocument
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&document); err != nil {
		return err
	}
	keys := make(map[string]ed25519.PublicKey, len(document.Keys))
	for _, candidate := range document.Keys {
		if candidate.ID == "" || candidate.Type != "OKP" || candidate.Curve != "Ed25519" || candidate.Algorithm != "EdDSA" {
			continue
		}
		encoded, err := base64.RawURLEncoding.DecodeString(candidate.X)
		if err != nil || len(encoded) != ed25519.PublicKeySize {
			continue
		}
		keys[candidate.ID] = ed25519.PublicKey(encoded)
	}
	if len(keys) == 0 {
		return errors.New("JWKS contains no Ed25519 signing key")
	}
	v.keys, v.expires = keys, time.Now().Add(v.ttl)
	return nil
}
