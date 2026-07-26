package jobs

import (
	"testing"
	"time"

	"github.com/alessio/tutorio/internal/transcript"
)

func TestSourceMetricsUseCleanedSegmentTextAndSourceExtent(t *testing.T) {
	segments := []transcript.Segment{
		{Text: "Hello 世界", End: 30 * time.Second, Reference: transcript.Reference{Kind: "page", PageEnd: 2}},
		{Text: "Second section", End: 75 * time.Second, Reference: transcript.Reference{Kind: "page", PageEnd: 4}},
	}
	got := sourceMetrics("pdf", segments)

	if got.ExtractionMethod != "pdf-text" || got.Words != 4 || got.Characters != 23 {
		t.Fatalf("unexpected text metrics: %#v", got)
	}
	if got.DurationSeconds != 75 || got.PhysicalPages != 4 {
		t.Fatalf("unexpected source extent: %#v", got)
	}
}

func TestSourceMetricsAreStableAcrossSegmentBoundaries(t *testing.T) {
	one := sourceMetrics("youtube", []transcript.Segment{{Text: "one two three"}})
	many := sourceMetrics("youtube", []transcript.Segment{{Text: "one two"}, {Text: "three"}})

	if one.Characters != many.Characters || one.Words != many.Words {
		t.Fatalf("segment boundaries changed metrics: one=%#v many=%#v", one, many)
	}
}
