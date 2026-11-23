package repository

import (
	"context"

	"github.com/hunterjob/hunterjob/api/entity"
)

func (s Store) ConnectedTelegram(ctx context.Context) ([]entity.Connection, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+connectionColumns+" FROM chat_connections WHERE provider = 'telegram' AND status = 'connected' AND enabled")
	return collect(rows, err, scanConnection)
}

func (s Store) Undelivered(ctx context.Context, userID string) ([]entity.Match, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+matchColumns+` FROM job_matches
		WHERE user_id = $1 AND NOT delivered AND status <> 'dismissed'
		ORDER BY matched_at DESC, id DESC LIMIT 10`, userID)
	return collect(rows, err, scanMatch)
}

func (s Store) RecordDelivery(ctx context.Context, d entity.NotificationDelivery) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO notification_deliveries
		(user_id, provider, match_ids, status, attempts, sent_at, error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		d.UserID, d.Provider, d.MatchIDs, d.Status, d.Attempts, d.SentAt, d.Error, d.CreatedAt)
	if err != nil {
		return err
	}
	if d.SentAt != nil {
		_, err = tx.Exec(ctx, "UPDATE job_matches SET delivered = true, delivered_at = $3 WHERE user_id = $1 AND id = ANY($2)", d.UserID, d.MatchIDs, d.SentAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
