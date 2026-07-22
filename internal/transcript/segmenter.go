package transcript

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
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
	var start, end = doc.Cues[0].Start, doc.Cues[0].End
	reference := doc.Cues[0].Reference
	flush := func() {
		if b.Len() == 0 {
			return
		}
		result = append(result, Segment{Index: len(result), Start: start, End: end, Text: strings.TrimSpace(b.String()), Reference: reference})
		b.Reset()
	}
	for _, originalCue := range doc.Cues {
		cues := splitCue(originalCue, s.maxCharacters)
		for _, cue := range cues {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if b.Len() > 0 && b.Len()+len(cue.Text)+1 > s.maxCharacters {
				flush()
				start = cue.Start
				reference = cue.Reference
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(cue.Text)
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
	if len(cue.Text) <= limit {
		return []Cue{cue}
	}
	parts := splitText(cue.Text, limit)
	result := make([]Cue, 0, len(parts))
	duration := cue.End - cue.Start
	for index, text := range parts {
		start := cue.Start + timeFraction(duration, index, len(parts))
		end := cue.Start + timeFraction(duration, index+1, len(parts))
		result = append(result, Cue{Start: start, End: end, Text: text, Reference: cue.Reference})
	}
	return result
}

func splitText(text string, limit int) []string {
	var parts []string
	for len(text) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		if whitespace := strings.LastIndexAny(text[:cut], " \t\n"); whitespace > 0 {
			cut = whitespace
		}
		parts = append(parts, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	if text = strings.TrimSpace(text); text != "" {
		parts = append(parts, text)
	}
	return parts
}

func timeFraction(duration time.Duration, part, total int) time.Duration {
	return time.Duration(int64(duration) * int64(part) / int64(total))
}
