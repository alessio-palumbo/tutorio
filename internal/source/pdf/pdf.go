// Package pdf implements local, text-based PDF ingestion through Poppler.
package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/transcript"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, message)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

type Source struct {
	binary string
	runner CommandRunner
}

func New(binary string, runner CommandRunner) *Source {
	if strings.TrimSpace(binary) == "" {
		binary = "pdftotext"
	}
	return &Source{binary: binary, runner: runner}
}

func (*Source) Type() string { return "pdf" }

func (s *Source) Fetch(ctx context.Context, request source.Request) (transcript.Document, error) {
	output, err := s.runner.Run(ctx, s.binary, "-layout", "-enc", "UTF-8", request.URI, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return transcript.Document{}, fmt.Errorf("%s was not found; install Poppler or configure tools.pdftotext_path: %w", s.binary, err)
		}
		return transcript.Document{}, fmt.Errorf("extract PDF text with %s: %w", s.binary, err)
	}
	pages := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\f")
	cues := make([]transcript.Cue, 0, len(pages))
	for index, page := range pages {
		if err = ctx.Err(); err != nil {
			return transcript.Document{}, err
		}
		page = strings.TrimSpace(page)
		if page == "" {
			continue
		}
		pageNumber := index + 1
		cues = append(cues, transcript.Cue{Text: page, Reference: transcript.Reference{Kind: "page", PageStart: pageNumber, PageEnd: pageNumber, Label: fmt.Sprintf("Page %d", pageNumber)}})
	}
	if len(cues) == 0 {
		return transcript.Document{}, fmt.Errorf("PDF contains no extractable text; scanned or image-only PDFs require OCR, which is not implemented yet")
	}
	title := strings.TrimSuffix(filepath.Base(request.URI), filepath.Ext(request.URI))
	if request.Title != "" {
		title = request.Title
	}
	return transcript.Document{SourceID: request.URI, Title: title, Cues: cues}, nil
}
