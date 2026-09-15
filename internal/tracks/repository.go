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
	List(ctx context.Context, limit, offset int) ([]Track, int, error)
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
