package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port, MongoURI, Database, RedisAddr, RabbitURL, UserAgent, OpenAIKey, AIProvider, OpenAIModel string
	Timeout                                                                                       time.Duration
	MaxBodySize                                                                                   int64
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
	return Config{Port: value("API_PORT", "8080"), MongoURI: value("MONGODB_URI", "mongodb://localhost:27017"), Database: value("MONGODB_DATABASE", "hunterjob"), RedisAddr: value("REDIS_ADDR", "localhost:6379"), RabbitURL: value("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), UserAgent: value("CRAWLER_USER_AGENT", "HunterJobBot/0.1"), OpenAIKey: os.Getenv("OPENAI_API_KEY"), AIProvider: value("AI_PROVIDER", "deterministic"), OpenAIModel: value("OPENAI_MODEL", "gpt-4o-mini"), Timeout: t, MaxBodySize: s}
}
func value(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
