package jobs

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/source"
	"github.com/alessio/tutorio/internal/transcript"
)

type managerRunner struct {
	store   *managerStore
	started chan string
}

func (r *managerRunner) CreateJob(ctx context.Context, request source.Request) (Job, error) {
	now := time.Now().UTC()
	job := Job{ID: "job-1", SourceType: request.Type, SourceURI: request.URI, Status: StatusPending, Stage: "queued", CreatedAt: now, UpdatedAt: now}
	return job, r.store.Create(ctx, job)
}
func (r *managerRunner) RunJob(ctx context.Context, job Job, _ source.Request) (guide.Guide, error) {
	job.Status, job.Stage = StatusRunning, "source"
	_ = r.store.Update(ctx, job)
	r.started <- job.ID
	<-ctx.Done()
	return guide.Guide{}, ctx.Err()
}
func (r *managerRunner) RetryJob(ctx context.Context, id string) (guide.Guide, error) {
	job, _ := r.store.Get(ctx, id)
	return r.RunJob(ctx, job, source.Request{})
}

type managerStore struct {
	mu   sync.Mutex
	jobs map[string]Job
}

func newManagerStore() *managerStore { return &managerStore{jobs: map[string]Job{}} }
func (s *managerStore) Create(_ context.Context, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}
func (s *managerStore) Update(_ context.Context, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}
func (*managerStore) SaveSegments(context.Context, string, []transcript.Segment) error { return nil }
func (*managerStore) CompleteSegment(context.Context, Segment) error                   { return nil }
func (*managerStore) RecordSegmentFailure(context.Context, Segment) error              { return nil }
func (s *managerStore) Get(_ context.Context, id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id], nil
}
func (s *managerStore) List(context.Context, int) ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		result = append(result, job)
	}
	return result, nil
}
func (*managerStore) Segments(context.Context, string) ([]Segment, error) { return nil, nil }

func TestManagerQueuesAndCancelsWork(t *testing.T) {
	store := newManagerStore()
	runner := &managerRunner{store: store, started: make(chan string, 1)}
	manager := NewManager(runner, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	manager.Start(context.Background())
	defer manager.Stop()
	job, err := manager.Enqueue(context.Background(), source.Request{Type: "youtube", URI: "https://example.test/video"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("queued job did not start")
	}
	if err = manager.Cancel(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stored, _ := store.Get(context.Background(), job.ID)
		if stored.Status == StatusCancelled {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("cancelled job did not reach cancelled state")
}

func TestManagerRunsPendingWorkFirstAndRequeuesActiveJob(t *testing.T) {
	store := newManagerStore()
	runner := &managerRunner{store: store, started: make(chan string, 4)}
	manager := NewManager(runner, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	manager.Start(context.Background())
	defer manager.Stop()
	now := time.Now().UTC()
	for _, id := range []string{"active", "first", "next"} {
		if err := store.Create(context.Background(), Job{ID: id, SourceType: "test", Status: StatusPending, Stage: "queued", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.enqueue(queuedJob{id: "active"}); err != nil {
		t.Fatal(err)
	}
	if started := <-runner.started; started != "active" {
		t.Fatalf("started %q, want active", started)
	}
	if err := manager.enqueue(queuedJob{id: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.enqueue(queuedJob{id: "next"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RunFirst(context.Background(), "next"); err != nil {
		t.Fatal(err)
	}
	select {
	case started := <-runner.started:
		if started != "next" {
			t.Fatalf("started %q, want run-first job", started)
		}
	case <-time.After(time.Second):
		t.Fatal("run-first job did not start")
	}
	paused, err := store.Get(context.Background(), "active")
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != StatusPending || paused.Stage != "queued" {
		t.Fatalf("active job was not safely requeued: %#v", paused)
	}
}

func TestRecoveryResumesInterruptedJobsBeforeNewerPendingWork(t *testing.T) {
	newestFirst := []Job{
		{ID: "new-pending", Status: StatusPending},
		{ID: "interrupted", Status: StatusRunning},
		{ID: "old-pending", Status: StatusPending},
	}
	got := recoveryOrder(newestFirst)
	if len(got) != 3 || got[0].ID != "interrupted" || got[1].ID != "old-pending" || got[2].ID != "new-pending" {
		t.Fatalf("unexpected recovery order: %#v", got)
	}
}

func TestManagerShutdownPreservesActiveJobForRecovery(t *testing.T) {
	store := newManagerStore()
	runner := &managerRunner{store: store, started: make(chan string, 1)}
	manager := NewManager(runner, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	manager.Start(context.Background())
	now := time.Now().UTC()
	job := Job{ID: "interrupted", SourceType: "test", Status: StatusPending, Stage: "queued", CreatedAt: now, UpdatedAt: now}
	if err := store.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if err := manager.enqueue(queuedJob{id: job.ID}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("job did not start")
	}
	manager.Stop()
	stored, err := store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != StatusRunning || stored.Stage != "interrupted" {
		t.Fatalf("shutdown discarded resumable state: %#v", stored)
	}
}
