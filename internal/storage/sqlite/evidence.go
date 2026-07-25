package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alessio/tutorio/internal/evidence"
)

type EvidenceRepository struct{ db *sql.DB }

func NewEvidenceRepository(db *sql.DB) *EvidenceRepository { return &EvidenceRepository{db: db} }

func (r *EvidenceRepository) SaveSource(ctx context.Context, value evidence.Source) error {
	metadata, err := json.Marshal(value.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO sources(id,kind,locator,title,fingerprint,metadata_json,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET locator=excluded.locator,title=excluded.title,metadata_json=excluded.metadata_json`, value.ID, value.Kind, value.Locator, value.Title, value.Fingerprint, metadata, formatTime(value.CreatedAt))
	return err
}

func (r *EvidenceRepository) SaveChunks(ctx context.Context, values []evidence.SourceChunk) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, value := range values {
		metadata, marshalErr := json.Marshal(value.Metadata)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO source_chunks(id,source_id,kind,text,physical_page,printed_label,sequence,metadata_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, value.ID, value.SourceID, value.Kind, value.Text, value.Location.PhysicalPage, value.Location.PrintedLabel, value.Sequence, metadata, formatTime(value.CreatedAt)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO evidence(id,source_id,kind,chunk_id,created_at) VALUES(?,?,?,?,?)`, evidence.EvidenceIDForChunk(value.ID), value.SourceID, evidence.EvidenceText, value.ID, formatTime(value.CreatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *EvidenceRepository) GetSource(ctx context.Context, id string) (evidence.Source, error) {
	var value evidence.Source
	var metadata []byte
	var created string
	err := r.db.QueryRowContext(ctx, `SELECT id,kind,locator,title,fingerprint,metadata_json,created_at FROM sources WHERE id=?`, id).Scan(&value.ID, &value.Kind, &value.Locator, &value.Title, &value.Fingerprint, &metadata, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return evidence.Source{}, fmt.Errorf("source %q not found", id)
	}
	if err != nil {
		return evidence.Source{}, err
	}
	value.CreatedAt, err = parseTime(created)
	if err == nil {
		err = json.Unmarshal(metadata, &value.Metadata)
	}
	return value, err
}

func (r *EvidenceRepository) GetEvidence(ctx context.Context, id string) (evidence.Evidence, error) {
	var value evidence.Evidence
	var sourceMetadata, chunkMetadata []byte
	var sourceCreated, chunkCreated, evidenceCreated string
	err := r.db.QueryRowContext(ctx, `SELECT e.id,e.source_id,e.kind,e.chunk_id,e.created_at,s.kind,s.locator,s.title,s.fingerprint,s.metadata_json,s.created_at,c.kind,c.text,c.physical_page,c.printed_label,c.sequence,c.metadata_json,c.created_at FROM evidence e JOIN sources s ON s.id=e.source_id JOIN source_chunks c ON c.id=e.chunk_id WHERE e.id=?`, id).Scan(&value.ID, &value.SourceID, &value.Kind, &value.ChunkID, &evidenceCreated, &value.Source.Kind, &value.Source.Locator, &value.Source.Title, &value.Source.Fingerprint, &sourceMetadata, &sourceCreated, &value.Chunk.Kind, &value.Chunk.Text, &value.Chunk.Location.PhysicalPage, &value.Chunk.Location.PrintedLabel, &value.Chunk.Sequence, &chunkMetadata, &chunkCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return evidence.Evidence{}, fmt.Errorf("evidence %q not found", id)
	}
	if err != nil {
		return evidence.Evidence{}, err
	}
	value.Source.ID, value.Chunk.ID, value.Chunk.SourceID = value.SourceID, value.ChunkID, value.SourceID
	if value.CreatedAt, err = parseTime(evidenceCreated); err != nil {
		return evidence.Evidence{}, err
	}
	if value.Source.CreatedAt, err = parseTime(sourceCreated); err != nil {
		return evidence.Evidence{}, err
	}
	if value.Chunk.CreatedAt, err = parseTime(chunkCreated); err != nil {
		return evidence.Evidence{}, err
	}
	if err = json.Unmarshal(sourceMetadata, &value.Source.Metadata); err != nil {
		return evidence.Evidence{}, err
	}
	if err = json.Unmarshal(chunkMetadata, &value.Chunk.Metadata); err != nil {
		return evidence.Evidence{}, err
	}
	value.Previous, err = r.neighbour(ctx, value.SourceID, value.Chunk.Sequence, "<", "DESC")
	if err != nil {
		return evidence.Evidence{}, err
	}
	value.Next, err = r.neighbour(ctx, value.SourceID, value.Chunk.Sequence, ">", "ASC")
	return value, err
}

func (r *EvidenceRepository) neighbour(ctx context.Context, sourceID string, sequence int, comparison, order string) (*evidence.SourceChunk, error) {
	query := fmt.Sprintf(`SELECT id,kind,text,physical_page,printed_label,sequence,metadata_json,created_at FROM source_chunks WHERE source_id=? AND sequence %s ? ORDER BY sequence %s LIMIT 1`, comparison, order)
	var value evidence.SourceChunk
	var metadata []byte
	var created string
	err := r.db.QueryRowContext(ctx, query, sourceID, sequence).Scan(&value.ID, &value.Kind, &value.Text, &value.Location.PhysicalPage, &value.Location.PrintedLabel, &value.Sequence, &metadata, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value.SourceID = sourceID
	value.CreatedAt, err = parseTime(created)
	if err == nil {
		err = json.Unmarshal(metadata, &value.Metadata)
	}
	return &value, err
}

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
