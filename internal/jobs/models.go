package jobs

import (
	"context"
	"time"

	"github.com/alessio/tutorio/internal/guide"
	"github.com/alessio/tutorio/internal/transcript"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Job struct {
	ID          string     `json:"id"`
	SourceType  string     `json:"source_type"`
	SourceURI   string     `json:"source_uri"`
	Status      Status     `json:"status"`
	Stage       string     `json:"stage"`
	Current     int        `json:"current"`
	Total       int        `json:"total"`
	Error       string     `json:"error,omitempty"`
	GuideID     string     `json:"guide_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
type Segment struct {
	JobID                string             `json:"job_id"`
	Index                int                `json:"index"`
	Transcript           transcript.Segment `json:"transcript"`
	Guide                guide.Guide        `json:"guide"`
	Status               Status             `json:"status"`
	Model                string             `json:"model,omitempty"`
	PromptTokens         int                `json:"prompt_tokens,omitempty"`
	OutputTokens         int                `json:"output_tokens,omitempty"`
	DurationMilliseconds int64              `json:"duration_milliseconds,omitempty"`
	RawResponse          string             `json:"raw_response,omitempty"`
	Error                string             `json:"error,omitempty"`
}

type Store interface {
	Create(ctx context.Context, job Job) error
	Update(ctx context.Context, job Job) error
	SaveSegments(ctx context.Context, jobID string, segments []transcript.Segment) error
	CompleteSegment(ctx context.Context, segment Segment) error
	RecordSegmentFailure(ctx context.Context, segment Segment) error
	Get(ctx context.Context, id string) (Job, error)
	List(ctx context.Context, limit int) ([]Job, error)
	Segments(ctx context.Context, jobID string) ([]Segment, error)
}
type discardStore struct{}

func (discardStore) Create(context.Context, Job) error                                { return nil }
func (discardStore) Update(context.Context, Job) error                                { return nil }
func (discardStore) SaveSegments(context.Context, string, []transcript.Segment) error { return nil }
func (discardStore) CompleteSegment(context.Context, Segment) error                   { return nil }
func (discardStore) RecordSegmentFailure(context.Context, Segment) error              { return nil }
func (discardStore) Get(context.Context, string) (Job, error)                         { return Job{}, nil }
func (discardStore) List(context.Context, int) ([]Job, error)                         { return nil, nil }
func (discardStore) Segments(context.Context, string) ([]Segment, error)              { return nil, nil }
