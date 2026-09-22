package geospatial

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Repository persists geofences and containment state and runs spatial
// queries. Spatial decisions are delegated to PostGIS.
type Repository interface {
	Create(ctx context.Context, geofence Geofence) (Geofence, error)
	Get(ctx context.Context, id string) (Geofence, error)
	List(ctx context.Context, limit, offset int) ([]Geofence, int, error)
	SetActive(ctx context.Context, id string, active bool) (Geofence, error)
	ContainingPoint(ctx context.Context, point geo.Point) ([]Geofence, error)
	StateFor(ctx context.Context, geofenceID, trackID string) (*bool, error)
	SetState(ctx context.Context, geofenceID, trackID string, inside bool) error
	StatesForTrack(ctx context.Context, trackID string) (map[string]bool, error)
	ReconcileStates(ctx context.Context, trackID string, containingIDs []string, scopePrefix string, observedAt time.Time) ([]StateTransition, error)
	AssetsWithinRadius(ctx context.Context, center geo.Point, radiusM float64, limit int) ([]AssetDistance, error)
	NearestAssets(ctx context.Context, center geo.Point, limit int) ([]AssetDistance, error)
}

// StateTransition is a containment change committed by ReconcileStates.
type StateTransition struct {
	GeofenceID string
	Inside     bool
}
