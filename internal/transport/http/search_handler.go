package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/application/search"
	"net/http"
	"strings"
)

type SearchHandler struct{ service search.Service }

func NewSearchHandler(s search.Service) SearchHandler { return SearchHandler{service: s} }
func (h SearchHandler) AI(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if e := c.ShouldBindJSON(&req); e != nil || strings.TrimSpace(req.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}
	intent, results, e := h.service.Search(c, req.Query)
	if e != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI search unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"intent": intent, "data": results})
}
