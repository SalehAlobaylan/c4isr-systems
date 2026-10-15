package observations

import "context"

// ListFilter narrows observation reads.
type ListFilter struct {
	TrackID string
	// AreaOnly restricts results to observations carrying a footprint, and
	// optionally to one phenomenon kind.
	//
	// This exists because point observations outnumber area observations by
	// orders of magnitude on any real feed. Asking for the newest N observations
	// and filtering client-side would return mostly radar pings and miss every
	// hazard, so the filter has to reach the database.
	AreaOnly bool
	// AreaKind selects one phenomenon class. Empty with AreaOnly returns every
	// kind.
	AreaKind AreaKind
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
