package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/hunterjob/hunterjob/api/infra"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/repository"
	"github.com/hunterjob/hunterjob/api/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, config.Load()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, cfg config.Config) error {
	resources, err := infra.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer resources.Close()
	store := repository.Store{DB: resources.Database}
	telegram := &agent.TelegramProvider{
		BotUsername: cfg.TelegramBotUsername, BotToken: cfg.TelegramBotToken,
		WebhookSecret: cfg.TelegramWebhookSecret, Store: store,
	}
	service.Digest{Store: store, Telegram: telegram}.Run(ctx)
	return nil
}
