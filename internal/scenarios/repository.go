package scenarios

import "context"

// Repository persists scenario run control-plane state. Operational data
// produced by a run lives in the domain tables, never here.
type Repository interface {
	Create(ctx context.Context, run Run) (Run, error)
	Get(ctx context.Context, id string) (Run, error)
	List(ctx context.Context, limit, offset int) ([]Run, int, error)
	UpdateStatus(ctx context.Context, id, status, errorMessage string) (Run, error)
	UpdateSpeed(ctx context.Context, id string, speed float64) (Run, error)
	UpdateProgress(ctx context.Context, id string, virtualTimeMs int64) error
}
