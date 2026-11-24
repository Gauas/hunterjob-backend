package agent

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/repository"
)

type TelegramProvider struct {
	BotUsername   string
	BotToken      string
	WebhookSecret string
	Store         repository.Store
	Client        *http.Client
}

func (t *TelegramProvider) Available() bool {
	return t != nil && t.BotUsername != "" && t.BotToken != "" && t.WebhookSecret != ""
}
func (t *TelegramProvider) Connect(ctx context.Context, userID string) (ConnectionResult, error) {
	token, err := (channel{name: "telegram", store: t.Store}).newLink(ctx, userID)
	if err != nil {
		return ConnectionResult{}, err
	}
	return ConnectionResult{Provider: "telegram", ConnectionURL: "https://t.me/" + url.PathEscape(strings.TrimPrefix(t.BotUsername, "@")) + "?start=" + token, ExpiresIn: 600}, nil
}

func (t *TelegramProvider) Disconnect(ctx context.Context, userID string) error {
	return t.Store.Disconnect(ctx, userID, "telegram")
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
		return errors.New("telegram request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned %d", response.StatusCode)
	}
	return nil
}
func (t *TelegramProvider) Webhook(c *gin.Context) {
	if !t.Available() || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Telegram-Bot-Api-Secret-Token")), []byte(t.WebhookSecret)) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
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
	ctx := c.Request.Context()
	recipient := strconv.FormatInt(update.Message.Chat.ID, 10)
	bound, err := t.Store.BindLink(ctx, "telegram", parts[1], recipient)
	if errors.Is(err, repository.ErrRecipientInUse) {
		_ = t.Send(ctx, recipient, "This Telegram chat is already linked to another HunterJob account. Disconnect it there before trying again.")
		c.Status(http.StatusOK)
		return
	}
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	if bound {
		_ = t.Send(ctx, recipient, "HunterJob connected. Your daily job digest will arrive here.")
	}
	c.Status(http.StatusOK)
}
