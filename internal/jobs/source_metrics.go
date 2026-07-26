package jobs

import (
	"strings"
	"unicode/utf8"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/transcript"
)

func sourceMetrics(sourceType string, segments []transcript.Segment) guide.SourceMetrics {
	var text strings.Builder
	var durationSeconds float64
	var physicalPages int
	for _, segment := range segments {
		value := strings.TrimSpace(segment.Text)
		if value != "" {
			if text.Len() > 0 {
				text.WriteByte(' ')
			}
			text.WriteString(value)
		}
		durationSeconds = max(durationSeconds, segment.End.Seconds())
		physicalPages = max(physicalPages, segment.Reference.PageEnd)
		for _, chunk := range segment.Chunks {
			physicalPages = max(physicalPages, chunk.Reference.PageEnd)
		}
	}
	value := text.String()
	return guide.SourceMetrics{
		ExtractionMethod: extractionMethod(sourceType),
		Characters:       utf8.RuneCountInString(value),
		Words:            len(strings.Fields(value)),
		DurationSeconds:  durationSeconds,
		PhysicalPages:    physicalPages,
	}
}

func storedSourceMetrics(sourceType string, segments []Segment) guide.SourceMetrics {
	values := make([]transcript.Segment, 0, len(segments))
	for _, segment := range segments {
		values = append(values, segment.Transcript)
	}
	return sourceMetrics(sourceType, values)
}

func extractionMethod(sourceType string) string {
	switch sourceType {
	case "youtube":
		return "youtube-captions"
	case "pdf":
		return "pdf-text"
	case "transcript_file":
		return "imported-transcript"
	default:
		return sourceType
	}
}
