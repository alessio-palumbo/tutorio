package transcript

import (
	"context"
	"strings"
	"time"
	"unicode"
)

// Segmenter splits a document at cue boundaries.
type Segmenter interface {
	Segment(ctx context.Context, document Document) ([]Segment, error)
}
type CueSegmenter struct{ maxCharacters int }

func NewSegmenter(maxCharacters int) CueSegmenter {
	if maxCharacters <= 0 {
		maxCharacters = 12000
	}
	return CueSegmenter{maxCharacters: maxCharacters}
}
func (s CueSegmenter) Segment(ctx context.Context, doc Document) ([]Segment, error) {
	if len(doc.Cues) == 0 {
		return []Segment{}, nil
	}
	var result []Segment
	var b strings.Builder
	characters := 0
	var start, end = doc.Cues[0].Start, doc.Cues[0].End
	reference := doc.Cues[0].Reference
	titleHint := doc.Cues[0].TitleHint
	var chunks []SourceChunk
	flush := func() {
		if b.Len() == 0 {
			return
		}
		result = append(result, Segment{Index: len(result), Start: start, End: end, Text: strings.TrimSpace(b.String()), Reference: reference, Chunks: chunks, TitleHint: titleHint})
		b.Reset()
		characters = 0
		chunks = nil
		titleHint = ""
	}
	begin := func(cue Cue) {
		start = cue.Start
		end = cue.End
		reference = cue.Reference
		titleHint = cue.TitleHint
	}
	minimumBoundarySize := max(1, s.maxCharacters/3)
	for _, originalCue := range doc.Cues {
		cues := splitCue(originalCue, s.maxCharacters)
		for _, cue := range cues {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			cueCharacters := len([]rune(cue.Text))
			structuralBoundary := cue.BoundaryKind == "chapter" || cue.BoundaryKind == "heading" && characters >= minimumBoundarySize
			longPause := b.Len() > 0 && characters >= minimumBoundarySize && cue.Start-end >= 8*time.Second
			if b.Len() > 0 && (structuralBoundary || longPause) {
				flush()
				begin(cue)
			}
			if b.Len() > 0 && characters+cueCharacters+1 > s.maxCharacters {
				flush()
				begin(cue)
			}
			if b.Len() == 0 {
				begin(cue)
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
				characters++
			}
			b.WriteString(cue.Text)
			characters += cueCharacters
			if cue.ChunkID != "" {
				chunks = append(chunks, SourceChunk{ID: cue.ChunkID, Kind: cue.ChunkKind, Text: cue.Text, Reference: cue.Reference, Sequence: cue.Sequence})
			}
			end = cue.End
			if cue.Reference.Kind != "" {
				if reference.Kind == "" {
					reference = cue.Reference
				}
				if cue.Reference.PageEnd > reference.PageEnd {
					reference.PageEnd = cue.Reference.PageEnd
				}
			}
		}
	}
	flush()
	return result, nil
}

func splitCue(cue Cue, limit int) []Cue {
	if len([]rune(cue.Text)) <= limit {
		return []Cue{cue}
	}
	parts := splitText(cue.Text, limit)
	result := make([]Cue, 0, len(parts))
	duration := cue.End - cue.Start
	for index, text := range parts {
		start := cue.Start + timeFraction(duration, index, len(parts))
		end := cue.Start + timeFraction(duration, index+1, len(parts))
		part := Cue{Start: start, End: end, Text: text, Reference: cue.Reference, ChunkID: cue.ChunkID, ChunkKind: cue.ChunkKind, Sequence: cue.Sequence, TitleHint: cue.TitleHint}
		if index == 0 {
			part.BoundaryKind = cue.BoundaryKind
		}
		result = append(result, part)
	}
	return result
}

func splitText(text string, limit int) []string {
	runes := []rune(strings.TrimSpace(text))
	var parts []string
	for len(runes) > limit {
		cut := limit
		for cut > limit/2 && !unicode.IsSpace(runes[cut-1]) {
			cut--
		}
		if cut <= limit/2 {
			cut = limit
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	if value := strings.TrimSpace(string(runes)); value != "" {
		parts = append(parts, value)
	}
	return parts
}

func timeFraction(duration time.Duration, part, total int) time.Duration {
	return time.Duration(int64(duration) * int64(part) / int64(total))
}
