package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/auth"
)

type Webhook struct {
	Verify  gin.HandlerFunc
	Receive gin.HandlerFunc
}

type Controller struct {
	Agent    Handler
	Webhooks map[string]Webhook
	Auth     auth.Middleware
	Ready    func(context.Context) error
}

func (ctrl Controller) Register(router *gin.Engine) {
	v := router.Group("/v1/hunterjob")
	v.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	v.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ctrl.Ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	for provider, webhook := range ctrl.Webhooks {
		path := "/webhooks/" + provider
		if webhook.Verify != nil {
			v.GET(path, webhook.Verify)
		}
		v.POST(path, webhook.Receive)
	}
	personal := v.Group("", ctrl.Auth.Require())
	personal.GET("/dashboard", ctrl.Agent.Dashboard)
	personal.GET("/search-preference", ctrl.Agent.GetPreference)
	personal.PUT("/search-preference", ctrl.Agent.PutPreference)
	personal.GET("/matches", ctrl.Agent.ListMatches)
	personal.GET("/matches/:id", ctrl.Agent.GetMatch)
	personal.PATCH("/matches/:id/status", ctrl.Agent.PatchMatchStatus)
	personal.GET("/connections", ctrl.Agent.ListConnections)
	personal.POST("/connections/:provider/connect", ctrl.Agent.Connect)
	personal.POST("/connections/:provider/disconnect", ctrl.Agent.Disconnect)
	personal.PATCH("/connections/:provider", ctrl.Agent.PatchConnection)
}
