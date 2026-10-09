package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"github.com/hunterjob/hunterjob/api/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	app, err := NewApplication(ctx, config.Load())
	if err != nil {
		return err
	}
	log.Printf("api listening on %s", app.server.HTTP.Addr)
	return errors.Join(app.Run(ctx), app.Close())
}
