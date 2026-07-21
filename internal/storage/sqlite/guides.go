package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alessio/tutorio/internal/guide"
)

type GuideRepository struct{ db *sql.DB }

func NewGuideRepository(db *sql.DB) *GuideRepository { return &GuideRepository{db: db} }
func (r *GuideRepository) Save(ctx context.Context, g guide.Guide) (guide.Guide, error) {
	now := time.Now().UTC()
	if g.ID == "" {
		g.ID = fmt.Sprintf("guide_%d", now.UnixNano())
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	content, err := json.Marshal(g)
	if err != nil {
		return guide.Guide{}, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO guides(id,source_type,source_uri,source_id,title,overview,content_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,overview=excluded.overview,content_json=excluded.content_json,updated_at=excluded.updated_at`, g.ID, g.SourceType, g.SourceURI, g.SourceID, g.Title, g.Overview, content, g.CreatedAt.Format(time.RFC3339Nano), g.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return guide.Guide{}, err
	}
	return g, nil
}
func (r *GuideRepository) Get(ctx context.Context, id string) (guide.Guide, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT content_json FROM guides WHERE id=?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return guide.Guide{}, fmt.Errorf("guide %q not found", id)
	}
	if err != nil {
		return guide.Guide{}, err
	}
	var value guide.Guide
	err = json.Unmarshal(raw, &value)
	return value, err
}
func (r *GuideRepository) List(ctx context.Context, limit int) ([]guide.Summary, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,source_type,title,overview,created_at FROM guides ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]guide.Summary, 0)
	for rows.Next() {
		var item guide.Summary
		var created string
		if err := rows.Scan(&item.ID, &item.SourceType, &item.Title, &item.Overview, &created); err != nil {
			return nil, err
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// Delete removes a guide and the persisted compilation job that produced it.
// job_segments are removed by their ON DELETE CASCADE constraint.
func (r *GuideRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT content_json FROM guides WHERE id=?`, id).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("guide %q not found", id)
	} else if err != nil {
		return err
	}
	var value guide.Guide
	if err = json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value.Generation.JobID != "" {
		if _, err = tx.ExecContext(ctx, `DELETE FROM jobs WHERE id=?`, value.Generation.JobID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM guides WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
