package sources

import "context"

// Repository persists sources. Implementations live in infrastructure layers
// and must not contain workflow logic.
type Repository interface {
	Create(ctx context.Context, source Source) (Source, error)
	Get(ctx context.Context, id string) (Source, error)
	Exists(ctx context.Context, id string) (bool, error)
	List(ctx context.Context, limit, offset int) ([]Source, int, error)
	UpdateStatus(ctx context.Context, id string, status Status) (Source, error)
}
