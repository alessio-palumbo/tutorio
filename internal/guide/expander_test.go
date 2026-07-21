package guide

import (
	"context"
	"testing"

	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/transcript"
)

type expansionProvider struct{}

func (expansionProvider) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Model: "local-test", Content: `{"title":"Why it matters","explanation":"A grounded explanation.","key_points":["One"],"examples":[],"caveats":[],"evidence":["exact source words","invented evidence"]}`}, nil
}

func TestExpandCreatesSourceGroundedDeepDive(t *testing.T) {
	result, err := NewLLMGenerator(expansionProvider{}).Expand(context.Background(), ExpandRequest{GuideTitle: "Lesson", Section: transcript.Segment{Index: 2, Text: "These are exact source words from the tutorial."}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "deep_dive_2" || result.SourceSegment != 2 || result.Model != "local-test" || len(result.Evidence) != 1 || result.Evidence[0] != "exact source words" {
		t.Fatalf("unexpected deep dive: %#v", result)
	}
}
