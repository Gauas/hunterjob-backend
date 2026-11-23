package agent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/repository"
)

type MessengerProvider struct {
	channel
	pageID, pageToken, appSecret, verifyToken string
	client                                    *http.Client
}

func NewMessenger(store repository.Store, pageID, pageToken, appSecret, verifyToken string) *MessengerProvider {
	if pageID == "" || pageToken == "" || appSecret == "" || verifyToken == "" {
		return nil
	}
	return &MessengerProvider{
		channel: channel{name: "messenger", url: "https://m.me/" + url.PathEscape(pageID), store: store},
		pageID:  pageID, pageToken: pageToken, appSecret: appSecret, verifyToken: verifyToken,
	}
}

func (m *MessengerProvider) Verify(c *gin.Context) {
	challenge := c.Query("hub.challenge")
	if c.Query("hub.mode") != "subscribe" || challenge == "" ||
		!hmac.Equal([]byte(c.Query("hub.verify_token")), []byte(m.verifyToken)) {
		c.Status(http.StatusForbidden)
		return
	}
	c.Data(http.StatusOK, "text/plain", []byte(challenge))
}

func (m *MessengerProvider) Webhook(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	mac := hmac.New(sha256.New, []byte(m.appSecret))
	mac.Write(body)
	signature, ok := strings.CutPrefix(c.GetHeader("X-Hub-Signature-256"), "sha256=")
	decoded, err := hex.DecodeString(signature)
	if !ok || err != nil || !hmac.Equal(decoded, mac.Sum(nil)) {
		c.Status(http.StatusUnauthorized)
		return
	}
	var event struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Message struct {
					Text string `json:"text"`
					Echo bool   `json:"is_echo"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &event) != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if event.Object == "page" {
		for _, entry := range event.Entry {
			if entry.ID != m.pageID {
				continue
			}
			for _, item := range entry.Messaging {
				if item.Message.Echo || item.Recipient.ID != m.pageID || item.Sender.ID == "" {
					continue
				}
				if err := m.handleText(c.Request.Context(), item.Sender.ID, item.Message.Text, 1800, m.Send); err != nil {
					c.Status(http.StatusServiceUnavailable)
					return
				}
			}
		}
	}
	c.Status(http.StatusOK)
}

// Send is called only in response to a user message in the standard messaging window.
func (m *MessengerProvider) Send(ctx context.Context, recipient, text string) error {
	endpoint := "https://graph.facebook.com/v25.0/" + url.PathEscape(m.pageID) + "/messages"
	var result struct {
		MessageID string `json:"message_id"`
	}
	err := postJSON(ctx, m.client, endpoint, map[string]string{"Authorization": "Bearer " + m.pageToken}, map[string]any{
		"messaging_type": "RESPONSE",
		"recipient":      map[string]string{"id": recipient},
		"message":        map[string]string{"text": text},
	}, &result)
	if err == nil && result.MessageID == "" {
		return errors.New("Messenger did not acknowledge the message")
	}
	return err
}
