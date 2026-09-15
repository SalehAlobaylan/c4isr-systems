package observations

import "context"

// ListFilter narrows observation reads.
type ListFilter struct {
	TrackID string
}

// Repository persists observations.
type Repository interface {
	// Create stores the observation. It returns ErrDuplicateID when the id
	// already exists; callers then load the existing row.
	Create(ctx context.Context, observation Observation) (Observation, error)
	Get(ctx context.Context, id string) (Observation, error)
	List(ctx context.Context, filter ListFilter, limit, offset int) ([]Observation, int, error)
	MarkProcessed(ctx context.Context, id string) error
}
