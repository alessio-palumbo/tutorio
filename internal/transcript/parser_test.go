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
