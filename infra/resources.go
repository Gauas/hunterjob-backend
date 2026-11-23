package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Resources owns the database and cache connections for a running process.
type Resources struct {
	Database *pgxpool.Pool
	Cache    *redis.Client
}

func New(ctx context.Context, cfg config.Config) (*Resources, error) {
	options := &redis.Options{Addr: cfg.RedisAddr}
	if strings.HasPrefix(cfg.RedisAddr, "redis://") || strings.HasPrefix(cfg.RedisAddr, "rediss://") {
		var err error
		options, err = redis.ParseURL(cfg.RedisAddr)
		if err != nil {
			return nil, err
		}
	}
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := storage.OpenPostgres(openCtx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Resources{Database: db, Cache: redis.NewClient(options)}, nil
}

func (r *Resources) Ready(ctx context.Context) error {
	if err := r.Database.Ping(ctx); err != nil {
		return err
	}
	return r.Cache.Ping(ctx).Err()
}

func (r *Resources) Close() error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.Cache != nil {
		errs = append(errs, r.Cache.Close())
	}
	if r.Database != nil {
		r.Database.Close()
	}
	return errors.Join(errs...)
}
