package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port, MongoURI, Database, RedisAddr, RabbitURL               string
	TelegramBotUsername, TelegramBotToken, TelegramWebhookSecret string
}

func Load() Config {
	return Config{Port: value("API_PORT", "8080"), MongoURI: value("MONGODB_URI", "mongodb://localhost:27017"), Database: value("MONGODB_DATABASE", "hunterjob"), RedisAddr: value("REDIS_ADDR", "localhost:6379"), RabbitURL: value("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), TelegramBotUsername: os.Getenv("TELEGRAM_BOT_USERNAME"), TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"), TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET")}
}

func (c Config) ValidateAPI() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("API_PORT is required")
	}
	return nil
}

func value(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
