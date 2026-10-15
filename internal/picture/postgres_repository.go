package picture

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository reconstructs the picture from the evidence tables.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// TracksAt returns every track that existed at the requested instant, with the
// last state each had recorded by then.
func (r *PostgresRepository) TracksAt(ctx context.Context, at time.Time, scopePrefix string) ([]Track, error) {
	rows, err := dbgen.New(r.pool).TracksAt(ctx, dbgen.TracksAtParams{
		At:          pgconv.TS(at),
		ScopePrefix: scopePrefix,
	})
	if err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(rows))
	for _, row := range rows {
		track := Track{
			ID:          row.TrackID,
			ExternalRef: pgconv.Deref(row.ExternalRef),
			Status:      row.Status,
			FirstSeenAt: pgconv.Time(row.FirstSeenAt),
			LastSeenAt:  pgconv.Time(row.LastSeenAt),
			ObservedAt:  pgconv.Time(row.ObservedAt),
			Speed:       row.Speed,
			Heading:     row.Heading,
		}
		if row.HasPosition {
			track.Position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

// AssetsAt returns every asset with telemetry recorded by the requested instant.
func (r *PostgresRepository) AssetsAt(ctx context.Context, at time.Time, scopePrefix string) ([]Asset, error) {
	rows, err := dbgen.New(r.pool).AssetsAt(ctx, dbgen.AssetsAtParams{
		At:          pgconv.TS(at),
		ScopePrefix: scopePrefix,
	})
	if err != nil {
		return nil, err
	}
	assets := make([]Asset, 0, len(rows))
	for _, row := range rows {
		asset := Asset{
			ID:              row.AssetID,
			Name:            row.Name,
			Type:            row.Type,
			Status:          row.Status,
			ObservedAt:      pgconv.Time(row.ObservedAt),
			Speed:           row.Speed,
			Heading:         row.Heading,
			ConnectionState: pgconv.Deref(row.ConnectionState),
			Health:          pgconv.Deref(row.Health),
		}
		if row.HasPosition {
			asset.Position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

// AlertsAt returns the alerts that had already been raised by the instant.
func (r *PostgresRepository) AlertsAt(ctx context.Context, at time.Time, limit int) ([]Alert, error) {
	rows, err := dbgen.New(r.pool).AlertsAt(ctx, dbgen.AlertsAtParams{
		At:         pgconv.TS(at),
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	alerts := make([]Alert, 0, len(rows))
	for _, row := range rows {
		alerts = append(alerts, Alert{
			ID:              row.ID,
			Type:            row.Type,
			Severity:        row.Severity,
			State:           row.State,
			Title:           row.Title,
			Message:         pgconv.Deref(row.Message),
			SourceReference: pgconv.Map(row.SourceReference),
			TrackID:         pgconv.Deref(row.TrackID),
			AssetID:         pgconv.Deref(row.AssetID),
			GeofenceID:      pgconv.Deref(row.GeofenceID),
			CreatedAt:       pgconv.Time(row.CreatedAt),
		})
	}
	return alerts, nil
}

// GeofencesAt returns the zones in force, which replay does not alter.
func (r *PostgresRepository) GeofencesAt(ctx context.Context, limit int) ([]Geofence, error) {
	rows, err := dbgen.New(r.pool).GeofencesAt(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	geofences := make([]Geofence, 0, len(rows))
	for _, row := range rows {
		geofences = append(geofences, Geofence{
			ID:       row.ID,
			Name:     row.Name,
			Type:     row.Type,
			Severity: row.Severity,
			Active:   row.Active,
			GeoJSON:  row.Geojson,
		})
	}
	return geofences, nil
}

// Window reports the span of evidence a scrubber can traverse.
func (r *PostgresRepository) Window(ctx context.Context, scopePrefix string) (Window, error) {
	row, err := dbgen.New(r.pool).TrackHistoryWindow(ctx, scopePrefix)
	if err != nil {
		return Window{}, err
	}
	return Window{
		Earliest: pgconv.Time(row.Earliest),
		Latest:   pgconv.Time(row.Latest),
	}, nil
}
