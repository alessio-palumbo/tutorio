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
func (*memoryRepository) Delete(context.Context, string) error               { return nil }
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

func TestActiveSectionAdvancesBeyondCompletedCount(t *testing.T) {
	if got := activeSection(9, 12); got != 10 {
		t.Fatalf("active section = %d, want 10", got)
	}
	if got := activeSection(12, 12); got != 12 {
		t.Fatalf("completed job section = %d, want 12", got)
	}
}

func TestMarkEditedSteps(t *testing.T) {
	previous := []guide.Step{{ID: "step_0_1", Title: "Original", Actions: []string{"one"}}}
	updated := []guide.Step{{ID: "step_0_1", Title: "Revised", Actions: []string{"one"}}}
	markEditedSteps(previous, updated)
	if !updated[0].UserEdited {
		t.Fatal("changed step was not marked as user edited")
	}
}

func TestSectionSafeStepsPreservesEditsOutsideRegeneratedSection(t *testing.T) {
	saved := []guide.Step{
		{ID: "step_0_1", SourceSegment: 0, Title: "Manual title", UserEdited: true},
		{ID: "step_1_1", SourceSegment: 1, Title: "Old generated title"},
	}
	sections := []Segment{
		{Index: 0, Guide: guide.Guide{Steps: []guide.Step{{ID: "step_0_1", SourceSegment: 0, Title: "Original model title"}}}},
		{Index: 1, Guide: guide.Guide{Steps: []guide.Step{{ID: "step_1_1", SourceSegment: 1, Title: "New generated title"}}}},
	}
	got := sectionSafeSteps(saved, sections, 1)
	if got[0].Title != "Manual title" || !got[0].UserEdited {
		t.Fatalf("manual edit was lost: %#v", got)
	}
	if got[1].Title != "New generated title" || got[1].Number != 2 {
		t.Fatalf("regenerated section was not replaced: %#v", got)
	}
}
