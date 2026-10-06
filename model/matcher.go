package model

import (
	"strings"
	"time"
)

// LakeJob is the normalized job.new event contract owned by Job Lake.
type LakeJob struct {
	ID             string     `json:"id"`
	ExternalID     string     `json:"external_id"`
	Source         string     `json:"source"`
	Title          string     `json:"title"`
	CompanyName    string     `json:"company_name"`
	CompanyLogoURL string     `json:"company_logo_url"`
	Location       string     `json:"location"`
	EmploymentType string     `json:"employment_type"`
	ExperienceMin  *int       `json:"experience_min"`
	ExperienceMax  *int       `json:"experience_max"`
	Skills         []string   `json:"skills"`
	Description    string     `json:"description"`
	JobURL         string     `json:"job_url"`
	PublishedAt    *time.Time `json:"published_at"`
	DiscoveredAt   time.Time  `json:"discovered_at"`
	Status         string     `json:"status"`
}

func containsWord(text, word string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(word)))
}
func Evaluate(job LakeJob, p Preference) (string, []string, bool) {
	if job.Status != "active" || !p.Enabled {
		return "", nil, false
	}
	location := false
	for _, wanted := range p.Locations {
		if strings.EqualFold(strings.TrimSpace(job.Location), wanted) || strings.EqualFold(wanted, "remote") && containsWord(job.Location, "remote") {
			location = true
			break
		}
	}
	if !location {
		return "", nil, false
	}
	for _, excluded := range p.ExcludedKeywords {
		if excluded != "" && (containsWord(job.Title, excluded) || containsWord(job.Description, excluded)) {
			return "", nil, false
		}
	}
	roleWords := strings.Fields(strings.ToLower(p.Role))
	roleHits := 0
	distinctive := 0
	distinctiveHits := 0
	for _, word := range roleWords {
		if len(word) > 2 && containsWord(job.Title, word) {
			roleHits++
			if word != "engineer" && word != "developer" && word != "specialist" {
				distinctiveHits++
			}
		}
		if len(word) > 2 && word != "engineer" && word != "developer" && word != "specialist" {
			distinctive++
		}
	}
	if roleHits == 0 || distinctive > 0 && distinctiveHits == 0 {
		return "", nil, false
	}
	if p.Experience.MaxYears != nil && job.ExperienceMin != nil && *job.ExperienceMin > *p.Experience.MaxYears {
		return "", nil, false
	}
	if job.ExperienceMax != nil && *job.ExperienceMax < p.Experience.MinYears {
		return "", nil, false
	}
	matched := []string{}
	for _, keyword := range p.Keywords {
		for _, skill := range job.Skills {
			if strings.EqualFold(skill, keyword) {
				matched = append(matched, keyword)
				break
			}
		}
		if !containsStringFold(matched, keyword) && containsWord(job.Description, keyword) {
			matched = append(matched, keyword)
		}
	}
	points := 20 + 20 + 20 // location, title and experience passed the hard filters
	if roleHits == len(roleWords) {
		points += 20
	} else {
		points += 10
	}
	if len(p.Keywords) == 0 {
		points += 10
	} else {
		points += 20 * len(matched) / len(p.Keywords)
	}
	if points >= 80 {
		return "strong", matched, true
	}
	if points >= 60 {
		return "good", matched, true
	}
	return "possible", matched, true
}
func containsStringFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}
