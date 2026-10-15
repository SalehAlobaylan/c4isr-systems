package picture

import (
	"context"
	"time"
)

// Repository reads the append-oriented evidence needed to reconstruct the
// picture. Every method takes the instant to reconstruct and the scenario
// namespace to restrict to; none of them read current-state projections,
// because those cannot answer "what was true then".
type Repository interface {
	TracksAt(ctx context.Context, at time.Time, scopePrefix string) ([]Track, error)
	AssetsAt(ctx context.Context, at time.Time, scopePrefix string) ([]Asset, error)
	AlertsAt(ctx context.Context, at time.Time, limit int) ([]Alert, error)
	GeofencesAt(ctx context.Context, limit int) ([]Geofence, error)
	Window(ctx context.Context, scopePrefix string) (Window, error)
}
