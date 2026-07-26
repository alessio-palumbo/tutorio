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
	Start        time.Duration `json:"start"`
	End          time.Duration `json:"end"`
	Text         string        `json:"text"`
	Reference    Reference     `json:"reference,omitempty"`
	ChunkID      string        `json:"chunk_id,omitempty"`
	ChunkKind    string        `json:"chunk_kind,omitempty"`
	Sequence     int           `json:"sequence,omitempty"`
	BoundaryKind string        `json:"boundary_kind,omitempty"`
	TitleHint    string        `json:"title_hint,omitempty"`
}

// Document is the normalized transcript exchanged by pipeline stages.
type Document struct {
	SourceID    string `json:"source_id"`
	Title       string `json:"title"`
	Language    string `json:"language,omitempty"`
	Cues        []Cue  `json:"cues"`
	SourceKind  string `json:"source_kind,omitempty"`
	SourceURI   string `json:"source_uri,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Extractor   string `json:"extractor,omitempty"`
}

type SourceChunk struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	Reference Reference `json:"reference"`
	Sequence  int       `json:"sequence"`
}

// Segment is a bounded group of cues suitable for an LLM context.
type Segment struct {
	Index     int           `json:"index"`
	Start     time.Duration `json:"start"`
	End       time.Duration `json:"end"`
	Text      string        `json:"text"`
	Reference Reference     `json:"reference,omitempty"`
	Chunks    []SourceChunk `json:"chunks,omitempty"`
	TitleHint string        `json:"title_hint,omitempty"`
}
