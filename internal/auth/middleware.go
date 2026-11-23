package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/pkg/authn"
)

// Middleware authenticates application requests locally using cached JWKS.
type Middleware struct{ Verifier *authn.Verifier }

func (m Middleware) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.Verifier == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authentication is unavailable"})
			return
		}
		raw, err := authn.Bearer(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bearer token is required"})
			return
		}
		identity, err := m.Verifier.Verify(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, authn.ErrJWKSUnavailable) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authentication keys are unavailable"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}
		c.Request = c.Request.WithContext(authn.WithIdentity(c.Request.Context(), identity))
		c.Next()
	}
}

func UserID(c *gin.Context) string {
	identity, ok := authn.FromContext(c.Request.Context())
	if !ok {
		return ""
	}
	return identity.UserID
}
