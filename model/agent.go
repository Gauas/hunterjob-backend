package model

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Experience struct {
	Type     string `bson:"type" json:"type"`
	MinYears int    `bson:"min_years" json:"min_years"`
	MaxYears *int   `bson:"max_years" json:"max_years"`
}
type Preference struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID           string             `bson:"user_id" json:"-"`
	Role             string             `bson:"role" json:"role"`
	Experience       Experience         `bson:"experience" json:"experience"`
	Locations        []string           `bson:"locations" json:"locations"`
	Keywords         []string           `bson:"keywords" json:"keywords"`
	ExcludedKeywords []string           `bson:"excluded_keywords" json:"excluded_keywords"`
	Frequency        string             `bson:"frequency" json:"frequency"`
	Enabled          bool               `bson:"enabled" json:"enabled"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}
type JobSnapshot struct {
	Title          string `bson:"title" json:"title"`
	ExperienceMin  *int   `bson:"experience_min,omitempty" json:"experience_min,omitempty"`
	ExperienceMax  *int   `bson:"experience_max,omitempty" json:"experience_max,omitempty"`
	CompanyName    string `bson:"company_name" json:"company_name"`
	CompanyLogoURL string `bson:"company_logo_url,omitempty" json:"company_logo_url,omitempty"`
	JobURL         string `bson:"job_url" json:"job_url"`
}
type Match struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          string             `bson:"user_id" json:"-"`
	JobID           string             `bson:"job_id" json:"job_id"`
	DedupeKey       string             `bson:"dedupe_key" json:"-"`
	PreferenceID    primitive.ObjectID `bson:"preference_id" json:"preference_id"`
	MatchLevel      string             `bson:"match_level" json:"match_level"`
	MatchedKeywords []string           `bson:"matched_keywords" json:"matched_keywords"`
	MatchReason     string             `bson:"match_reason" json:"match_reason"`
	Status          string             `bson:"status" json:"status"`
	MatchedAt       time.Time          `bson:"matched_at" json:"matched_at"`
	Delivered       bool               `bson:"delivered" json:"delivered"`
	DeliveredAt     *time.Time         `bson:"delivered_at,omitempty" json:"delivered_at,omitempty"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	Job             JobSnapshot        `bson:"job" json:"job"`
}
type Connection struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         string             `bson:"user_id" json:"-"`
	Provider       string             `bson:"provider" json:"provider"`
	Status         string             `bson:"status" json:"status"`
	Enabled        bool               `bson:"enabled" json:"enabled"`
	ExternalUserID string             `bson:"external_user_id,omitempty" json:"-"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}
type ConnectionView struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Connected bool   `json:"connected"`
	Enabled   bool   `json:"enabled"`
}

type NotificationDelivery struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	UserID    string               `bson:"user_id" json:"-"`
	Provider  string               `bson:"provider" json:"provider"`
	MatchIDs  []primitive.ObjectID `bson:"match_ids" json:"match_ids"`
	Status    string               `bson:"status" json:"status"`
	Attempts  int                  `bson:"attempts" json:"attempts"`
	SentAt    *time.Time           `bson:"sent_at,omitempty" json:"sent_at,omitempty"`
	Error     string               `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt time.Time            `bson:"created_at" json:"created_at"`
}

var Providers = []string{"telegram", "discord", "zalo", "messenger", "whatsapp", "email"}

func NormalizeExperience(value string) (Experience, bool) {
	var min int
	var max *int
	setMax := func(n int) { max = &n }
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "internship":
		setMax(0)
	case "fresher", "0-1":
		setMax(1)
	case "1-2":
		min = 1
		setMax(2)
	case "2-3":
		min = 2
		setMax(3)
	case "3-5":
		min = 3
		setMax(5)
	case "5+":
		min = 5
	default:
		return Experience{}, false
	}
	return Experience{Type: "range", MinYears: min, MaxYears: max}, true
}

func CleanStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}
