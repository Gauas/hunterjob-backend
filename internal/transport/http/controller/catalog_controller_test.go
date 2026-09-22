package httptransport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type catalogRepositoryStub struct {
	jobs           []job.Job
	job            job.Job
	jobErr         error
	filter         bson.M
	skip, limit    int64
	clicked        primitive.ObjectID
	companies      []company.Company
	companiesTotal int64
	trackClickErr  error
}

func (s *catalogRepositoryStub) ListJobs(_ context.Context, filter bson.M, skip, limit int64) ([]job.Job, error) {
	s.filter, s.skip, s.limit = filter, skip, limit
	return s.jobs, nil
}
func (s *catalogRepositoryStub) CountJobs(context.Context, bson.M) (int64, error) {
	return int64(len(s.jobs)), nil
}
func (s *catalogRepositoryStub) Job(context.Context, primitive.ObjectID) (job.Job, error) {
	return s.job, s.jobErr
}
func (s *catalogRepositoryStub) TrackClick(_ context.Context, id primitive.ObjectID, _ time.Time) error {
	s.clicked = id
	return s.trackClickErr
}
func (s *catalogRepositoryStub) ListCompanies(_ context.Context, skip, limit int64) ([]company.Company, error) {
	s.skip, s.limit = skip, limit
	return s.companies, nil
}
func (s *catalogRepositoryStub) CountCompanies(context.Context) (int64, error) {
	return s.companiesTotal, nil
}

func TestListJobsUsesValidatedFiltersAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &catalogRepositoryStub{jobs: []job.Job{{Title: "Platform Engineer"}}}
	router := gin.New()
	router.GET("/jobs", NewCatalogController(repository).ListJobs)

	request := httptest.NewRequest(http.MethodGet, "/jobs?page=2&limit=2&keyword=go.%2B&location=Ha%20Noi&remote=true", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if repository.skip != 2 || repository.limit != 2 {
		t.Fatalf("pagination = skip %d, limit %d; want skip 2, limit 2", repository.skip, repository.limit)
	}
	if repository.filter["active"] != true || repository.filter["locations.remote"] != true {
		t.Fatalf("unexpected filter: %#v", repository.filter)
	}
	keywordFilter := repository.filter["$or"].(bson.A)[0].(bson.M)["title"].(bson.M)["$regex"]
	if keywordFilter != "go\\.\\+" {
		t.Fatalf("keyword regex = %q, want escaped literal", keywordFilter)
	}
}

func TestListJobsRejectsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/jobs", NewCatalogController(&catalogRepositoryStub{}).ListJobs)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jobs?limit=101", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestRedirectToApplicationTracksOnlyValidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := primitive.NewObjectID()
	repository := &catalogRepositoryStub{job: job.Job{ApplyURL: "https://careers.example.com/apply"}}
	router := gin.New()
	router.GET("/jobs/:id/redirect", NewCatalogController(repository).RedirectToApplication)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jobs/"+id.Hex()+"/redirect", nil))
	if response.Code != http.StatusFound || response.Header().Get("Location") != repository.job.ApplyURL {
		t.Fatalf("redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
	if repository.clicked != id {
		t.Fatal("application click was not tracked")
	}

	repository.job = job.Job{ApplyURL: "javascript:alert(1)"}
	repository.clicked = primitive.NilObjectID
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jobs/"+id.Hex()+"/redirect", nil))
	if response.Code != http.StatusNotFound || repository.clicked != primitive.NilObjectID {
		t.Fatalf("invalid application URL response = %d, clicked = %s", response.Code, repository.clicked.Hex())
	}
}

func TestGetJobNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/jobs/:id", NewCatalogController(&catalogRepositoryStub{jobErr: errors.New("missing")}).GetJob)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jobs/"+primitive.NewObjectID().Hex(), nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
