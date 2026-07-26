package guide

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alessio/tutorio/internal/evidence"
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
	OnFailure  func(result SectionResult)
}
type SectionResult struct {
	Index                      int
	Segment                    transcript.Segment
	Guide                      Guide
	Model                      string
	PromptTokens               int
	OutputTokens               int
	DurationMilliseconds       int64
	PromptDurationMilliseconds int64
	OutputDurationMilliseconds int64
	RawResponse                string
	Error                      string
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
		metrics.Index = segment.Index
		metrics.Segment = segment
		if err != nil {
			metrics.Error = err.Error()
			if req.OnFailure != nil {
				req.OnFailure(metrics)
			}
			return Guide{}, fmt.Errorf("generate section %d of %d: %w", index+1, len(req.Segments), err)
		}
		partials = append(partials, partial)
		metadata.Model = metrics.Model
		metadata.PromptTokens += metrics.PromptTokens
		metadata.OutputTokens += metrics.OutputTokens
		metadata.DurationMilliseconds += metrics.DurationMilliseconds
		metadata.PromptDurationMilliseconds += metrics.PromptDurationMilliseconds
		metadata.OutputDurationMilliseconds += metrics.OutputDurationMilliseconds
		if req.OnSegment != nil {
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
		Index        int                      `json:"index"`
		StartSeconds float64                  `json:"start_seconds"`
		EndSeconds   float64                  `json:"end_seconds"`
		Reference    transcript.Reference     `json:"reference,omitempty"`
		TitleHint    string                   `json:"title_hint,omitempty"`
		Text         string                   `json:"text"`
		Chunks       []transcript.SourceChunk `json:"source_chunks,omitempty"`
	}{Index: segment.Index, StartSeconds: segment.Start.Seconds(), EndSeconds: segment.End.Seconds(), Reference: segment.Reference, TitleHint: segment.TitleHint, Text: segment.Text, Chunks: segment.Chunks}
	data, err := json.Marshal(promptSegment)
	if err != nil {
		return Guide{}, SectionResult{}, err
	}
	prompt := `Reconstruct this section of a tutorial as part of a practical manual, not a summary. Preserve sequence, explanations, exact commands, warnings, mistakes, and timestamps. Extract every keyboard shortcut mentioned and include concise command/shortcut references in the cheat_sheet. Do not invent information from sections you have not seen. The title field must be a concise, content-specific description of this section alone. Do not reuse the overall tutorial title or add generic labels such as "Part 1" or "Section 1". Use the supplied title_hint when it accurately describes the section. The supplied start_seconds and end_seconds are absolute positions in the complete source; every timestamp you return must use that same absolute timeline and fall within this range. Return only JSON matching these fields: title, overview, prerequisites, final_outcome, steps, important_concepts, commands, keyboard_shortcuts, warnings, common_mistakes, cheat_sheet, appendix, source_timestamps. prerequisites, important_concepts, warnings, common_mistakes, cheat_sheet, and appendix must be arrays of plain strings, never objects. Prerequisites are prior knowledge or required tools, not tutorial steps; keep them concise and do not repeat equivalent requirements. Write mathematical notation as valid LaTeX between $ delimiters and JSON-escape every backslash. Every step must include number, title, explanation, actions, commands, warnings, timestamps, and source_excerpt containing a short verbatim supporting excerpt from this transcript section. Timestamps use numeric start_seconds and end_seconds measured from the start of the complete video, plus label. Unknown arrays must be empty.`
	if segment.Reference.Kind == "page" {
		prompt = `Reconstruct this section of a document as part of a practical learning manual, not a summary. Preserve sequence, explanations, exact commands, warnings, mistakes, and terminology. Extract useful shortcuts and concise references in the cheat_sheet. Do not invent information from pages you have not seen. The title field must be a concise, content-specific description of this section alone. Do not reuse the overall document title or add generic labels such as "Part 1" or "Section 1". Use the supplied title_hint when it accurately describes the section. Return only JSON matching these fields: title, overview, prerequisites, final_outcome, steps, important_concepts, commands, keyboard_shortcuts, warnings, common_mistakes, cheat_sheet, appendix, source_timestamps. prerequisites, important_concepts, warnings, common_mistakes, cheat_sheet, and appendix must be arrays of plain strings, never objects. Prerequisites are prior knowledge or required tools, not procedural steps; keep them concise and do not repeat equivalent requirements. Write mathematical notation as valid LaTeX between $ delimiters and JSON-escape every backslash. Every step must include number, title, explanation, actions, commands, warnings, an empty timestamps array, and evidence_chunk_ids. evidence_chunk_ids must be an array containing at most five IDs copied exactly from source_chunks that directly support the step. Never invent an ID and never cite a chunk merely because it is nearby. If no supplied chunk directly supports a step, return an empty evidence_chunk_ids array. Unknown arrays must be empty.`
	}
	response, err := g.provider.Complete(ctx, llm.Request{Format: "json", Temperature: 0, MaxTokens: g.maxTokens, ContextSize: g.contextSize, Messages: []llm.Message{{Role: "system", Content: prompt}, {Role: "user", Content: fmt.Sprintf("Tutorial title: %s\nSection %d of %d:\n%s", title, current, total, data)}}})
	if err != nil {
		return Guide{}, SectionResult{}, err
	}
	metrics := SectionResult{
		Model:                      response.Model,
		PromptTokens:               response.PromptTokens,
		OutputTokens:               response.OutputTokens,
		DurationMilliseconds:       response.DurationNanos / int64(time.Millisecond),
		PromptDurationMilliseconds: response.PromptDurationNanos / int64(time.Millisecond),
		OutputDurationMilliseconds: response.OutputDurationNanos / int64(time.Millisecond),
		RawResponse:                response.Content,
	}
	var result Guide
	normalized, err := normalizeGuideJSON(response.Content)
	if err != nil {
		return Guide{}, metrics, fmt.Errorf("normalize generated guide: %w", err)
	}
	if err := json.Unmarshal(normalized, &result); err != nil {
		return Guide{}, metrics, fmt.Errorf("decode generated guide: %w", err)
	}
	if result.Title == "" || repeatsGuideTitle(result.Title, title) {
		if segment.TitleHint != "" {
			result.Title = segment.TitleHint
		} else if len(result.Steps) > 0 && strings.TrimSpace(result.Steps[0].Title) != "" {
			result.Title = strings.TrimSpace(result.Steps[0].Title)
		}
	}
	for index := range result.Steps {
		result.Steps[index].ID = fmt.Sprintf("step_%d_%d", segment.Index, index+1)
		result.Steps[index].SourceSegment = segment.Index
	}
	result.Prerequisites = appendUniqueSimilar(nil, result.Prerequisites...)
	resolveStepCitations(&result, segment)
	anchorGuideSource(&result, segment)
	if len(segment.Chunks) == 0 {
		validateSourceExcerpts(&result, segment.Text)
	}
	return result, metrics, nil
}

func repeatsGuideTitle(sectionTitle, guideTitle string) bool {
	section := strings.ToLower(strings.TrimSpace(sectionTitle))
	whole := strings.ToLower(strings.TrimSpace(guideTitle))
	if section == "" || whole == "" {
		return false
	}
	if section == whole {
		return true
	}
	if !strings.HasPrefix(section, whole) {
		return false
	}
	suffix := strings.Trim(strings.TrimPrefix(section, whole), " \t\r\n:–—-·")
	for _, prefix := range []string{"part", "section", "chapter"} {
		if strings.HasPrefix(suffix, prefix) {
			number := strings.TrimSpace(strings.TrimPrefix(suffix, prefix))
			if number != "" {
				for _, char := range number {
					if char < '0' || char > '9' {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

const maxCitationsPerStep = 5

func resolveStepCitations(value *Guide, segment transcript.Segment) {
	allowed := make(map[string]transcript.SourceChunk, len(segment.Chunks))
	for _, chunk := range segment.Chunks {
		allowed[chunk.ID] = chunk
	}
	for stepIndex := range value.Steps {
		step := &value.Steps[stepIndex]
		step.Citations = nil
		step.References = nil
		step.SourceExcerpt = ""
		seen := make(map[string]struct{}, len(step.EvidenceChunkIDs))
		for _, chunkID := range step.EvidenceChunkIDs {
			chunk, ok := allowed[chunkID]
			if !ok {
				continue
			}
			if _, duplicate := seen[chunkID]; duplicate {
				continue
			}
			seen[chunkID] = struct{}{}
			evidenceID := evidence.EvidenceIDForChunk(chunkID)
			citationHash := sha256.Sum256([]byte(step.ID + "\x00" + evidenceID))
			label := chunk.Reference.Label
			if label == "" && chunk.Reference.PageStart > 0 {
				label = fmt.Sprintf("PDF page %d", chunk.Reference.PageStart)
			}
			step.Citations = append(step.Citations, Citation{ID: fmt.Sprintf("cit_%x", citationHash), EvidenceID: evidenceID, Support: SupportDirect, Label: label})
			step.References = appendUniqueReferences(step.References, SourceReference{Kind: "page", PageStart: chunk.Reference.PageStart, PageEnd: chunk.Reference.PageEnd, Label: label})
			if step.SourceExcerpt == "" {
				step.SourceExcerpt = chunk.Text
			}
			if len(step.Citations) == maxCitationsPerStep {
				break
			}
		}
		step.EvidenceChunkIDs = nil
	}
}

func validateSourceExcerpts(value *Guide, transcriptText string) {
	haystack := strings.ToLower(compactWhitespace(transcriptText))
	for index := range value.Steps {
		excerpt := compactWhitespace(value.Steps[index].SourceExcerpt)
		if excerpt == "" || !strings.Contains(haystack, strings.ToLower(excerpt)) {
			value.Steps[index].SourceExcerpt = ""
		} else {
			value.Steps[index].SourceExcerpt = excerpt
		}
	}
}
func compactWhitespace(value string) string { return strings.Join(strings.Fields(value), " ") }

var guideArrayFields = []string{"prerequisites", "steps", "important_concepts", "commands", "keyboard_shortcuts", "warnings", "common_mistakes", "cheat_sheet", "appendix", "source_timestamps"}
var stepArrayFields = []string{"actions", "commands", "warnings", "timestamps", "evidence_chunk_ids"}
var guideStringArrayFields = []string{"prerequisites", "important_concepts", "warnings", "common_mistakes", "cheat_sheet", "appendix"}
var stepStringArrayFields = []string{"actions", "commands", "warnings", "evidence_chunk_ids"}

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
				normalizeTextField(step, "source_excerpt")
				normalizeTimestampArray(step, "timestamps")
			}
		}
	}
	return json.Marshal(value)
}

func normalizeTextField(value map[string]any, field string) {
	raw, ok := value[field]
	if !ok || raw == nil {
		value[field] = ""
		return
	}
	if _, ok := raw.(string); ok {
		return
	}
	encoded, err := json.Marshal(raw)
	if err != nil || !meaningfulText(string(encoded)) {
		value[field] = ""
		return
	}
	value[field] = string(encoded)
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
	start, startOK := timestampSeconds(firstValue(value, "start_seconds", "start", "start_time", "timestamp", "time"))
	end, endOK := timestampSeconds(firstValue(value, "end_seconds", "end", "end_time"))
	if !startOK && endOK {
		start = end
	}
	if !endOK {
		end = start
	}
	value["start_seconds"] = start
	value["end_seconds"] = end
	if _, ok := value["label"].(string); !ok {
		value["label"] = ""
	}
}

func firstValue(value map[string]any, keys ...string) any {
	for _, key := range keys {
		if item, ok := value[key]; ok && item != nil {
			return item
		}
	}
	return nil
}

func timestampSeconds(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		return parseSeconds(strings.TrimSpace(typed))
	default:
		return 0, false
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
		for _, text := range humanReadableValues(field, item) {
			if meaningfulText(text) {
				result = append(result, text)
			}
		}
	}
	value[field] = result
}

func humanReadableValues(field string, raw any) []string {
	switch typed := raw.(type) {
	case string:
		return []string{strings.TrimSpace(typed)}
	case []any:
		var result []string
		for _, item := range typed {
			result = append(result, humanReadableValues(field, item)...)
		}
		return result
	case map[string]any:
		if text := describedObject(field, typed); text != "" {
			return []string{text}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]string, 0, len(keys))
		for _, key := range keys {
			for _, text := range humanReadableValues(field, typed[key]) {
				if meaningfulText(text) {
					result = append(result, humanizeKey(key)+": "+text)
				}
			}
		}
		return result
	case float64, bool:
		return []string{fmt.Sprint(typed)}
	default:
		return nil
	}
}

func describedObject(field string, value map[string]any) string {
	var primaryKeys, detailKeys []string
	switch field {
	case "warnings":
		primaryKeys, detailKeys = []string{"warning"}, []string{"details", "context"}
	case "common_mistakes":
		primaryKeys, detailKeys = []string{"mistake"}, []string{"correction", "consequence", "description"}
	case "prerequisites":
		primaryKeys, detailKeys = []string{"prerequisite", "requirement", "title"}, []string{"explanation", "description"}
	case "appendix":
		primaryKeys, detailKeys = []string{"title", "note"}, []string{"content", "description", "details"}
	}
	primary := firstScalar(value, primaryKeys...)
	if primary == "" {
		return ""
	}
	detail := firstScalar(value, detailKeys...)
	if detail != "" && detail != primary {
		return primary + " — " + detail
	}
	return primary
}

func firstScalar(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func humanizeKey(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
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
		result.Prerequisites = appendUniqueSimilar(result.Prerequisites, item.Prerequisites...)
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
		result.SourceReferences = appendUniqueReferences(result.SourceReferences, item.SourceReferences...)
	}
	populateCheatSheet(&result)
	return result
}

// Merge combines independently generated source sections into one guide.
func Merge(title string, sections []Guide) Guide { return mergeGuides(title, sections) }

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

func anchorGuideSource(value *Guide, segment transcript.Segment) {
	if segment.Reference.Kind == "page" {
		pageStart, pageEnd := segment.Reference.PageStart, segment.Reference.PageEnd
		if pageEnd < pageStart {
			pageEnd = pageStart
		}
		for index := range value.Steps {
			value.Steps[index].Timestamps = nil
		}
		value.SourceTimestamps = nil
		value.SourceReferences = []SourceReference{{Kind: "page", PageStart: pageStart, PageEnd: pageEnd, Label: segment.Reference.Label}}
		return
	}
	anchorGuideTimestamps(value, segment)
	for index := range value.Steps {
		value.Steps[index].References = referencesFromTimestamps(value.Steps[index].Timestamps)
	}
	value.SourceReferences = referencesFromTimestamps(value.SourceTimestamps)
}

func referencesFromTimestamps(values []Timestamp) []SourceReference {
	result := make([]SourceReference, 0, len(values))
	for _, value := range values {
		result = append(result, SourceReference{Kind: "time", StartSeconds: value.StartSeconds, EndSeconds: value.EndSeconds, Label: value.Label})
	}
	return result
}

func appendUniqueReferences(values []SourceReference, additions ...SourceReference) []SourceReference {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[fmt.Sprintf("%s:%g:%g:%d:%d:%s", value.Kind, value.StartSeconds, value.EndSeconds, value.PageStart, value.PageEnd, value.Label)] = struct{}{}
	}
	for _, value := range additions {
		key := fmt.Sprintf("%s:%g:%g:%d:%d:%s", value.Kind, value.StartSeconds, value.EndSeconds, value.PageStart, value.PageEnd, value.Label)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, value)
	}
	return values
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

func appendUniqueSimilar(values []string, additions ...string) []string {
	for _, addition := range additions {
		key := comparableText(addition)
		if !meaningfulText(key) {
			continue
		}
		duplicate := false
		for _, existing := range values {
			other := comparableText(existing)
			if key == other || (len(key) >= 18 && len(other) >= 18 && (strings.Contains(key, other) || strings.Contains(other, key))) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			values = append(values, strings.TrimSpace(addition))
		}
	}
	return values
}

func comparableText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}), " ")
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
