package agent

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/repository"
)

type ZaloProvider struct {
	channel
	oaID, accessToken, webhookSecret string
	client                           *http.Client
}

func NewZalo(store repository.Store, oaID, accessToken, webhookSecret string) *ZaloProvider {
	if oaID == "" || accessToken == "" || webhookSecret == "" {
		return nil
	}
	return &ZaloProvider{
		channel: channel{name: "zalo", url: "https://zalo.me/" + url.PathEscape(oaID), store: store},
		oaID:    oaID, accessToken: accessToken, webhookSecret: webhookSecret,
	}
}

func (z *ZaloProvider) Connect(ctx context.Context, userID string) (ConnectionResult, error) {
	result, err := z.channel.Connect(ctx, userID)
	if err == nil {
		result.Command = "#" + strings.TrimPrefix(result.Command, "/")
	}
	return result, err
}

func (z *ZaloProvider) Webhook(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var event struct {
		AppID     string `json:"app_id"`
		Timestamp string `json:"timestamp"`
		Name      string `json:"event_name"`
		Sender    struct {
			ID string `json:"id"`
		} `json:"sender"`
		Recipient struct {
			ID string `json:"id"`
		} `json:"recipient"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
	}
	if json.Unmarshal(body, &event) != nil || event.AppID == "" || event.Timestamp == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	checksum := sha256.Sum256([]byte(event.AppID + string(body) + event.Timestamp + z.webhookSecret))
	signature, ok := strings.CutPrefix(c.GetHeader("X-ZEvent-Signature"), "mac=")
	decoded, err := hex.DecodeString(signature)
	if !ok || err != nil || subtle.ConstantTimeCompare(decoded, checksum[:]) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}
	if event.Name == "user_send_text" && event.Recipient.ID == z.oaID && event.Sender.ID != "" {
		if err := z.handleText(c.Request.Context(), event.Sender.ID, event.Message.Text, 1800, z.Send); err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
	}
	c.Status(http.StatusOK)
}

func (z *ZaloProvider) Send(ctx context.Context, recipient, text string) error {
	const endpoint = "https://openapi.zalo.me/v3.0/oa/message/cs"
	var result struct {
		Error *int `json:"error"`
	}
	err := postJSON(ctx, z.client, endpoint, map[string]string{"access_token": z.accessToken}, map[string]any{
		"recipient": map[string]string{"user_id": recipient},
		"message":   map[string]string{"text": text},
	}, &result)
	if err != nil {
		return err
	}
	if result.Error == nil {
		return fmt.Errorf("Zalo did not acknowledge the message")
	}
	if *result.Error != 0 {
		return fmt.Errorf("Zalo returned error %d", *result.Error)
	}
	return nil
}
