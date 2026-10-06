package repository

import (
	"context"

	"github.com/hunterjob/hunterjob/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func (s Store) ConnectedTelegram(ctx context.Context) ([]model.Connection, error) {
	cur, err := s.DB.Collection("chat_connections").Find(ctx, bson.M{"provider": "telegram", "status": "connected", "enabled": true})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []model.Connection
	return items, cur.All(ctx, &items)
}

func (s Store) Undelivered(ctx context.Context, userID string) ([]model.Match, error) {
	cur, err := s.DB.Collection("job_matches").Find(ctx, bson.M{"user_id": userID, "delivered": false, "status": bson.M{"$ne": "dismissed"}}, options.Find().SetSort(bson.D{{Key: "matched_at", Value: -1}}).SetLimit(10))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var matches []model.Match
	return matches, cur.All(ctx, &matches)
}

func (s Store) RecordDelivery(ctx context.Context, delivery model.NotificationDelivery) error {
	_, err := s.DB.Collection("notification_deliveries").InsertOne(ctx, delivery)
	return err
}

func (s Store) MarkDelivered(ctx context.Context, userID string, ids []primitive.ObjectID, at time.Time) error {
	_, err := s.DB.Collection("job_matches").UpdateMany(ctx, bson.M{"_id": bson.M{"$in": ids}, "user_id": userID}, bson.M{"$set": bson.M{"delivered": true, "delivered_at": at}})
	return err
}
