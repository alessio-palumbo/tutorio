// Package evidence owns immutable source provenance and resolution contracts.
package evidence

import "time"

type Source struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Locator     string            `json:"-"`
	Title       string            `json:"title"`
	Fingerprint string            `json:"fingerprint"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type SourceChunkKind string

const (
	SourceChunkText SourceChunkKind = "text"
)

type SourceLocation struct {
	PhysicalPage int    `json:"physical_page,omitempty"`
	PrintedLabel string `json:"printed_label,omitempty"`
}

type SourceChunk struct {
	ID        string            `json:"id"`
	SourceID  string            `json:"source_id"`
	Kind      SourceChunkKind   `json:"kind"`
	Text      string            `json:"text"`
	Location  SourceLocation    `json:"location"`
	Sequence  int               `json:"sequence"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type Kind string

const (
	EvidenceText  Kind = "text"
	EvidenceImage Kind = "image"
)

type Evidence struct {
	ID        string       `json:"id"`
	SourceID  string       `json:"source_id"`
	Kind      Kind         `json:"kind"`
	ChunkID   string       `json:"chunk_id"`
	CreatedAt time.Time    `json:"created_at"`
	Source    Source       `json:"source"`
	Chunk     SourceChunk  `json:"chunk"`
	Previous  *SourceChunk `json:"previous,omitempty"`
	Next      *SourceChunk `json:"next,omitempty"`
}

type Visual struct {
	Kind         Kind   `json:"kind"`
	SourceID     string `json:"source_id"`
	PhysicalPage int    `json:"physical_page"`
	MediaType    string `json:"media_type"`
	DataURL      string `json:"data_url"`
}
