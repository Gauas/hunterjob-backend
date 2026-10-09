package repository

import (
	"context"
	"errors"
	"github.com/hunterjob/hunterjob/api/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct{ DB *mongo.Database }

func (s Store) Indexes(ctx context.Context) error {
	indexes := []struct {
		collection string
		models     []mongo.IndexModel
	}{
		{"search_preferences", []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetName("user_unique").SetUnique(true)}}},
		{"job_matches", []mongo.IndexModel{
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "job_id", Value: 1}}, Options: options.Index().SetName("user_job_unique").SetUnique(true)},
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "dedupe_key", Value: 1}}, Options: options.Index().SetName("user_dedupe_unique").SetUnique(true).SetPartialFilterExpression(bson.M{"dedupe_key": bson.M{"$type": "string"}})},
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "matched_at", Value: -1}}, Options: options.Index().SetName("user_recent_matches")},
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "status", Value: 1}, {Key: "matched_at", Value: -1}}, Options: options.Index().SetName("user_status_recent_matches")},
		}},
		{"chat_connections", []mongo.IndexModel{
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "provider", Value: 1}}, Options: options.Index().SetName("user_provider_unique").SetUnique(true)},
			{Keys: bson.D{{Key: "provider", Value: 1}, {Key: "external_user_id", Value: 1}}, Options: options.Index().SetName("telegram_chat_unique").SetUnique(true).SetPartialFilterExpression(bson.M{"provider": "telegram", "status": "connected", "external_user_id": bson.M{"$type": "string"}})},
		}},
		{"notification_deliveries", []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("user_recent_deliveries")}}},
	}
	for _, item := range indexes {
		if _, err := s.DB.Collection(item.collection).Indexes().CreateMany(ctx, item.models); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) Preference(ctx context.Context, userID string) (*model.Preference, error) {
	var p model.Preference
	err := s.DB.Collection("search_preferences").FindOne(ctx, bson.M{"user_id": userID}).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
func (s Store) SavePreference(ctx context.Context, p model.Preference) (*model.Preference, error) {
	now := time.Now().UTC()
	update := bson.M{"$set": bson.M{"role": p.Role, "experience": p.Experience, "locations": p.Locations, "keywords": p.Keywords, "excluded_keywords": p.ExcludedKeywords, "frequency": p.Frequency, "enabled": true, "updated_at": now}, "$setOnInsert": bson.M{"user_id": p.UserID, "created_at": now}}
	var saved model.Preference
	err := s.DB.Collection("search_preferences").FindOneAndUpdate(ctx, bson.M{"user_id": p.UserID}, update, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&saved)
	return &saved, err
}
func (s Store) Matches(ctx context.Context, userID, status, level string, before *model.Match, limit int64) ([]model.Match, error) {
	filter := bson.M{"user_id": userID}
	if status != "" {
		filter["status"] = status
	}
	if level != "" {
		filter["match_level"] = level
	}
	if before != nil {
		filter["$or"] = bson.A{bson.M{"matched_at": bson.M{"$lt": before.MatchedAt}}, bson.M{"matched_at": before.MatchedAt, "_id": bson.M{"$lt": before.ID}}}
	}
	cur, err := s.DB.Collection("job_matches").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "matched_at", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var matches []model.Match
	err = cur.All(ctx, &matches)
	if matches == nil {
		matches = []model.Match{}
	}
	return matches, err
}
func (s Store) Match(ctx context.Context, userID string, id primitive.ObjectID) (*model.Match, error) {
	var m model.Match
	err := s.DB.Collection("job_matches").FindOne(ctx, bson.M{"_id": id, "user_id": userID}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (s Store) MatchByID(ctx context.Context, id primitive.ObjectID) (*model.Match, error) {
	var m model.Match
	err := s.DB.Collection("job_matches").FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (s Store) SetMatchStatus(ctx context.Context, userID string, id primitive.ObjectID, status string) (*model.Match, error) {
	var m model.Match
	err := s.DB.Collection("job_matches").FindOneAndUpdate(ctx, bson.M{"_id": id, "user_id": userID}, bson.M{"$set": bson.M{"status": status}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (s Store) Connections(ctx context.Context, userID string) ([]model.Connection, error) {
	cur, err := s.DB.Collection("chat_connections").Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []model.Connection
	return items, cur.All(ctx, &items)
}
func (s Store) Connection(ctx context.Context, userID, provider string) (*model.Connection, error) {
	var item model.Connection
	err := s.DB.Collection("chat_connections").FindOne(ctx, bson.M{"user_id": userID, "provider": provider}).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (s Store) SaveConnection(ctx context.Context, item model.Connection) error {
	now := time.Now().UTC()
	_, err := s.DB.Collection("chat_connections").UpdateOne(ctx, bson.M{"user_id": item.UserID, "provider": item.Provider}, bson.M{"$set": bson.M{"status": item.Status, "enabled": item.Enabled, "external_user_id": item.ExternalUserID, "updated_at": now}, "$setOnInsert": bson.M{"created_at": now}}, options.Update().SetUpsert(true))
	return err
}
