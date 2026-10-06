package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/storage"
	"go.mongodb.org/mongo-driver/mongo"
)

// Resources owns the database and cache connections for a running process.
type Resources struct {
	Database *mongo.Database
	Mongo    *mongo.Client
	Cache    *redis.Client
}

func New(ctx context.Context, cfg config.Config) (r *Resources, err error) {
	r = &Resources{}
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r.Database, r.Mongo, err = storage.OpenMongo(openCtx, cfg.MongoURI, cfg.Database)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = r.Close()
		}
	}()
	options := &redis.Options{Addr: cfg.RedisAddr}
	if strings.HasPrefix(cfg.RedisAddr, "redis://") || strings.HasPrefix(cfg.RedisAddr, "rediss://") {
		options, err = redis.ParseURL(cfg.RedisAddr)
		if err != nil {
			return nil, err
		}
	}
	r.Cache = redis.NewClient(options)
	return r, nil
}

func (r *Resources) Ready(ctx context.Context) error {
	if err := r.Mongo.Ping(ctx, nil); err != nil {
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
	if r.Mongo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		errs = append(errs, r.Mongo.Disconnect(ctx))
	}
	return errors.Join(errs...)
}
