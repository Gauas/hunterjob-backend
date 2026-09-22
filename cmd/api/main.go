package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appsearch "github.com/hunterjob/hunterjob/api/internal/application/search"
	appsource "github.com/hunterjob/hunterjob/api/internal/application/source"
	"github.com/hunterjob/hunterjob/api/internal/config"
	infraai "github.com/hunterjob/hunterjob/api/internal/infrastructure/ai"
	"github.com/hunterjob/hunterjob/api/internal/infrastructure/queue"
	"github.com/hunterjob/hunterjob/api/internal/repository"
	controller "github.com/hunterjob/hunterjob/api/internal/transport/http/controller"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, client, e := repository.NewMongo(ctx, cfg.MongoURI, cfg.Database)
	if e != nil {
		log.Fatal(e)
	}
	defer client.Disconnect(context.Background())
	r := gin.New()
	r.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/health", "/ready"},
		}),
		gin.Recovery(),
		func(c *gin.Context) {
			c.Header("X-Request-ID", uuid.NewString())
			c.Next()
		},
	)
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/ready", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ready"}) })
	v := r.Group("/api/v1")
	q := queue.RabbitMQ{URL: cfg.RabbitURL}
	admin := controller.NewAdminController(appsource.NewService(s, q))
	catalog := controller.NewCatalogController(s)
	aiProvider := infraai.OpenAICompatible{APIURL: cfg.AIAPIURL, APIKey: cfg.AIAPIKey}
	aiSearch := controller.NewSearchController(appsearch.NewService(s, aiProvider))
	v.GET("/jobs", catalog.ListJobs)
	v.GET("/jobs/latest", catalog.LatestJobs)
	v.GET("/jobs/:id", catalog.GetJob)
	v.GET("/jobs/:id/redirect", catalog.RedirectToApplication)
	v.GET("/companies", catalog.ListCompanies)
	v.POST("/admin/sources", admin.AddSource)
	v.POST("/admin/sources/:id/crawl", admin.CrawlOne)
	v.POST("/admin/crawler/restart", admin.RestartCrawler)
	v.POST("/search/ai", aiSearch.AI)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		log.Printf("api listening on %s", server.Addr)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}
