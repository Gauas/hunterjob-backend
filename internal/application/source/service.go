package source

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/company"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidURL = errors.New("career_url must be a public HTTP(S) URL")

type CreateInput struct {
	CompanyName, CompanyWebsite, CompanyLogoURL, CareerURL, CareerPageURL, Provider, JobURLTemplate, ApplyURLTemplate string
	CrawlIntervalMinutes                                                                                              int
}

type Repository interface {
	FindOrCreateCompany(context.Context, string, string, string) (company.Company, error)
	CreateSource(context.Context, company.Source) (company.Source, error)
	EnabledSourceIDs(context.Context) ([]primitive.ObjectID, error)
	SourceExists(context.Context, primitive.ObjectID) (bool, error)
}

type Queue interface {
	EnqueueCrawl(context.Context, primitive.ObjectID) error
}

type Service struct {
	repo  Repository
	queue Queue
}

func NewService(repo Repository, queue Queue) Service { return Service{repo: repo, queue: queue} }
func (s Service) Add(ctx context.Context, in CreateInput) (company.Source, error) {
	if strings.TrimSpace(in.CompanyName) == "" {
		return company.Source{}, errors.New("company_name is required")
	}
	u, e := url.Parse(in.CareerURL)
	if e != nil || u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "" {
		return company.Source{}, ErrInvalidURL
	}
	if strings.TrimSpace(in.CareerPageURL) != "" {
		pageURL, parseErr := url.Parse(strings.TrimSpace(in.CareerPageURL))
		if parseErr != nil || pageURL.Scheme != "http" && pageURL.Scheme != "https" || pageURL.Hostname() == "" {
			return company.Source{}, errors.New("career_page_url must be an absolute HTTP(S) URL")
		}
	}
	if e = validateURLTemplate(in.JobURLTemplate, "job_url_template"); e != nil {
		return company.Source{}, e
	}
	if e = validateURLTemplate(in.ApplyURLTemplate, "apply_url_template"); e != nil {
		return company.Source{}, e
	}
	c, e := s.repo.FindOrCreateCompany(ctx, in.CompanyName, in.CompanyWebsite, strings.TrimSpace(in.CompanyLogoURL))
	if e != nil {
		return company.Source{}, e
	}
	if in.Provider == "" {
		in.Provider = "generic"
	}
	if in.CrawlIntervalMinutes <= 0 {
		in.CrawlIntervalMinutes = 360
	}
	now := time.Now().UTC()
	return s.repo.CreateSource(ctx, company.Source{CompanyID: c.ID, CareerURL: in.CareerURL, CareerPageURL: strings.TrimSpace(in.CareerPageURL), Provider: in.Provider, JobURLTemplate: strings.TrimSpace(in.JobURLTemplate), ApplyURLTemplate: strings.TrimSpace(in.ApplyURLTemplate), URLPatterns: []string{"/job/", "/jobs/", "/career/", "/careers/", "/position/"}, CrawlIntervalMinutes: in.CrawlIntervalMinutes, Enabled: true, NextCrawlAt: now, CreatedAt: now, UpdatedAt: now})
}

func validateURLTemplate(value, field string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.Contains(value, "{id}") {
		return errors.New(field + " must contain {id}")
	}
	rendered := strings.NewReplacer("{id}", "example-id", "{slug}", "example-job", "{title}", "example-job").Replace(value)
	u, err := url.Parse(rendered)
	if err != nil || u.IsAbs() && (u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "") {
		return errors.New(field + " must be an absolute or source-relative HTTP(S) URL template")
	}
	return nil
}

func (s Service) Restart(ctx context.Context) (int, error) {
	ids, e := s.repo.EnabledSourceIDs(ctx)
	if e != nil {
		return 0, e
	}
	for _, id := range ids {
		if e = s.queue.EnqueueCrawl(ctx, id); e != nil {
			return 0, e
		}
	}
	return len(ids), nil
}

func (s Service) CrawlOne(ctx context.Context, id primitive.ObjectID) error {
	ok, e := s.repo.SourceExists(ctx, id)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("enabled source not found")
	}
	return s.queue.EnqueueCrawl(ctx, id)
}
