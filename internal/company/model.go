package company

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Company struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Slug      string             `bson:"slug" json:"slug"`
	Website   string             `bson:"website" json:"website"`
	LogoURL   string             `bson:"logo_url" json:"logo_url"`
	Locations []string           `bson:"locations" json:"locations"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}
type Source struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CompanyID            primitive.ObjectID `bson:"company_id" json:"company_id"`
	CareerURL            string             `bson:"career_url" json:"career_url"`
	Provider             string             `bson:"provider" json:"provider"`
	URLPatterns          []string           `bson:"url_patterns" json:"url_patterns"`
	CrawlIntervalMinutes int                `bson:"crawl_interval_minutes" json:"crawl_interval_minutes"`
	Enabled              bool               `bson:"enabled" json:"enabled"`
	LastContentHash      string             `bson:"last_content_hash" json:"last_content_hash"`
	ConsecutiveFailures  int                `bson:"consecutive_failures" json:"consecutive_failures"`
	CreatedAt            time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time          `bson:"updated_at" json:"updated_at"`
}
