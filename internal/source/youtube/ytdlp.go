// Package youtube implements YouTube transcript ingestion through yt-dlp.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, message)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

type YTDLP struct {
	binary string
	runner CommandRunner
	parser transcript.FileParser
}

func New(binary string, runner CommandRunner) *YTDLP {
	if binary == "" {
		binary = "yt-dlp"
	}
	return &YTDLP{binary: binary, runner: runner, parser: transcript.NewFileParser()}
}
func (*YTDLP) Type() string { return "youtube" }
func (y *YTDLP) Fetch(ctx context.Context, req source.Request) (transcript.Document, error) {
	dir, err := os.MkdirTemp("", "tutorio-youtube-")
	if err != nil {
		return transcript.Document{}, err
	}
	defer os.RemoveAll(dir)
	metadata, err := y.runner.Run(ctx, y.binary, "--dump-single-json", "--skip-download", req.URI)
	if err != nil {
		return transcript.Document{}, fmt.Errorf("read YouTube metadata: %w: %s", err, strings.TrimSpace(string(metadata)))
	}
	var meta struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return transcript.Document{}, fmt.Errorf("decode metadata: %w", err)
	}
	output := filepath.Join(dir, "transcript")
	result, err := y.runner.Run(ctx, y.binary, "--skip-download", "--write-subs", "--write-auto-subs", "--sub-langs", "en.*,en", "--sub-format", "vtt", "-o", output, req.URI)
	if err != nil {
		return transcript.Document{}, fmt.Errorf("retrieve transcript: %w: %s", err, strings.TrimSpace(string(result)))
	}
	files, err := filepath.Glob(output + "*.vtt")
	if err != nil || len(files) == 0 {
		return transcript.Document{}, fmt.Errorf("no supported transcript found for %s", req.URI)
	}
	f, err := os.Open(files[0])
	if err != nil {
		return transcript.Document{}, err
	}
	defer f.Close()
	doc, err := y.parser.Parse(ctx, meta.ID, f)
	doc.Title = meta.Title
	return doc, err
}
