package repository

import (
	"context"
	"errors"

	"github.com/hunterjob/hunterjob/api/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

var ErrRecipientInUse = errors.New("chat recipient is already connected")

const preferenceColumns = "id, user_id, role, experience, locations, keywords, excluded_keywords, frequency, enabled, created_at, updated_at"
const matchColumns = "id, user_id, job_id, coalesce(dedupe_key, ''), preference_id, match_level, matched_keywords, match_reason, status, matched_at, delivered, delivered_at, created_at, job"
const connectionColumns = "id, user_id, provider, status, enabled, external_user_id, created_at, updated_at"

func scanPreference(row pgx.CollectableRow) (entity.Preference, error) {
	var p entity.Preference
	err := row.Scan(&p.ID, &p.UserID, &p.Role, &p.Experience, &p.Locations, &p.Keywords, &p.ExcludedKeywords, &p.Frequency, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func scanMatch(row pgx.CollectableRow) (entity.Match, error) {
	var m entity.Match
	err := row.Scan(&m.ID, &m.UserID, &m.JobID, &m.DedupeKey, &m.PreferenceID, &m.MatchLevel, &m.MatchedKeywords, &m.MatchReason, &m.Status, &m.MatchedAt, &m.Delivered, &m.DeliveredAt, &m.CreatedAt, &m.Job)
	return m, err
}

func scanConnection(row pgx.CollectableRow) (entity.Connection, error) {
	var c entity.Connection
	err := row.Scan(&c.ID, &c.UserID, &c.Provider, &c.Status, &c.Enabled, &c.ExternalUserID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func collect[T any](rows pgx.Rows, err error, scan pgx.RowToFunc[T]) ([]T, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func one[T any](rows pgx.Rows, err error, scan pgx.RowToFunc[T]) (*T, error) {
	if err != nil {
		return nil, err
	}
	item, err := pgx.CollectOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s Store) Preference(ctx context.Context, userID string) (*entity.Preference, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+preferenceColumns+" FROM search_preferences WHERE user_id = $1", userID)
	return one(rows, err, scanPreference)
}

func (s Store) SavePreference(ctx context.Context, p entity.Preference) (*entity.Preference, error) {
	rows, err := s.DB.Query(ctx, `
		INSERT INTO search_preferences (user_id, role, experience, locations, keywords, excluded_keywords, frequency, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
		ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role, experience = EXCLUDED.experience,
			locations = EXCLUDED.locations, keywords = EXCLUDED.keywords, excluded_keywords = EXCLUDED.excluded_keywords,
			frequency = EXCLUDED.frequency, enabled = true, updated_at = now()
		RETURNING `+preferenceColumns,
		p.UserID, p.Role, p.Experience, p.Locations, p.Keywords, p.ExcludedKeywords, p.Frequency)
	return one(rows, err, scanPreference)
}

func (s Store) Matches(ctx context.Context, userID, status, level string, before *entity.Match, limit int64) ([]entity.Match, error) {
	var beforeTime, beforeID any
	if before != nil {
		beforeTime, beforeID = before.MatchedAt, before.ID
	}
	rows, err := s.DB.Query(ctx, `SELECT `+matchColumns+` FROM job_matches
		WHERE user_id = $1 AND ($2 = '' OR status = $2) AND ($3 = '' OR match_level = $3)
		  AND ($4::timestamptz IS NULL OR (matched_at, id) < ($4, $5))
		ORDER BY matched_at DESC, id DESC LIMIT $6`, userID, status, level, beforeTime, beforeID, limit)
	items, err := collect(rows, err, scanMatch)
	if items == nil && err == nil {
		items = []entity.Match{}
	}
	return items, err
}

func (s Store) Match(ctx context.Context, userID, id string) (*entity.Match, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+matchColumns+" FROM job_matches WHERE user_id = $1 AND id = $2", userID, id)
	return one(rows, err, scanMatch)
}

func (s Store) SetMatchStatus(ctx context.Context, userID, id, status string) (*entity.Match, error) {
	rows, err := s.DB.Query(ctx, "UPDATE job_matches SET status = $3 WHERE user_id = $1 AND id = $2 RETURNING "+matchColumns, userID, id, status)
	return one(rows, err, scanMatch)
}

func (s Store) Connections(ctx context.Context, userID string) ([]entity.Connection, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+connectionColumns+" FROM chat_connections WHERE user_id = $1", userID)
	return collect(rows, err, scanConnection)
}

func (s Store) Connection(ctx context.Context, userID, provider string) (*entity.Connection, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+connectionColumns+" FROM chat_connections WHERE user_id = $1 AND provider = $2", userID, provider)
	return one(rows, err, scanConnection)
}

func (s Store) SaveConnection(ctx context.Context, item entity.Connection) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO chat_connections (user_id, provider, status, enabled, external_user_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, provider) DO UPDATE SET status = EXCLUDED.status,
			enabled = EXCLUDED.enabled, external_user_id = EXCLUDED.external_user_id, updated_at = now()`,
		item.UserID, item.Provider, item.Status, item.Enabled, item.ExternalUserID)
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.ConstraintName == "chat_connections_recipient" {
		return ErrRecipientInUse
	}
	return err
}
