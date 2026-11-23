package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ZaloOAID, ZaloAccessToken, ZaloWebhookSecret                                  string
	MessengerPageID, MessengerPageToken, MessengerAppSecret, MessengerVerifyToken string
	Port, DatabaseURL, RabbitURL                                                  string
	JWKSURL, JWTIssuer, JWTAudience                                               string
	TelegramBotUsername, TelegramBotToken, TelegramWebhookSecret                  string
}

func Load() Config {
	return Config{
		ZaloOAID:              os.Getenv("ZALO_OA_ID"),
		ZaloAccessToken:       os.Getenv("ZALO_ACCESS_TOKEN"),
		ZaloWebhookSecret:     os.Getenv("ZALO_WEBHOOK_SECRET"),
		MessengerPageID:       os.Getenv("MESSENGER_PAGE_ID"),
		MessengerPageToken:    os.Getenv("MESSENGER_PAGE_TOKEN"),
		MessengerAppSecret:    os.Getenv("MESSENGER_APP_SECRET"),
		MessengerVerifyToken:  os.Getenv("MESSENGER_VERIFY_TOKEN"),
		Port:                  value("API_PORT", "8080"),
		DatabaseURL:           value("DATABASE_URL", "postgres://hunterjob:hunterjob@localhost:5432/hunterjob?sslmode=disable"),
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
