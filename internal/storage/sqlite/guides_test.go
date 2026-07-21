package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alessio/tutorio/internal/guide"
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
