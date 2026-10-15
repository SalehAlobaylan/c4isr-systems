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
	// ListTrackFreshnessSignals returns every track's status and last-seen
	// time, for the same reason the asset equivalent exists: a recovery is
	// invisible to a query that only returns entities in trouble.
	ListTrackFreshnessSignals(ctx context.Context) ([]TrackFreshnessSignal, error)
	// ListFreshnessSignals returns every asset's declared and observed
	// freshness inputs so the monitor can classify each one and report only
	// genuine transitions.
	ListFreshnessSignals(ctx context.Context) ([]FreshnessSignal, error)
	ListByAsset(ctx context.Context, assetID string, limit, offset int) ([]Sample, int, error)
}
