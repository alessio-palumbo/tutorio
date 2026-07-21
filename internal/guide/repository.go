package guide

import "context"

type Repository interface {
	Save(ctx context.Context, guide Guide) (Guide, error)
	Get(ctx context.Context, id string) (Guide, error)
	List(ctx context.Context, limit int) ([]Summary, error)
}
