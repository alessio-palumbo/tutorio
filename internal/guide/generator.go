package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/transcript"
)

type GenerateRequest struct {
	Title      string
	SourceType string
	SourceURI  string
	SourceID   string
	Segments   []transcript.Segment
	OnProgress func(current, total int)
	OnSegment  func(result SectionResult)
}
type SectionResult struct {
	Index                int
	Segment              transcript.Segment
	Guide                Guide
	Model                string
	PromptTokens         int
	OutputTokens         int
	DurationMilliseconds int64
}
type Generator interface {
	Generate(ctx context.Context, request GenerateRequest) (Guide, error)
}
type LLMGenerator struct {
	provider    llm.Provider
	maxTokens   int
	contextSize int
}

func NewLLMGenerator(provider llm.Provider, limits ...int) *LLMGenerator {
	limit := 8192
	if len(limits) > 0 && limits[0] > 0 {
		limit = limits[0]
	}
	contextSize := 32768
	if len(limits) > 1 && limits[1] > 0 {
		contextSize = limits[1]
	}
	return &LLMGenerator{provider: provider, maxTokens: limit, contextSize: contextSize}
}
func (g *LLMGenerator) Generate(ctx context.Context, req GenerateRequest) (Guide, error) {
	if len(req.Segments) == 0 {
		return Guide{}, fmt.Errorf("transcript contains no content to generate")
	}
	partials := make([]Guide, 0, len(req.Segments))
	metadata := Generation{SegmentCount: len(req.Segments), ContextWindow: g.contextSize, MaxOutputTokens: g.maxTokens}
	for index, segment := range req.Segments {
		partial, metrics, err := g.generateSegment(ctx, req.Title, segment, index+1, len(req.Segments))
		if err != nil {
			return Guide{}, fmt.Errorf("generate section %d of %d: %w", index+1, len(req.Segments), err)
		}
		partials = append(partials, partial)
		metadata.Model = metrics.Model
		metadata.PromptTokens += metrics.PromptTokens
		metadata.OutputTokens += metrics.OutputTokens
		metadata.DurationMilliseconds += metrics.DurationMilliseconds
		if req.OnSegment != nil {
			metrics.Index = index
			metrics.Segment = segment
			metrics.Guide = partial
			req.OnSegment(metrics)
		}
		if req.OnProgress != nil {
			req.OnProgress(index+1, len(req.Segments))
		}
	}
	result := mergeGuides(req.Title, partials)
	result.SourceType = req.SourceType
	result.SourceURI = req.SourceURI
	result.SourceID = req.SourceID
	result.Generation = metadata
	return result, nil
}

func (g *LLMGenerator) generateSegment(ctx context.Context, title string, segment transcript.Segment, current, total int) (Guide, SectionResult, error) {
	promptSegment := struct {
		Index        int     `json:"index"`
		StartSeconds float64 `json:"start_seconds"`
		EndSeconds   float64 `json:"end_seconds"`
		Text         string  `json:"text"`
	}{Index: segment.Index, StartSeconds: segment.Start.Seconds(), EndSeconds: segment.End.Seconds(), Text: segment.Text}
	data, err := json.Marshal(promptSegment)
	if err != nil {
		return Guide{}, SectionResult{}, err
	}
	prompt := `Reconstruct this section of a tutorial as part of a practical manual, not a summary. Preserve sequence, explanations, exact commands, warnings, mistakes, and timestamps. Extract every keyboard shortcut mentioned and include concise command/shortcut references in the cheat_sheet. Do not invent information from sections you have not seen. The supplied start_seconds and end_seconds are absolute positions in the complete source; every timestamp you return must use that same absolute timeline and fall within this range. Return only JSON matching these fields: title, overview, prerequisites, final_outcome, steps, important_concepts, commands, keyboard_shortcuts, warnings, common_mistakes, cheat_sheet, appendix, source_timestamps. Every step must include number, title, explanation, actions, commands, warnings, timestamps, and source_excerpt containing a short verbatim supporting excerpt from this transcript section. Timestamps use start_seconds, end_seconds, label. Unknown arrays must be empty.`
	response, err := g.provider.Complete(ctx, llm.Request{Format: "json", Temperature: 0, MaxTokens: g.maxTokens, ContextSize: g.contextSize, Messages: []llm.Message{{Role: "system", Content: prompt}, {Role: "user", Content: fmt.Sprintf("Tutorial title: %s\nSection %d of %d:\n%s", title, current, total, data)}}})
	if err != nil {
		return Guide{}, SectionResult{}, err
	}
	var result Guide
	normalized, err := normalizeGuideJSON(response.Content)
	if err != nil {
		return Guide{}, SectionResult{}, fmt.Errorf("normalize generated guide: %w", err)
	}
	if err := json.Unmarshal(normalized, &result); err != nil {
		return Guide{}, SectionResult{}, fmt.Errorf("decode generated guide: %w", err)
	}
	anchorGuideTimestamps(&result, segment)
	metrics := SectionResult{Model: response.Model, PromptTokens: response.PromptTokens, OutputTokens: response.OutputTokens, DurationMilliseconds: response.DurationNanos / int64(time.Millisecond)}
	return result, metrics, nil
}

var guideArrayFields = []string{"prerequisites", "steps", "important_concepts", "commands", "keyboard_shortcuts", "warnings", "common_mistakes", "cheat_sheet", "appendix", "source_timestamps"}
var stepArrayFields = []string{"actions", "commands", "warnings", "timestamps"}
var guideStringArrayFields = []string{"prerequisites", "important_concepts", "warnings", "common_mistakes", "cheat_sheet", "appendix"}
var stepStringArrayFields = []string{"actions", "commands", "warnings"}

func normalizeGuideJSON(content string) ([]byte, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var value map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &value); err != nil {
		return nil, err
	}
	for _, field := range guideArrayFields {
		normalizeArray(value, field)
	}
	for _, field := range guideStringArrayFields {
		normalizeStringArray(value, field)
	}
	normalizeTimestampArray(value, "source_timestamps")
	normalizeCommandArray(value, "commands")
	normalizeShortcutArray(value, "keyboard_shortcuts")
	if steps, ok := value["steps"].([]any); ok {
		for _, raw := range steps {
			if step, ok := raw.(map[string]any); ok {
				for _, field := range stepArrayFields {
					normalizeArray(step, field)
				}
				for _, field := range stepStringArrayFields {
					normalizeStringArray(step, field)
				}
				normalizeTimestampArray(step, "timestamps")
			}
		}
	}
	return json.Marshal(value)
}

func normalizeTimestampArray(value map[string]any, field string) {
	raw, ok := value[field].([]any)
	if !ok {
		return
	}
	result := make([]any, 0, len(raw))
	for _, item := range raw {
		switch typed := item.(type) {
		case map[string]any:
			normalizeTimestampObject(typed)
			result = append(result, typed)
		case float64:
			result = append(result, map[string]any{"start_seconds": typed, "end_seconds": typed, "label": ""})
		case string:
			if seconds, ok := parseSeconds(typed); ok {
				result = append(result, map[string]any{"start_seconds": seconds, "end_seconds": seconds, "label": typed})
			}
		}
	}
	value[field] = result
}

func normalizeTimestampObject(value map[string]any) {
	if _, ok := value["start_seconds"]; !ok {
		value["start_seconds"] = value["start"]
	}
	if _, ok := value["end_seconds"]; !ok {
		value["end_seconds"] = value["end"]
	}
	for _, field := range []string{"start_seconds", "end_seconds"} {
		if text, ok := value[field].(string); ok {
			if seconds, valid := parseSeconds(text); valid {
				value[field] = seconds
			}
		}
	}
	if value["end_seconds"] == nil {
		value["end_seconds"] = value["start_seconds"]
	}
	if value["label"] == nil {
		value["label"] = ""
	}
}
func parseSeconds(value string) (float64, bool) {
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		return seconds, true
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	total := 0.0
	for _, part := range parts {
		number, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, false
		}
		total = total*60 + number
	}
	return total, true
}

func normalizeCommandArray(value map[string]any, field string) {
	raw, ok := value[field].([]any)
	if !ok {
		return
	}
	for index, item := range raw {
		if text, ok := item.(string); ok {
			raw[index] = map[string]any{"value": text, "description": ""}
			continue
		}
		if object, ok := item.(map[string]any); ok {
			if object["value"] == nil {
				object["value"] = object["command"]
			}
			if object["description"] == nil {
				object["description"] = ""
			}
		}
	}
}
func normalizeShortcutArray(value map[string]any, field string) {
	raw, ok := value[field].([]any)
	if !ok {
		return
	}
	for index, item := range raw {
		if text, ok := item.(string); ok {
			raw[index] = map[string]any{"keys": text, "action": "", "context": ""}
			continue
		}
		if object, ok := item.(map[string]any); ok {
			if object["keys"] == nil {
				object["keys"] = object["shortcut"]
			}
			if object["action"] == nil {
				object["action"] = ""
			}
			if object["context"] == nil {
				object["context"] = ""
			}
		}
	}
}

func normalizeStringArray(value map[string]any, field string) {
	raw, ok := value[field].([]any)
	if !ok {
		return
	}
	result := make([]any, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			if meaningfulText(text) {
				result = append(result, text)
			}
			continue
		}
		encoded, err := json.Marshal(item)
		if err == nil && meaningfulText(string(encoded)) {
			result = append(result, string(encoded))
		}
	}
	value[field] = result
}

func normalizeArray(value map[string]any, field string) {
	raw, ok := value[field]
	if !ok || raw == nil {
		value[field] = []any{}
		return
	}
	if _, ok := raw.([]any); !ok {
		value[field] = []any{raw}
	}
}

func mergeGuides(title string, guides []Guide) Guide {
	result := Guide{Title: title}
	for _, item := range guides {
		result.Overview = joinText(result.Overview, item.Overview)
		result.Prerequisites = appendUnique(result.Prerequisites, item.Prerequisites...)
		if item.FinalOutcome != "" {
			result.FinalOutcome = item.FinalOutcome
		}
		for _, step := range item.Steps {
			step.Number = len(result.Steps) + 1
			result.Steps = append(result.Steps, step)
		}
		result.ImportantConcepts = appendUnique(result.ImportantConcepts, item.ImportantConcepts...)
		result.Commands = appendUniqueCommands(result.Commands, item.Commands...)
		result.KeyboardShortcuts = appendUniqueShortcuts(result.KeyboardShortcuts, item.KeyboardShortcuts...)
		result.Warnings = appendUnique(result.Warnings, item.Warnings...)
		result.CommonMistakes = appendUnique(result.CommonMistakes, item.CommonMistakes...)
		result.CheatSheet = appendUnique(result.CheatSheet, item.CheatSheet...)
		result.Appendix = appendUnique(result.Appendix, item.Appendix...)
		result.SourceTimestamps = append(result.SourceTimestamps, item.SourceTimestamps...)
	}
	populateCheatSheet(&result)
	return result
}

func anchorGuideTimestamps(value *Guide, segment transcript.Segment) {
	start, end := segment.Start.Seconds(), segment.End.Seconds()
	for index := range value.Steps {
		step := &value.Steps[index]
		if len(step.Timestamps) == 0 {
			count := float64(len(value.Steps))
			stepStart := start + (end-start)*float64(index)/count
			stepEnd := start + (end-start)*float64(index+1)/count
			step.Timestamps = []Timestamp{{StartSeconds: stepStart, EndSeconds: stepEnd, Label: step.Title}}
			continue
		}
		for timestampIndex := range step.Timestamps {
			anchorTimestamp(&step.Timestamps[timestampIndex], start, end)
		}
	}
	for index := range value.SourceTimestamps {
		anchorTimestamp(&value.SourceTimestamps[index], start, end)
	}
	if len(value.SourceTimestamps) == 0 {
		value.SourceTimestamps = []Timestamp{{StartSeconds: start, EndSeconds: end, Label: fmt.Sprintf("Source section %d", segment.Index+1)}}
	}
}

func anchorTimestamp(value *Timestamp, start, end float64) {
	if value.StartSeconds < start-1 {
		value.StartSeconds += start
		value.EndSeconds += start
	}
	if value.StartSeconds < start {
		value.StartSeconds = start
	}
	if value.StartSeconds > end {
		value.StartSeconds = end
	}
	if value.EndSeconds < value.StartSeconds {
		value.EndSeconds = value.StartSeconds
	}
	if value.EndSeconds > end {
		value.EndSeconds = end
	}
}

func populateCheatSheet(value *Guide) {
	for _, shortcut := range value.KeyboardShortcuts {
		text := strings.TrimSpace(shortcut.Keys)
		if shortcut.Action != "" {
			text += " — " + shortcut.Action
		}
		value.CheatSheet = appendUnique(value.CheatSheet, text)
	}
	for _, command := range value.Commands {
		text := strings.TrimSpace(command.Value)
		if command.Description != "" {
			text += " — " + command.Description
		}
		value.CheatSheet = appendUnique(value.CheatSheet, text)
	}
	for _, step := range value.Steps {
		for _, command := range step.Commands {
			value.CheatSheet = appendUnique(value.CheatSheet, command)
		}
	}
}

func joinText(existing, next string) string {
	if next == "" || strings.Contains(existing, next) {
		return existing
	}
	if existing == "" {
		return next
	}
	return existing + "\n\n" + next
}
func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		seen[strings.TrimSpace(v)] = struct{}{}
	}
	for _, v := range additions {
		key := strings.TrimSpace(v)
		if !meaningfulText(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, v)
	}
	return values
}

func meaningfulText(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "{}", "[]", "null":
		return false
	default:
		return true
	}
}
func appendUniqueCommands(values []Command, additions ...Command) []Command {
	seen := map[string]struct{}{}
	for _, v := range values {
		seen[v.Value] = struct{}{}
	}
	for _, v := range additions {
		if _, ok := seen[v.Value]; v.Value == "" || ok {
			continue
		}
		seen[v.Value] = struct{}{}
		values = append(values, v)
	}
	return values
}
func appendUniqueShortcuts(values []Shortcut, additions ...Shortcut) []Shortcut {
	seen := map[string]struct{}{}
	for _, v := range values {
		seen[v.Keys+"\x00"+v.Action] = struct{}{}
	}
	for _, v := range additions {
		key := v.Keys + "\x00" + v.Action
		if _, ok := seen[key]; v.Keys == "" || ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, v)
	}
	return values
}
