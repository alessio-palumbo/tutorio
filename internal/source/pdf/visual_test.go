package pdf

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alessio/tutorio/internal/evidence"
)

type visualRunner struct {
	name string
	args []string
	data []byte
	err  error
}

func (r *visualRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name, r.args = name, args
	return r.data, r.err
}

func TestPageRendererReturnsLocalPNGData(t *testing.T) {
	runner := &visualRunner{data: []byte("\x89PNG\r\n")}
	got, err := NewPageRenderer("pdftocairo", runner).Render(context.Background(), evidence.Source{ID: "source-1", Kind: "pdf", Locator: "/private/book.pdf"}, 17)
	if err != nil {
		t.Fatal(err)
	}
	if got.PhysicalPage != 17 || got.SourceID != "source-1" || got.MediaType != "image/png" || !strings.HasPrefix(got.DataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected visual: %#v", got)
	}
	if runner.name != "pdftocairo" || !containsArguments(runner.args, "-f", "17", "/private/book.pdf", "-") {
		t.Fatalf("unexpected renderer invocation: %q %#v", runner.name, runner.args)
	}
}

func TestPageRendererRejectsInvalidRequests(t *testing.T) {
	renderer := NewPageRenderer("pdftocairo", &visualRunner{err: errors.New("should not run")})
	if _, err := renderer.Render(context.Background(), evidence.Source{Kind: "video"}, 1); err == nil {
		t.Fatal("expected non-PDF source to fail")
	}
	if _, err := renderer.Render(context.Background(), evidence.Source{Kind: "pdf"}, 0); err == nil {
		t.Fatal("expected invalid page to fail")
	}
}

func containsArguments(values []string, expected ...string) bool {
	for _, item := range expected {
		found := false
		for _, value := range values {
			if value == item {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
