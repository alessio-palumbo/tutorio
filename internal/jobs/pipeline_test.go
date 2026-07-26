package jobs

import (
	"context"
	"errors"
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

type fakeOverviewSynthesizer struct {
	result guide.OverviewResult
	err    error
}

type recordingProgress struct {
	values []Progress
}

func (r *recordingProgress) Report(_ context.Context, value Progress) {
	r.values = append(r.values, value)
}

func (f fakeOverviewSynthesizer) SynthesizeOverview(context.Context, guide.OverviewRequest) (guide.OverviewResult, error) {
	return f.result, f.err
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
	if got.SourceMetrics.Words != 3 || got.SourceMetrics.Characters != 13 {
		t.Fatalf("unexpected source metrics: %#v", got.SourceMetrics)
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

func TestSectionRegenerationProgressCarriesOperationIdentity(t *testing.T) {
	reporter := &recordingProgress{}
	pipeline := &Pipeline{progress: reporter}
	pipeline.reportSectionRegeneration(context.Background(), "guide-1", 0, "generating", "Generating replacement…", 0, 0)

	if len(reporter.values) != 1 {
		t.Fatalf("got %d progress events", len(reporter.values))
	}
	got := reporter.values[0]
	if got.Operation != "section_regeneration" || got.GuideID != "guide-1" || got.SectionIndex != 0 || got.Stage != "generating" {
		t.Fatalf("unexpected progress identity: %#v", got)
	}
}

func TestRecoveryPreservesSourceIdentityAndTitle(t *testing.T) {
	sections := []Segment{
		{Status: StatusCompleted, Guide: guide.Guide{Title: "Attention Is All You Need", SourceID: "source-1"}},
	}
	job := Job{SourceTitle: "Attention Is All You Need", SourceID: "source-1"}
	if got := recoveredTitle(job, sections); got != "Attention Is All You Need" {
		t.Fatalf("recovered title = %q", got)
	}
	if got := recoveredSourceID(job, sections); got != "source-1" {
		t.Fatalf("recovered source ID = %q", got)
	}
}

func TestLegacyRecoveryUsesCompletedSectionMetadata(t *testing.T) {
	sections := []Segment{
		{Status: StatusCompleted, Guide: guide.Guide{Title: "Original title", SourceID: "legacy-source"}},
	}
	if got := recoveredTitle(Job{}, sections); got != "Original title" {
		t.Fatalf("legacy recovered title = %q", got)
	}
	if got := recoveredSourceID(Job{}, sections); got != "legacy-source" {
		t.Fatalf("legacy recovered source ID = %q", got)
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

func TestOverviewSynthesisUpdatesStateWithoutSourceText(t *testing.T) {
	pipeline := &Pipeline{
		overview: fakeOverviewSynthesizer{result: guide.OverviewResult{Text: "A concise guide overview.", Model: "local-model"}},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	value := guide.Guide{Title: "Lesson", Overview: "Section one. Section two.", FinalOutcome: "Done"}
	sections := []Segment{
		{Status: StatusCompleted, Guide: guide.Guide{Title: "One", Overview: "First section summary."}},
		{Status: StatusCompleted, Guide: guide.Guide{Title: "Two", Overview: "Second section summary."}},
	}
	if err := pipeline.synthesizeOverview(context.Background(), &value, sections); err != nil {
		t.Fatal(err)
	}
	if value.Overview != "A concise guide overview." || value.OverviewGeneration.Status != guide.OverviewReady || value.OverviewGeneration.Model != "local-model" {
		t.Fatalf("unexpected overview state: %#v", value.OverviewGeneration)
	}
}

func TestOverviewSynthesisFailurePreservesMergedGuide(t *testing.T) {
	pipeline := &Pipeline{
		overview: fakeOverviewSynthesizer{err: errors.New("model unavailable")},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	value := guide.Guide{Title: "Lesson", Overview: "Existing merged summaries.", FinalOutcome: "Done"}
	err := pipeline.synthesizeOverview(context.Background(), &value, []Segment{{Status: StatusCompleted, Guide: guide.Guide{Title: "One", Overview: "Summary"}}})
	if err == nil {
		t.Fatal("expected synthesis failure")
	}
	if value.Overview != "Existing merged summaries." || value.OverviewGeneration.Status != guide.OverviewFailed {
		t.Fatalf("failed synthesis damaged guide: %#v", value)
	}
}
