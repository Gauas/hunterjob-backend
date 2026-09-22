package httptransport

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	defaultPageSize int64 = 20
	maxPageSize     int64 = 100
)

// CatalogRepository is the read/write boundary used by the public catalog API.
// Keeping MongoDB details behind this interface makes the HTTP contract testable.
type CatalogRepository interface {
	ListJobs(context.Context, bson.M, int64, int64) ([]job.Job, error)
	CountJobs(context.Context, bson.M) (int64, error)
	Job(context.Context, primitive.ObjectID) (job.Job, error)
	TrackClick(context.Context, primitive.ObjectID, time.Time) error
	ListCompanies(context.Context, int64, int64) ([]company.Company, error)
	CountCompanies(context.Context) (int64, error)
}

type CatalogController struct{ repository CatalogRepository }

func NewCatalogController(repository CatalogRepository) CatalogController {
	return CatalogController{repository: repository}
}

func (h CatalogController) ListJobs(c *gin.Context) {
	page, limit, ok := pagination(c)
	if !ok {
		return
	}
	filter, ok := jobFilter(c)
	if !ok {
		return
	}

	total, err := h.repository.CountJobs(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count jobs"})
		return
	}
	jobs, err := h.repository.ListJobs(c, filter, (page-1)*limit, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load jobs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs, "meta": pageMeta(page, limit, total)})
}

func (h CatalogController) LatestJobs(c *gin.Context) {
	limit, ok := positiveInt(c, "limit", defaultPageSize, maxPageSize)
	if !ok {
		return
	}
	jobs, err := h.repository.ListJobs(c, bson.M{"active": true, "$and": bson.A{unexpiredJobs(time.Now().UTC())}}, 0, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load jobs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

func (h CatalogController) GetJob(c *gin.Context) {
	id, ok := objectIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.repository.Job(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h CatalogController) RedirectToApplication(c *gin.Context) {
	id, ok := objectIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.repository.Job(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	applicationURL, err := url.Parse(item.ApplyURL)
	if err != nil || applicationURL.Hostname() == "" || (applicationURL.Scheme != "http" && applicationURL.Scheme != "https") {
		c.JSON(http.StatusNotFound, gin.H{"error": "application URL is unavailable"})
		return
	}
	if err := h.repository.TrackClick(c, id, time.Now().UTC()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not track application click"})
		return
	}
	c.Redirect(http.StatusFound, item.ApplyURL)
}

func (h CatalogController) ListCompanies(c *gin.Context) {
	page, limit, ok := pagination(c)
	if !ok {
		return
	}
	total, err := h.repository.CountCompanies(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count companies"})
		return
	}
	companies, err := h.repository.ListCompanies(c, (page-1)*limit, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load companies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": companies, "meta": pageMeta(page, limit, total)})
}

func jobFilter(c *gin.Context) (bson.M, bool) {
	filter := bson.M{"active": true, "$and": bson.A{unexpiredJobs(time.Now().UTC())}}
	active, hasActive, ok := optionalBool(c, "active")
	if !ok {
		return nil, false
	}
	if hasActive {
		filter["active"] = active
	}
	if location := strings.TrimSpace(c.Query("location")); location != "" {
		filter["locations.city"] = bson.M{"$regex": regexp.QuoteMeta(location), "$options": "i"}
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		safeKeyword := regexp.QuoteMeta(keyword)
		filter["$or"] = bson.A{
			bson.M{"title": bson.M{"$regex": safeKeyword, "$options": "i"}},
			bson.M{"normalized_title": bson.M{"$regex": safeKeyword, "$options": "i"}},
			bson.M{"skills": bson.M{"$regex": safeKeyword, "$options": "i"}},
		}
	}
	if level := strings.TrimSpace(c.Query("level")); level != "" {
		filter["levels"] = bson.M{"$regex": regexp.QuoteMeta(level), "$options": "i"}
	}
	remote, hasRemote, ok := optionalBool(c, "remote")
	if !ok {
		return nil, false
	}
	if hasRemote {
		filter["locations.remote"] = remote
	}
	return filter, true
}

func unexpiredJobs(now time.Time) bson.M {
	return bson.M{"$or": bson.A{
		bson.M{"expired_at": bson.M{"$exists": false}},
		bson.M{"expired_at": nil},
		bson.M{"expired_at": bson.M{"$gt": now}},
	}}
}

func optionalBool(c *gin.Context, name string) (bool, bool, bool) {
	raw := c.Query(name)
	if raw == "" {
		return false, false, true
	}
	value, err := strconv.ParseBool(raw)
	if err == nil {
		return value, true, true
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be true or false"})
	return false, false, false
}

func pagination(c *gin.Context) (int64, int64, bool) {
	page, ok := positiveInt(c, "page", 1, 1_000_000)
	if !ok {
		return 0, 0, false
	}
	limit, ok := positiveInt(c, "limit", defaultPageSize, maxPageSize)
	if !ok {
		return 0, 0, false
	}
	return page, limit, true
}

func positiveInt(c *gin.Context, name string, fallback, max int64) (int64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 || value > max {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be between 1 and " + strconv.FormatInt(max, 10)})
		return 0, false
	}
	return value, true
}

func objectIDParam(c *gin.Context, name string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return primitive.NilObjectID, false
	}
	return id, true
}

func pageMeta(page, limit, total int64) gin.H {
	return gin.H{"page": page, "limit": limit, "total": total, "total_pages": (total + limit - 1) / limit}
}
