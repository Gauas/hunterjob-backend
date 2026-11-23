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
	telegram := &agent.TelegramProvider{BotUsername: cfg.TelegramBotUsername, BotToken: cfg.TelegramBotToken, WebhookSecret: cfg.TelegramWebhookSecret, Store: store}
	providers := agent.ProviderRegistry{}
	webhooks := map[string]controller.Webhook{}
	if telegram.Available() {
		providers["telegram"] = telegram
		webhooks["telegram"] = controller.Webhook{Receive: telegram.Webhook}
	}
	messenger := agent.NewMessenger(store, cfg.MessengerPageID, cfg.MessengerPageToken, cfg.MessengerAppSecret, cfg.MessengerVerifyToken)
	if messenger != nil {
		providers["messenger"] = messenger
		webhooks["messenger"] = controller.Webhook{Verify: messenger.Verify, Receive: messenger.Webhook}
	}
	zalo := agent.NewZalo(store, cfg.ZaloOAID, cfg.ZaloAccessToken, cfg.ZaloWebhookSecret)
	if zalo != nil {
		providers["zalo"] = zalo
		webhooks["zalo"] = controller.Webhook{Receive: zalo.Webhook}
	}
	verifier, err := authn.NewVerifier(authn.Config{JWKSURL: cfg.JWKSURL, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience})
	if err != nil {
		return nil, errors.Join(err, resources.Close())
	}
	ctrl := controller.Controller{
		Agent:    controller.Handler{Store: store, Providers: providers},
		Webhooks: webhooks,
		Auth:     auth.Middleware{Verifier: verifier},
		Ready:    resources.Ready,
	}
	return &Application{server: transport.NewServer(cfg.Port, ctrl), resources: resources}, nil
}

func (a *Application) Run(ctx context.Context) error { return a.server.Run(ctx) }
func (a *Application) Close() error                  { return a.resources.Close() }
