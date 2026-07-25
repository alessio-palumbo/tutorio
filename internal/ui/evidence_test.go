package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/alessio/tutorio/internal/evidence"
	"github.com/alessio/tutorio/internal/guide"
)

type guideRepositoryStub struct{ value guide.Guide }

func (r guideRepositoryStub) Save(context.Context, guide.Guide) (guide.Guide, error) {
	return guide.Guide{}, errors.New("not implemented")
}
func (r guideRepositoryStub) Get(context.Context, string) (guide.Guide, error) { return r.value, nil }
func (r guideRepositoryStub) List(context.Context, int) ([]guide.Summary, error) {
	return nil, errors.New("not implemented")
}
func (r guideRepositoryStub) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type evidenceRepositoryStub struct {
	value evidence.Evidence
	err   error
}

type visualProviderStub struct {
	source evidence.Source
	page   int
}

func (p *visualProviderStub) Render(_ context.Context, source evidence.Source, page int) (evidence.Visual, error) {
	p.source, p.page = source, page
	return evidence.Visual{Kind: evidence.EvidenceImage, SourceID: source.ID, PhysicalPage: page, MediaType: "image/png", DataURL: "data:image/png;base64,AA=="}, nil
}

func (r evidenceRepositoryStub) SaveSource(context.Context, evidence.Source) error {
	return errors.New("not implemented")
}
func (r evidenceRepositoryStub) SaveChunks(context.Context, []evidence.SourceChunk) error {
	return errors.New("not implemented")
}
func (r evidenceRepositoryStub) GetSource(context.Context, string) (evidence.Source, error) {
	return evidence.Source{}, errors.New("not implemented")
}
func (r evidenceRepositoryStub) GetEvidence(context.Context, string) (evidence.Evidence, error) {
	return r.value, r.err
}

func TestCitationEvidenceMustBelongToGuide(t *testing.T) {
	saved := guide.Guide{ID: "guide-1", SourceID: "source-1", Steps: []guide.Step{{Citations: []guide.Citation{{ID: "citation-1", EvidenceID: "evidence-1"}}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := NewApp(nil, guideRepositoryStub{value: saved}, evidenceRepositoryStub{value: evidence.Evidence{ID: "evidence-1", SourceID: "source-1", Chunk: evidence.SourceChunk{Text: "exact source"}}}, logger, nil, nil)

	got, err := app.GetCitationEvidence("guide-1", "citation-1")
	if err != nil || got.Chunk.Text != "exact source" {
		t.Fatalf("expected owned evidence, got %#v, %v", got, err)
	}
	if _, err = app.GetCitationEvidence("guide-1", "not-owned"); err == nil {
		t.Fatal("expected an unowned citation to be rejected")
	}
}

func TestCitationEvidenceRejectsDifferentSource(t *testing.T) {
	saved := guide.Guide{ID: "guide-1", SourceID: "source-1", Steps: []guide.Step{{Citations: []guide.Citation{{ID: "citation-1", EvidenceID: "evidence-1"}}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := NewApp(nil, guideRepositoryStub{value: saved}, evidenceRepositoryStub{value: evidence.Evidence{ID: "evidence-1", SourceID: "source-2"}}, logger, nil, nil)

	if _, err := app.GetCitationEvidence("guide-1", "citation-1"); err == nil {
		t.Fatal("expected evidence from a different source to be rejected")
	}
}

func TestCitationVisualUsesAuthorizedEvidenceLocation(t *testing.T) {
	saved := guide.Guide{ID: "guide-1", SourceID: "source-1", Steps: []guide.Step{{Citations: []guide.Citation{{ID: "citation-1", EvidenceID: "evidence-1"}}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	visuals := &visualProviderStub{}
	app := NewApp(nil, guideRepositoryStub{value: saved}, evidenceRepositoryStub{value: evidence.Evidence{
		ID: "evidence-1", SourceID: "source-1",
		Source: evidence.Source{ID: "source-1", Kind: "pdf", Locator: "/private/book.pdf"},
		Chunk:  evidence.SourceChunk{Location: evidence.SourceLocation{PhysicalPage: 17}},
	}}, logger, nil, nil).WithVisualProvider(visuals)

	got, err := app.GetCitationVisual("guide-1", "citation-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.PhysicalPage != 17 || visuals.page != 17 || visuals.source.Locator != "/private/book.pdf" {
		t.Fatalf("visual did not use authorized source location: %#v %#v", got, visuals)
	}
}
