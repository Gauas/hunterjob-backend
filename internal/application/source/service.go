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
	CompanyName, CompanyWebsite, CareerURL, Provider string
	CrawlIntervalMinutes                             int
}
type Repository interface {
	FindOrCreateCompany(context.Context, string, string) (company.Company, error)
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
	c, e := s.repo.FindOrCreateCompany(ctx, in.CompanyName, in.CompanyWebsite)
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
	return s.repo.CreateSource(ctx, company.Source{CompanyID: c.ID, CareerURL: in.CareerURL, Provider: in.Provider, URLPatterns: []string{"/job/", "/jobs/", "/career/", "/careers/", "/position/"}, CrawlIntervalMinutes: in.CrawlIntervalMinutes, Enabled: true, CreatedAt: now, UpdatedAt: now})
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
