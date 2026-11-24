package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hunterjob/hunterjob/api/entity"
	"github.com/jackc/pgx/v5"
)

func (s Store) PendingTelegram(ctx context.Context, dayStart time.Time) ([]entity.Connection, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+connectionColumns+` FROM chat_connections c
		WHERE c.provider = 'telegram' AND c.status = 'connected' AND c.enabled
		  AND EXISTS (SELECT 1 FROM job_matches m WHERE m.user_id = c.user_id AND NOT m.delivered AND m.status <> 'dismissed')
		  AND NOT EXISTS (SELECT 1 FROM notification_deliveries d WHERE d.user_id = c.user_id AND d.provider = c.provider AND d.sent_at >= $1)`, dayStart)
	return collect(rows, err, scanConnection)
}

// DeliverDigest serializes a user's attempts across worker replicas. It commits
// the audit row and match flags together after the provider acknowledges a send.
func (s Store) DeliverDigest(ctx context.Context, connection entity.Connection, dayStart time.Time, send func([]entity.Match) (int, error)) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	defer tx.Rollback(finishCtx)

	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtext($1), hashtext($2))", connection.Provider, connection.UserID).Scan(&locked); err != nil || !locked {
		return err
	}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM chat_connections WHERE user_id = $1 AND provider = $2 AND status = 'connected'
		AND enabled AND external_user_id = $3
	) AND NOT EXISTS (
		SELECT 1 FROM notification_deliveries WHERE user_id = $1 AND provider = $2 AND sent_at >= $4
	)`, connection.UserID, connection.Provider, connection.ExternalUserID, dayStart).Scan(&eligible)
	if err != nil || !eligible {
		return err
	}
	rows, err := tx.Query(ctx, "SELECT "+matchColumns+` FROM job_matches
		WHERE user_id = $1 AND NOT delivered AND status <> 'dismissed'
		ORDER BY matched_at DESC, id DESC LIMIT 10`, connection.UserID)
	matches, err := collect(rows, err, scanMatch)
	if err != nil || len(matches) == 0 {
		return err
	}
	count, sendErr := send(matches)
	if count < 1 || count > len(matches) {
		return errors.New("invalid digest match count")
	}
	ids := make([]string, count)
	for i, match := range matches[:count] {
		ids[i] = match.ID
	}
	now := time.Now().UTC()
	delivery := entity.NotificationDelivery{
		UserID: connection.UserID, Provider: connection.Provider, MatchIDs: ids,
		Status: "sent", Attempts: 1, SentAt: &now, CreatedAt: now,
	}
	if sendErr != nil {
		delivery.Status, delivery.SentAt, delivery.Error = "failed", nil, sendErr.Error()
	}
	if err := recordDelivery(finishCtx, tx, delivery); err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	if err := tx.Commit(finishCtx); err != nil {
		return err
	}
	return sendErr
}

func recordDelivery(ctx context.Context, tx pgx.Tx, d entity.NotificationDelivery) error {
	_, err := tx.Exec(ctx, `INSERT INTO notification_deliveries
		(user_id, provider, match_ids, status, attempts, sent_at, error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		d.UserID, d.Provider, d.MatchIDs, d.Status, d.Attempts, d.SentAt, d.Error, d.CreatedAt)
	if err != nil || d.SentAt == nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE job_matches SET delivered = true, delivered_at = $3 WHERE user_id = $1 AND id = ANY($2)", d.UserID, d.MatchIDs, d.SentAt)
	return err
}
