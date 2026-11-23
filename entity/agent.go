package entity

import (
	"strings"
	"time"
)

type Experience struct {
	Type     string `json:"type"`
	MinYears int    `json:"min_years"`
	MaxYears *int   `json:"max_years"`
}
type Preference struct {
	ID               string     `json:"id"`
	UserID           string     `json:"-"`
	Role             string     `json:"role"`
	Experience       Experience `json:"experience"`
	Locations        []string   `json:"locations"`
	Keywords         []string   `json:"keywords"`
	ExcludedKeywords []string   `json:"excluded_keywords"`
	Frequency        string     `json:"frequency"`
	Enabled          bool       `json:"enabled"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
type JobSnapshot struct {
	Title          string `json:"title"`
	ExperienceMin  *int   `json:"experience_min,omitempty"`
	ExperienceMax  *int   `json:"experience_max,omitempty"`
	CompanyName    string `json:"company_name"`
	CompanyLogoURL string `json:"company_logo_url,omitempty"`
	JobURL         string `json:"job_url"`
}
type Match struct {
	ID              string      `json:"id"`
	UserID          string      `json:"-"`
	JobID           string      `json:"job_id"`
	DedupeKey       string      `json:"-"`
	PreferenceID    string      `json:"preference_id"`
	MatchLevel      string      `json:"match_level"`
	MatchedKeywords []string    `json:"matched_keywords"`
	MatchReason     string      `json:"match_reason"`
	Status          string      `json:"status"`
	MatchedAt       time.Time   `json:"matched_at"`
	Delivered       bool        `json:"delivered"`
	DeliveredAt     *time.Time  `json:"delivered_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	Job             JobSnapshot `json:"job"`
}
type Connection struct {
	ID             string    `json:"id"`
	UserID         string    `json:"-"`
	Provider       string    `json:"provider"`
	Status         string    `json:"status"`
	Enabled        bool      `json:"enabled"`
	ExternalUserID string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type ConnectionView struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Connected bool   `json:"connected"`
	Enabled   bool   `json:"enabled"`
}

type NotificationDelivery struct {
	ID        string     `json:"id"`
	UserID    string     `json:"-"`
	Provider  string     `json:"provider"`
	MatchIDs  []string   `json:"match_ids"`
	Status    string     `json:"status"`
	Attempts  int        `json:"attempts"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
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
