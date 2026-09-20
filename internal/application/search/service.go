package search

import (
	"context"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"strings"
)

type Intent struct {
	Roles, Locations, Levels, Skills []string `json:"roles,omitempty"`
	ExperienceMax                    int      `json:"experience_max,omitempty"`
}
type Provider interface {
	ParseSearchQuery(context.Context, string) (Intent, error)
}
type Repository interface {
	SearchCandidates(context.Context, Intent, int64) ([]job.Job, error)
}
type Result struct {
	MatchScore  int      `json:"match_score"`
	Job         job.Job  `json:"job"`
	Matched     []string `json:"matched"`
	Missing     []string `json:"missing"`
	Explanation string   `json:"explanation"`
}
type Service struct {
	repo Repository
	ai   Provider
}

func NewService(repo Repository, ai Provider) Service { return Service{repo: repo, ai: ai} }
func (s Service) Search(ctx context.Context, query string) (Intent, []Result, error) {
	in, e := s.ai.ParseSearchQuery(ctx, query)
	if e != nil {
		return Intent{}, nil, e
	}
	jobs, e := s.repo.SearchCandidates(ctx, in, 50)
	if e != nil {
		return Intent{}, nil, e
	}
	out := make([]Result, 0, len(jobs))
	for _, j := range jobs {
		matched := []string{}
		missing := []string{}
		for _, skill := range in.Skills {
			if contains(j.Skills, skill) || strings.Contains(strings.ToLower(j.Description), strings.ToLower(skill)) {
				matched = append(matched, skill)
			} else {
				missing = append(missing, skill)
			}
		}
		for _, loc := range in.Locations {
			for _, have := range j.Locations {
				if strings.EqualFold(loc, have.City) || (strings.EqualFold(loc, "remote") && have.Remote) {
					matched = append(matched, loc)
					break
				}
			}
		}
		score := 40 + len(matched)*12 - len(missing)*4
		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}
		out = append(out, Result{MatchScore: score, Job: j, Matched: matched, Missing: missing, Explanation: "Matches only verified job fields and description text."})
	}
	return in, out, nil
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if strings.EqualFold(v, target) {
			return true
		}
	}
	return false
}
