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
