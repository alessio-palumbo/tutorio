package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/transcript"
)

func TestGuideRepositoryRoundTrip(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "tutorio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewGuideRepository(db)
	saved, err := repository.Save(context.Background(), guide.Guide{SourceType: "test", SourceURI: "fixture", SourceID: "one", Title: "A lesson", Overview: "Overview", FinalOutcome: "Done", Steps: []guide.Step{{Number: 1, Title: "Start", Explanation: "Begin"}}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("expected generated ID")
	}
	loaded, err := repository.Get(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != saved.Title {
		t.Fatalf("got title %q", loaded.Title)
	}
	items, err := repository.List(context.Background(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("got %#v, %v", items, err)
	}
}

func TestJobStorePersistsSegmentsAndMetrics(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewJobStore(db)
	now := time.Now().UTC()
	job := jobs.Job{ID: "job_1", SourceType: "youtube", SourceURI: "https://example.test", Status: jobs.StatusRunning, Stage: "source", CreatedAt: now, UpdatedAt: now}
	if err = store.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	segments := []transcript.Segment{{Index: 0, Text: "hello"}}
	if err = store.SaveSegments(context.Background(), job.ID, segments); err != nil {
		t.Fatal(err)
	}
	if err = store.CompleteSegment(context.Background(), jobs.Segment{JobID: job.ID, Index: 0, Transcript: segments[0], Guide: guide.Guide{Title: "Part"}, Status: jobs.StatusCompleted, Model: "test", PromptTokens: 10, OutputTokens: 20, DurationMilliseconds: 30}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.Segments(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].OutputTokens != 20 || stored[0].Guide.Title != "Part" {
		t.Fatalf("unexpected segments: %#v", stored)
	}
}
