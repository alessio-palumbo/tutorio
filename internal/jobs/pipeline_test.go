package jobs

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/transcript"
)

type fakeSource struct{}

func (fakeSource) Type() string { return "test" }
func (fakeSource) Fetch(context.Context, source.Request) (transcript.Document, error) {
	return transcript.Document{SourceID: "source-1", Title: "Lesson", Cues: []transcript.Cue{{Text: "Do the thing."}}}, nil
}

type fakeGenerator struct{}

func (fakeGenerator) Generate(_ context.Context, r guide.GenerateRequest) (guide.Guide, error) {
	return guide.Guide{Title: r.Title, Overview: "A guide", FinalOutcome: "Done", Steps: []guide.Step{{Number: 1, Title: "Act", Explanation: "Do it"}}}, nil
}

type memoryRepository struct{ saved guide.Guide }

func (m *memoryRepository) Save(_ context.Context, g guide.Guide) (guide.Guide, error) {
	m.saved = g
	return g, nil
}
func (*memoryRepository) Get(context.Context, string) (guide.Guide, error)   { return guide.Guide{}, nil }
func (*memoryRepository) List(context.Context, int) ([]guide.Summary, error) { return nil, nil }
func TestPipelineRunsStages(t *testing.T) {
	repo := &memoryRepository{}
	pipeline := NewPipeline(source.NewRegistry(fakeSource{}), transcript.NewCleaner(), transcript.NewSegmenter(100), fakeGenerator{}, guide.NewStructuralVerifier(), repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := pipeline.Run(context.Background(), source.Request{Type: "test", URI: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Lesson" || repo.saved.SourceID != "source-1" {
		t.Fatalf("unexpected guide: %#v", got)
	}
}
