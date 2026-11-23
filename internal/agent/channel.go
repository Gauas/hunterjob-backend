package agent

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hunterjob/hunterjob/api/repository"
)

type ConnectionResult struct {
	Provider      string `json:"provider"`
	ConnectionURL string `json:"connection_url"`
	Command       string `json:"command,omitempty"`
	ExpiresIn     int    `json:"expires_in"`
}

type ChatProvider interface {
	Connect(context.Context, string) (ConnectionResult, error)
	Disconnect(context.Context, string) error
	Send(context.Context, string, string) error
}

type ProviderRegistry map[string]ChatProvider

func (r ProviderRegistry) Get(name string) ChatProvider { return r[name] }
func (r ProviderRegistry) Available(name string) bool   { return r[name] != nil }

type channel struct {
	name  string
	url   string
	store repository.Store
}

func (c channel) newLink(ctx context.Context, userID string) (string, error) {
	var seed [24]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(seed[:])
	return token, c.store.CreateLink(ctx, c.name, userID, token)
}

func (c channel) Connect(ctx context.Context, userID string) (ConnectionResult, error) {
	token, err := c.newLink(ctx, userID)
	if err != nil {
		return ConnectionResult{}, err
	}
	return ConnectionResult{Provider: c.name, ConnectionURL: c.url, Command: "/link " + token, ExpiresIn: 600}, nil
}

func (c channel) Disconnect(ctx context.Context, userID string) error {
	return c.store.Disconnect(ctx, userID, c.name)
}

func (c channel) handleText(ctx context.Context, recipient, text string, limit int, send func(context.Context, string, string) error) error {
	parts := strings.Fields(text)
	if len(parts) == 2 && (parts[0] == "/link" || parts[0] == "#link") && len(parts[1]) == 32 {
		bound, err := c.store.BindLink(ctx, c.name, parts[1], recipient)
		switch {
		case errors.Is(err, repository.ErrRecipientInUse):
			return send(ctx, recipient, "This chat is already linked to another HunterJob account.")
		case err != nil:
			return err
		case !bound:
			return nil
		default:
			return send(ctx, recipient, "HunterJob connected. Send /jobs to see your latest matches.")
		}
	}
	if len(parts) != 1 || !(parts[0] == "/jobs" || parts[0] == "#jobs" || strings.EqualFold(parts[0], "jobs")) {
		return nil
	}
	matches, linked, err := c.store.RecipientMatches(ctx, c.name, recipient)
	if err != nil {
		return err
	}
	if !linked {
		return send(ctx, recipient, "Connect this chat from your HunterJob dashboard.")
	}
	if len(matches) == 0 {
		return send(ctx, recipient, "No job matches yet.")
	}
	message := "Latest HunterJob matches"
	for _, match := range matches {
		row := fmt.Sprintf("\n\n%s\n%s\n%s", match.Job.Title, match.Job.CompanyName, match.Job.JobURL)
		if utf8.RuneCountInString(message+row) > limit {
			continue
		}
		message += row
	}
	if message == "Latest HunterJob matches" {
		message = "See your latest matches on your HunterJob dashboard."
	}
	return send(ctx, recipient, message)
}
