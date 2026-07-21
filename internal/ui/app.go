// Package ui exposes application use cases to Wails bindings.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/source"
)

type App struct {
	ctx      context.Context
	pipeline *jobs.Pipeline
	guides   guide.Repository
	logger   *slog.Logger
	progress *EventReporter
}

func NewApp(pipeline *jobs.Pipeline, guides guide.Repository, logger *slog.Logger, progress ...*EventReporter) *App {
	app := &App{pipeline: pipeline, guides: guides, logger: logger}
	if len(progress) > 0 {
		app.progress = progress[0]
	}
	return app
}
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if a.progress != nil {
		a.progress.Attach(ctx)
	}
	a.logger.InfoContext(ctx, "tutorio started")
}
func (a *App) CompileYouTube(uri string) (guide.Guide, error) {
	if !strings.HasPrefix(uri, "https://") && !strings.HasPrefix(uri, "http://") {
		return guide.Guide{}, fmt.Errorf("enter a valid YouTube URL")
	}
	return a.pipeline.Run(a.context(), source.Request{Type: "youtube", URI: uri})
}
func (a *App) ImportTranscript(path string) (guide.Guide, error) {
	if strings.TrimSpace(path) == "" {
		return guide.Guide{}, fmt.Errorf("transcript path is required")
	}
	return a.pipeline.Run(a.context(), source.Request{Type: "transcript_file", URI: path})
}
func (a *App) ListGuides() ([]guide.Summary, error)    { return a.guides.List(a.context(), 100) }
func (a *App) GetGuide(id string) (guide.Guide, error) { return a.guides.Get(a.context(), id) }
func (a *App) SaveGuide(value guide.Guide) (guide.Guide, error) {
	return a.pipeline.SaveGuide(a.context(), value)
}
func (a *App) ListGuideSections(id string) ([]jobs.Segment, error) {
	return a.pipeline.Sections(a.context(), id)
}
func (a *App) RegenerateSection(id string, index int) (guide.Guide, error) {
	return a.pipeline.RegenerateSection(a.context(), id, index)
}
func (a *App) context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}
