package main

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/crawler"
	infraai "github.com/hunterjob/hunterjob/api/internal/infrastructure/ai"
	infraqueue "github.com/hunterjob/hunterjob/api/internal/infrastructure/queue"
	"github.com/hunterjob/hunterjob/api/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type crawlMessage struct {
	SourceID string `json:"source_id"`
}

func main() {
	cfg := config.Load()
	ctx := context.Background()
	s, client, e := repository.NewMongo(ctx, cfg.MongoURI, cfg.Database)
	if e != nil {
		log.Fatal(e)
	}
	defer client.Disconnect(ctx)
	conn, e := amqp.Dial(cfg.RabbitURL)
	if e != nil {
		log.Fatal(e)
	}
	defer conn.Close()
	ch, e := conn.Channel()
	if e != nil {
		log.Fatal(e)
	}
	defer ch.Close()
	_ = ch.Qos(1, 0, false)
	args := amqp.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": "career.crawl.dead"}
	_, e = ch.QueueDeclare("career.crawl", true, false, false, false, args)
	if e != nil {
		log.Fatal(e)
	}
	_, _ = ch.QueueDeclare("career.crawl.dead", true, false, false, false, nil)
	msgs, e := ch.Consume("career.crawl", "crawler-worker", false, false, false, false, nil)
	if e != nil {
		log.Fatal(e)
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	fetcher := crawler.HTTPFetcher{Client: &http.Client{Timeout: cfg.Timeout}, UserAgent: cfg.UserAgent, MaxBody: cfg.MaxBodySize}
	normalizer := infraai.OpenAICompatible{APIURL: cfg.AIAPIURL, APIKey: cfg.AIAPIKey}
	service := crawler.Service{Repository: s, Fetcher: fetcher, Normalizer: normalizer}
	publisher := infraqueue.RabbitMQ{URL: cfg.RabbitURL}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	schedulerCtx, cancelScheduler := context.WithCancel(ctx)
	defer cancelScheduler()
	go runScheduler(schedulerCtx, s, rdb, publisher, cfg.SchedulerInterval)
	for {
		select {
		case <-stop:
			return
		case m, ok := <-msgs:
			if !ok {
				return
			}
			var msg crawlMessage
			if e := json.Unmarshal(m.Body, &msg); e != nil {
				_ = m.Nack(false, false)
				continue
			}
			id, e := primitive.ObjectIDFromHex(msg.SourceID)
			if e != nil {
				_ = m.Nack(false, false)
				continue
			}
			lock := "crawler:lock:" + msg.SourceID
			locked, lockErr := rdb.SetNX(ctx, lock, "1", 10*time.Minute).Result()
			if lockErr != nil || !locked {
				_ = m.Nack(false, false)
				continue
			}
			var src struct {
				ID          primitive.ObjectID `bson:"_id"`
				CompanyID   primitive.ObjectID `bson:"company_id"`
				CareerURL   string             `bson:"career_url"`
				Provider    string             `bson:"provider"`
				URLPatterns []string           `bson:"url_patterns"`
			}
			e = s.DB.Collection("career_sources").FindOne(ctx, bson.M{"_id": id}).Decode(&src)
			if e == nil { // map only fields worker needs
				source := structToSource(src)
				_, e = service.Crawl(ctx, source)
			}
			_ = rdb.Del(ctx, lock)
			if recordErr := s.RecordSourceCrawl(ctx, id, time.Now().UTC(), e); recordErr != nil {
				log.Printf("record crawl result %s: %v", id.Hex(), recordErr)
			}
			if e != nil {
				log.Printf("crawl %s: %v", id.Hex(), e)
				_ = m.Nack(false, false)
			} else {
				_ = m.Ack(false)
			}
		}
	}
}

type crawlPublisher interface {
	EnqueueCrawl(context.Context, primitive.ObjectID) error
}

func runScheduler(ctx context.Context, repo *repository.MongoRepository, rdb *redis.Client, publisher crawlPublisher, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := scheduleDueSources(ctx, repo, rdb, publisher); err != nil && ctx.Err() == nil {
			log.Printf("crawl scheduler: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func scheduleDueSources(ctx context.Context, repo *repository.MongoRepository, rdb *redis.Client, publisher crawlPublisher) error {
	const schedulerLock = "crawler:scheduler:lock"
	locked, err := rdb.SetNX(ctx, schedulerLock, "1", 55*time.Second).Result()
	if err != nil || !locked {
		return err
	}
	defer rdb.Del(context.Background(), schedulerLock)
	now := time.Now().UTC()
	sources, err := repo.DueSources(ctx, now)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if err := publisher.EnqueueCrawl(ctx, source.ID); err != nil {
			return err
		}
		if err := repo.MarkSourceScheduled(ctx, source, now); err != nil {
			return err
		}
	}
	return nil
}

func structToSource(v struct {
	ID          primitive.ObjectID `bson:"_id"`
	CompanyID   primitive.ObjectID `bson:"company_id"`
	CareerURL   string             `bson:"career_url"`
	Provider    string             `bson:"provider"`
	URLPatterns []string           `bson:"url_patterns"`
}) company.Source {
	return company.Source{ID: v.ID, CompanyID: v.CompanyID, CareerURL: v.CareerURL, Provider: v.Provider, URLPatterns: v.URLPatterns}
}
