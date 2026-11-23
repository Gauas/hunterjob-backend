package main

import (
	"context"
	"errors"

	transport "github.com/hunterjob/hunterjob/api/http"
	"github.com/hunterjob/hunterjob/api/http/controller"
	"github.com/hunterjob/hunterjob/api/infra"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/internal/auth"
	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/pkg/authn"
	"github.com/hunterjob/hunterjob/api/repository"
)

type Application struct {
	server    *transport.Server
	resources *infra.Resources
}

func NewApplication(ctx context.Context, cfg config.Config) (*Application, error) {
	if err := cfg.ValidateAPI(); err != nil {
		return nil, err
	}
	resources, err := infra.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	store := repository.Store{DB: resources.Database}
	if err := store.Indexes(ctx); err != nil {
		return nil, errors.Join(err, resources.Close())
	}
	telegram := &agent.TelegramProvider{BotUsername: cfg.TelegramBotUsername, BotToken: cfg.TelegramBotToken, WebhookSecret: cfg.TelegramWebhookSecret, Redis: resources.Cache, Store: store}
	verifier, err := authn.NewVerifier(authn.Config{JWKSURL: cfg.JWKSURL, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience})
	if err != nil {
		return nil, errors.Join(err, resources.Close())
	}
	ctrl := controller.Controller{
		Agent:    controller.Handler{Store: store, Providers: agent.ProviderRegistry{Telegram: telegram}},
		Telegram: telegram,
		Auth:     auth.Middleware{Verifier: verifier},
		Ready:    resources.Ready,
	}
	return &Application{server: transport.NewServer(cfg.Port, ctrl), resources: resources}, nil
}

func (a *Application) Run(ctx context.Context) error { return a.server.Run(ctx) }
func (a *Application) Close() error                  { return a.resources.Close() }
