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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	aiProvider := infraai.OpenAICompatible{APIURL: cfg.AIAPIURL, APIKey: cfg.AIAPIKey}
	aiSearch := controller.NewSearchController(appsearch.NewService(s, aiProvider))
	v.GET("/jobs", func(c *gin.Context) {
		f := bson.M{}
		if c.Query("active") != "false" {
			f["active"] = true
		}
		if x := c.Query("location"); x != "" {
			f["locations.city"] = bson.M{"$regex": x, "$options": "i"}
		}
		if x := c.Query("keyword"); x != "" {
			f["$or"] = []bson.M{{"title": bson.M{"$regex": x, "$options": "i"}}, {"skills": bson.M{"$regex": x, "$options": "i"}}}
		}
		js, e := s.Jobs(c, f, 100)
		if e != nil {
			c.JSON(500, gin.H{"error": "could not load jobs"})
			return
		}
		c.JSON(200, gin.H{"data": js})
	})
	v.GET("/jobs/latest", func(c *gin.Context) {
		js, e := s.Jobs(c, bson.M{"active": true}, 20)
		if e != nil {
			c.JSON(500, gin.H{"error": "could not load jobs"})
			return
		}
		c.JSON(200, gin.H{"data": js})
	})
	v.GET("/jobs/:id", func(c *gin.Context) {
		id, e := primitive.ObjectIDFromHex(c.Param("id"))
		if e != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		j, e := s.Job(c, id)
		if e != nil {
			c.JSON(404, gin.H{"error": "job not found"})
			return
		}
		c.JSON(200, gin.H{"data": j})
	})
	v.GET("/jobs/:id/redirect", func(c *gin.Context) {
		id, e := primitive.ObjectIDFromHex(c.Param("id"))
		if e != nil {
			c.Status(400)
			return
		}
		j, e := s.Job(c, id)
		if e != nil || !strings.HasPrefix(j.ApplyURL, "http") {
			c.Status(404)
			return
		}
		_, _ = s.DB.Collection("click_events").InsertOne(c, bson.M{"job_id": id, "created_at": time.Now().UTC()})
		c.Redirect(http.StatusFound, j.ApplyURL)
	})
	v.GET("/companies", func(c *gin.Context) {
		cur, e := s.DB.Collection("companies").Find(c, bson.M{})
		if e != nil {
			c.JSON(500, gin.H{"error": "could not load companies"})
			return
		}
		defer cur.Close(c)
		var rows []bson.M
		_ = cur.All(c, &rows)
		c.JSON(200, gin.H{"data": rows})
	})
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
