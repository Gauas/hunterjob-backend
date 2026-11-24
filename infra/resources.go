package infra

import (
	"context"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Resources struct {
	Database *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*Resources, error) {
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := storage.OpenPostgres(openCtx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Resources{Database: db}, nil
}

func (r *Resources) Ready(ctx context.Context) error { return r.Database.Ping(ctx) }

func (r *Resources) Close() error {
	r.Database.Close()
	return nil
}
