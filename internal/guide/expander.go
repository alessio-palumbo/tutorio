package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/transcript"
)

type ExpandRequest struct {
	GuideTitle string
	Section    transcript.Segment
	Steps      []Step
}

type Expander interface {
	Expand(ctx context.Context, request ExpandRequest) (DeepDive, error)
}

func (g *LLMGenerator) Expand(ctx context.Context, request ExpandRequest) (DeepDive, error) {
	if strings.TrimSpace(request.Section.Text) == "" {
		return DeepDive{}, fmt.Errorf("source section is empty")
	}
	stepData, err := json.Marshal(request.Steps)
	if err != nil {
		return DeepDive{}, err
	}
	system := `Explain this tutorial section in greater depth while remaining strictly grounded in the supplied source transcript. Clarify reasoning, terminology, relationships, and practical implications that the source supports. Do not use outside knowledge and do not invent steps. Return only JSON with title and explanation strings, plus key_points, examples, caveats, and evidence as arrays of plain strings. Evidence entries must be short verbatim excerpts from the source transcript.`
	response, err := g.provider.Complete(ctx, llm.Request{Format: "json", Temperature: 0, MaxTokens: g.maxTokens, ContextSize: g.contextSize, Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: fmt.Sprintf("Guide: %s\nExisting section steps: %s\nSource transcript section:\n%s", request.GuideTitle, stepData, request.Section.Text)}}})
	if err != nil {
		return DeepDive{}, err
	}
	content := strings.TrimSpace(response.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var raw map[string]any
	if err = json.Unmarshal([]byte(strings.TrimSpace(content)), &raw); err != nil {
		return DeepDive{}, fmt.Errorf("decode deep dive: %w", err)
	}
	for _, field := range []string{"key_points", "examples", "caveats", "evidence"} {
		normalizeArray(raw, field)
		normalizeStringArray(raw, field)
	}
	normalized, err := json.Marshal(raw)
	if err != nil {
		return DeepDive{}, err
	}
	var result DeepDive
	if err = json.Unmarshal(normalized, &result); err != nil {
		return DeepDive{}, fmt.Errorf("decode normalized deep dive: %w", err)
	}
	validateEvidence(&result, request.Section.Text)
	result.ID = fmt.Sprintf("deep_dive_%d", request.Section.Index)
	result.SourceSegment = request.Section.Index
	result.Model = response.Model
	result.CreatedAt = time.Now().UTC()
	return result, nil
}

func validateEvidence(value *DeepDive, transcriptText string) {
	haystack := strings.ToLower(compactWhitespace(transcriptText))
	result := value.Evidence[:0]
	for _, item := range value.Evidence {
		item = compactWhitespace(item)
		if item != "" && strings.Contains(haystack, strings.ToLower(item)) {
			result = append(result, item)
		}
	}
	value.Evidence = result
}
