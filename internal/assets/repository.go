package assets

import "context"

// Repository persists assets and their current state. Implementations live in
// infrastructure layers and must not contain workflow logic.
type Repository interface {
	Create(ctx context.Context, asset Asset) (Asset, error)
	Get(ctx context.Context, id string) (Asset, error)
	Exists(ctx context.Context, id string) (bool, error)
	List(ctx context.Context, limit, offset int) ([]Asset, int, error)
	UpdateStatus(ctx context.Context, id string, status Status) (Asset, error)
}
