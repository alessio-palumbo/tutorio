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
		DeepDives: []guide.DeepDive{{Title: "Why it matters", Explanation: "More detail", Evidence: []string{"spoken evidence"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{"# Lesson", "> Supporting transcript: spoken evidence", "t=65s", "## Deep dives", "### Why it matters"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestRenderIncludesPDFPageReferences(t *testing.T) {
	content, err := New().Render(context.Background(), guide.Guide{Title: "Book", Overview: "Overview", FinalOutcome: "Learned", SourceURI: "/tmp/My Book.pdf", Steps: []guide.Step{{Number: 1, Title: "Read", Explanation: "Understand", References: []guide.SourceReference{{Kind: "page", PageStart: 4, PageEnd: 6}}}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "pages 4-6") || !strings.Contains(text, "file:///tmp/My%20Book.pdf#page=4") {
		t.Fatalf("missing page reference in:\n%s", text)
	}
}
