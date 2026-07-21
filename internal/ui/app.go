// Package ui exposes application use cases to Wails bindings.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/alessio/tutorio/internal/exporter"
	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/jobs"
	"github.com/alessio/tutorio/internal/source"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	pipeline *jobs.Pipeline
	guides   guide.Repository
	logger   *slog.Logger
	progress *EventReporter
	exporter exporter.Exporter
	manager  *jobs.Manager
}

func NewApp(pipeline *jobs.Pipeline, guides guide.Repository, logger *slog.Logger, output exporter.Exporter, manager *jobs.Manager, progress ...*EventReporter) *App {
	app := &App{pipeline: pipeline, guides: guides, logger: logger, exporter: output, manager: manager}
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
	if a.manager != nil {
		a.manager.Start(ctx)
	}
	a.logger.InfoContext(ctx, "tutorio started")
}
func (a *App) Shutdown(context.Context) {
	if a.manager != nil {
		a.manager.Stop()
	}
}
func (a *App) CompileYouTube(uri string) (guide.Guide, error) {
	if !strings.HasPrefix(uri, "https://") && !strings.HasPrefix(uri, "http://") {
		return guide.Guide{}, fmt.Errorf("enter a valid YouTube URL")
	}
	return a.pipeline.Run(a.context(), source.Request{Type: "youtube", URI: uri})
}
func (a *App) QueueYouTube(uri string) (jobs.Job, error) {
	if !strings.HasPrefix(uri, "https://") && !strings.HasPrefix(uri, "http://") {
		return jobs.Job{}, fmt.Errorf("enter a valid YouTube URL")
	}
	if a.manager == nil {
		return jobs.Job{}, fmt.Errorf("background job manager is not configured")
	}
	return a.manager.Enqueue(a.context(), source.Request{Type: "youtube", URI: uri})
}
func (a *App) ImportTranscript(path string) (guide.Guide, error) {
	if strings.TrimSpace(path) == "" {
		return guide.Guide{}, fmt.Errorf("transcript path is required")
	}
	return a.pipeline.Run(a.context(), source.Request{Type: "transcript_file", URI: path})
}
func (a *App) ListGuides() ([]guide.Summary, error)    { return a.guides.List(a.context(), 100) }
func (a *App) GetGuide(id string) (guide.Guide, error) { return a.guides.Get(a.context(), id) }
func (a *App) DeleteGuide(id string) error             { return a.guides.Delete(a.context(), id) }
func (a *App) SaveGuide(value guide.Guide) (guide.Guide, error) {
	return a.pipeline.SaveGuide(a.context(), value)
}
func (a *App) ListGuideSections(id string) ([]jobs.Segment, error) {
	return a.pipeline.Sections(a.context(), id)
}
func (a *App) RegenerateSection(id string, index int) (guide.Guide, error) {
	return a.pipeline.RegenerateSection(a.context(), id, index)
}
func (a *App) ListJobs() ([]jobs.Job, error) { return a.pipeline.Jobs(a.context()) }
func (a *App) RetryJob(id string) (jobs.Job, error) {
	if a.manager == nil {
		return jobs.Job{}, fmt.Errorf("background job manager is not configured")
	}
	return a.manager.Retry(a.context(), id)
}
func (a *App) CancelJob(id string) error {
	if a.manager == nil {
		return fmt.Errorf("background job manager is not configured")
	}
	return a.manager.Cancel(a.context(), id)
}
func (a *App) ExportMarkdown(id string) (string, error) {
	if a.exporter == nil {
		return "", fmt.Errorf("Markdown exporter is not configured")
	}
	value, err := a.guides.Get(a.context(), id)
	if err != nil {
		return "", err
	}
	content, err := a.exporter.Render(a.context(), value)
	if err != nil {
		return "", err
	}
	name := regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(strings.ToLower(value.Title), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "tutorio-guide"
	}
	path, err := runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{DefaultFilename: name + a.exporter.Extension(), Filters: []runtime.FileFilter{{DisplayName: "Markdown guide", Pattern: "*.md"}}})
	if err != nil || path == "" {
		return path, err
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
func (a *App) context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}
