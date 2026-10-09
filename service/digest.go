package service

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/model"
	"github.com/hunterjob/hunterjob/api/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"strconv"
	"strings"
	"time"
)

func DailyLoop(ctx context.Context, store repository.Store, telegram *agent.TelegramProvider, cache *redis.Client) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		deliverDaily(ctx, store, telegram, cache)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func deliverDaily(ctx context.Context, store repository.Store, telegram *agent.TelegramProvider, cache *redis.Client) {
	if !telegram.Available() {
		return
	}
	// Each user gets a stable slot across the 07:00–09:00 UTC+7 delivery window.
	now := time.Now().In(time.FixedZone("ICT", 7*3600))
	if now.Hour() < 7 || now.Hour() >= 9 {
		return
	}
	connections, err := store.ConnectedTelegram(ctx)
	if err != nil {
		log.Printf("daily connections: %v", err)
		return
	}
	for _, connection := range connections {
		slot := int(hash(connection.UserID) % 12)
		if (now.Hour()-7)*6+now.Minute()/10 != slot {
			continue
		}
		lock := "digest:" + now.Format("2006-01-02") + ":" + connection.UserID
		ok, err := cache.SetNX(ctx, lock, "1", 24*time.Hour).Result()
		if err != nil || !ok {
			continue
		}
		matches, err := store.Undelivered(ctx, connection.UserID)
		if err != nil || len(matches) == 0 {
			_ = cache.Del(ctx, lock).Err()
			continue
		}
		message := formatDigest(matches)
		err = telegram.Send(ctx, connection.ExternalUserID, message)
		status := "sent"
		if err != nil {
			status = "failed"
			_ = cache.Del(ctx, lock).Err()
			log.Printf("digest %s: %v", connection.UserID, err)
		}
		ids := make([]primitive.ObjectID, len(matches))
		for i, match := range matches {
			ids[i] = match.ID
		}
		delivery := model.NotificationDelivery{UserID: connection.UserID, Provider: "telegram", MatchIDs: ids, Status: status, Attempts: 1, CreatedAt: time.Now().UTC()}
		if err == nil {
			sent := time.Now().UTC()
			delivery.SentAt = &sent
		} else {
			delivery.Error = err.Error()
		}
		if recordErr := store.RecordDelivery(ctx, delivery); recordErr != nil {
			log.Printf("record digest %s: %v", connection.UserID, recordErr)
		}
		if err == nil {
			_ = store.MarkDelivered(ctx, connection.UserID, ids, time.Now().UTC())
		}
	}
}
func formatDigest(matches []model.Match) string {
	var b strings.Builder
	b.WriteString("New jobs matched for you\n\n")
	for i, item := range matches {
		experience := "Experience not specified"
		if item.Job.ExperienceMin != nil {
			experience = strconv.Itoa(*item.Job.ExperienceMin) + "+ years"
			if item.Job.ExperienceMax != nil {
				experience = strconv.Itoa(*item.Job.ExperienceMin) + "–" + strconv.Itoa(*item.Job.ExperienceMax) + " years"
			}
		}
		b.WriteString(strings.Join([]string{strconv.Itoa(i+1) + ". " + item.Job.Title, item.Job.CompanyName, experience, item.Job.JobURL, ""}, "\n"))
	}
	return b.String()
}
func hash(value string) uint32 {
	var result uint32 = 2166136261
	for _, c := range []byte(value) {
		result ^= uint32(c)
		result *= 16777619
	}
	return result
}
