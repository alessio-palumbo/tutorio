// Package ui exposes application use cases to Wails bindings.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/alessio/tutorio/internal/evidence"
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
	evidence evidence.Repository
}

func NewApp(pipeline *jobs.Pipeline, guides guide.Repository, evidenceRepository evidence.Repository, logger *slog.Logger, output exporter.Exporter, manager *jobs.Manager, progress ...*EventReporter) *App {
	app := &App{pipeline: pipeline, guides: guides, evidence: evidenceRepository, logger: logger, exporter: output, manager: manager}
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
func (a *App) SelectAndQueueFile() (jobs.Job, error) {
	if a.manager == nil {
		return jobs.Job{}, fmt.Errorf("background job manager is not configured")
	}
	path, err := runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
		Title: "Import source file",
		Filters: []runtime.FileFilter{
			{DisplayName: "Supported sources", Pattern: "*.pdf;*.txt;*.srt;*.vtt"},
			{DisplayName: "PDF documents", Pattern: "*.pdf"},
			{DisplayName: "Transcript files", Pattern: "*.txt;*.srt;*.vtt"},
		},
	})
	if err != nil || path == "" {
		return jobs.Job{}, err
	}
	sourceType := "transcript_file"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".srt", ".vtt":
	case ".pdf":
		sourceType = "pdf"
	default:
		return jobs.Job{}, fmt.Errorf("unsupported source file %q", filepath.Ext(path))
	}
	return a.manager.Enqueue(a.context(), source.Request{Type: sourceType, URI: path})
}
func (a *App) ListGuides() ([]guide.Summary, error)    { return a.guides.List(a.context(), 100) }
func (a *App) GetGuide(id string) (guide.Guide, error) { return a.guides.Get(a.context(), id) }
func (a *App) DeleteGuide(id string) (bool, error) {
	value, err := a.guides.Get(a.context(), id)
	if err != nil {
		return false, err
	}
	choice, err := runtime.MessageDialog(a.context(), runtime.MessageDialogOptions{
		Type:          runtime.WarningDialog,
		Title:         "Delete guide?",
		Message:       fmt.Sprintf("Delete “%s”?\n\nThis permanently removes the guide and its saved compilation data.", value.Title),
		Buttons:       []string{"Delete", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil || choice != "Delete" {
		return false, err
	}
	return true, a.guides.Delete(a.context(), id)
}
func (a *App) SaveGuide(value guide.Guide) (guide.Guide, error) {
	return a.pipeline.SaveGuide(a.context(), value)
}
func (a *App) ListGuideSections(id string) ([]jobs.Segment, error) {
	return a.pipeline.Sections(a.context(), id)
}
func (a *App) RegenerateSection(id string, index int) (guide.Guide, error) {
	return a.pipeline.RegenerateSection(a.context(), id, index)
}
func (a *App) ConfirmRegenerateSection(id string, index int) (bool, error) {
	value, err := a.guides.Get(a.context(), id)
	if err != nil {
		return false, err
	}
	hasEdits := false
	for _, step := range value.Steps {
		if step.SourceSegment == index && step.UserEdited {
			hasEdits = true
			break
		}
	}
	message := fmt.Sprintf("Regenerate section %d with Ollama?", index+1)
	if hasEdits {
		message += "\n\nThis section contains manual edits, which will be replaced. Edits in other sections will be preserved."
	}
	choice, err := runtime.MessageDialog(a.context(), runtime.MessageDialogOptions{Type: runtime.WarningDialog, Title: "Regenerate section?", Message: message, Buttons: []string{"Regenerate", "Cancel"}, DefaultButton: "Cancel", CancelButton: "Cancel"})
	return choice == "Regenerate", err
}
func (a *App) DelveSection(id string, index int) (guide.Guide, error) {
	return a.pipeline.DelveSection(a.context(), id, index)
}
func (a *App) ListJobs() ([]jobs.Job, error) { return a.pipeline.Jobs(a.context()) }
func (a *App) GetJobSections(id string) ([]jobs.Segment, error) {
	return a.pipeline.JobSections(a.context(), id)
}
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
func (a *App) GetCitationEvidence(guideID, citationID string) (evidence.Evidence, error) {
	value, err := a.guides.Get(a.context(), guideID)
	if err != nil {
		return evidence.Evidence{}, err
	}
	citation, ok := findCitation(value, citationID)
	if !ok {
		return evidence.Evidence{}, fmt.Errorf("citation does not belong to this guide")
	}
	if a.evidence == nil {
		return evidence.Evidence{}, fmt.Errorf("evidence repository is not configured")
	}
	result, err := a.evidence.GetEvidence(a.context(), citation.EvidenceID)
	if err != nil {
		return evidence.Evidence{}, fmt.Errorf("load citation evidence: %w", err)
	}
	if result.SourceID != value.SourceID {
		return evidence.Evidence{}, fmt.Errorf("citation source does not belong to this guide")
	}
	return result, nil
}

func (a *App) OpenCitationSource(guideID, citationID string) error {
	result, err := a.GetCitationEvidence(guideID, citationID)
	if err != nil {
		return err
	}
	return a.openRegisteredSource(result.Source, result.Chunk.Location.PhysicalPage)
}

func (a *App) OpenGuideSource(guideID string, page int) error {
	value, err := a.guides.Get(a.context(), guideID)
	if err != nil {
		return err
	}
	if value.SourceType != "pdf" {
		return fmt.Errorf("guide source is not a PDF")
	}
	if a.evidence != nil {
		if registered, sourceErr := a.evidence.GetSource(a.context(), value.SourceID); sourceErr == nil {
			return a.openRegisteredSource(registered, page)
		}
	}
	// Legacy guides predate registered sources. Their stored server-side locator
	// remains a safe compatibility fallback; the frontend never supplies it.
	return a.openRegisteredSource(evidence.Source{ID: value.SourceID, Kind: "pdf", Locator: value.SourceURI, Title: value.Title}, page)
}

func findCitation(value guide.Guide, citationID string) (guide.Citation, bool) {
	for _, step := range value.Steps {
		for _, citation := range step.Citations {
			if citation.ID == citationID {
				return citation, true
			}
		}
	}
	return guide.Citation{}, false
}

func (a *App) openRegisteredSource(source evidence.Source, page int) error {
	if source.Kind != "pdf" {
		return fmt.Errorf("source kind %q cannot be opened as a PDF", source.Kind)
	}
	path := filepath.Clean(source.Locator)
	if path == "." || !filepath.IsAbs(path) {
		return fmt.Errorf("registered source path is invalid")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	name, args, err := sourceOpenCommand(goruntime.GOOS, path)
	if err != nil {
		return err
	}
	command := exec.CommandContext(a.context(), name, args...)
	if err = command.Start(); err != nil {
		return fmt.Errorf("open source file at page %d: %w", page, err)
	}
	go func() { _ = command.Wait() }()
	return nil
}

func sourceOpenCommand(goos, path string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{path}, nil
	case "linux":
		return "xdg-open", []string{path}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", path}, nil
	default:
		return "", nil, fmt.Errorf("opening source files is not supported on %s", goos)
	}
}
func (a *App) context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}
