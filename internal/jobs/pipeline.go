// Package jobs orchestrates application use cases without depending on adapters.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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
	expander   guide.Expander
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
	return &Pipeline{sources: s, cleaner: c, segmenter: sg, generator: g, verifier: v, repository: r, logger: l, progress: reporter, store: discardStore{}}
}
func (p *Pipeline) WithExpander(expander guide.Expander) *Pipeline {
	p.expander = expander
	return p
}
func (p *Pipeline) WithStore(store Store) *Pipeline {
	if store != nil {
		p.store = store
	}
	return p
}
func (p *Pipeline) Run(ctx context.Context, request source.Request) (guide.Guide, error) {
	job, err := p.CreateJob(ctx, request)
	if err != nil {
		return guide.Guide{}, err
	}
	return p.RunJob(ctx, job, request)
}

// CreateJob persists a queued unit of work before expensive processing begins.
func (p *Pipeline) CreateJob(ctx context.Context, request source.Request) (Job, error) {
	now := time.Now().UTC()
	job := Job{ID: fmt.Sprintf("job_%d", now.UnixNano()), SourceType: request.Type, SourceURI: request.URI, Status: StatusPending, Stage: "queued", CreatedAt: now, UpdatedAt: now}
	if err := p.store.Create(ctx, job); err != nil {
		return Job{}, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

// RunJob executes a previously persisted job.
func (p *Pipeline) RunJob(ctx context.Context, job Job, request source.Request) (guide.Guide, error) {
	job.Status = StatusRunning
	job.Stage = "source"
	job.Error = ""
	job.UpdatedAt = time.Now().UTC()
	if err := p.store.Update(ctx, job); err != nil {
		return guide.Guide{}, fmt.Errorf("start job: %w", err)
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
		_ = p.store.CompleteSegment(ctx, segmentResult(job.ID, result, StatusCompleted))
	}, OnFailure: func(result guide.SectionResult) {
		_ = p.store.RecordSegmentFailure(ctx, segmentResult(job.ID, result, StatusFailed))
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
	if errors.Is(err, context.Canceled) {
		job.Status = StatusCancelled
		job.Stage = "cancelled"
		job.Error = ""
		job.UpdatedAt = time.Now().UTC()
		_ = p.store.Update(ctx, *job)
		p.report(ctx, "cancelled", "Compilation cancelled.", job.Current, job.Total)
		return err
	}
	job.Status = StatusFailed
	job.Error = err.Error()
	job.UpdatedAt = time.Now().UTC()
	_ = p.store.Update(ctx, *job)
	p.report(ctx, "failed", err.Error(), job.Current, job.Total)
	return err
}

func (p *Pipeline) report(ctx context.Context, stage, message string, current, total int) {
	p.progress.Report(ctx, Progress{Stage: stage, Message: message, Current: current, Total: total})
}

func (p *Pipeline) Jobs(ctx context.Context) ([]Job, error) { return p.store.List(ctx, 100) }

func (p *Pipeline) RetryJob(ctx context.Context, jobID string) (guide.Guide, error) {
	job, err := p.store.Get(ctx, jobID)
	if err != nil {
		return guide.Guide{}, err
	}
	sections, err := p.store.Segments(ctx, jobID)
	if err != nil {
		return guide.Guide{}, err
	}
	if len(sections) == 0 {
		return guide.Guide{}, fmt.Errorf("job has no persisted transcript sections")
	}
	job.Status = StatusRunning
	job.Stage = "generating"
	job.Error = ""
	job.UpdatedAt = time.Now().UTC()
	_ = p.store.Update(ctx, job)
	partials := make([]guide.Guide, 0, len(sections))
	metadata := guide.Generation{JobID: job.ID, SegmentCount: len(sections)}
	title := "Recovered tutorial"
	for index := range sections {
		section := &sections[index]
		if section.Status != StatusCompleted || len(section.Guide.Steps) == 0 {
			p.report(ctx, "generating", fmt.Sprintf("Retrying section %d of %d…", index+1, len(sections)), index, len(sections))
			generated, generationErr := p.generator.Generate(ctx, guide.GenerateRequest{Title: title, SourceType: job.SourceType, SourceURI: job.SourceURI, Segments: []transcript.Segment{section.Transcript}, OnFailure: func(result guide.SectionResult) {
				_ = p.store.RecordSegmentFailure(ctx, segmentResult(job.ID, result, StatusFailed))
			}})
			if generationErr != nil {
				return guide.Guide{}, p.fail(ctx, &job, generationErr)
			}
			section.Guide = generated
			section.Status = StatusCompleted
			section.Model = generated.Generation.Model
			section.PromptTokens = generated.Generation.PromptTokens
			section.OutputTokens = generated.Generation.OutputTokens
			section.DurationMilliseconds = generated.Generation.DurationMilliseconds
			if err = p.store.CompleteSegment(ctx, *section); err != nil {
				return guide.Guide{}, p.fail(ctx, &job, err)
			}
		}
		if section.Guide.Title != "" {
			title = section.Guide.Title
		}
		partials = append(partials, section.Guide)
		metadata.Model = section.Model
		metadata.PromptTokens += section.PromptTokens
		metadata.OutputTokens += section.OutputTokens
		metadata.DurationMilliseconds += section.DurationMilliseconds
	}
	result := guide.Merge(title, partials)
	result.SourceType = job.SourceType
	result.SourceURI = job.SourceURI
	result.Generation = metadata
	if err = p.verifier.Verify(ctx, result); err != nil {
		return guide.Guide{}, p.fail(ctx, &job, err)
	}
	result, err = p.repository.Save(ctx, result)
	if err != nil {
		return guide.Guide{}, p.fail(ctx, &job, err)
	}
	completed := time.Now().UTC()
	job.Status = StatusCompleted
	job.Stage = "complete"
	job.GuideID = result.ID
	job.Current = len(sections)
	job.Total = len(sections)
	job.CompletedAt = &completed
	job.UpdatedAt = completed
	_ = p.store.Update(ctx, job)
	p.report(ctx, "complete", "Recovered job completed and saved.", 1, 1)
	return result, nil
}

func (p *Pipeline) Sections(ctx context.Context, guideID string) ([]Segment, error) {
	stored, err := p.repository.Get(ctx, guideID)
	if err != nil {
		return nil, err
	}
	if stored.Generation.JobID == "" {
		return []Segment{}, nil
	}
	sections, err := p.store.Segments(ctx, stored.Generation.JobID)
	for index := range sections {
		sections[index].RawResponse = ""
		sections[index].Error = ""
	}
	return sections, err
}

func (p *Pipeline) JobSections(ctx context.Context, jobID string) ([]Segment, error) {
	return p.store.Segments(ctx, jobID)
}

func (p *Pipeline) SaveGuide(ctx context.Context, value guide.Guide) (guide.Guide, error) {
	stored, err := p.repository.Get(ctx, value.ID)
	if err != nil {
		return guide.Guide{}, err
	}
	value.SourceType = stored.SourceType
	value.SourceURI = stored.SourceURI
	value.SourceID = stored.SourceID
	value.CreatedAt = stored.CreatedAt
	value.Generation = stored.Generation
	markEditedSteps(stored.Steps, value.Steps)
	if err = p.verifier.Verify(ctx, value); err != nil {
		return guide.Guide{}, err
	}
	return p.repository.Save(ctx, value)
}

func (p *Pipeline) RegenerateSection(ctx context.Context, guideID string, sectionIndex int) (guide.Guide, error) {
	stored, err := p.repository.Get(ctx, guideID)
	if err != nil {
		return guide.Guide{}, err
	}
	if stored.Generation.JobID == "" {
		return guide.Guide{}, fmt.Errorf("this guide predates persisted sections and cannot regenerate one section")
	}
	sections, err := p.store.Segments(ctx, stored.Generation.JobID)
	if err != nil {
		return guide.Guide{}, err
	}
	position := -1
	for index, item := range sections {
		if item.Index == sectionIndex {
			position = index
			break
		}
	}
	if position < 0 {
		return guide.Guide{}, fmt.Errorf("section %d not found", sectionIndex+1)
	}
	p.report(ctx, "generating", fmt.Sprintf("Regenerating section %d…", sectionIndex+1), 0, 1)
	replacement, err := p.generator.Generate(ctx, guide.GenerateRequest{Title: stored.Title, SourceType: stored.SourceType, SourceURI: stored.SourceURI, SourceID: stored.SourceID, Segments: []transcript.Segment{sections[position].Transcript}})
	if err != nil {
		return guide.Guide{}, fmt.Errorf("regenerate section %d: %w", sectionIndex+1, err)
	}
	sections[position].Guide = replacement
	sections[position].Status = StatusCompleted
	sections[position].Model = replacement.Generation.Model
	sections[position].PromptTokens = replacement.Generation.PromptTokens
	sections[position].OutputTokens = replacement.Generation.OutputTokens
	sections[position].DurationMilliseconds = replacement.Generation.DurationMilliseconds
	if err = p.store.CompleteSegment(ctx, sections[position]); err != nil {
		return guide.Guide{}, err
	}
	partials := make([]guide.Guide, 0, len(sections))
	metadata := guide.Generation{JobID: stored.Generation.JobID, SegmentCount: len(sections), ContextWindow: stored.Generation.ContextWindow, MaxOutputTokens: stored.Generation.MaxOutputTokens}
	for _, section := range sections {
		partials = append(partials, section.Guide)
		metadata.Model = section.Model
		metadata.PromptTokens += section.PromptTokens
		metadata.OutputTokens += section.OutputTokens
		metadata.DurationMilliseconds += section.DurationMilliseconds
	}
	rebuilt := guide.Merge(stored.Title, partials)
	rebuilt.Steps = sectionSafeSteps(stored.Steps, sections, sectionIndex)
	rebuilt.ID = stored.ID
	rebuilt.SourceType = stored.SourceType
	rebuilt.SourceURI = stored.SourceURI
	rebuilt.SourceID = stored.SourceID
	rebuilt.CreatedAt = stored.CreatedAt
	rebuilt.Generation = metadata
	rebuilt.DeepDives = stored.DeepDives
	if err = p.verifier.Verify(ctx, rebuilt); err != nil {
		return guide.Guide{}, err
	}
	rebuilt, err = p.repository.Save(ctx, rebuilt)
	if err == nil {
		p.report(ctx, "complete", "Section regenerated and guide updated.", 1, 1)
	}
	return rebuilt, err
}

func (p *Pipeline) DelveSection(ctx context.Context, guideID string, sectionIndex int) (guide.Guide, error) {
	if p.expander == nil {
		return guide.Guide{}, fmt.Errorf("deep-dive generation is not configured")
	}
	stored, err := p.repository.Get(ctx, guideID)
	if err != nil {
		return guide.Guide{}, err
	}
	sections, err := p.Sections(ctx, guideID)
	if err != nil {
		return guide.Guide{}, err
	}
	var sourceSection *Segment
	for index := range sections {
		if sections[index].Index == sectionIndex {
			sourceSection = &sections[index]
			break
		}
	}
	if sourceSection == nil {
		return guide.Guide{}, fmt.Errorf("section %d not found", sectionIndex+1)
	}
	steps := make([]guide.Step, 0)
	for _, step := range stored.Steps {
		if step.SourceSegment == sectionIndex {
			steps = append(steps, step)
		}
	}
	p.report(ctx, "expanding", fmt.Sprintf("Delving deeper into section %d…", sectionIndex+1), 0, 1)
	deepDive, err := p.expander.Expand(ctx, guide.ExpandRequest{GuideTitle: stored.Title, Section: sourceSection.Transcript, Steps: steps})
	if err != nil {
		return guide.Guide{}, fmt.Errorf("delve into section %d: %w", sectionIndex+1, err)
	}
	replaced := false
	for index := range stored.DeepDives {
		if stored.DeepDives[index].SourceSegment == sectionIndex {
			stored.DeepDives[index] = deepDive
			replaced = true
			break
		}
	}
	if !replaced {
		stored.DeepDives = append(stored.DeepDives, deepDive)
	}
	if err = p.verifier.Verify(ctx, stored); err != nil {
		return guide.Guide{}, err
	}
	stored, err = p.repository.Save(ctx, stored)
	if err == nil {
		p.report(ctx, "complete", fmt.Sprintf("Section %d deep dive saved.", sectionIndex+1), 1, 1)
	}
	return stored, err
}

func markEditedSteps(previous, updated []guide.Step) {
	byID := make(map[string]guide.Step, len(previous))
	for _, step := range previous {
		if step.ID != "" {
			byID[step.ID] = step
		}
	}
	for index := range updated {
		before, ok := byID[updated[index].ID]
		if !ok && index < len(previous) {
			before, ok = previous[index], true
		}
		if ok && editableStepChanged(before, updated[index]) {
			updated[index].UserEdited = true
		}
	}
}

func editableStepChanged(before, after guide.Step) bool {
	return before.Title != after.Title || before.Explanation != after.Explanation ||
		!slices.Equal(before.Actions, after.Actions) || !slices.Equal(before.Commands, after.Commands) ||
		!slices.Equal(before.Warnings, after.Warnings)
}

// sectionSafeSteps rebuilds steps in source order while retaining the saved copy
// of every section that was not explicitly regenerated.
func sectionSafeSteps(saved []guide.Step, sections []Segment, regenerated int) []guide.Step {
	bySection := make(map[int][]guide.Step, len(sections))
	for _, step := range saved {
		bySection[step.SourceSegment] = append(bySection[step.SourceSegment], step)
	}
	result := make([]guide.Step, 0, len(saved))
	for _, section := range sections {
		steps := bySection[section.Index]
		if section.Index == regenerated || len(steps) == 0 {
			steps = section.Guide.Steps
		}
		for _, step := range steps {
			step.Number = len(result) + 1
			result = append(result, step)
		}
	}
	return result
}

func segmentResult(jobID string, result guide.SectionResult, status Status) Segment {
	return Segment{JobID: jobID, Index: result.Index, Transcript: result.Segment, Guide: result.Guide, Status: status, Model: result.Model, PromptTokens: result.PromptTokens, OutputTokens: result.OutputTokens, DurationMilliseconds: result.DurationMilliseconds, RawResponse: result.RawResponse, Error: result.Error}
}
