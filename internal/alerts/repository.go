package alerts

import "context"

// Repository persists alerts and their lifecycle transitions.
type Repository interface {
	Create(ctx context.Context, alert Alert) (Alert, error)
	Get(ctx context.Context, id string) (Alert, error)
	List(ctx context.Context, filter ListFilter, limit, offset int) ([]Alert, int, error)
	Acknowledge(ctx context.Context, id, operator string) (Alert, error)
	Resolve(ctx context.Context, id, operator string) (Alert, error)
	SetIncident(ctx context.Context, alertID, incidentID string) error
	FindUnresolvedForGeofenceTrack(ctx context.Context, geofenceID, trackID string) (*Alert, error)
	// FindUnresolvedForObservationGeofence returns the newest unresolved area
	// overlap alert for an observation and zone pair, or nil.
	FindUnresolvedForObservationGeofence(ctx context.Context, observationID, geofenceID string) (*Alert, error)
}
