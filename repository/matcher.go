package repository

import (
	"context"

	"github.com/hunterjob/hunterjob/api/entity"
)

func (s Store) ActivePreferences(ctx context.Context) ([]entity.Preference, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+preferenceColumns+" FROM search_preferences WHERE enabled")
	return collect(rows, err, scanPreference)
}

func (s Store) SaveJobMatch(ctx context.Context, match entity.Match) (bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `
		INSERT INTO job_matches (user_id, job_id, dedupe_key, preference_id, match_level,
			matched_keywords, match_reason, status, matched_at, created_at, job)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT DO NOTHING`, match.UserID, match.JobID, match.DedupeKey, match.PreferenceID,
		match.MatchLevel, match.MatchedKeywords, match.MatchReason, match.Status, match.MatchedAt,
		match.CreatedAt, match.Job)
	if err != nil {
		return false, err
	}
	inserted := result.RowsAffected() == 1
	if !inserted {
		// Job ID takes precedence if the event matches two existing aliases.
		_, err = tx.Exec(ctx, `UPDATE job_matches SET job = $4 WHERE id = (
			SELECT id FROM job_matches WHERE user_id = $1 AND (job_id = $2 OR dedupe_key = $3)
			ORDER BY (job_id = $2) DESC LIMIT 1
		)`, match.UserID, match.JobID, match.DedupeKey, match.Job)
		if err != nil {
			return false, err
		}
	}
	return inserted, tx.Commit(ctx)
}
