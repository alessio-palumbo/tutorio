// Package local implements transcript-file ingestion.
package local

import (
	"context"
	"fmt"
	"os"

	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/transcript"
)

type TranscriptFile struct{ parser transcript.Parser }

func NewTranscriptFile(parser transcript.Parser) *TranscriptFile {
	return &TranscriptFile{parser: parser}
}
func (*TranscriptFile) Type() string { return "transcript_file" }
func (s *TranscriptFile) Fetch(ctx context.Context, req source.Request) (transcript.Document, error) {
	file, err := os.Open(req.URI)
	if err != nil {
		return transcript.Document{}, fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()
	doc, err := s.parser.Parse(ctx, req.URI, file)
	if req.Title != "" {
		doc.Title = req.Title
	}
	return doc, err
}
