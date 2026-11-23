package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/hunterjob/hunterjob/api/entity"
	"github.com/hunterjob/hunterjob/api/infra"
	"github.com/hunterjob/hunterjob/api/internal/config"
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
	matcher := service.Matcher{Store: store}
	for ctx.Err() == nil {
		if err := consume(ctx, cfg.RabbitURL, matcher); err != nil && ctx.Err() == nil {
			log.Printf("matcher retrying in 5 seconds: %v", err)
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
		}
	}
	return nil
}

func consume(ctx context.Context, rabbitURL string, matcher service.Matcher) error {
	conn, err := amqp.DialConfig(rabbitURL, amqp.Config{Dial: amqp.DefaultDial(10 * time.Second)})
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	defer func() { _ = conn.CloseDeadline(time.Now().Add(5 * time.Second)) }()
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("job.new consumer closed")
			}
			var job entity.LakeJob
			if json.Unmarshal(message.Body, &job) != nil || job.ID == "" {
				if err := message.Nack(false, false); err != nil {
					return fmt.Errorf("dead-letter malformed job: %w", err)
				}
				continue
			}
			if _, err := matcher.ProcessJob(ctx, job); err != nil {
				// Closing the channel requeues the unacknowledged event after a backoff.
				return fmt.Errorf("match job %s: %w", job.ID, err)
			}
			if err := message.Ack(false); err != nil {
				return fmt.Errorf("acknowledge job %s: %w", job.ID, err)
			}
		}
	}
}
