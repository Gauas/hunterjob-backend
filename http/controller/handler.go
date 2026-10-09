package controller

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/internal/auth"
	"github.com/hunterjob/hunterjob/api/model"
	"github.com/hunterjob/hunterjob/api/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	Store     repository.Store
	Providers agent.ProviderRegistry
}

type preferenceInput struct {
	Role             string   `json:"role"`
	Experience       string   `json:"experience"`
	Locations        []string `json:"locations"`
	Keywords         []string `json:"keywords"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	Frequency        string   `json:"frequency"`
}

func (h Handler) GetPreference(c *gin.Context) {
	p, err := h.Store.Preference(c, auth.UserID(c))
	if err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}
func (h Handler) PutPreference(c *gin.Context) {
	var input preferenceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		bad(c, "invalid request")
		return
	}
	input.Role = strings.TrimSpace(input.Role)
	exp, ok := model.NormalizeExperience(input.Experience)
	if input.Role == "" || len(input.Role) > 120 || !ok {
		bad(c, "role and valid experience are required")
		return
	}
	locations := model.CleanStrings(input.Locations)
	if len(locations) == 0 {
		bad(c, "at least one location is required")
		return
	}
	if len(locations) > 10 || len(input.Keywords) > 30 || len(input.ExcludedKeywords) > 30 {
		bad(c, "too many preference values")
		return
	}
	frequency := strings.ToLower(strings.TrimSpace(input.Frequency))
	if frequency == "" {
		frequency = "daily"
	}
	if frequency != "daily" {
		bad(c, "frequency must be daily")
		return
	}
	p, err := h.Store.SavePreference(c, model.Preference{UserID: auth.UserID(c), Role: input.Role, Experience: exp, Locations: locations, Keywords: model.CleanStrings(input.Keywords), ExcludedKeywords: model.CleanStrings(input.ExcludedKeywords), Frequency: frequency})
	if err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

func (h Handler) Dashboard(c *gin.Context) {
	user := auth.UserID(c)
	p, err := h.Store.Preference(c, user)
	if err != nil {
		serverError(c)
		return
	}
	matches, err := h.Store.Matches(c, user, "", "", nil, 5)
	if err != nil {
		serverError(c)
		return
	}
	connections, err := h.connectionViews(c, user)
	if err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"search_preference": p, "recent_jobs": matches, "connections": connections})
}

func encodeCursor(m model.Match) string {
	b, _ := json.Marshal(struct {
		Time time.Time `json:"t"`
		ID   string    `json:"i"`
	}{m.MatchedAt, m.ID.Hex()})
	return base64.RawURLEncoding.EncodeToString(b)
}
func decodeCursor(value string) (*model.Match, error) {
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	var data struct {
		Time time.Time `json:"t"`
		ID   string    `json:"i"`
	}
	if err = json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	id, err := primitive.ObjectIDFromHex(data.ID)
	if err != nil || data.Time.IsZero() {
		return nil, strconv.ErrSyntax
	}
	return &model.Match{ID: id, MatchedAt: data.Time}, nil
}
func (h Handler) ListMatches(c *gin.Context) {
	limit := int64(20)
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1 || value > 100 {
			bad(c, "limit must be between 1 and 100")
			return
		}
		limit = value
	}
	status, level := c.Query("status"), c.Query("match_level")
	if status != "" && !validStatus(status) {
		bad(c, "invalid status")
		return
	}
	if level != "" && level != "strong" && level != "good" && level != "possible" {
		bad(c, "invalid match_level")
		return
	}
	var before *model.Match
	if cursor := c.Query("cursor"); cursor != "" {
		var err error
		before, err = decodeCursor(cursor)
		if err != nil {
			bad(c, "invalid cursor")
			return
		}
	}
	items, err := h.Store.Matches(c, auth.UserID(c), status, level, before, limit+1)
	if err != nil {
		serverError(c)
		return
	}
	next := ""
	if int64(len(items)) > limit {
		items = items[:limit]
		next = encodeCursor(items[len(items)-1])
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": next})
}
func (h Handler) GetMatch(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		bad(c, "invalid match id")
		return
	}
	m, err := h.Store.Match(c, auth.UserID(c), id)
	if err != nil {
		serverError(c)
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}
func (h Handler) PatchMatchStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		bad(c, "invalid match id")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil || !validStatus(input.Status) {
		bad(c, "invalid status")
		return
	}
	m, err := h.Store.SetMatchStatus(c, auth.UserID(c), id, input.Status)
	if err != nil {
		serverError(c)
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}
func validStatus(value string) bool {
	switch value {
	case "new", "viewed", "saved", "applied", "dismissed":
		return true
	}
	return false
}
func bad(c *gin.Context, message string) { c.JSON(http.StatusBadRequest, gin.H{"error": message}) }
func serverError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "request could not be completed"})
}

func (h Handler) connectionViews(c *gin.Context, user string) ([]model.ConnectionView, error) {
	items, err := h.Store.Connections(c, user)
	if err != nil {
		return nil, err
	}
	byProvider := map[string]model.Connection{}
	for _, item := range items {
		byProvider[item.Provider] = item
	}
	views := make([]model.ConnectionView, 0, len(model.Providers))
	for _, name := range model.Providers {
		item := byProvider[name]
		views = append(views, model.ConnectionView{Provider: name, Available: h.Providers.Available(name), Connected: item.Status == "connected", Enabled: item.Enabled})
	}
	return views, nil
}
func (h Handler) ListConnections(c *gin.Context) {
	items, err := h.connectionViews(c, auth.UserID(c))
	if err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (h Handler) Connect(c *gin.Context) {
	provider := c.Param("provider")
	service := h.Providers.Get(provider)
	if service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "provider is not available"})
		return
	}
	result, err := service.Connect(c, auth.UserID(c))
	if err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h Handler) Disconnect(c *gin.Context) {
	provider := c.Param("provider")
	service := h.Providers.Get(provider)
	if service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "provider is not available"})
		return
	}
	if err := service.Disconnect(c, auth.UserID(c)); err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": provider, "connected": false, "enabled": false})
}
func (h Handler) PatchConnection(c *gin.Context) {
	provider := c.Param("provider")
	if h.Providers.Get(provider) == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "provider is not available"})
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Enabled == nil {
		bad(c, "enabled is required")
		return
	}
	item, err := h.Store.Connection(c, auth.UserID(c), provider)
	if err != nil {
		serverError(c)
		return
	}
	if item == nil || item.Status != "connected" {
		c.JSON(http.StatusConflict, gin.H{"error": "connect provider first"})
		return
	}
	item.Enabled = *input.Enabled
	if err := h.Store.SaveConnection(c, *item); err != nil {
		serverError(c)
		return
	}
	c.JSON(http.StatusOK, model.ConnectionView{Provider: provider, Available: true, Connected: true, Enabled: item.Enabled})
}
