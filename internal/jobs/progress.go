package jobs

import "context"

// Progress describes observable pipeline work without coupling the use case to Wails.
type Progress struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// ProgressReporter is implemented by delivery adapters such as the Wails UI.
type ProgressReporter interface {
	Report(ctx context.Context, progress Progress)
}

type discardProgress struct{}

func (discardProgress) Report(context.Context, Progress) {}
