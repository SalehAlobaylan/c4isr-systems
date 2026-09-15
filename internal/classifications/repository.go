package classifications

import "context"

// Repository persists classifications. Implementations live in infrastructure
// layers and must not contain workflow logic.
type Repository interface {
	Create(ctx context.Context, classification Classification) (Classification, error)
	Get(ctx context.Context, id string) (Classification, error)
	ListByTrack(ctx context.Context, trackID string, limit, offset int) ([]Classification, int, error)
	// LatestForTrack returns nil when the track has no classifications.
	LatestForTrack(ctx context.Context, trackID string) (*Classification, error)
}
