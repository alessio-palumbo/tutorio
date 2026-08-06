package html

import (
	"context"
	"strings"
	"testing"

	"github.com/alessio/tutorio/internal/guide"
)

func TestRenderProducesStandaloneResponsiveGuide(t *testing.T) {
	content, err := New().Render(context.Background(), guide.Guide{
		Title: "Safe <Guide>", Overview: "A portable guide.", FinalOutcome: "Done",
		SourceURI: "https://youtube.com/watch?v=test",
		Steps: []guide.Step{
			{SourceSegment: 0, Title: "First", Explanation: "Explain", Actions: []string{"Act"}, Timestamps: []guide.Timestamp{{StartSeconds: 65, Label: "Start"}}},
			{SourceSegment: 1, Title: "Second", Explanation: "Continue"},
		},
		KeyboardShortcuts: []guide.Shortcut{{Keys: "⌘S", Action: "Save"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{"<!doctype html>", `name="viewport"`, "@media print", "Safe &lt;Guide&gt;", "t=65s", "Keyboard shortcuts", "⌘S"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in HTML export", expected)
		}
	}
	if strings.Count(text, `<div class="number">1</div>`) != 2 {
		t.Fatalf("step numbering did not reset per source section: %s", text)
	}
	if strings.Contains(text, "Safe <Guide>") {
		t.Fatal("guide title was not HTML escaped")
	}
}

func TestRenderHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Render(ctx, guide.Guide{Title: "Guide"}); err == nil {
		t.Fatal("expected cancelled render to fail")
	}
}
