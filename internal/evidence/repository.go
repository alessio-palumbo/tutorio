package evidence

import "context"

type Repository interface {
	SaveSource(ctx context.Context, source Source) error
	SaveChunks(ctx context.Context, chunks []SourceChunk) error
	GetSource(ctx context.Context, id string) (Source, error)
	GetEvidence(ctx context.Context, id string) (Evidence, error)
}
