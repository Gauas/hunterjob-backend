package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/hunterjob/hunterjob/api/model"
	"github.com/hunterjob/hunterjob/api/repository"
	"go.mongodb.org/mongo-driver/mongo"
)

type ConnectionResult struct {
	Provider      string `json:"provider"`
	ConnectionURL string `json:"connection_url"`
}
type ChatProvider interface {
	Connect(context.Context, string) (ConnectionResult, error)
	Disconnect(context.Context, string) error
	Send(context.Context, string, string) error
}
type ProviderRegistry struct{ Telegram *TelegramProvider }

func (r ProviderRegistry) Get(name string) ChatProvider {
	if name == "telegram" && r.Telegram != nil && r.Telegram.Available() {
		return r.Telegram
	}
	return nil
}
func (r ProviderRegistry) Available(name string) bool { return r.Get(name) != nil }

type TelegramProvider struct {
	BotUsername   string
	BotToken      string
	WebhookSecret string
	Redis         *redis.Client
	Store         repository.Store
	Client        *http.Client
}

func (t *TelegramProvider) Available() bool {
	return t != nil && t.BotUsername != "" && t.BotToken != "" && t.WebhookSecret != "" && t.Redis != nil
}
func (t *TelegramProvider) Connect(ctx context.Context, userID string) (ConnectionResult, error) {
	var tokenBytes [24]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return ConnectionResult{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes[:])
	if err := t.Redis.Set(ctx, "telegram:link:"+token, userID, 10*time.Minute).Err(); err != nil {
		return ConnectionResult{}, err
	}
	return ConnectionResult{Provider: "telegram", ConnectionURL: "https://t.me/" + url.PathEscape(strings.TrimPrefix(t.BotUsername, "@")) + "?start=" + token}, nil
}
func (t *TelegramProvider) Disconnect(ctx context.Context, userID string) error {
	item, err := t.Store.Connection(ctx, userID, "telegram")
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}
	item.Status = "disconnected"
	item.Enabled = false
	item.ExternalUserID = ""
	return t.Store.SaveConnection(ctx, *item)
}
func (t *TelegramProvider) Send(ctx context.Context, recipient, message string) error {
	body, _ := json.Marshal(map[string]any{"chat_id": recipient, "text": message, "disable_web_page_preview": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+t.BotToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned %d", response.StatusCode)
	}
	return nil
}
func (t *TelegramProvider) Webhook(c *gin.Context) {
	if !t.Available() || c.GetHeader("X-Telegram-Bot-Api-Secret-Token") != t.WebhookSecret {
		c.Status(http.StatusUnauthorized)
		return
	}
	var update struct {
		Message struct {
			Text string `json:"text"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 || parts[0] != "/start" || update.Message.Chat.ID == 0 || update.Message.Chat.Type != "private" {
		c.Status(http.StatusOK)
		return
	}
	key := "telegram:link:" + parts[1]
	userID, err := t.Redis.Get(c, key).Result()
	if errors.Is(err, redis.Nil) {
		c.Status(http.StatusOK)
		return
	}
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	item := model.Connection{UserID: userID, Provider: "telegram", Status: "connected", Enabled: true, ExternalUserID: strconv.FormatInt(update.Message.Chat.ID, 10)}
	if err := t.Store.SaveConnection(c, item); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			_ = t.Redis.Del(c, key).Err()
			_ = t.Send(c, item.ExternalUserID, "This Telegram chat is already linked to another HunterJob account. Disconnect it there before trying again.")
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusServiceUnavailable)
		return
	}
	_ = t.Redis.Del(c, key).Err()
	_ = t.Send(c, item.ExternalUserID, "HunterJob connected. Your daily job digest will arrive here.")
	c.Status(http.StatusOK)
}
