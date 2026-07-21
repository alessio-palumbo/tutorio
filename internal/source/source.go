// Package source defines ingestion boundaries for content providers.
package source

import (
	"context"
	"fmt"

	"github.com/alessio/tutorio/internal/transcript"
)

type Request struct {
	Type  string `json:"type"`
	URI   string `json:"uri"`
	Title string `json:"title,omitempty"`
}
type Source interface {
	Type() string
	Fetch(ctx context.Context, request Request) (transcript.Document, error)
}
type Resolver interface {
	Resolve(sourceType string) (Source, error)
}
type Registry struct{ sources map[string]Source }

func NewRegistry(sources ...Source) *Registry {
	r := &Registry{sources: make(map[string]Source, len(sources))}
	for _, s := range sources {
		r.sources[s.Type()] = s
	}
	return r
}
func (r *Registry) Resolve(kind string) (Source, error) {
	s, ok := r.sources[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported source type %q", kind)
	}
	return s, nil
}
