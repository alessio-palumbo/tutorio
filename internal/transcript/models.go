// Package transcript contains source-neutral transcript processing.
package transcript

import "time"

type Reference struct {
	Kind      string `json:"kind"`
	PageStart int    `json:"page_start,omitempty"`
	PageEnd   int    `json:"page_end,omitempty"`
	Label     string `json:"label,omitempty"`
}

// Cue is a timestamped piece of source text.
type Cue struct {
	Start     time.Duration `json:"start"`
	End       time.Duration `json:"end"`
	Text      string        `json:"text"`
	Reference Reference     `json:"reference,omitempty"`
}

// Document is the normalized transcript exchanged by pipeline stages.
type Document struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Language string `json:"language,omitempty"`
	Cues     []Cue  `json:"cues"`
}

// Segment is a bounded group of cues suitable for an LLM context.
type Segment struct {
	Index     int           `json:"index"`
	Start     time.Duration `json:"start"`
	End       time.Duration `json:"end"`
	Text      string        `json:"text"`
	Reference Reference     `json:"reference,omitempty"`
}
