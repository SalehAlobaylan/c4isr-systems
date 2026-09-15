package alerts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores alerts in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts an alert.
func (r *PostgresRepository) Create(ctx context.Context, alert Alert) (Alert, error) {
	row, err := dbgen.New(r.pool).CreateAlert(ctx, dbgen.CreateAlertParams{
		ID:              alert.ID,
		Type:            alert.Type,
		Severity:        string(alert.Severity),
		Title:           alert.Title,
		Message:         pgconv.TextPtr(alert.Message),
		SourceReference: pgconv.JSONB(alert.SourceReference),
		TrackID:         pgconv.TextPtr(alert.TrackID),
		AssetID:         pgconv.TextPtr(alert.AssetID),
		GeofenceID:      pgconv.TextPtr(alert.GeofenceID),
		IncidentID:      pgconv.TextPtr(alert.IncidentID),
	})
	if err != nil {
		return Alert{}, err
	}
	return toDomain(row), nil
}

// Get loads an alert by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Alert, error) {
	row, err := dbgen.New(r.pool).GetAlert(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Alert{}, apperr.NotFound("alert", id)
		}
		return Alert{}, err
	}
	return toDomain(row), nil
}

// List returns alerts newest first. Empty filter fields are passed as SQL NULL
// so that they do not constrain the result.
func (r *PostgresRepository) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Alert, int, error) {
	q := dbgen.New(r.pool)
	params := dbgen.ListAlertsParams{
		State:       pgconv.TextPtr(filter.State),
		Severity:    pgconv.TextPtr(filter.Severity),
		TrackID:     pgconv.TextPtr(filter.TrackID),
		IncidentID:  pgconv.TextPtr(filter.IncidentID),
		LimitCount:  int32(limit),
		OffsetCount: int32(offset),
	}
	rows, err := q.ListAlerts(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountAlerts(ctx, dbgen.CountAlertsParams{
		State:      pgconv.TextPtr(filter.State),
		Severity:   pgconv.TextPtr(filter.Severity),
		TrackID:    pgconv.TextPtr(filter.TrackID),
		IncidentID: pgconv.TextPtr(filter.IncidentID),
	})
	if err != nil {
		return nil, 0, err
	}

	out := make([]Alert, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

// Acknowledge moves an ACTIVE alert to ACKNOWLEDGED.
func (r *PostgresRepository) Acknowledge(ctx context.Context, id, operator string) (Alert, error) {
	row, err := dbgen.New(r.pool).AcknowledgeAlert(ctx, dbgen.AcknowledgeAlertParams{
		AcknowledgedBy: pgconv.TextPtr(operator),
		ID:             id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Alert{}, apperr.Conflict("alert is not active")
		}
		return Alert{}, err
	}
	return toDomain(row), nil
}

// Resolve moves an ACTIVE or ACKNOWLEDGED alert to RESOLVED.
func (r *PostgresRepository) Resolve(ctx context.Context, id, operator string) (Alert, error) {
	row, err := dbgen.New(r.pool).ResolveAlert(ctx, dbgen.ResolveAlertParams{
		ResolvedBy: pgconv.TextPtr(operator),
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Alert{}, apperr.Conflict("alert is already resolved")
		}
		return Alert{}, err
	}
	return toDomain(row), nil
}

// SetIncident links an alert to an incident workspace.
func (r *PostgresRepository) SetIncident(ctx context.Context, alertID, incidentID string) error {
	return dbgen.New(r.pool).SetAlertIncident(ctx, dbgen.SetAlertIncidentParams{
		IncidentID: pgconv.TextPtr(incidentID),
		ID:         alertID,
	})
}

// FindUnresolvedForGeofenceTrack returns the newest unresolved alert for a
// geofence and track pair, or nil when none exists.
func (r *PostgresRepository) FindUnresolvedForGeofenceTrack(ctx context.Context, geofenceID, trackID string) (*Alert, error) {
	row, err := dbgen.New(r.pool).FindUnresolvedAlertForGeofenceTrack(ctx, dbgen.FindUnresolvedAlertForGeofenceTrackParams{
		GeofenceID: pgconv.TextPtr(geofenceID),
		TrackID:    pgconv.TextPtr(trackID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	alert := toDomain(row)
	return &alert, nil
}

func toDomain(row dbgen.Alert) Alert {
	return Alert{
		ID:              row.ID,
		Type:            row.Type,
		Severity:        Severity(row.Severity),
		State:           State(row.State),
		Title:           row.Title,
		Message:         pgconv.Deref(row.Message),
		SourceReference: pgconv.Map(row.SourceReference),
		TrackID:         pgconv.Deref(row.TrackID),
		AssetID:         pgconv.Deref(row.AssetID),
		GeofenceID:      pgconv.Deref(row.GeofenceID),
		IncidentID:      pgconv.Deref(row.IncidentID),
		CreatedAt:       pgconv.Time(row.CreatedAt),
		UpdatedAt:       pgconv.Time(row.UpdatedAt),
		AcknowledgedAt:  pgconv.TimePtr(row.AcknowledgedAt),
		AcknowledgedBy:  pgconv.Deref(row.AcknowledgedBy),
		ResolvedAt:      pgconv.TimePtr(row.ResolvedAt),
		ResolvedBy:      pgconv.Deref(row.ResolvedBy),
	}
}

var _ Repository = (*PostgresRepository)(nil)
