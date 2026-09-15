package incidents

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores incidents in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts an incident.
func (r *PostgresRepository) Create(ctx context.Context, incident Incident) (Incident, error) {
	row, err := dbgen.New(r.pool).CreateIncident(ctx, dbgen.CreateIncidentParams{
		ID:               incident.ID,
		Title:            incident.Title,
		Description:      incident.Description,
		Priority:         string(incident.Priority),
		Status:           string(incident.Status),
		AssignedOperator: pgconv.TextPtr(incident.AssignedOperator),
	})
	if err != nil {
		return Incident{}, err
	}
	return toDomain(row), nil
}

// Get loads an incident by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Incident, error) {
	row, err := dbgen.New(r.pool).GetIncident(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Incident{}, apperr.NotFound("incident", id)
		}
		return Incident{}, err
	}
	return toDomain(row), nil
}

// GetDetail loads an incident together with its attached evidence.
func (r *PostgresRepository) GetDetail(ctx context.Context, id string) (Detail, error) {
	q := dbgen.New(r.pool)
	row, err := q.GetIncident(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, apperr.NotFound("incident", id)
		}
		return Detail{}, err
	}
	detail := Detail{Incident: toDomain(row)}

	alerts, err := q.ListIncidentAlerts(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	detail.Alerts = make([]RelatedAlert, 0, len(alerts))
	for _, alert := range alerts {
		detail.Alerts = append(detail.Alerts, toRelatedAlert(alert))
	}

	tracks, err := q.ListIncidentTracks(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	detail.Tracks = make([]RelatedTrack, 0, len(tracks))
	for _, track := range tracks {
		detail.Tracks = append(detail.Tracks, toRelatedTrack(track))
	}

	assets, err := q.ListIncidentAssets(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	detail.Assets = make([]RelatedAsset, 0, len(assets))
	for _, asset := range assets {
		detail.Assets = append(detail.Assets, toRelatedAsset(asset))
	}

	observations, err := q.ListIncidentObservations(ctx, dbgen.ListIncidentObservationsParams{
		IncidentID: id,
		LimitCount: DefaultLimit,
	})
	if err != nil {
		return Detail{}, err
	}
	detail.Observations = make([]RelatedObservation, 0, len(observations))
	for _, observation := range observations {
		detail.Observations = append(detail.Observations, toRelatedObservation(observation))
	}

	assessments, err := q.ListIncidentAssessments(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	detail.Assessments = make([]RelatedAssessment, 0, len(assessments))
	for _, assessment := range assessments {
		detail.Assessments = append(detail.Assessments, toRelatedAssessment(assessment))
	}

	return detail, nil
}

// List returns incidents newest first with an optional status filter.
func (r *PostgresRepository) List(ctx context.Context, status string, limit, offset int) ([]Incident, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListIncidents(ctx, dbgen.ListIncidentsParams{
		Status:      pgconv.TextPtr(status),
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountIncidents(ctx, pgconv.TextPtr(status))
	if err != nil {
		return nil, 0, err
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

// UpdateStatus changes incident lifecycle status.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id string, status Status) (Incident, error) {
	row, err := dbgen.New(r.pool).UpdateIncidentStatus(ctx, dbgen.UpdateIncidentStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Incident{}, apperr.NotFound("incident", id)
		}
		return Incident{}, err
	}
	return toDomain(row), nil
}

// Update edits incident metadata.
func (r *PostgresRepository) Update(ctx context.Context, incident Incident) (Incident, error) {
	row, err := dbgen.New(r.pool).UpdateIncident(ctx, dbgen.UpdateIncidentParams{
		Title:            incident.Title,
		Description:      incident.Description,
		Priority:         string(incident.Priority),
		AssignedOperator: pgconv.TextPtr(incident.AssignedOperator),
		ID:               incident.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Incident{}, apperr.NotFound("incident", incident.ID)
		}
		return Incident{}, err
	}
	return toDomain(row), nil
}

// AttachAlert links an alert to an incident.
func (r *PostgresRepository) AttachAlert(ctx context.Context, incidentID, alertID string) error {
	err := dbgen.New(r.pool).AddIncidentAlert(ctx, dbgen.AddIncidentAlertParams{
		IncidentID: incidentID,
		AlertID:    alertID,
	})
	return relationError(err, "alert", alertID)
}

// AttachTrack links a track to an incident.
func (r *PostgresRepository) AttachTrack(ctx context.Context, incidentID, trackID string) error {
	err := dbgen.New(r.pool).AddIncidentTrack(ctx, dbgen.AddIncidentTrackParams{
		IncidentID: incidentID,
		TrackID:    trackID,
	})
	return relationError(err, "track", trackID)
}

// AttachAsset links an asset to an incident.
func (r *PostgresRepository) AttachAsset(ctx context.Context, incidentID, assetID string) error {
	err := dbgen.New(r.pool).AddIncidentAsset(ctx, dbgen.AddIncidentAssetParams{
		IncidentID: incidentID,
		AssetID:    assetID,
	})
	return relationError(err, "asset", assetID)
}

// AttachObservation links an observation to an incident.
func (r *PostgresRepository) AttachObservation(ctx context.Context, incidentID, observationID string) error {
	err := dbgen.New(r.pool).AddIncidentObservation(ctx, dbgen.AddIncidentObservationParams{
		IncidentID:    incidentID,
		ObservationID: observationID,
	})
	return relationError(err, "observation", observationID)
}

// AttachAssessment links an assessment to an incident.
func (r *PostgresRepository) AttachAssessment(ctx context.Context, incidentID, assessmentID string) error {
	err := dbgen.New(r.pool).AddIncidentAssessment(ctx, dbgen.AddIncidentAssessmentParams{
		IncidentID:   incidentID,
		AssessmentID: assessmentID,
	})
	return relationError(err, "assessment", assessmentID)
}

func toDomain(row dbgen.Incident) Incident {
	return Incident{
		ID:               row.ID,
		Title:            row.Title,
		Description:      row.Description,
		Priority:         Priority(row.Priority),
		Status:           Status(row.Status),
		AssignedOperator: pgconv.Deref(row.AssignedOperator),
		CreatedAt:        pgconv.Time(row.CreatedAt),
		UpdatedAt:        pgconv.Time(row.UpdatedAt),
		ResolvedAt:       pgconv.TimePtr(row.ResolvedAt),
		ClosedAt:         pgconv.TimePtr(row.ClosedAt),
	}
}

func toRelatedAlert(row dbgen.Alert) RelatedAlert {
	return RelatedAlert{
		ID:        row.ID,
		Type:      row.Type,
		Severity:  row.Severity,
		State:     row.State,
		Title:     row.Title,
		CreatedAt: pgconv.Time(row.CreatedAt),
	}
}

func toRelatedTrack(row dbgen.ListIncidentTracksRow) RelatedTrack {
	return RelatedTrack{
		ID:          row.ID,
		ExternalRef: pgconv.Deref(row.ExternalRef),
		Status:      row.Status,
		Position:    position(row.HasPosition, row.Lat, row.Lng),
		LastSeenAt:  pgconv.Time(row.LastSeenAt),
	}
}

func toRelatedAsset(row dbgen.ListIncidentAssetsRow) RelatedAsset {
	return RelatedAsset{
		ID:              row.ID,
		Name:            row.Name,
		Type:            row.Type,
		Status:          row.Status,
		Position:        position(row.HasPosition, row.Lat, row.Lng),
		ConnectionState: pgconv.Deref(row.ConnectionState),
	}
}

func toRelatedObservation(row dbgen.ListIncidentObservationsRow) RelatedObservation {
	return RelatedObservation{
		ID:         row.ID,
		SourceID:   row.SourceID,
		Type:       row.ObservationType,
		ObservedAt: pgconv.Time(row.ObservedAt),
		Position:   position(row.HasPosition, row.Lat, row.Lng),
	}
}

func toRelatedAssessment(row dbgen.Assessment) RelatedAssessment {
	return RelatedAssessment{
		ID:          row.ID,
		SubjectType: row.SubjectType,
		SubjectID:   row.SubjectID,
		Type:        row.Type,
		Conclusion:  row.Conclusion,
		Method:      row.Method,
		Confidence:  row.Confidence,
		CreatedAt:   pgconv.Time(row.CreatedAt),
	}
}

func position(hasPosition bool, lat, lng float64) *geo.Point {
	if !hasPosition {
		return nil
	}
	return &geo.Point{Lat: lat, Lng: lng}
}

func relationError(err error, kind, id string) error {
	if err == nil {
		return nil
	}
	if isForeignKeyViolation(err) {
		return apperr.BadRequest("unknown " + kind + ": " + id)
	}
	return err
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var _ Repository = (*PostgresRepository)(nil)
