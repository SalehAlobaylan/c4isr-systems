package geospatial

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores geofences and containment state in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a geofence with its geometry.
func (r *PostgresRepository) Create(ctx context.Context, geofence Geofence) (Geofence, error) {
	row, err := dbgen.New(r.pool).CreateGeofence(ctx, dbgen.CreateGeofenceParams{
		ID:       geofence.ID,
		Name:     geofence.Name,
		Type:     string(geofence.Type),
		Geojson:  geofence.GeoJSON,
		Severity: string(geofence.Severity),
		Active:   geofence.Active,
		Metadata: pgconv.JSONB(geofence.Metadata),
	})
	if err != nil {
		return Geofence{}, err
	}
	return toDomain(row), nil
}

// Get loads a geofence by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Geofence, error) {
	row, err := dbgen.New(r.pool).GetGeofence(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Geofence{}, apperr.NotFound("geofence", id)
		}
		return Geofence{}, err
	}
	return fromGetRow(row), nil
}

// List returns geofences newest first with a total count.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Geofence, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListGeofences(ctx, dbgen.ListGeofencesParams{
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountGeofences(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Geofence, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromListRow(row))
	}
	return out, int(total), nil
}

// SetActive enables or disables a geofence.
func (r *PostgresRepository) SetActive(ctx context.Context, id string, active bool) (Geofence, error) {
	row, err := dbgen.New(r.pool).UpdateGeofenceActive(ctx, dbgen.UpdateGeofenceActiveParams{
		Active: active,
		ID:     id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Geofence{}, apperr.NotFound("geofence", id)
		}
		return Geofence{}, err
	}
	return fromActiveRow(row), nil
}

// ContainingPoint lists active geofences containing the point.
func (r *PostgresRepository) ContainingPoint(ctx context.Context, point geo.Point) ([]Geofence, error) {
	rows, err := dbgen.New(r.pool).ListActiveGeofencesContainingPoint(ctx, dbgen.ListActiveGeofencesContainingPointParams{
		Lng: point.Lng,
		Lat: point.Lat,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Geofence, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromContainingRow(row))
	}
	return out, nil
}

// StateFor returns the persisted containment state for a geofence and track,
// or nil when no state has ever been recorded.
func (r *PostgresRepository) StateFor(ctx context.Context, geofenceID, trackID string) (*bool, error) {
	row, err := dbgen.New(r.pool).GetGeofenceState(ctx, dbgen.GetGeofenceStateParams{
		GeofenceID: geofenceID,
		TrackID:    trackID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	inside := row.Inside
	return &inside, nil
}

// SetState persists a containment transition.
func (r *PostgresRepository) SetState(ctx context.Context, geofenceID, trackID string, inside bool) error {
	return dbgen.New(r.pool).UpsertGeofenceState(ctx, dbgen.UpsertGeofenceStateParams{
		GeofenceID: geofenceID,
		TrackID:    trackID,
		Inside:     inside,
	})
}

// StatesForTrack returns the persisted containment state of every geofence the
// track has been evaluated against.
func (r *PostgresRepository) StatesForTrack(ctx context.Context, trackID string) (map[string]bool, error) {
	rows, err := dbgen.New(r.pool).ListGeofenceStatesByTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[row.GeofenceID] = row.Inside
	}
	return out, nil
}

// AssetsWithinRadius lists assets with a position inside the radius, nearest
// first.
func (r *PostgresRepository) AssetsWithinRadius(ctx context.Context, center geo.Point, radiusM float64, limit int) ([]AssetDistance, error) {
	rows, err := dbgen.New(r.pool).ListAssetsWithinRadius(ctx, dbgen.ListAssetsWithinRadiusParams{
		Lng:        center.Lng,
		Lat:        center.Lat,
		RadiusM:    radiusM,
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AssetDistance, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromWithinRadiusRow(row))
	}
	return out, nil
}

// NearestAssets lists assets with a position, nearest first.
func (r *PostgresRepository) NearestAssets(ctx context.Context, center geo.Point, limit int) ([]AssetDistance, error) {
	rows, err := dbgen.New(r.pool).NearestAssets(ctx, dbgen.NearestAssetsParams{
		Lng:        center.Lng,
		Lat:        center.Lat,
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AssetDistance, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromNearestRow(row))
	}
	return out, nil
}

type geofenceRow struct {
	ID        string
	Name      string
	Type      string
	Severity  string
	Active    bool
	Metadata  []byte
	CreatedAt pgtype.Timestamptz
	UpdatedAt pgtype.Timestamptz
	Geojson   string
}

func toDomain(row dbgen.CreateGeofenceRow) Geofence {
	return fromGeofenceRow(geofenceRow(row))
}

func fromGetRow(row dbgen.GetGeofenceRow) Geofence {
	return fromGeofenceRow(geofenceRow(row))
}

func fromListRow(row dbgen.ListGeofencesRow) Geofence {
	return fromGeofenceRow(geofenceRow(row))
}

func fromContainingRow(row dbgen.ListActiveGeofencesContainingPointRow) Geofence {
	return fromGeofenceRow(geofenceRow(row))
}

func fromActiveRow(row dbgen.UpdateGeofenceActiveRow) Geofence {
	return fromGeofenceRow(geofenceRow(row))
}

func fromGeofenceRow(row geofenceRow) Geofence {
	return Geofence{
		ID:        row.ID,
		Name:      row.Name,
		Type:      Type(row.Type),
		Severity:  Severity(row.Severity),
		Active:    row.Active,
		GeoJSON:   row.Geojson,
		Metadata:  pgconv.Map(row.Metadata),
		CreatedAt: pgconv.Time(row.CreatedAt),
		UpdatedAt: pgconv.Time(row.UpdatedAt),
	}
}

type assetDistanceRow struct {
	ID              string
	Name            string
	Type            string
	Status          string
	ConnectionState string
	LastSeenAt      pgtype.Timestamptz
	HasPosition     bool
	Lat             float64
	Lng             float64
	DistanceM       float64
}

func fromWithinRadiusRow(row dbgen.ListAssetsWithinRadiusRow) AssetDistance {
	return fromAssetRow(assetDistanceRow(row))
}

func fromNearestRow(row dbgen.NearestAssetsRow) AssetDistance {
	return fromAssetRow(assetDistanceRow(row))
}

func fromAssetRow(row assetDistanceRow) AssetDistance {
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return AssetDistance{
		AssetID:         row.ID,
		Name:            row.Name,
		Type:            row.Type,
		Status:          row.Status,
		ConnectionState: row.ConnectionState,
		Position:        position,
		DistanceM:       row.DistanceM,
		LastSeenAt:      pgconv.TimePtr(row.LastSeenAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)
