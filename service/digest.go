package service

import (
	"context"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hunterjob/hunterjob/api/entity"
	"github.com/hunterjob/hunterjob/api/internal/agent"
	"github.com/hunterjob/hunterjob/api/repository"
)

type Digest struct {
	Store    repository.Store
	Telegram *agent.TelegramProvider
}

func (d Digest) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for ctx.Err() == nil {
		d.deliverDaily(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d Digest) deliverDaily(ctx context.Context, now time.Time) {
	if !d.Telegram.Available() {
		return
	}
	// Vietnam uses UTC+7. Start each user in a stable ten-minute slot.
	now = now.In(time.FixedZone("ICT", 7*3600))
	if now.Hour() < 7 || now.Hour() >= 9 {
		return
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	connections, err := d.Store.PendingTelegram(ctx, dayStart)
	if err != nil {
		log.Printf("daily connections: %v", err)
		return
	}
	for _, connection := range connections {
		slot := dailySlot(connection.UserID)
		if (now.Hour()-7)*6+now.Minute()/10 < slot || ctx.Err() != nil {
			continue
		}
		err := d.Store.DeliverDigest(ctx, connection, dayStart, func(matches []entity.Match) (int, error) {
			message, count := formatDigest(matches)
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return count, d.Telegram.Send(sendCtx, connection.ExternalUserID, message)
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("daily digest for %s: %v", connection.UserID, err)
		}
	}
}

func formatDigest(matches []entity.Match) (string, int) {
	const telegramLimit = 4096
	const header = "New jobs matched for you\n"
	var message strings.Builder
	message.WriteString(header)
	remaining := telegramLimit - len(header)
	for i, item := range matches {
		title := excerpt(item.Job.Title, 200)
		company := excerpt(item.Job.CompanyName, 100)
		link := item.Job.JobURL
		if utf8.RuneCountInString(link) > 3000 {
			link = "Open your HunterJob dashboard to view this posting."
		}
		row := strings.Join([]string{"", strconv.Itoa(i+1) + ". " + title, company, experienceLabel(item.Job), link, ""}, "\n")
		size := utf8.RuneCountInString(row)
		if size > remaining {
			return message.String(), i
		}
		message.WriteString(row)
		remaining -= size
	}
	return message.String(), len(matches)
}

func experienceLabel(job entity.JobSnapshot) string {
	switch {
	case job.ExperienceMin != nil && job.ExperienceMax != nil:
		return strconv.Itoa(*job.ExperienceMin) + "–" + strconv.Itoa(*job.ExperienceMax) + " years"
	case job.ExperienceMin != nil:
		return strconv.Itoa(*job.ExperienceMin) + "+ years"
	case job.ExperienceMax != nil:
		return "Up to " + strconv.Itoa(*job.ExperienceMax) + " years"
	default:
		return "Experience not specified"
	}
}

func excerpt(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return value
}

func dailySlot(userID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	return int(h.Sum32() % 12)
}
