package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireTrustedIdentityHeaders(t *testing.T) {
	const userID = "7f5eb9c0-7a1a-4f3e-9a32-779c6bd0b831"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/private", (Middleware{}).Require(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": UserID(c)})
	})
	for _, tc := range []struct {
		name, key, device, session string
		want                       int
	}{
		{"missing headers", "", "", "", http.StatusUnauthorized},
		{"invalid key", "attacker", "device", "session", http.StatusUnauthorized},
		{"missing session", userID, "device", "", http.StatusUnauthorized},
		{"valid identity", userID, "device", "session", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/private", nil)
			request.Header.Set("X-Gauas-User-Key", tc.key)
			request.Header.Set("X-Gauas-Device-ID", tc.device)
			request.Header.Set("X-Gauas-Session-ID", tc.session)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
			if tc.want == http.StatusOK && response.Body.String() != `{"user_id":"`+userID+`"}` {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}
