package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port, MongoURI, Database, RedisAddr, RabbitURL, UserAgent, AIAPIURL, AIAPIKey string
	Timeout                                                                       time.Duration
	AITimeout                                                                     time.Duration
	SchedulerInterval                                                             time.Duration
	MaxBodySize                                                                   int64
}

func Load() Config {
	t, err := time.ParseDuration(value("CRAWLER_TIMEOUT", "15s"))
	if err != nil {
		t = 15 * time.Second
	}
	s, err := strconv.ParseInt(value("CRAWLER_MAX_BODY_SIZE", "2097152"), 10, 64)
	if err != nil {
		s = 2097152
	}
	schedulerInterval, err := time.ParseDuration(value("CRAWLER_SCHEDULER_INTERVAL", "1m"))
	if err != nil || schedulerInterval <= 0 {
		schedulerInterval = time.Minute
	}
	aiTimeout, err := time.ParseDuration(value("AI_TIMEOUT", "300s"))
	if err != nil || aiTimeout <= 0 {
		aiTimeout = 300 * time.Second
	}
	return Config{Port: value("API_PORT", "8080"), MongoURI: value("MONGODB_URI", "mongodb://localhost:27017"), Database: value("MONGODB_DATABASE", "hunterjob"), RedisAddr: value("REDIS_ADDR", "localhost:6379"), RabbitURL: value("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), UserAgent: value("CRAWLER_USER_AGENT", "HunterJobBot/0.1"), AIAPIURL: os.Getenv("AI_API_URL"), AIAPIKey: os.Getenv("AI_API_KEY"), Timeout: t, AITimeout: aiTimeout, SchedulerInterval: schedulerInterval, MaxBodySize: s}
}

func value(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
