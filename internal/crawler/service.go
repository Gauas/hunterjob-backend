package crawler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
)

type Service struct {
	Repository JobRepository
	Fetcher    Fetcher
	Normalizer Normalizer
}

type JobRepository interface {
	UpsertJob(context.Context, job.Job) error
}

type NormalizedJob struct {
	SourceJobID     string         `json:"source_job_id"`
	Title           string         `json:"title"`
	NormalizedTitle string         `json:"normalized_title"`
	Locations       []job.Location `json:"locations"`
	Levels          []string       `json:"levels"`
	EmploymentType  string         `json:"employment_type"`
	Experience      job.Experience `json:"experience"`
	Skills          []string       `json:"skills"`
	Description     string         `json:"description"`
	OriginalURL     string         `json:"original_url"`
	ApplyURL        string         `json:"apply_url"`
	ExpiredAt       *time.Time     `json:"expired_at"`
}

type Normalizer interface {
	NormalizeJobs(context.Context, company.Source, Response) ([]NormalizedJob, error)
}

func (s Service) Crawl(ctx context.Context, source company.Source) (int, error) {
	response, e := s.Fetcher.Fetch(ctx, source)
	if e != nil {
		return 0, e
	}
	normalizedJobs, e := s.Normalizer.NormalizeJobs(ctx, source, response)
	if e != nil {
		return 0, e
	}
	now := time.Now().UTC()
	stored := 0
	for _, item := range normalizedJobs {
		if item.ExpiredAt != nil && !item.ExpiredAt.After(now) {
			continue
		}
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		originalURL, e := absoluteHTTPURL(source.CareerURL, item.OriginalURL)
		if e != nil {
			originalURL = source.CareerURL
		}
		applyURL, e := absoluteHTTPURL(source.CareerURL, item.ApplyURL)
		if e != nil {
			applyURL = originalURL
		}
		normalizedTitle := strings.TrimSpace(item.NormalizedTitle)
		if normalizedTitle == "" {
			normalizedTitle = normalize(item.Title)
		}
		city := ""
		if len(item.Locations) > 0 {
			city = item.Locations[0].City
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(item.Description)))
		j := job.Job{CompanyID: source.CompanyID, SourceID: source.ID, SourceJobID: strings.TrimSpace(item.SourceJobID), Title: item.Title, NormalizedTitle: normalizedTitle, Locations: item.Locations, Levels: item.Levels, EmploymentType: item.EmploymentType, Experience: item.Experience, Skills: item.Skills, Description: item.Description, OriginalURL: originalURL, CanonicalURL: originalURL, ApplyURL: applyURL, ExpiredAt: item.ExpiredAt, ContentHash: hash, Fingerprint: job.Fingerprint(source.CompanyID, normalizedTitle, city)}
		if e = s.Repository.UpsertJob(ctx, j); e != nil {
			return stored, e
		}
		stored++
	}
	return stored, nil
}

func absoluteHTTPURL(baseURL, raw string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return "", fmt.Errorf("invalid job URL")
	}
	u = base.ResolveReference(u)
	if u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "" {
		return "", fmt.Errorf("invalid job URL")
	}
	return u.String(), nil
}

func normalize(s string) string {
	for _, x := range []string{"Junior", "Senior", "Intern", "Fresher", "-", "|"} {
		s = strings.ReplaceAll(s, x, "")
	}
	return strings.TrimSpace(s)
}
