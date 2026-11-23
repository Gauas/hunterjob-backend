package entity

import (
	"strings"

	"github.com/hunterjob/hunterjob/api/internal/geography"
	"github.com/hunterjob/hunterjob/api/internal/jobevent"
)

type LakeJob = jobevent.Job

func containsWord(text, word string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(word)))
}
func Evaluate(job LakeJob, p Preference) (string, []string, bool) {
	if job.Status != "active" || !p.Enabled {
		return "", nil, false
	}
	location := false
	for _, wanted := range p.Locations {
		if strings.EqualFold(strings.TrimSpace(job.Location), wanted) ||
			geography.IsHoChiMinh(job.Location) && geography.IsHoChiMinh(wanted) ||
			strings.EqualFold(wanted, "remote") && containsWord(job.Location, "remote") {
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
