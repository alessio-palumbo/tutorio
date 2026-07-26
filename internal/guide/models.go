// Package guide owns the structured learning-guide domain.
package guide

import "time"

type Timestamp struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Label        string  `json:"label"`
}
type SourceReference struct {
	Kind         string  `json:"kind"`
	StartSeconds float64 `json:"start_seconds,omitempty"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
	PageStart    int     `json:"page_start,omitempty"`
	PageEnd      int     `json:"page_end,omitempty"`
	Label        string  `json:"label,omitempty"`
}
type SupportKind string

const (
	SupportDirect      SupportKind = "direct"
	SupportInferred    SupportKind = "inferred"
	SupportUnsupported SupportKind = "unsupported"
)

type Citation struct {
	ID         string      `json:"id"`
	EvidenceID string      `json:"evidence_id"`
	Support    SupportKind `json:"support"`
	Label      string      `json:"label,omitempty"`
}
type Step struct {
	ID               string            `json:"id,omitempty"`
	Number           int               `json:"number"`
	SourceSegment    int               `json:"source_segment"`
	UserEdited       bool              `json:"user_edited,omitempty"`
	Title            string            `json:"title"`
	Explanation      string            `json:"explanation"`
	Actions          []string          `json:"actions"`
	Commands         []string          `json:"commands,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Timestamps       []Timestamp       `json:"timestamps,omitempty"`
	References       []SourceReference `json:"references,omitempty"`
	Citations        []Citation        `json:"citations,omitempty"`
	SourceExcerpt    string            `json:"source_excerpt,omitempty"`
	EvidenceChunkIDs []string          `json:"evidence_chunk_ids,omitempty"`
}
type Shortcut struct {
	Keys    string `json:"keys"`
	Action  string `json:"action"`
	Context string `json:"context,omitempty"`
}
type Command struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}
type DeepDive struct {
	ID            string    `json:"id"`
	SourceSegment int       `json:"source_segment"`
	Title         string    `json:"title"`
	Explanation   string    `json:"explanation"`
	KeyPoints     []string  `json:"key_points"`
	Examples      []string  `json:"examples"`
	Caveats       []string  `json:"caveats"`
	Evidence      []string  `json:"evidence"`
	Model         string    `json:"model,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
type Guide struct {
	ID                 string             `json:"id"`
	SourceType         string             `json:"source_type"`
	SourceURI          string             `json:"source_uri"`
	SourceID           string             `json:"source_id"`
	SourceMetrics      SourceMetrics      `json:"source_metrics,omitempty"`
	Title              string             `json:"title"`
	Overview           string             `json:"overview"`
	OverviewGeneration OverviewGeneration `json:"overview_generation,omitempty"`
	Prerequisites      []string           `json:"prerequisites"`
	FinalOutcome       string             `json:"final_outcome"`
	Steps              []Step             `json:"steps"`
	ImportantConcepts  []string           `json:"important_concepts"`
	Commands           []Command          `json:"commands"`
	KeyboardShortcuts  []Shortcut         `json:"keyboard_shortcuts"`
	Warnings           []string           `json:"warnings"`
	CommonMistakes     []string           `json:"common_mistakes"`
	CheatSheet         []string           `json:"cheat_sheet"`
	Appendix           []string           `json:"appendix"`
	SourceTimestamps   []Timestamp        `json:"source_timestamps"`
	SourceReferences   []SourceReference  `json:"source_references,omitempty"`
	DeepDives          []DeepDive         `json:"deep_dives,omitempty"`
	Generation         Generation         `json:"generation"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

type SourceMetrics struct {
	ExtractionMethod string  `json:"extraction_method,omitempty"`
	Characters       int     `json:"characters,omitempty"`
	Words            int     `json:"words,omitempty"`
	DurationSeconds  float64 `json:"duration_seconds,omitempty"`
	PhysicalPages    int     `json:"physical_pages,omitempty"`
}

type OverviewStatus string

const (
	OverviewMissing OverviewStatus = "missing"
	OverviewReady   OverviewStatus = "ready"
	OverviewStale   OverviewStatus = "stale"
	OverviewFailed  OverviewStatus = "failed"
)

type OverviewGeneration struct {
	Status    OverviewStatus `json:"status,omitempty"`
	Model     string         `json:"model,omitempty"`
	Error     string         `json:"error,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}
type Generation struct {
	JobID                      string `json:"job_id,omitempty"`
	Model                      string `json:"model,omitempty"`
	DurationMilliseconds       int64  `json:"duration_milliseconds,omitempty"`
	PromptDurationMilliseconds int64  `json:"prompt_duration_milliseconds,omitempty"`
	OutputDurationMilliseconds int64  `json:"output_duration_milliseconds,omitempty"`
	SegmentCount               int    `json:"segment_count,omitempty"`
	PromptTokens               int    `json:"prompt_tokens,omitempty"`
	OutputTokens               int    `json:"output_tokens,omitempty"`
	ContextWindow              int    `json:"context_window,omitempty"`
	MaxOutputTokens            int    `json:"max_output_tokens,omitempty"`
}
type Summary struct {
	ID         string    `json:"id"`
	SourceType string    `json:"source_type"`
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	CreatedAt  time.Time `json:"created_at"`
}
