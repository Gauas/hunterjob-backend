package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hunterjob/hunterjob/api/pkg/authn"
)

func TestRequireVerifiedBearerAndIgnoreIdentityHeaders(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kid": "key-1", "kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(public)}}})
	}))
	defer jwks.Close()
	verifier, err := authn.NewVerifier(authn.Config{JWKSURL: jwks.URL, Issuer: "gauas-auth", Audience: "gauas-api"})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/private", (Middleware{Verifier: verifier}).Require(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": UserID(c)})
	})
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set("X-Gauas-User-Key", "forged")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("forged identity header: status = %d", response.Code)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub": "7f5eb9c0-7a1a-4f3e-9a32-779c6bd0b831", "sid": "42", "device_id": "device-1", "jti": "jti-1",
		"iss": "gauas-auth", "aud": "gauas-api", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	})
	token.Header["kid"] = "key-1"
	raw, err := token.SignedString(private)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	request.Header.Set("X-Gauas-User-Key", "forged")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"user_id":"7f5eb9c0-7a1a-4f3e-9a32-779c6bd0b831"}` {
		t.Fatalf("verified token: status = %d, body = %s", response.Code, response.Body.String())
	}
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer unavailable.Close()
	brokenVerifier, err := authn.NewVerifier(authn.Config{JWKSURL: unavailable.URL, Issuer: "gauas-auth", Audience: "gauas-api"})
	if err != nil {
		t.Fatal(err)
	}
	brokenRouter := gin.New()
	brokenRouter.GET("/private", (Middleware{Verifier: brokenVerifier}).Require(), func(c *gin.Context) { c.Status(http.StatusOK) })
	response = httptest.NewRecorder()
	brokenRouter.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("JWKS unavailable: status = %d", response.Code)
	}
}
