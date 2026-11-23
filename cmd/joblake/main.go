package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/jobevent"
	"github.com/hunterjob/hunterjob/api/internal/joblake"
)

func main() {
	sourcesPath := flag.String("sources", "", "JSON source registry (defaults to the built-in HCMC boards)")
	interval := flag.Duration("interval", 6*time.Hour, "delay between syncs")
	once := flag.Bool("once", false, "sync once and exit")
	preview := flag.Bool("preview", false, "sync once and print job events without RabbitMQ")
	flag.Parse()
	if *interval <= 0 {
		log.Fatal("-interval must be positive")
	}
	sources, err := joblake.LoadSources(*sourcesPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	for ctx.Err() == nil {
		err := syncSources(ctx, sources, *preview)
		if ctx.Err() != nil {
			return
		}
		if *once || *preview {
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		delay := *interval
		if err != nil {
			log.Printf("career feed sync: %v", err)
			delay = min(delay, 5*time.Minute)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func syncSources(ctx context.Context, sources []joblake.Source, preview bool) error {
	encoder := json.NewEncoder(os.Stdout)
	emit := func(_ context.Context, job jobevent.Job) error { return encoder.Encode(job) }
	if !preview {
		publisher, err := joblake.OpenPublisher(ctx, config.Load().RabbitURL)
		if err != nil {
			return err
		}
		defer publisher.Close()
		emit = publisher.Publish
	}
	crawler := joblake.NewSmartRecruiters()
	var sourceErrors []error
	for _, source := range sources {
		jobs, err := crawler.Fetch(ctx, source)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			sourceErrors = append(sourceErrors, fmt.Errorf("%s: %w", source.Company, err))
			continue
		}
		for _, job := range jobs {
			if err := emit(ctx, job); err != nil {
				return fmt.Errorf("emit job for %s: %w", source.Company, err)
			}
		}
		log.Printf("%s: %d HCMC job events", source.Company, len(jobs))
	}
	return errors.Join(sourceErrors...)
}
