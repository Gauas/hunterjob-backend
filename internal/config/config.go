package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port, MongoURI, Database, RedisAddr, RabbitURL               string
	JWKSURL, JWTIssuer, JWTAudience                              string
	TelegramBotUsername, TelegramBotToken, TelegramWebhookSecret string
}

func Load() Config {
	return Config{
		Port:                  value("API_PORT", "8080"),
		MongoURI:              value("MONGODB_URI", "mongodb://localhost:27017"),
		Database:              value("MONGODB_DATABASE", "hunterjob"),
		RedisAddr:             value("REDIS_ADDR", "localhost:6379"),
		RabbitURL:             value("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		JWKSURL:               value("JWKS_URL", "https://api.gauas.com/.well-known/jwks.json"),
		JWTIssuer:             value("JWT_ISSUER", "gauas-auth"),
		JWTAudience:           value("JWT_AUDIENCE", "gauas-api"),
		TelegramBotUsername:   os.Getenv("TELEGRAM_BOT_USERNAME"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
	}
}

func (c Config) ValidateAPI() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("API_PORT is required")
	}
	if c.JWKSURL == "" || c.JWTIssuer == "" || c.JWTAudience == "" {
		return fmt.Errorf("JWKS_URL, JWT_ISSUER and JWT_AUDIENCE are required")
	}
	return nil
}

func value(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
