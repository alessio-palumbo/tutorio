// Package pdf implements local, text-based PDF ingestion through Poppler.
package pdf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

const extractorVersion = "pdftotext-layout-v1"

var chunkBoundary = regexp.MustCompile(`\n[\t ]*\n+`)

func New(binary string, runner CommandRunner) *Source {
	if strings.TrimSpace(binary) == "" {
		binary = "pdftotext"
	}
	return &Source{binary: binary, runner: runner}
}

func (*Source) Type() string { return "pdf" }

func (s *Source) Fetch(ctx context.Context, request source.Request) (transcript.Document, error) {
	fingerprint, err := fingerprintFile(request.URI)
	if err != nil {
		return transcript.Document{}, fmt.Errorf("fingerprint PDF: %w", err)
	}
	output, err := s.runner.Run(ctx, s.binary, "-layout", "-enc", "UTF-8", request.URI, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return transcript.Document{}, fmt.Errorf("%s was not found; install Poppler or configure tools.pdftotext_path: %w", s.binary, err)
		}
		return transcript.Document{}, fmt.Errorf("extract PDF text with %s: %w", s.binary, err)
	}
	pages := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\f")
	cues := make([]transcript.Cue, 0, len(pages)*3)
	sequence := 0
	for index, page := range pages {
		if err = ctx.Err(); err != nil {
			return transcript.Document{}, err
		}
		pageNumber := index + 1
		for _, text := range splitSourceChunks(page, 4000) {
			id := sourceChunkID(fingerprint, pageNumber, text)
			cues = append(cues, transcript.Cue{Text: text, Reference: transcript.Reference{Kind: "page", PageStart: pageNumber, PageEnd: pageNumber, Label: fmt.Sprintf("PDF page %d", pageNumber)}, ChunkID: id, ChunkKind: "text", Sequence: sequence})
			sequence++
		}
	}
	if len(cues) == 0 {
		return transcript.Document{}, fmt.Errorf("PDF contains no extractable text; scanned or image-only PDFs require OCR, which is not implemented yet")
	}
	title := strings.TrimSuffix(filepath.Base(request.URI), filepath.Ext(request.URI))
	if request.Title != "" {
		title = request.Title
	}
	return transcript.Document{SourceID: "src_" + fingerprint, SourceKind: "pdf", SourceURI: request.URI, Fingerprint: fingerprint, Extractor: extractorVersion, Title: title, Cues: cues}, nil
}

func fingerprintFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func sourceChunkID(fingerprint string, physicalPage int, text string) string {
	identity := fingerprint + "\x00" + extractorVersion + "\x00" + strconv.Itoa(physicalPage) + "\x00text\x00" + normalizeChunkText(text)
	return fmt.Sprintf("chk_%x", sha256.Sum256([]byte(identity)))
}

func splitSourceChunks(page string, limit int) []string {
	page = strings.ReplaceAll(page, "\r\n", "\n")
	blocks := chunkBoundary.Split(strings.TrimSpace(page), -1)
	result := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = normalizeChunkText(block)
		for len(block) > limit {
			cut := strings.LastIndexAny(block[:limit], " \t\n")
			if cut < limit/2 {
				cut = limit
			}
			result = append(result, strings.TrimSpace(block[:cut]))
			block = strings.TrimSpace(block[cut:])
		}
		if block != "" {
			result = append(result, block)
		}
	}
	return result
}

func normalizeChunkText(value string) string { return strings.Join(strings.Fields(value), " ") }
