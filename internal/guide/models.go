// Package guide owns the structured learning-guide domain.
package guide

import "time"

type Timestamp struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Label        string  `json:"label"`
}
type Step struct {
	ID            string      `json:"id,omitempty"`
	Number        int         `json:"number"`
	SourceSegment int         `json:"source_segment"`
	UserEdited    bool        `json:"user_edited,omitempty"`
	Title         string      `json:"title"`
	Explanation   string      `json:"explanation"`
	Actions       []string    `json:"actions"`
	Commands      []string    `json:"commands,omitempty"`
	Warnings      []string    `json:"warnings,omitempty"`
	Timestamps    []Timestamp `json:"timestamps,omitempty"`
	SourceExcerpt string      `json:"source_excerpt,omitempty"`
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
type Guide struct {
	ID                string      `json:"id"`
	SourceType        string      `json:"source_type"`
	SourceURI         string      `json:"source_uri"`
	SourceID          string      `json:"source_id"`
	Title             string      `json:"title"`
	Overview          string      `json:"overview"`
	Prerequisites     []string    `json:"prerequisites"`
	FinalOutcome      string      `json:"final_outcome"`
	Steps             []Step      `json:"steps"`
	ImportantConcepts []string    `json:"important_concepts"`
	Commands          []Command   `json:"commands"`
	KeyboardShortcuts []Shortcut  `json:"keyboard_shortcuts"`
	Warnings          []string    `json:"warnings"`
	CommonMistakes    []string    `json:"common_mistakes"`
	CheatSheet        []string    `json:"cheat_sheet"`
	Appendix          []string    `json:"appendix"`
	SourceTimestamps  []Timestamp `json:"source_timestamps"`
	Generation        Generation  `json:"generation"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}
type Generation struct {
	JobID                string `json:"job_id,omitempty"`
	Model                string `json:"model,omitempty"`
	DurationMilliseconds int64  `json:"duration_milliseconds,omitempty"`
	SegmentCount         int    `json:"segment_count,omitempty"`
	PromptTokens         int    `json:"prompt_tokens,omitempty"`
	OutputTokens         int    `json:"output_tokens,omitempty"`
	ContextWindow        int    `json:"context_window,omitempty"`
	MaxOutputTokens      int    `json:"max_output_tokens,omitempty"`
}
type Summary struct {
	ID         string    `json:"id"`
	SourceType string    `json:"source_type"`
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	CreatedAt  time.Time `json:"created_at"`
}
