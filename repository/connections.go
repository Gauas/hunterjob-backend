package repository

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/hunterjob/hunterjob/api/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s Store) CreateLink(ctx context.Context, provider, userID, token string) error {
	hash := sha256.Sum256([]byte(token))
	_, err := s.DB.Exec(ctx, `INSERT INTO chat_link_tokens (provider, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, now() + interval '10 minutes')
		ON CONFLICT (provider, user_id) DO UPDATE SET token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at`,
		provider, userID, hash[:])
	return err
}

func (s Store) BindLink(ctx context.Context, provider, token, recipient string) (bool, error) {
	hash := sha256.Sum256([]byte(token))
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `DELETE FROM chat_link_tokens WHERE provider = $1 AND token_hash = $2
		AND expires_at > now() RETURNING user_id`, provider, hash[:]).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_connections (user_id, provider, status, enabled, external_user_id)
		VALUES ($1, $2, 'connected', true, $3)
		ON CONFLICT (user_id, provider) DO UPDATE SET status = 'connected', enabled = true,
			external_user_id = EXCLUDED.external_user_id, updated_at = now()`, userID, provider, recipient)
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.ConstraintName == "chat_connections_recipient" {
		return false, ErrRecipientInUse
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) Disconnect(ctx context.Context, userID, provider string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "DELETE FROM chat_link_tokens WHERE user_id = $1 AND provider = $2", userID, provider); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE chat_connections SET status = 'disconnected', enabled = false,
			external_user_id = '', updated_at = now() WHERE user_id = $1 AND provider = $2`, userID, provider)
		return err
	})
}

func (s Store) EnableConnection(ctx context.Context, userID, provider string, enabled bool) (bool, error) {
	tag, err := s.DB.Exec(ctx, `UPDATE chat_connections SET enabled = $3, updated_at = now()
		WHERE user_id = $1 AND provider = $2 AND status = 'connected'`, userID, provider, enabled)
	return tag.RowsAffected() > 0, err
}

func (s Store) RecipientMatches(ctx context.Context, provider, recipient string) ([]entity.Match, bool, error) {
	var userID string
	err := s.DB.QueryRow(ctx, `SELECT user_id FROM chat_connections WHERE provider = $1
		AND external_user_id = $2 AND status = 'connected'`, provider, recipient).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := s.DB.Query(ctx, "SELECT "+matchColumns+` FROM job_matches WHERE user_id = $1 AND status <> 'dismissed'
		ORDER BY matched_at DESC, id DESC LIMIT 3`, userID)
	matches, err := collect(rows, err, scanMatch)
	return matches, true, err
}
