package markdown

import (
	"context"
	"strings"
	"testing"

	"github.com/alessio/tutorio/internal/guide"
)

func TestRenderIncludesEvidenceAndSourceLink(t *testing.T) {
	content, err := New().Render(context.Background(), guide.Guide{
		Title: "Lesson", Overview: "Overview", FinalOutcome: "Done",
		SourceURI: "https://youtube.com/watch?v=test",
		Steps:     []guide.Step{{Number: 1, Title: "Start", Explanation: "Explain", SourceExcerpt: "spoken evidence", Timestamps: []guide.Timestamp{{StartSeconds: 65, Label: "Start"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{"# Lesson", "> Supporting transcript: spoken evidence", "t=65s"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in:\n%s", expected, text)
		}
	}
}
