package transcript

import (
	"context"
	"html"
	"regexp"
	"strings"
)

// Cleaner normalizes transcript text without changing timestamps.
type Cleaner interface {
	Clean(ctx context.Context, document Document) (Document, error)
}
type TextCleaner struct{}

func NewCleaner() TextCleaner { return TextCleaner{} }

var tagPattern = regexp.MustCompile(`<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)

func (TextCleaner) Clean(ctx context.Context, doc Document) (Document, error) {
	cleaned := make([]Cue, 0, len(doc.Cues))
	previous := ""
	for _, cue := range doc.Cues {
		if err := ctx.Err(); err != nil {
			return Document{}, err
		}
		text := strings.TrimSpace(whitespacePattern.ReplaceAllString(tagPattern.ReplaceAllString(html.UnescapeString(cue.Text), ""), " "))
		if text == "" || text == previous {
			continue
		}
		cue.Text = text
		cleaned = append(cleaned, cue)
		previous = text
	}
	doc.Cues = cleaned
	return doc, nil
}
