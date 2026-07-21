package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

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
	got, err := NewLLMGenerator(provider).Generate(context.Background(), GenerateRequest{Title: "Lesson", Segments: []transcript.Segment{{Text: "one"}, {Text: "two"}}, OnProgress: func(current, total int) { progress = current }})
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatalf("got %d provider calls", provider.calls)
	}
	if len(got.Steps) != 2 || got.Steps[1].Number != 2 {
		t.Fatalf("unexpected merged steps: %#v", got.Steps)
	}
	if progress != 2 {
		t.Fatalf("got progress %d", progress)
	}
}

func TestNormalizeGuideJSONWrapsSingleObjects(t *testing.T) {
	raw := `{"title":"Lesson","steps":{"number":1,"title":"Start","explanation":"Go","timestamps":12},"appendix":[{}, {"title":"Reference","content":"Details"}],"cheat_sheet":[{}],"source_timestamps":[30,"00:01:00"],"commands":["go test ./..."],"keyboard_shortcuts":["Cmd+S"]}`
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
	if len(got.CheatSheet) != 0 {
		t.Fatalf("unexpected cheat sheet artifacts: %#v", got.CheatSheet)
	}
	if len(got.SourceTimestamps) != 2 || got.SourceTimestamps[1].StartSeconds != 60 {
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
