//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/alessio/tutorio/internal/config"
	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/source/youtube"
	"github.com/alessio/tutorio/internal/storage/sqlite"
	"github.com/alessio/tutorio/internal/transcript"
)

type testProgress struct{ t *testing.T }

func (p testProgress) Report(_ context.Context, value jobs.Progress) { p.t.Log(value.Message) }

func TestCompileYouTube(t *testing.T) {
	uri := os.Getenv("TUTORIO_TEST_URL")
	if uri == "" {
		t.Skip("set TUTORIO_TEST_URL to run the local end-to-end test")
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider := llm.NewOllama(http.DefaultClient, cfg.Ollama.BaseURL, cfg.Ollama.Model)
	repository := sqlite.NewGuideRepository(db)
	sources := source.NewRegistry(youtube.New(cfg.Tools.YTDLPPath, youtube.OSCommandRunner{}))
	pipeline := jobs.NewPipeline(sources, transcript.NewCleaner(), transcript.NewSegmenter(cfg.Processing.SegmentCharacters), guide.NewLLMGenerator(provider, cfg.Ollama.MaxOutputTokens, cfg.Ollama.ContextWindow), guide.NewStructuralVerifier(), repository, logger, testProgress{t})
	result, err := pipeline.Run(context.Background(), source.Request{Type: "youtube", URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) == 0 {
		t.Fatal("compiled guide has no steps")
	}
	t.Logf("compiled %q with %d steps", result.Title, len(result.Steps))
}
