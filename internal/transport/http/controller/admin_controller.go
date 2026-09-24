package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/application/source"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminController struct{ sources source.Service }

func NewAdminController(s source.Service) AdminController { return AdminController{sources: s} }

type addSourceRequest struct {
	CompanyName          string `json:"company_name" binding:"required"`
	CompanyWebsite       string `json:"company_website"`
	CompanyLogoURL       string `json:"company_logo_url"`
	CareerURL            string `json:"career_url" binding:"required"`
	CareerPageURL        string `json:"career_page_url"`
	Provider             string `json:"provider"`
	JobURLTemplate       string `json:"job_url_template"`
	ApplyURLTemplate     string `json:"apply_url_template"`
	CrawlIntervalMinutes int    `json:"crawl_interval_minutes"`
}

func (h AdminController) AddSource(c *gin.Context) {
	var req addSourceRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	src, e := h.sources.Add(c, source.CreateInput{CompanyName: req.CompanyName, CompanyWebsite: req.CompanyWebsite, CompanyLogoURL: req.CompanyLogoURL, CareerURL: req.CareerURL, CareerPageURL: req.CareerPageURL, Provider: req.Provider, JobURLTemplate: req.JobURLTemplate, ApplyURLTemplate: req.ApplyURLTemplate, CrawlIntervalMinutes: req.CrawlIntervalMinutes})
	if e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": e.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": src})
}

func (h AdminController) CrawlOne(c *gin.Context) {
	id, e := primitive.ObjectIDFromHex(c.Param("id"))
	if e != nil {
		c.JSON(400, gin.H{"error": "invalid source id"})
		return
	}
	if e = h.sources.CrawlOne(c, id); e != nil {
		c.JSON(404, gin.H{"error": e.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"source_id": id.Hex(), "status": "queued"})
}

func (h AdminController) RestartCrawler(c *gin.Context) {
	n, e := h.sources.Restart(c)
	if e != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "could not queue crawl schedule"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"sources_queued": n, "status": "restarted"})
}
