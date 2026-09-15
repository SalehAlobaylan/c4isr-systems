package incidents

import "context"

// Repository persists incidents and their evidence relations.
type Repository interface {
	Create(ctx context.Context, incident Incident) (Incident, error)
	Get(ctx context.Context, id string) (Incident, error)
	GetDetail(ctx context.Context, id string) (Detail, error)
	List(ctx context.Context, status string, limit, offset int) ([]Incident, int, error)
	UpdateStatus(ctx context.Context, id string, status Status) (Incident, error)
	Update(ctx context.Context, incident Incident) (Incident, error)
	AttachAlert(ctx context.Context, incidentID, alertID string) error
	AttachTrack(ctx context.Context, incidentID, trackID string) error
	AttachAsset(ctx context.Context, incidentID, assetID string) error
	AttachObservation(ctx context.Context, incidentID, observationID string) error
	AttachAssessment(ctx context.Context, incidentID, assessmentID string) error
}
