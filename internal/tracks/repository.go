package tracks

import (
	"context"
	"time"
)

// Repository persists tracks, their projected state, and their history.
// Implementations live in infrastructure layers and must not contain workflow
// logic.
type Repository interface {
	Create(ctx context.Context, track Track) (Track, error)
	Get(ctx context.Context, id string) (Track, error)
	FindByExternalRef(ctx context.Context, ref string) (Track, error)
	FindByObservationID(ctx context.Context, observationID string) (Track, error)
	List(ctx context.Context, limit, offset int) ([]Track, int, error)
	// ApplyObservation atomically advances the track watermark and projected
	// state for a newer observation (or the first projection when a track has no
	// state row). It returns false when another observation has already won.
	ApplyObservation(ctx context.Context, update StateUpdate, observedAt, expectedLastSeenAt time.Time) (bool, error)
	// ApplyObservationWithHistory commits the state projection, optional history
	// point, and observation process marker as one unit. A failed history insert
	// therefore cannot leave the watermark advanced while the evidence remains
	// unprocessed.
	ApplyObservationWithHistory(ctx context.Context, update StateUpdate, observedAt, expectedLastSeenAt time.Time, history *HistoryPoint, observationID string) (bool, error)
	Touch(ctx context.Context, id string, observedAt time.Time) error
	Close(ctx context.Context, id string) (Track, error)
	UpsertState(ctx context.Context, update StateUpdate) error
	// AttachObservation links evidence to a track, returning false when the
	// observation is already attached.
	AttachObservation(ctx context.Context, trackID, observationID string) (bool, error)
	AddHistoryEntry(ctx context.Context, entry HistoryPoint) error
	ListHistory(ctx context.Context, trackID string, limit int) ([]HistoryPoint, error)
	CountObservations(ctx context.Context, trackID string) (int, error)
	CountObservationsBySource(ctx context.Context, trackID string) ([]SourceCount, error)
}
