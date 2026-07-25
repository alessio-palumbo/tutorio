package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alessio/tutorio/internal/llm"
)

type OverviewRequest struct {
	Title        string            `json:"title"`
	FinalOutcome string            `json:"final_outcome"`
	Sections     []OverviewSection `json:"sections"`
}

type OverviewSection struct {
	Title    string `json:"title"`
	Overview string `json:"overview"`
}

type OverviewResult struct {
	Text                 string
	Model                string
	PromptTokens         int
	OutputTokens         int
	DurationMilliseconds int64
}

type OverviewSynthesizer interface {
	SynthesizeOverview(ctx context.Context, request OverviewRequest) (OverviewResult, error)
}

func (g *LLMGenerator) SynthesizeOverview(ctx context.Context, request OverviewRequest) (OverviewResult, error) {
	if len(request.Sections) == 0 {
		return OverviewResult{}, fmt.Errorf("guide has no stored sections to summarize")
	}
	input, err := json.Marshal(request)
	if err != nil {
		return OverviewResult{}, fmt.Errorf("encode overview input: %w", err)
	}
	prompt := `Write a concise guide-level overview from the supplied section titles and section overviews. Explain the guide's overall purpose, progression, and practical result. Do not enumerate or repeat every section. Use 80 to 150 words. Do not add facts absent from the supplied material. Return only JSON with one string field named "overview".`
	response, err := g.provider.Complete(ctx, llm.Request{
		Format:      "json",
		Temperature: 0,
		MaxTokens:   min(g.maxTokens, 512),
		ContextSize: g.contextSize,
		Messages: []llm.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: string(input)},
		},
	})
	if err != nil {
		return OverviewResult{}, err
	}
	var decoded struct {
		Overview string `json:"overview"`
	}
	normalized, err := normalizeGuideJSON(response.Content)
	if err != nil {
		return OverviewResult{}, fmt.Errorf("normalize generated overview: %w", err)
	}
	if err = json.Unmarshal(normalized, &decoded); err != nil {
		return OverviewResult{}, fmt.Errorf("decode generated overview: %w", err)
	}
	decoded.Overview = strings.TrimSpace(decoded.Overview)
	if decoded.Overview == "" {
		return OverviewResult{}, fmt.Errorf("generated overview is empty")
	}
	return OverviewResult{
		Text:                 decoded.Overview,
		Model:                response.Model,
		PromptTokens:         response.PromptTokens,
		OutputTokens:         response.OutputTokens,
		DurationMilliseconds: response.DurationNanos / int64(time.Millisecond),
	}, nil
}
