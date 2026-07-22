package pdf

import (
	"context"
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
	doc, err := New("pdftotext", fakeRunner{output: "First page\fSecond page\f"}).Fetch(context.Background(), source.Request{Type: "pdf", URI: "/tmp/handbook.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "handbook" || len(doc.Cues) != 2 || doc.Cues[1].Reference.PageStart != 2 {
		t.Fatalf("unexpected document: %#v", doc)
	}
}

func TestFetchRejectsImageOnlyPDF(t *testing.T) {
	_, err := New("pdftotext", fakeRunner{output: " \f \f"}).Fetch(context.Background(), source.Request{Type: "pdf", URI: "/tmp/scanned.pdf"})
	if err == nil || !strings.Contains(err.Error(), "require OCR") {
		t.Fatalf("unexpected error: %v", err)
	}
}
