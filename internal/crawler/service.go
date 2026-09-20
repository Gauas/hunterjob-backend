package crawler

import (
	"context"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"github.com/hunterjob/hunterjob/api/internal/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
)

type Service struct {
	Store   *store.Store
	Adapter Adapter
}

func (s Service) Crawl(ctx context.Context, source company.Source) (int, error) {
	candidates, e := s.Adapter.DiscoverJobs(ctx, source)
	if e != nil {
		return 0, e
	}
	for _, c := range candidates {
		id, _ := primitive.ObjectIDFromHex(c.CompanyID)
		city := location(c.RawLocation)
		normalized := normalize(c.RawTitle)
		j := job.Job{CompanyID: id, SourceID: source.ID, Title: c.RawTitle, NormalizedTitle: normalized, Locations: []job.Location{{City: city, Country: "Vietnam", Remote: strings.EqualFold(city, "Remote")}}, Levels: levels(c.RawText), EmploymentType: "full_time", Skills: skills(c.RawText), Description: c.RawText, OriginalURL: c.OriginalURL, CanonicalURL: c.OriginalURL, ApplyURL: c.OriginalURL, ContentHash: Hash(c.RawText), Fingerprint: job.Fingerprint(id, normalized, city)}
		if e = s.Store.UpsertJob(ctx, j); e != nil {
			return 0, e
		}
	}
	return len(candidates), nil
}
func normalize(s string) string {
	for _, x := range []string{"Junior", "Senior", "Intern", "Fresher", "-", "|"} {
		s = strings.ReplaceAll(s, x, "")
	}
	return strings.TrimSpace(s)
}
func levels(s string) []string {
	l := strings.ToLower(s)
	for _, x := range []string{"intern", "fresher", "junior", "senior"} {
		if strings.Contains(l, x) {
			return []string{x}
		}
	}
	return nil
}
func skills(s string) []string {
	var o []string
	for _, x := range []string{"Go", "Java", "Docker", "Kubernetes", "PostgreSQL", "AWS", "React", "Python"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(x)) {
			o = append(o, x)
		}
	}
	return o
}
