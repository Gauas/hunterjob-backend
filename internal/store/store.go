package store

import (
	"context"
	"regexp"
	"strings"
	"time"

	appsearch "github.com/hunterjob/hunterjob/api/internal/application/search"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/job"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct{ DB *mongo.Database }

func New(ctx context.Context, uri, database string) (*Store, *mongo.Client, error) {
	c, e := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if e != nil {
		return nil, nil, e
	}
	if e = c.Ping(ctx, nil); e != nil {
		return nil, nil, e
	}
	s := &Store{c.Database(database)}
	_, e = s.DB.Collection("jobs").Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "original_url", Value: 1}}, Options: options.Index().SetUnique(true)}, {Keys: bson.D{{Key: "fingerprint", Value: 1}}}, {Keys: bson.D{{Key: "active", Value: 1}, {Key: "last_seen_at", Value: -1}}}})
	return s, c, e
}

func (s *Store) FindOrCreateCompany(ctx context.Context, name, website string) (company.Company, error) {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	var c company.Company
	e := s.DB.Collection("companies").FindOne(ctx, bson.M{"slug": slug}).Decode(&c)
	if e == nil {
		return c, nil
	}
	if e != mongo.ErrNoDocuments {
		return c, e
	}
	now := time.Now().UTC()
	c = company.Company{Name: name, Slug: slug, Website: website, CreatedAt: now, UpdatedAt: now}
	r, e := s.DB.Collection("companies").InsertOne(ctx, c)
	if e == nil {
		c.ID = r.InsertedID.(primitive.ObjectID)
	}
	return c, e
}

func (s *Store) CreateSource(ctx context.Context, source company.Source) (company.Source, error) {
	existing := company.Source{}
	e := s.DB.Collection("career_sources").FindOne(ctx, bson.M{"career_url": source.CareerURL}).Decode(&existing)
	if e == nil {
		return existing, nil
	}
	if e != mongo.ErrNoDocuments {
		return company.Source{}, e
	}
	r, e := s.DB.Collection("career_sources").InsertOne(ctx, source)
	if e == nil {
		source.ID = r.InsertedID.(primitive.ObjectID)
	}
	return source, e
}

func (s *Store) EnabledSourceIDs(ctx context.Context) ([]primitive.ObjectID, error) {
	cur, e := s.DB.Collection("career_sources").Find(ctx, bson.M{"enabled": true}, options.Find().SetProjection(bson.M{"_id": 1}))
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	e = cur.All(ctx, &rows)
	ids := make([]primitive.ObjectID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	return ids, e
}

func (s *Store) SourceExists(ctx context.Context, id primitive.ObjectID) (bool, error) {
	n, e := s.DB.Collection("career_sources").CountDocuments(ctx, bson.M{"_id": id, "enabled": true})
	return n > 0, e
}
func (s *Store) SearchCandidates(ctx context.Context, in appsearch.Intent, limit int64) ([]job.Job, error) {
	f := bson.M{"active": true}
	ands := bson.A{}
	if len(in.Roles) > 0 {
		ors := bson.A{}
		for _, r := range in.Roles {
			ors = append(ors, bson.M{"normalized_title": bson.M{"$regex": regexp.QuoteMeta(r), "$options": "i"}})
		}
		ands = append(ands, bson.M{"$or": ors})
	}
	if len(in.Locations) > 0 {
		ors := bson.A{}
		for _, l := range in.Locations {
			if strings.EqualFold(l, "remote") {
				ors = append(ors, bson.M{"locations.remote": true})
			} else {
				ors = append(ors, bson.M{"locations.city": bson.M{"$regex": regexp.QuoteMeta(l), "$options": "i"}})
			}
		}
		ands = append(ands, bson.M{"$or": ors})
	}
	if len(in.Skills) > 0 {
		ands = append(ands, bson.M{"skills": bson.M{"$in": in.Skills}})
	}
	if in.ExperienceMax > 0 {
		ands = append(ands, bson.M{"experience.min_years": bson.M{"$lte": in.ExperienceMax}})
	}
	if len(ands) > 0 {
		f["$and"] = ands
	}
	return s.Jobs(ctx, f, limit)
}

func (s *Store) SeedTechvify(ctx context.Context) error {
	now := time.Now().UTC()
	c := company.Company{Name: "Techvify", Slug: "techvify", Website: "https://techvify.com", Locations: []string{"Da Nang", "Ha Noi"}, CreatedAt: now, UpdatedAt: now}
	var found company.Company
	e := s.DB.Collection("companies").FindOne(ctx, bson.M{"slug": "techvify"}).Decode(&found)
	if e == mongo.ErrNoDocuments {
		r, x := s.DB.Collection("companies").InsertOne(ctx, c)
		if x != nil {
			return x
		}
		c.ID = r.InsertedID.(primitive.ObjectID)
	} else if e != nil {
		return e
	} else {
		c = found
	}
	count, e := s.DB.Collection("career_sources").CountDocuments(ctx, bson.M{"company_id": c.ID})
	if e != nil || count > 0 {
		return e
	}
	_, e = s.DB.Collection("career_sources").InsertOne(ctx, company.Source{CompanyID: c.ID, CareerURL: "https://careers.techvify.com/jobs", Provider: "generic", URLPatterns: []string{"/job/", "/jobs/", "/career/"}, CrawlIntervalMinutes: 360, Enabled: true, CreatedAt: now, UpdatedAt: now})
	return e
}
func (s *Store) Sources(ctx context.Context) ([]company.Source, error) {
	cur, e := s.DB.Collection("career_sources").Find(ctx, bson.M{"enabled": true})
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	var v []company.Source
	e = cur.All(ctx, &v)
	return v, e
}
func (s *Store) Jobs(ctx context.Context, f bson.M, limit int64) ([]job.Job, error) {
	cur, e := s.DB.Collection("jobs").Find(ctx, f, options.Find().SetLimit(limit).SetSort(bson.D{{Key: "last_seen_at", Value: -1}}))
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	var js []job.Job
	e = cur.All(ctx, &js)
	return js, e
}
func (s *Store) Job(ctx context.Context, id primitive.ObjectID) (job.Job, error) {
	var j job.Job
	e := s.DB.Collection("jobs").FindOne(ctx, bson.M{"_id": id}).Decode(&j)
	return j, e
}
func (s *Store) UpsertJob(ctx context.Context, j job.Job) error {
	now := time.Now().UTC()
	j.LastSeenAt = now
	j.UpdatedAt = now
	j.Active = true
	if j.FirstSeenAt.IsZero() {
		j.FirstSeenAt = now
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	_, e := s.DB.Collection("jobs").UpdateOne(ctx, bson.M{"original_url": j.OriginalURL}, bson.M{"$set": j, "$setOnInsert": bson.M{"first_seen_at": j.FirstSeenAt, "created_at": j.CreatedAt}}, options.Update().SetUpsert(true))
	return e
}
