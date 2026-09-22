package telemetry

import (
	"context"
	"time"
)

// Repository persists telemetry samples and the asset state projection.
// Implementations live in infrastructure layers and must not contain workflow
// logic.
type Repository interface {
	Create(ctx context.Context, sample Sample) (Sample, error)
	StateSnapshot(ctx context.Context, assetID string) (StateSnapshot, error)
	// ApplyState advances the projection only when expectedLastSeenAt still
	// matches the database row. A nil expected value represents a never-seen
	// asset and is part of the compare-and-set contract.
	ApplyState(ctx context.Context, update StateUpdate, expectedLastSeenAt *time.Time) (bool, error)
	ListStaleAssetIDs(ctx context.Context, cutoff time.Time) ([]string, error)
	ListByAsset(ctx context.Context, assetID string, limit, offset int) ([]Sample, int, error)
}
