package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alessio/tutorio/internal/source"
)

type fakeRunner struct {
	output string
	err    error
}

func (r fakeRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return []byte(r.output), r.err
}

func TestFetchPreservesPDFPageNumbers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handbook.pdf")
	if err := os.WriteFile(path, []byte("pdf fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := New("pdftotext", fakeRunner{output: "Heading\n\nFirst page text\fSecond page\f"}).Fetch(context.Background(), source.Request{Type: "pdf", URI: path})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "handbook" || len(doc.Cues) != 3 || doc.Cues[2].Reference.PageStart != 2 || doc.Cues[0].ChunkID == "" || doc.Cues[0].Sequence != 0 || doc.SourceID == path {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if doc.Cues[0].BoundaryKind != "heading" || doc.Cues[0].TitleHint != "Heading" {
		t.Fatalf("heading structure was not preserved: %#v", doc.Cues[0])
	}
}

func TestFetchRejectsImageOnlyPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scanned.pdf")
	if writeErr := os.WriteFile(path, []byte("image fixture"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err := New("pdftotext", fakeRunner{output: " \f \f"}).Fetch(context.Background(), source.Request{Type: "pdf", URI: path})
	if err == nil || !strings.Contains(err.Error(), "require OCR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChunkIdentityDoesNotDependOnExtractionSequence(t *testing.T) {
	first := sourceChunkID("fingerprint", 7, "  Exact   source text ")
	second := sourceChunkID("fingerprint", 7, "Exact source text")
	if first != second {
		t.Fatalf("normalised chunk IDs differ: %s %s", first, second)
	}
}
