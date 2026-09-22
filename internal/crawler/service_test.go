package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fetcherStub struct{ response Response }

func (s fetcherStub) Fetch(context.Context, company.Source) (Response, error) { return s.response, nil }

type normalizerStub struct{ jobs []NormalizedJob }

func (s normalizerStub) NormalizeJobs(context.Context, company.Source, Response) ([]NormalizedJob, error) {
	return s.jobs, nil
}

type jobRepositoryStub struct{ jobs []job.Job }

func (s *jobRepositoryStub) UpsertJob(_ context.Context, item job.Job) error {
	s.jobs = append(s.jobs, item)
	return nil
}

func TestCrawlSkipsExpiredJobsAndStoresUnexpiredJobs(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	repository := &jobRepositoryStub{}
	source := company.Source{ID: primitive.NewObjectID(), CompanyID: primitive.NewObjectID(), CareerURL: "https://careers.example.com/jobs"}
	service := Service{
		Repository: repository,
		Fetcher:    fetcherStub{response: Response{URL: source.CareerURL}},
		Normalizer: normalizerStub{jobs: []NormalizedJob{
			{Title: "Expired", OriginalURL: "/jobs/expired", ExpiredAt: &past},
			{Title: "Current", NormalizedTitle: "Current", OriginalURL: "/jobs/current", ApplyURL: "/apply/current", ExpiredAt: &future},
		}},
	}

	stored, err := service.Crawl(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if stored != 1 || len(repository.jobs) != 1 {
		t.Fatalf("stored = %d, repository jobs = %d; want 1", stored, len(repository.jobs))
	}
	if repository.jobs[0].Title != "Current" || repository.jobs[0].OriginalURL != "https://careers.example.com/jobs/current" {
		t.Fatalf("unexpected stored job: %#v", repository.jobs[0])
	}
}
