package transcript

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFileParserVTT(t *testing.T) {
	input := "WEBVTT\n\n00:00:01.000 --> 00:00:03.500\nOpen the terminal.\n\n00:03.500 --> 00:05.000\nRun the command.\n"
	doc, err := NewFileParser().Parse(context.Background(), "lesson.vtt", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cues) != 2 {
		t.Fatalf("got %d cues, want 2", len(doc.Cues))
	}
	if doc.Cues[0].Start != time.Second {
		t.Fatalf("got start %s", doc.Cues[0].Start)
	}
}

func TestCleanerRemovesMarkupAndDuplicates(t *testing.T) {
	doc := Document{Cues: []Cue{{Text: "<c>Hello</c>   world"}, {Text: "Hello world"}}}
	got, err := NewCleaner().Clean(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cues) != 1 || got.Cues[0].Text != "Hello world" {
		t.Fatalf("unexpected cues: %#v", got.Cues)
	}
}

func TestCleanerCarriesBoundaryPastDuplicateCue(t *testing.T) {
	doc := Document{Cues: []Cue{
		{Text: "Repeated words"},
		{Text: "Repeated words", BoundaryKind: "chapter", TitleHint: "New chapter"},
		{Text: "New material"},
	}}
	got, err := NewCleaner().Clean(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cues) != 2 || got.Cues[1].BoundaryKind != "chapter" || got.Cues[1].TitleHint != "New chapter" {
		t.Fatalf("boundary was not retained: %#v", got.Cues)
	}
}

func TestSegmenterHandlesEmptyDocument(t *testing.T) {
	got, err := NewSegmenter(10).Segment(context.Background(), Document{})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestSegmenterHardSplitsOversizedCue(t *testing.T) {
	doc := Document{Cues: []Cue{{Start: time.Second, End: 11 * time.Second, Text: strings.Repeat("word ", 30)}}}
	got, err := NewSegmenter(40).Segment(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("got %d segments", len(got))
	}
	for _, segment := range got {
		if len(segment.Text) > 40 {
			t.Fatalf("segment exceeds limit: %d", len(segment.Text))
		}
	}
	if got[0].Start != time.Second || got[len(got)-1].End != 11*time.Second {
		t.Fatalf("timestamps not preserved: %#v", got)
	}
}

func TestSegmenterPreservesSourceChunks(t *testing.T) {
	doc := Document{Cues: []Cue{
		{Text: "First extracted unit", ChunkID: "chunk_one", ChunkKind: "text", Sequence: 0, Reference: Reference{Kind: "page", PageStart: 3, PageEnd: 3}},
		{Text: "Second extracted unit", ChunkID: "chunk_two", ChunkKind: "text", Sequence: 1, Reference: Reference{Kind: "page", PageStart: 4, PageEnd: 4}},
	}}
	got, err := NewSegmenter(1000).Segment(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Chunks) != 2 || got[0].Chunks[1].ID != "chunk_two" || got[0].Reference.PageEnd != 4 {
		t.Fatalf("unexpected chunked segment: %#v", got)
	}
}

func TestSegmenterPrefersExplicitChapterBoundaries(t *testing.T) {
	doc := Document{Cues: []Cue{
		{Text: "Short introduction", End: 5 * time.Second, BoundaryKind: "chapter", TitleHint: "Introduction"},
		{Text: "Still introducing", Start: 5 * time.Second, End: 10 * time.Second},
		{Text: "First practical topic", Start: 10 * time.Second, End: 15 * time.Second, BoundaryKind: "chapter", TitleHint: "Practical topic"},
	}}
	got, err := NewSegmenter(1000).Segment(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TitleHint != "Introduction" || got[1].TitleHint != "Practical topic" {
		t.Fatalf("unexpected chapter segments: %#v", got)
	}
}

func TestSegmenterUsesHeadingAndLongPauseAsSoftBoundaries(t *testing.T) {
	doc := Document{Cues: []Cue{
		{Text: strings.Repeat("a", 40), End: 5 * time.Second},
		{Text: "Document heading", Start: 5 * time.Second, End: 6 * time.Second, BoundaryKind: "heading", TitleHint: "Document heading"},
		{Text: strings.Repeat("b", 40), Start: 6 * time.Second, End: 10 * time.Second},
		{Text: "After a pause", Start: 20 * time.Second, End: 22 * time.Second},
	}}
	got, err := NewSegmenter(100).Segment(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[1].TitleHint != "Document heading" || got[2].Text != "After a pause" {
		t.Fatalf("unexpected soft-boundary segments: %#v", got)
	}
}

func TestSegmenterLimitCountsUnicodeCharacters(t *testing.T) {
	doc := Document{Cues: []Cue{{Text: "世界世界世界", End: 6 * time.Second}}}
	got, err := NewSegmenter(4).Segment(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	for _, segment := range got {
		if len([]rune(segment.Text)) > 4 {
			t.Fatalf("segment exceeds rune limit: %q", segment.Text)
		}
	}
}
