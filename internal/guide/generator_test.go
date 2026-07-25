package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alessio/tutorio/internal/evidence"
	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/transcript"
)

type sequenceProvider struct{ calls int }

func (p *sequenceProvider) Complete(context.Context, llm.Request) (llm.Response, error) {
	p.calls++
	return llm.Response{Content: fmt.Sprintf(`{"title":"Lesson","overview":"Part %d","final_outcome":"Done","steps":[{"number":1,"title":"Step %d","explanation":"Explain","actions":[],"commands":[],"warnings":[],"timestamps":[]}],"prerequisites":[],"important_concepts":[],"commands":[],"keyboard_shortcuts":[],"warnings":[],"common_mistakes":[],"cheat_sheet":[],"appendix":[],"source_timestamps":[]}`, p.calls, p.calls)}, nil
}

func TestGeneratorCompilesAndMergesSegmentsIndependently(t *testing.T) {
	provider := &sequenceProvider{}
	progress := 0
	got, err := NewLLMGenerator(provider).Generate(context.Background(), GenerateRequest{Title: "Lesson", Segments: []transcript.Segment{{Index: 0, Text: "one"}, {Index: 1, Text: "two"}}, OnProgress: func(current, total int) { progress = current }})
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatalf("got %d provider calls", provider.calls)
	}
	if len(got.Steps) != 2 || got.Steps[1].Number != 2 {
		t.Fatalf("unexpected merged steps: %#v", got.Steps)
	}
	if got.Steps[0].ID != "step_0_1" || got.Steps[1].ID != "step_1_1" || got.Steps[1].SourceSegment != 1 {
		t.Fatalf("unexpected step provenance: %#v", got.Steps)
	}
	if progress != 2 {
		t.Fatalf("got progress %d", progress)
	}
}

func TestNormalizeGuideJSONWrapsSingleObjects(t *testing.T) {
	raw := `{"title":"Lesson","steps":{"number":1,"title":"Start","explanation":"Go","timestamps":12},"prerequisites":[{"title":"Install Go","explanation":"Version 1.25 or newer"}],"warnings":[{"warning":"Back up first","details":"The operation is destructive"}],"common_mistakes":[{"mistake":"Skipping tests","correction":"Run the suite"}],"appendix":[{}, {"title":"Reference","content":"Details"}],"cheat_sheet":[{}, {"Save":"Cmd+S","Tools":["B: Blade","A: Select"]}],"source_timestamps":[30,"00:01:00",{"start_seconds":"00:02:00","end_seconds":"not supplied","label":"Fallback end"}],"commands":["go test ./..."],"keyboard_shortcuts":["Cmd+S"]}`
	normalized, err := normalizeGuideJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got Guide
	if err := json.Unmarshal(normalized, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 1 || len(got.Steps[0].Timestamps) != 1 {
		t.Fatalf("unexpected guide: %#v", got)
	}
	if len(got.Appendix) != 1 {
		t.Fatalf("unexpected appendix: %#v", got.Appendix)
	}
	if len(got.CheatSheet) != 3 {
		t.Fatalf("unexpected cheat sheet values: %#v", got.CheatSheet)
	}
	if got.Prerequisites[0] != "Install Go — Version 1.25 or newer" || got.Warnings[0] != "Back up first — The operation is destructive" || got.CommonMistakes[0] != "Skipping tests — Run the suite" {
		t.Fatalf("structured lists were not made readable: %#v %#v %#v", got.Prerequisites, got.Warnings, got.CommonMistakes)
	}
	if len(got.SourceTimestamps) != 3 || got.SourceTimestamps[1].StartSeconds != 60 || got.SourceTimestamps[2].StartSeconds != 120 || got.SourceTimestamps[2].EndSeconds != 120 {
		t.Fatalf("unexpected timestamps: %#v", got.SourceTimestamps)
	}
	if len(got.Commands) != 1 || got.Commands[0].Value != "go test ./..." {
		t.Fatalf("unexpected commands: %#v", got.Commands)
	}
}

func TestAnchorTimestampsAndPopulateCheatSheet(t *testing.T) {
	value := Guide{Steps: []Step{{Title: "One", Timestamps: []Timestamp{{StartSeconds: 0, EndSeconds: 10}}}}, KeyboardShortcuts: []Shortcut{{Keys: "B", Action: "Blade tool"}}, Commands: []Command{{Value: "Cmd+B", Description: "Split clip"}}}
	anchorGuideTimestamps(&value, transcript.Segment{Index: 1, Start: 2 * time.Minute, End: 4 * time.Minute})
	if value.Steps[0].Timestamps[0].StartSeconds != 120 {
		t.Fatalf("unexpected timestamp: %#v", value.Steps[0].Timestamps)
	}
	merged := mergeGuides("Lesson", []Guide{value})
	if len(merged.CheatSheet) != 2 {
		t.Fatalf("unexpected cheat sheet: %#v", merged.CheatSheet)
	}
}

func TestAnchorPageReferencesWithoutInventingStepEvidence(t *testing.T) {
	value := Guide{Steps: []Step{{Title: "One"}, {Title: "Two"}}}
	anchorGuideSource(&value, transcript.Segment{Index: 1, Reference: transcript.Reference{Kind: "page", PageStart: 10, PageEnd: 12}})
	if len(value.Steps[0].Timestamps) != 0 || len(value.Steps[0].References) != 0 || len(value.Steps[1].References) != 0 {
		t.Fatalf("unexpected page references: %#v", value.Steps)
	}
	if len(value.SourceReferences) != 1 || value.SourceReferences[0].PageEnd != 12 {
		t.Fatalf("unexpected source references: %#v", value.SourceReferences)
	}
}

type evidenceProvider struct{}

func (evidenceProvider) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Content: `{"title":"Lesson","overview":"Evidence","final_outcome":"Done","steps":[{"number":1,"title":"Supported","explanation":"Explain","actions":[],"commands":[],"warnings":[],"timestamps":[],"source_excerpt":"model text must not be trusted","evidence_chunk_ids":["chunk-a","chunk-a","unknown","chunk-b","chunk-c","chunk-d","chunk-e","chunk-f"]},{"number":2,"title":"Uncited","explanation":"Explain","actions":[],"commands":[],"warnings":[],"timestamps":[],"evidence_chunk_ids":["unknown"]}],"prerequisites":[],"important_concepts":[],"commands":[],"keyboard_shortcuts":[],"warnings":[],"common_mistakes":[],"cheat_sheet":[],"appendix":[],"source_timestamps":[]}`}, nil
}

func TestGeneratorResolvesOnlyAllowedExactEvidence(t *testing.T) {
	chunks := make([]transcript.SourceChunk, 0, 6)
	for index, id := range []string{"chunk-a", "chunk-b", "chunk-c", "chunk-d", "chunk-e", "chunk-f"} {
		chunks = append(chunks, transcript.SourceChunk{ID: id, Kind: "text", Text: "Exact extracted text " + id, Sequence: index, Reference: transcript.Reference{Kind: "page", PageStart: 17 + index, PageEnd: 17 + index, Label: fmt.Sprintf("PDF page %d", 17+index)}})
	}
	got, err := NewLLMGenerator(evidenceProvider{}).Generate(context.Background(), GenerateRequest{Title: "Lesson", Segments: []transcript.Segment{{Index: 0, Text: "source", Reference: transcript.Reference{Kind: "page", PageStart: 17, PageEnd: 22}, Chunks: chunks}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps[0].Citations) != maxCitationsPerStep {
		t.Fatalf("expected capped citations, got %#v", got.Steps[0].Citations)
	}
	if got.Steps[0].Citations[0].EvidenceID != evidence.EvidenceIDForChunk("chunk-a") || got.Steps[0].Citations[0].Support != SupportDirect {
		t.Fatalf("unexpected citation: %#v", got.Steps[0].Citations[0])
	}
	if got.Steps[0].SourceExcerpt != "Exact extracted text chunk-a" || got.Steps[0].References[0].PageStart != 17 {
		t.Fatalf("source text or page was not preserved: %#v", got.Steps[0])
	}
	if len(got.Steps[1].Citations) != 0 || got.Steps[1].SourceExcerpt != "" || len(got.Steps[1].References) != 0 {
		t.Fatalf("unsupported step received invented evidence: %#v", got.Steps[1])
	}
}

func TestValidateSourceExcerptsRejectsUnsupportedText(t *testing.T) {
	value := Guide{Steps: []Step{{SourceExcerpt: "exact transcript words"}, {SourceExcerpt: "invented claim"}}}
	validateSourceExcerpts(&value, "These are exact   transcript words from the source.")
	if value.Steps[0].SourceExcerpt != "exact transcript words" {
		t.Fatalf("lost valid excerpt: %#v", value.Steps)
	}
	if value.Steps[1].SourceExcerpt != "" {
		t.Fatalf("kept unsupported excerpt: %#v", value.Steps)
	}
}

func TestAppendUniqueSimilarCollapsesRepeatedPrerequisites(t *testing.T) {
	got := appendUniqueSimilar(nil,
		"Basic understanding of neural networks.",
		"basic understanding of neural networks",
		"Basic understanding of neural networks and attention mechanisms",
		"Python installed",
	)
	if len(got) != 2 || got[1] != "Python installed" {
		t.Fatalf("unexpected prerequisites: %#v", got)
	}
}

type malformedProvider struct{}

func (malformedProvider) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Model: "test-model", Content: `{"steps":`}, nil
}

func TestGeneratorReportsRawResponseWhenSectionCannotDecode(t *testing.T) {
	var failure SectionResult
	_, err := NewLLMGenerator(malformedProvider{}).Generate(context.Background(), GenerateRequest{Title: "Lesson", Segments: []transcript.Segment{{Index: 3, Text: "source"}}, OnFailure: func(result SectionResult) { failure = result }})
	if err == nil {
		t.Fatal("expected malformed response to fail")
	}
	if failure.Index != 3 || failure.Model != "test-model" || failure.RawResponse != `{"steps":` || failure.Error == "" {
		t.Fatalf("missing failure diagnostics: %#v", failure)
	}
}
