package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/hunterjob/hunterjob/api/infra"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/model"
	"github.com/hunterjob/hunterjob/api/repository"
	"github.com/hunterjob/hunterjob/api/service"
	amqp "github.com/rabbitmq/amqp091-go"
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
	if err := store.Indexes(ctx); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	conn, err := amqp.Dial(cfg.RabbitURL)
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()
	if _, err := ch.QueueDeclare("job.new", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": "job.new.dead"}); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare("job.new.dead", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	messages, err := ch.Consume("job.new", "hunterjob-matcher", false, false, false, false, nil)
	if err != nil {
		return err
	}
	telegram := &agent.TelegramProvider{BotUsername: cfg.TelegramBotUsername, BotToken: cfg.TelegramBotToken, WebhookSecret: cfg.TelegramWebhookSecret, Redis: resources.Cache, Store: store}
	go service.DailyLoop(ctx, store, telegram, resources.Cache)
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				return errors.New("job.new consumer closed")
			}
			var job model.LakeJob
			if json.Unmarshal(message.Body, &job) != nil || job.ID == "" {
				_ = message.Nack(false, false)
				continue
			}
			if _, err := store.ProcessJob(ctx, job); err != nil {
				log.Printf("match job %s: %v", job.ID, err)
				_ = message.Nack(false, true)
			} else {
				_ = message.Ack(false)
			}
		}
	}
}
