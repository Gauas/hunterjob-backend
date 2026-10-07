package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifierCachesJWKSAndRejectsInvalidClaims(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kid": "key-1", "kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(public),
		}}})
	}))
	defer jwks.Close()
	verifier, err := NewVerifier(Config{JWKSURL: jwks.URL, Issuer: "gauas-auth", Audience: "gauas-api"})
	if err != nil {
		t.Fatal(err)
	}
	sign := func(audience string, expiry time.Time) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"sub": "user-1", "sid": "42", "device_id": "device-1", "jti": "jti-1",
			"iss": "gauas-auth", "aud": audience, "iat": time.Now().Add(-time.Minute).Unix(), "exp": expiry.Unix(),
		})
		token.Header["kid"] = "key-1"
		raw, err := token.SignedString(private)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	valid := sign("gauas-api", time.Now().Add(time.Minute))
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			identity, err := verifier.Verify(context.Background(), valid)
			if err != nil || identity.UserID != "user-1" || identity.SessionID != "42" {
				t.Errorf("verify: identity=%+v, err=%v", identity, err)
			}
		}()
	}
	group.Wait()
	if fetches.Load() != 1 {
		t.Fatalf("JWKS fetches = %d, want 1", fetches.Load())
	}
	for _, raw := range []string{sign("wrong-audience", time.Now().Add(time.Minute)), sign("gauas-api", time.Now().Add(-time.Minute))} {
		if _, err := verifier.Verify(context.Background(), raw); err == nil {
			t.Fatal("invalid JWT accepted")
		}
	}
	wrongAlgorithm := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1", "sid": "42", "device_id": "device-1", "jti": "jti-1",
		"iss": "gauas-auth", "aud": "gauas-api", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	})
	wrongAlgorithm.Header["kid"] = "key-1"
	forged, err := wrongAlgorithm.SignedString([]byte("not-an-ed25519-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), forged); err == nil {
		t.Fatal("non-EdDSA token accepted")
	}
}

func TestVerifierRefreshesUnknownKidForRotation(t *testing.T) {
	oldPublic, oldPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newPublic, newPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var rotated atomic.Bool
	var fetches atomic.Int32
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		keys := []any{map[string]string{"kid": "old", "kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(oldPublic)}}
		if rotated.Load() {
			keys = append(keys, map[string]string{"kid": "new", "kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(newPublic)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	defer jwks.Close()
	verifier, err := NewVerifier(Config{JWKSURL: jwks.URL, Issuer: "gauas-auth", Audience: "gauas-api"})
	if err != nil {
		t.Fatal(err)
	}
	sign := func(kid string, key ed25519.PrivateKey) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"sub": "user-1", "sid": "42", "device_id": "device-1", "jti": "jti-1",
			"iss": "gauas-auth", "aud": "gauas-api", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		})
		token.Header["kid"] = kid
		raw, err := token.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if _, err := verifier.Verify(context.Background(), sign("old", oldPrivate)); err != nil {
		t.Fatal(err)
	}
	rotated.Store(true)
	if _, err := verifier.Verify(context.Background(), sign("new", newPrivate)); err != nil {
		t.Fatal(err)
	}
	if fetches.Load() != 2 {
		t.Fatalf("JWKS fetches = %d, want 2 after rotation", fetches.Load())
	}
	if _, err := verifier.Verify(context.Background(), sign("old", oldPrivate)); err != nil {
		t.Fatalf("old verify-only key was rejected: %v", err)
	}
}
