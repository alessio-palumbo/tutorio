package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alessio/tutorio/internal/config"
	"github.com/alessio/tutorio/internal/exporter/markdown"
	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/llm"
	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/source/local"
	"github.com/alessio/tutorio/internal/source/pdf"
	"github.com/alessio/tutorio/internal/source/youtube"
	"github.com/alessio/tutorio/internal/storage/sqlite"
	"github.com/alessio/tutorio/internal/transcript"
	appui "github.com/alessio/tutorio/internal/ui"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// Wails v2 requires the application entrypoint at the project root.
//
//go:embed all:cmd/app/frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	configPath := config.Path()
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load configuration", "error", err)
		return
	}
	logger.Info("configuration loaded", "path", configPath, "ollama_model", cfg.Ollama.Model, "max_output_tokens", cfg.Ollama.MaxOutputTokens, "context_window", cfg.Ollama.ContextWindow)
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		logger.Error("create data directory", "error", err)
		return
	}
	db, err := sqlite.Open(context.Background(), cfg.Database.Path)
	if err != nil {
		logger.Error("open database", "error", err)
		return
	}
	defer db.Close()

	provider := llm.NewOllama(http.DefaultClient, cfg.Ollama.BaseURL, cfg.Ollama.Model)
	repository := sqlite.NewGuideRepository(db)
	jobStore := sqlite.NewJobStore(db)
	evidenceRepository := sqlite.NewEvidenceRepository(db)
	sources := source.NewRegistry(
		youtube.New(cfg.Tools.YTDLPPath, youtube.OSCommandRunner{}),
		local.NewTranscriptFile(transcript.NewFileParser()),
		pdf.New(cfg.Tools.PDFToTextPath, pdf.OSCommandRunner{}),
	)
	progress := appui.NewEventReporter()
	modelGenerator := guide.NewLLMGenerator(provider, cfg.Ollama.MaxOutputTokens, cfg.Ollama.ContextWindow)
	pipeline := jobs.NewPipeline(sources, transcript.NewCleaner(), transcript.NewSegmenter(cfg.Processing.SegmentCharacters), modelGenerator, guide.NewStructuralVerifier(), repository, logger, progress).WithStore(jobStore).WithExpander(modelGenerator).WithOverviewSynthesizer(modelGenerator).WithEvidenceRepository(evidenceRepository)
	manager := jobs.NewManager(pipeline, jobStore, logger)
	app := appui.NewApp(pipeline, repository, evidenceRepository, logger, markdown.New(), manager, progress).
		WithVisualProvider(pdf.NewPageRenderer(cfg.Tools.PDFToCairoPath, pdf.OSCommandRunner{}))

	err = wails.Run(&options.App{
		Title: "tutorio", Width: 1200, Height: 800,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 246, G: 244, B: 238, A: 1},
		Mac:              &mac.Options{About: &mac.AboutInfo{Title: "tutorio", Icon: appIcon}},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             []interface{}{app},
	})
	if err != nil {
		logger.Error("run desktop application", "error", err)
	}
}
