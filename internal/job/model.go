package job

import (
	"crypto/sha256"
	"encoding/hex"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

type Location struct {
	City    string `bson:"city" json:"city"`
	Country string `bson:"country" json:"country"`
	Remote  bool   `bson:"remote" json:"remote"`
}
type Experience struct {
	MinYears int `bson:"min_years" json:"min_years"`
	MaxYears int `bson:"max_years" json:"max_years"`
}
type Job struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CompanyID       primitive.ObjectID `bson:"company_id" json:"company_id"`
	SourceID        primitive.ObjectID `bson:"source_id" json:"source_id"`
	Title           string             `bson:"title" json:"title"`
	NormalizedTitle string             `bson:"normalized_title" json:"normalized_title"`
	Locations       []Location         `bson:"locations" json:"locations"`
	Levels          []string           `bson:"levels" json:"levels"`
	EmploymentType  string             `bson:"employment_type" json:"employment_type"`
	Experience      Experience         `bson:"experience" json:"experience"`
	Skills          []string           `bson:"skills" json:"skills"`
	Description     string             `bson:"description" json:"description"`
	ExpiredAt       *time.Time         `bson:"expired_at,omitempty" json:"expired_at,omitempty"`
	FirstSeenAt     time.Time          `bson:"first_seen_at" json:"first_seen_at"`
	LastSeenAt      time.Time          `bson:"last_seen_at" json:"last_seen_at"`
	Active          bool               `bson:"active" json:"active"`
	OriginalURL     string             `bson:"original_url" json:"original_url"`
	CanonicalURL    string             `bson:"canonical_url" json:"canonical_url"`
	ApplyURL        string             `bson:"apply_url" json:"apply_url"`
	ContentHash     string             `bson:"content_hash" json:"content_hash"`
	Fingerprint     string             `bson:"fingerprint" json:"-"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

func Fingerprint(companyID primitive.ObjectID, title, city string) string {
	h := sha256.Sum256([]byte(companyID.Hex() + "|" + strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(city))))
	return hex.EncodeToString(h[:])
}
