package youtube

import (
	"context"
	"os"
	"testing"

	"github.com/alessio/tutorio/internal/source"
)

type subtitleRunner struct{ calls int }

func (r *subtitleRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls++
	if r.calls == 1 {
		return []byte(`{"id":"video-1","title":"Timed lesson","chapters":[{"title":"Practical setup","start_time":60}]}`), nil
	}
	for index, arg := range args {
		if arg == "-o" && index+1 < len(args) {
			content := "WEBVTT\n\n00:01:02.000 --> 00:01:04.000\nDo the timed thing.\n"
			return nil, os.WriteFile(args[index+1]+".en.vtt", []byte(content), 0o600)
		}
	}
	return nil, nil
}

func TestFetchParsesDownloadedVTTAsTimedTranscript(t *testing.T) {
	runner := &subtitleRunner{}
	doc, err := New("yt-dlp", runner).Fetch(context.Background(), source.Request{Type: "youtube", URI: "https://example.test/video"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceID != "video-1" || len(doc.Cues) != 1 || doc.Cues[0].Start.Seconds() != 62 {
		t.Fatalf("unexpected timed document: %#v", doc)
	}
	if doc.Cues[0].BoundaryKind != "chapter" || doc.Cues[0].TitleHint != "Practical setup" {
		t.Fatalf("chapter metadata was not applied: %#v", doc.Cues[0])
	}
}
