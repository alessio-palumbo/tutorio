package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/transcript"
)

type JobStore struct{ db *sql.DB }

func NewJobStore(db *sql.DB) *JobStore { return &JobStore{db: db} }
func (s *JobStore) Create(ctx context.Context, j jobs.Job) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO jobs(id,source_type,source_uri,source_title,source_id,status,stage,current,total,error,guide_id,created_at,updated_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, j.ID, j.SourceType, j.SourceURI, j.SourceTitle, j.SourceID, j.Status, j.Stage, j.Current, j.Total, j.Error, j.GuideID, formatTime(j.CreatedAt), formatTime(j.UpdatedAt), nullableTime(j.CompletedAt))
	return err
}
func (s *JobStore) Update(ctx context.Context, j jobs.Job) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET source_title=?,source_id=?,status=?,stage=?,current=?,total=?,error=?,guide_id=?,updated_at=?,completed_at=? WHERE id=?`, j.SourceTitle, j.SourceID, j.Status, j.Stage, j.Current, j.Total, j.Error, j.GuideID, formatTime(j.UpdatedAt), nullableTime(j.CompletedAt), j.ID)
	return err
}
func (s *JobStore) SaveSegments(ctx context.Context, jobID string, segments []transcript.Segment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, segment := range segments {
		raw, err := json.Marshal(segment)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO job_segments(job_id,segment_index,transcript_json,status) VALUES(?,?,?,?)`, jobID, segment.Index, raw, jobs.StatusPending); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *JobStore) CompleteSegment(ctx context.Context, value jobs.Segment) error {
	transcriptJSON, err := json.Marshal(value.Transcript)
	if err != nil {
		return err
	}
	guideJSON, err := json.Marshal(value.Guide)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO job_segments(job_id,segment_index,transcript_json,guide_json,status,model,prompt_tokens,output_tokens,duration_ms,prompt_duration_ms,output_duration_ms,raw_response,error) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(job_id,segment_index) DO UPDATE SET transcript_json=excluded.transcript_json,guide_json=excluded.guide_json,status=excluded.status,model=excluded.model,prompt_tokens=excluded.prompt_tokens,output_tokens=excluded.output_tokens,duration_ms=excluded.duration_ms,prompt_duration_ms=excluded.prompt_duration_ms,output_duration_ms=excluded.output_duration_ms,raw_response=excluded.raw_response,error=excluded.error`, value.JobID, value.Index, transcriptJSON, guideJSON, value.Status, value.Model, value.PromptTokens, value.OutputTokens, value.DurationMilliseconds, value.PromptDurationMilliseconds, value.OutputDurationMilliseconds, value.RawResponse, value.Error)
	return err
}
func (s *JobStore) RecordSegmentFailure(ctx context.Context, value jobs.Segment) error {
	transcriptJSON, err := json.Marshal(value.Transcript)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO job_segments(job_id,segment_index,transcript_json,status,model,prompt_tokens,output_tokens,duration_ms,prompt_duration_ms,output_duration_ms,raw_response,error) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(job_id,segment_index) DO UPDATE SET status=excluded.status,model=excluded.model,prompt_tokens=excluded.prompt_tokens,output_tokens=excluded.output_tokens,duration_ms=excluded.duration_ms,prompt_duration_ms=excluded.prompt_duration_ms,output_duration_ms=excluded.output_duration_ms,raw_response=excluded.raw_response,error=excluded.error`, value.JobID, value.Index, transcriptJSON, value.Status, value.Model, value.PromptTokens, value.OutputTokens, value.DurationMilliseconds, value.PromptDurationMilliseconds, value.OutputDurationMilliseconds, value.RawResponse, value.Error)
	return err
}
func (s *JobStore) Get(ctx context.Context, id string) (jobs.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,source_type,source_uri,source_title,source_id,status,stage,current,total,error,guide_id,created_at,updated_at,completed_at FROM jobs WHERE id=?`, id)
	return scanJob(row)
}
func (s *JobStore) List(ctx context.Context, limit int) ([]jobs.Job, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_type,source_uri,source_title,source_id,status,stage,current,total,error,guide_id,created_at,updated_at,completed_at FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []jobs.Job{}
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *JobStore) Segments(ctx context.Context, jobID string) ([]jobs.Segment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT segment_index,transcript_json,guide_json,status,model,prompt_tokens,output_tokens,duration_ms,prompt_duration_ms,output_duration_ms,raw_response,error FROM job_segments WHERE job_id=? ORDER BY segment_index`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []jobs.Segment{}
	for rows.Next() {
		var item jobs.Segment
		var transcriptJSON []byte
		var guideJSON []byte
		if err := rows.Scan(&item.Index, &transcriptJSON, &guideJSON, &item.Status, &item.Model, &item.PromptTokens, &item.OutputTokens, &item.DurationMilliseconds, &item.PromptDurationMilliseconds, &item.OutputDurationMilliseconds, &item.RawResponse, &item.Error); err != nil {
			return nil, err
		}
		item.JobID = jobID
		if err = json.Unmarshal(transcriptJSON, &item.Transcript); err != nil {
			return nil, err
		}
		if len(guideJSON) > 0 {
			if err = json.Unmarshal(guideJSON, &item.Guide); err != nil {
				return nil, err
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanJob(row rowScanner) (jobs.Job, error) {
	var j jobs.Job
	var created, updated string
	var completed sql.NullString
	if err := row.Scan(&j.ID, &j.SourceType, &j.SourceURI, &j.SourceTitle, &j.SourceID, &j.Status, &j.Stage, &j.Current, &j.Total, &j.Error, &j.GuideID, &created, &updated, &completed); err != nil {
		return jobs.Job{}, err
	}
	var err error
	j.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return jobs.Job{}, err
	}
	j.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return jobs.Job{}, err
	}
	if completed.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, completed.String)
		if parseErr != nil {
			return jobs.Job{}, parseErr
		}
		j.CompletedAt = &value
	}
	return j, nil
}
func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
