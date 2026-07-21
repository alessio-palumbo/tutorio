package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/source"
)

type jobRunner interface {
	CreateJob(context.Context, source.Request) (Job, error)
	RunJob(context.Context, Job, source.Request) (guide.Guide, error)
	RetryJob(context.Context, string) (guide.Guide, error)
}

type queuedJob struct {
	id     string
	resume bool
}

// Manager runs expensive compilation work outside Wails request handlers.
// It intentionally uses one worker so local model requests do not compete for RAM.
type Manager struct {
	runner jobRunner
	store  Store
	logger *slog.Logger
	queue  chan queuedJob

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	active  map[string]context.CancelFunc
	queued  map[string]bool
	started bool
}

func NewManager(runner jobRunner, store Store, logger *slog.Logger) *Manager {
	return &Manager{runner: runner, store: store, logger: logger, queue: make(chan queuedJob, 100), active: map[string]context.CancelFunc{}, queued: map[string]bool{}}
}

// Start launches the worker and requeues work interrupted by the previous shutdown.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true
	m.mu.Unlock()
	go m.worker()
	m.recoverInterrupted()
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.started = false
	m.mu.Unlock()
}

func (m *Manager) Enqueue(ctx context.Context, request source.Request) (Job, error) {
	job, err := m.runner.CreateJob(ctx, request)
	if err != nil {
		return Job{}, err
	}
	if err = m.enqueue(queuedJob{id: job.ID}); err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		job.UpdatedAt = time.Now().UTC()
		_ = m.store.Update(ctx, job)
		return Job{}, err
	}
	return job, nil
}

func (m *Manager) Retry(ctx context.Context, id string) (Job, error) {
	job, err := m.store.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.Status == StatusRunning || job.Status == StatusPending {
		return job, nil
	}
	segments, err := m.store.Segments(ctx, id)
	if err != nil {
		return Job{}, err
	}
	job.Status, job.Stage, job.Error = StatusPending, "queued", ""
	job.CompletedAt = nil
	job.UpdatedAt = time.Now().UTC()
	if err = m.store.Update(ctx, job); err != nil {
		return Job{}, err
	}
	if err = m.enqueue(queuedJob{id: id, resume: len(segments) > 0}); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (m *Manager) Cancel(ctx context.Context, id string) error {
	job, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if job.Status == StatusCompleted || job.Status == StatusCancelled {
		return nil
	}
	m.mu.Lock()
	cancel := m.active[id]
	delete(m.queued, id)
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	job.Status, job.Stage, job.Error = StatusCancelled, "cancelled", ""
	job.UpdatedAt = time.Now().UTC()
	return m.store.Update(ctx, job)
}

func (m *Manager) enqueue(item queuedJob) error {
	m.mu.Lock()
	if !m.started || m.ctx == nil {
		m.mu.Unlock()
		return fmt.Errorf("job manager is not running")
	}
	if _, exists := m.queued[item.id]; exists {
		m.mu.Unlock()
		return nil
	}
	m.queued[item.id] = item.resume
	ctx := m.ctx
	m.mu.Unlock()
	select {
	case m.queue <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) worker() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case item := <-m.queue:
			m.run(item)
		}
	}
}

func (m *Manager) run(item queuedJob) {
	m.mu.Lock()
	resume, queued := m.queued[item.id]
	delete(m.queued, item.id)
	ctx, cancel := context.WithCancel(m.ctx)
	m.active[item.id] = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.active, item.id)
		m.mu.Unlock()
	}()
	if !queued {
		return
	}
	job, err := m.store.Get(ctx, item.id)
	if err != nil || job.Status == StatusCancelled {
		return
	}
	if resume {
		_, err = m.runner.RetryJob(ctx, item.id)
	} else {
		_, err = m.runner.RunJob(ctx, job, source.Request{Type: job.SourceType, URI: job.SourceURI})
	}
	if err == nil {
		return
	}
	if ctx.Err() != nil {
		job, getErr := m.store.Get(context.Background(), item.id)
		if getErr == nil {
			job.Status, job.Stage, job.Error = StatusCancelled, "cancelled", ""
			job.UpdatedAt = time.Now().UTC()
			_ = m.store.Update(context.Background(), job)
		}
		return
	}
	m.logger.Error("background job failed", "job_id", item.id, "error", err)
}

func (m *Manager) recoverInterrupted() {
	items, err := m.store.List(m.ctx, 500)
	if err != nil {
		m.logger.Error("list interrupted jobs", "error", err)
		return
	}
	for _, job := range items {
		if job.Status != StatusPending && job.Status != StatusRunning {
			continue
		}
		segments, segmentErr := m.store.Segments(m.ctx, job.ID)
		if segmentErr != nil {
			m.logger.Error("load interrupted job sections", "job_id", job.ID, "error", segmentErr)
			continue
		}
		job.Status, job.Stage, job.Error = StatusPending, "queued", ""
		job.UpdatedAt = time.Now().UTC()
		_ = m.store.Update(m.ctx, job)
		_ = m.enqueue(queuedJob{id: job.ID, resume: len(segments) > 0})
	}
}
