package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/entity"
)

type JobRepository interface {
	ActivePreferences(context.Context) ([]entity.Preference, error)
	SaveJobMatch(context.Context, entity.Match) (bool, error)
}

type Matcher struct{ Store JobRepository }

func (s Matcher) ProcessJob(ctx context.Context, item entity.LakeJob) (int, error) {
	if item.ID == "" || item.Title == "" || item.Status != "active" {
		return 0, nil
	}
	u, err := url.Parse(item.JobURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return 0, nil
	}
	if logo := strings.TrimSpace(item.CompanyLogoURL); logo != "" {
		parsed, parseErr := url.Parse(logo)
		if parseErr != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			item.CompanyLogoURL = ""
		}
	}
	preferences, err := s.Store.ActivePreferences(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, preference := range preferences {
		level, keywords, ok := entity.Evaluate(item, preference)
		if !ok {
			continue
		}
		now := time.Now().UTC()
		reason := fmt.Sprintf("%s in %s matches your %s preference", item.Title, item.Location, preference.Role)
		if len(keywords) > 0 {
			reason += " and mentions " + strings.Join(keywords, ", ")
		}
		match := entity.Match{UserID: preference.UserID, JobID: item.ID, DedupeKey: dedupeKey(item), PreferenceID: preference.ID, MatchLevel: level, MatchedKeywords: keywords, MatchReason: reason, Status: "new", MatchedAt: now, CreatedAt: now, Job: entity.JobSnapshot{Title: item.Title, ExperienceMin: item.ExperienceMin, ExperienceMax: item.ExperienceMax, CompanyName: item.CompanyName, CompanyLogoURL: item.CompanyLogoURL, JobURL: item.JobURL}}
		inserted, err := s.Store.SaveJobMatch(ctx, match)
		if err != nil {
			return count, err
		}
		if inserted {
			count++
		}
	}
	return count, nil
}

func dedupeKey(item entity.LakeJob) string {
	identity := item.ID
	if item.ExternalID != "" {
		identity = item.Source + "|" + item.ExternalID
	} else if item.JobURL != "" {
		identity = item.JobURL
	}
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(identity))))
	return hex.EncodeToString(hash[:])
}
