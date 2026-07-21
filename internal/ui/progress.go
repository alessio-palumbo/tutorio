package ui

import (
	"context"
	"sync"

	"github.com/alessio/tutorio/internal/jobs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// EventReporter translates use-case progress into Wails events.
type EventReporter struct {
	mu  sync.RWMutex
	ctx context.Context
}

func NewEventReporter() *EventReporter              { return &EventReporter{} }
func (r *EventReporter) Attach(ctx context.Context) { r.mu.Lock(); r.ctx = ctx; r.mu.Unlock() }
func (r *EventReporter) Report(_ context.Context, progress jobs.Progress) {
	r.mu.RLock()
	ctx := r.ctx
	r.mu.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, "pipeline:progress", progress)
	}
}
