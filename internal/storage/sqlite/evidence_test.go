package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alessio/tutorio/internal/evidence"
)

func TestEvidenceRepositoryResolvesChunkAndNeighbours(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewEvidenceRepository(db)
	now := time.Now().UTC()
	source := evidence.Source{ID: "src_one", Kind: "pdf", Locator: "/private/book.pdf", Title: "Book", Fingerprint: "fingerprint", CreatedAt: now}
	if err = repository.SaveSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	chunks := []evidence.SourceChunk{
		{ID: "chunk_a", SourceID: source.ID, Kind: evidence.SourceChunkText, Text: "Before", Location: evidence.SourceLocation{PhysicalPage: 4}, Sequence: 0, CreatedAt: now},
		{ID: "chunk_b", SourceID: source.ID, Kind: evidence.SourceChunkText, Text: "Selected exact source text", Location: evidence.SourceLocation{PhysicalPage: 5}, Sequence: 1, CreatedAt: now},
		{ID: "chunk_c", SourceID: source.ID, Kind: evidence.SourceChunkText, Text: "After", Location: evidence.SourceLocation{PhysicalPage: 6}, Sequence: 2, CreatedAt: now},
	}
	if err = repository.SaveChunks(ctx, chunks); err != nil {
		t.Fatal(err)
	}
	resolved, err := repository.GetEvidence(ctx, evidence.EvidenceIDForChunk("chunk_b"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Chunk.Text != "Selected exact source text" || resolved.Chunk.Location.PhysicalPage != 5 || resolved.Previous == nil || resolved.Previous.Text != "Before" || resolved.Next == nil || resolved.Next.Text != "After" {
		t.Fatalf("unexpected resolved evidence: %#v", resolved)
	}
}

func TestEvidenceMigrationIsRecorded(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration record: count=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=2`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("job metadata migration record: count=%d err=%v", count, err)
	}
}

func TestEvidenceRepositoryReportsMissingEvidence(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "missing-evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = NewEvidenceRepository(db).GetEvidence(ctx, "not-present"); err == nil {
		t.Fatal("expected missing evidence to return an error")
	}
}
