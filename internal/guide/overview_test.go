package guide

import (
	"context"
	"strings"
	"testing"

	"github.com/alessio/tutorio/internal/llm"
)

type overviewProvider struct {
	request llm.Request
}

func (p *overviewProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.request = request
	return llm.Response{Content: `{"overview":"This guide builds a complete local workflow from setup through verification."}`, Model: "test-model", PromptTokens: 40, OutputTokens: 18}, nil
}

func TestSynthesizeOverviewUsesOnlyStructuredSectionMaterial(t *testing.T) {
	provider := &overviewProvider{}
	result, err := NewLLMGenerator(provider).SynthesizeOverview(context.Background(), OverviewRequest{
		Title:        "Local workflow",
		FinalOutcome: "A verified result",
		Sections: []OverviewSection{
			{Title: "Setup", Overview: "Install and configure the required tools."},
			{Title: "Verification", Overview: "Run checks and inspect the result."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "test-model" || !strings.Contains(result.Text, "complete local workflow") {
		t.Fatalf("unexpected overview result: %#v", result)
	}
	if provider.request.MaxTokens != 512 || provider.request.Format != "json" {
		t.Fatalf("unexpected overview request: %#v", provider.request)
	}
	userPrompt := provider.request.Messages[len(provider.request.Messages)-1].Content
	if !strings.Contains(userPrompt, `"title":"Setup"`) || !strings.Contains(userPrompt, `"final_outcome":"A verified result"`) {
		t.Fatalf("structured section material missing from prompt: %s", userPrompt)
	}
}

func TestSynthesizeOverviewRejectsEmptySectionInput(t *testing.T) {
	_, err := NewLLMGenerator(&overviewProvider{}).SynthesizeOverview(context.Background(), OverviewRequest{Title: "Empty"})
	if err == nil {
		t.Fatal("expected empty section input to fail")
	}
}
