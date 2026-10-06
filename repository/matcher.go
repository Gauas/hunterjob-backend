package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s Store) ProcessJob(ctx context.Context, item model.LakeJob) (int, error) {
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
	filter := bson.M{"enabled": true, "locations": bson.M{"$regex": "^" + regexpEscape(item.Location) + "$", "$options": "i"}}
	cur, err := s.DB.Collection("search_preferences").Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	count := 0
	for cur.Next(ctx) {
		var preference model.Preference
		if err := cur.Decode(&preference); err != nil {
			return count, err
		}
		level, keywords, ok := model.Evaluate(item, preference)
		if !ok {
			continue
		}
		now := time.Now().UTC()
		reason := fmt.Sprintf("%s in %s matches your %s preference", item.Title, item.Location, preference.Role)
		if len(keywords) > 0 {
			reason += " and mentions " + strings.Join(keywords, ", ")
		}
		match := model.Match{UserID: preference.UserID, JobID: item.ID, DedupeKey: dedupeKey(item), PreferenceID: preference.ID, MatchLevel: level, MatchedKeywords: keywords, MatchReason: reason, Status: "new", MatchedAt: now, CreatedAt: now, Job: model.JobSnapshot{Title: item.Title, ExperienceMin: item.ExperienceMin, ExperienceMax: item.ExperienceMax, CompanyName: item.CompanyName, CompanyLogoURL: item.CompanyLogoURL, JobURL: item.JobURL}}
		_, err := s.DB.Collection("job_matches").InsertOne(ctx, match)
		if mongo.IsDuplicateKeyError(err) {
			// A replay from Job Lake can enrich an existing match without resetting its status.
			_, err = s.DB.Collection("job_matches").UpdateOne(ctx, bson.M{"user_id": preference.UserID, "$or": bson.A{bson.M{"dedupe_key": match.DedupeKey}, bson.M{"job_id": match.JobID}}}, bson.M{"$set": bson.M{"job": match.Job}})
			if err != nil {
				return count, err
			}
			continue
		}
		if err != nil {
			return count, err
		}
		count++
	}
	return count, cur.Err()
}
func dedupeKey(item model.LakeJob) string {
	identity := item.ID
	if item.ExternalID != "" {
		identity = item.Source + "|" + item.ExternalID
	} else if item.JobURL != "" {
		identity = item.JobURL
	}
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(identity))))
	return hex.EncodeToString(hash[:])
}
func regexpEscape(value string) string {
	var b strings.Builder
	for _, r := range value {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
