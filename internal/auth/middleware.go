package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const userKey = "hunterjob_user_id"

// Require consumes identity headers set by Traefik ForwardAuth. The API must only
// be reachable through that gateway; client-supplied identity headers are unsafe.
type Middleware struct{}

func (Middleware) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := strings.TrimSpace(c.GetHeader("X-Gauas-User-Key"))
		deviceID := strings.TrimSpace(c.GetHeader("X-Gauas-Device-ID"))
		sessionID := strings.TrimSpace(c.GetHeader("X-Gauas-Session-ID"))
		id, err := uuid.Parse(rawKey)
		if err != nil || id == uuid.Nil || deviceID == "" || sessionID == "" || strings.ContainsAny(deviceID+sessionID, "\r\n") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "trusted identity headers are required"})
			return
		}
		c.Set(userKey, id.String())
		c.Next()
	}
}

func UserID(c *gin.Context) string { return c.GetString(userKey) }
