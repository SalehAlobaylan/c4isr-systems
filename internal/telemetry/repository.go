package telemetry

import "context"

// Repository persists telemetry samples and the asset state projection.
// Implementations live in infrastructure layers and must not contain workflow
// logic.
type Repository interface {
	Create(ctx context.Context, sample Sample) (Sample, error)
	StateSnapshot(ctx context.Context, assetID string) (StateSnapshot, error)
	ApplyState(ctx context.Context, update StateUpdate) error
	ListByAsset(ctx context.Context, assetID string, limit, offset int) ([]Sample, int, error)
}
