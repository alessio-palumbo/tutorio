// Package jobs orchestrates application use cases without depending on adapters.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/transcript"
)

type Pipeline struct {
	sources    source.Resolver
	cleaner    transcript.Cleaner
	segmenter  transcript.Segmenter
	generator  guide.Generator
	verifier   guide.Verifier
	repository guide.Repository
	logger     *slog.Logger
	progress   ProgressReporter
	store      Store
}

func NewPipeline(s source.Resolver, c transcript.Cleaner, sg transcript.Segmenter, g guide.Generator, v guide.Verifier, r guide.Repository, l *slog.Logger, reporters ...ProgressReporter) *Pipeline {
	reporter := ProgressReporter(discardProgress{})
	if len(reporters) > 0 && reporters[0] != nil {
		reporter = reporters[0]
	}
	return &Pipeline{s, c, sg, g, v, r, l, reporter, discardStore{}}
}
func (p *Pipeline) WithStore(store Store) *Pipeline {
	if store != nil {
		p.store = store
	}
	return p
}
func (p *Pipeline) Run(ctx context.Context, request source.Request) (guide.Guide, error) {
	now := time.Now().UTC()
	job := Job{ID: fmt.Sprintf("job_%d", now.UnixNano()), SourceType: request.Type, SourceURI: request.URI, Status: StatusRunning, Stage: "source", CreatedAt: now, UpdatedAt: now}
	if err := p.store.Create(ctx, job); err != nil {
		return guide.Guide{}, fmt.Errorf("create job: %w", err)
	}
	logger := p.logger.With("source_type", request.Type, "source_uri", request.URI)
	p.report(ctx, "source", "Retrieving transcript…", 0, 0)
	adapter, err := p.sources.Resolve(request.Type)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, err)
	}
	doc, err := adapter.Fetch(ctx, request)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("source stage: %w", err))
	}
	logger.InfoContext(ctx, "source retrieved", "cues", len(doc.Cues))
	p.report(ctx, "cleaning", "Cleaning transcript…", 0, 0)
	p.updateJob(ctx, &job, "cleaning", 0, 0)
	doc, err = p.cleaner.Clean(ctx, doc)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("cleaning stage: %w", err))
	}
	p.report(ctx, "segmenting", "Splitting transcript into model-sized sections…", 0, 0)
	p.updateJob(ctx, &job, "segmenting", 0, 0)
	segments, err := p.segmenter.Segment(ctx, doc)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("segmentation stage: %w", err))
	}
	if err = p.store.SaveSegments(ctx, job.ID, segments); err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("store segments: %w", err))
	}
	logger.InfoContext(ctx, "transcript segmented", "segments", len(segments))
	p.report(ctx, "generating", "Generating guide sections…", 0, len(segments))
	p.updateJob(ctx, &job, "generating", 0, len(segments))
	generated, err := p.generator.Generate(ctx, guide.GenerateRequest{Title: doc.Title, SourceType: request.Type, SourceURI: request.URI, SourceID: doc.SourceID, Segments: segments, OnProgress: func(current, total int) {
		p.report(ctx, "generating", fmt.Sprintf("Generating section %d of %d…", current, total), current, total)
		p.updateJob(ctx, &job, "generating", current, total)
	}, OnSegment: func(result guide.SectionResult) {
		_ = p.store.CompleteSegment(ctx, Segment{JobID: job.ID, Index: result.Index, Transcript: result.Segment, Guide: result.Guide, Status: StatusCompleted, Model: result.Model, PromptTokens: result.PromptTokens, OutputTokens: result.OutputTokens, DurationMilliseconds: result.DurationMilliseconds})
	}})
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("generation stage: %w", err))
	}
	generated.SourceType = request.Type
	generated.SourceURI = request.URI
	generated.SourceID = doc.SourceID
	generated.Generation.JobID = job.ID
	if generated.Title == "" {
		generated.Title = doc.Title
	}
	p.report(ctx, "verifying", "Verifying guide structure…", 0, 0)
	if err = p.verifier.Verify(ctx, generated); err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("verification stage: %w", err))
	}
	p.report(ctx, "saving", "Saving guide to your library…", 0, 0)
	generated, err = p.repository.Save(ctx, generated)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, fmt.Errorf("storage stage: %w", err))
	}
	completed := time.Now().UTC()
	job.Status = StatusCompleted
	job.Stage = "complete"
	job.GuideID = generated.ID
	job.Current = len(segments)
	job.Total = len(segments)
	job.CompletedAt = &completed
	job.UpdatedAt = completed
	_ = p.store.Update(ctx, job)
	logger.InfoContext(ctx, "guide compiled", "title", generated.Title)
	p.report(ctx, "complete", "Guide compiled and saved.", 1, 1)
	return generated, nil
}

func (p *Pipeline) updateJob(ctx context.Context, job *Job, stage string, current, total int) {
	job.Stage = stage
	job.Current = current
	job.Total = total
	job.UpdatedAt = time.Now().UTC()
	_ = p.store.Update(ctx, *job)
}
func (p *Pipeline) fail(ctx context.Context, job *Job, err error) error {
	job.Status = StatusFailed
	job.Error = err.Error()
	job.UpdatedAt = time.Now().UTC()
	_ = p.store.Update(ctx, *job)
	return err
}

func (p *Pipeline) report(ctx context.Context, stage, message string, current, total int) {
	p.progress.Report(ctx, Progress{Stage: stage, Message: message, Current: current, Total: total})
}
