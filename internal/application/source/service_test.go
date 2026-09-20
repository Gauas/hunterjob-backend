package source

import (
	"context"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

type repoFake struct{ sources int }

func (r *repoFake) FindOrCreateCompany(context.Context, string, string) (company.Company, error) {
	return company.Company{ID: primitive.NewObjectID()}, nil
}
func (r *repoFake) CreateSource(_ context.Context, s company.Source) (company.Source, error) {
	s.ID = primitive.NewObjectID()
	return s, nil
}
func (r *repoFake) EnabledSourceIDs(context.Context) ([]primitive.ObjectID, error) {
	return []primitive.ObjectID{primitive.NewObjectID()}, nil
}
func (r *repoFake) SourceExists(context.Context, primitive.ObjectID) (bool, error) { return true, nil }

type queueFake struct{ n int }

func (q *queueFake) EnqueueCrawl(context.Context, primitive.ObjectID) error { q.n++; return nil }
func TestAddRejectsUnsafeScheme(t *testing.T) {
	r := &repoFake{}
	q := &queueFake{}
	_, e := NewService(r, q).Add(context.Background(), CreateInput{CompanyName: "Acme", CareerURL: "file:///tmp/jobs"})
	if e == nil {
		t.Fatal("expected invalid URL")
	}
}
func TestRestartQueuesEveryEnabledSource(t *testing.T) {
	r := &repoFake{}
	q := &queueFake{}
	n, e := NewService(r, q).Restart(context.Background())
	if e != nil || n != 1 || q.n != 1 {
		t.Fatalf("got n=%d queue=%d err=%v", n, q.n, e)
	}
}
