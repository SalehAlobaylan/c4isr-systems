package telemetry

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores telemetry samples in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a telemetry sample, reporting duplicate message ids.
func (r *PostgresRepository) Create(ctx context.Context, sample Sample) (Sample, error) {
	params := dbgen.CreateTelemetryParams{
		ID:              sample.ID,
		MessageID:       pgconv.TextPtr(sample.MessageID),
		AssetID:         sample.AssetID,
		SourceID:        pgconv.TextPtr(sample.SourceID),
		ObservedAt:      pgconv.TS(sample.ObservedAt),
		ReceivedAt:      pgconv.TS(sample.ReceivedAt),
		Speed:           sample.Speed,
		Heading:         sample.Heading,
		Health:          pgconv.TextPtr(sample.Health),
		ConnectionState: pgconv.TextPtr(sample.ConnectionState),
		Payload:         pgconv.JSONB(sample.Payload),
	}
	if sample.Position != nil {
		params.HasPosition = true
		params.Lat = sample.Position.Lat
		params.Lng = sample.Position.Lng
	}

	if _, err := dbgen.New(r.pool).CreateTelemetry(ctx, params); err != nil {
		if sample.MessageID != "" && (errors.Is(err, pgx.ErrNoRows) || isUniqueViolation(err)) {
			return Sample{}, ErrDuplicateMessage
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			if strings.Contains(pgErr.ConstraintName, "source_id") {
				return Sample{}, apperr.BadRequest("unknown source: " + sample.SourceID)
			}
			return Sample{}, apperr.Validation("unknown asset: " + sample.AssetID)
		}
		return Sample{}, err
	}
	return sample, nil
}

// StateSnapshot loads the current projected state for an asset, creating the
// projection row when it does not exist yet.
func (r *PostgresRepository) StateSnapshot(ctx context.Context, assetID string) (StateSnapshot, error) {
	q := dbgen.New(r.pool)
	if err := q.EnsureAssetState(ctx, assetID); err != nil {
		return StateSnapshot{}, err
	}
	row, err := q.GetAssetState(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StateSnapshot{}, apperr.NotFound("asset state", assetID)
		}
		return StateSnapshot{}, err
	}
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return StateSnapshot{
		AssetID:         row.AssetID,
		Position:        position,
		ConnectionState: row.ConnectionState,
		LastSeenAt:      pgconv.TimePtr(row.LastSeenAt),
	}, nil
}

// ApplyState writes a state projection update for an asset.
func (r *PostgresRepository) ApplyState(ctx context.Context, update StateUpdate, expectedLastSeenAt *time.Time) (bool, error) {
	expected := pgtype.Timestamptz{}
	if expectedLastSeenAt != nil {
		expected = pgconv.TS(*expectedLastSeenAt)
	}
	params := dbgen.UpsertAssetStateParams{
		AssetID:            update.AssetID,
		Speed:              update.Speed,
		Heading:            update.Heading,
		Health:             pgconv.TextPtr(update.Health),
		ConnectionState:    update.ConnectionState,
		ObservedAt:         pgconv.TS(update.ObservedAt),
		ExpectedLastSeenAt: expected,
	}
	if update.Position != nil {
		params.HasPosition = true
		params.Lat = update.Position.Lat
		params.Lng = update.Position.Lng
	}
	return dbgen.New(r.pool).UpsertAssetState(ctx, params)
}

// ListStaleAssetIDs returns assets whose latest accepted telemetry is older
// than cutoff. The monitor uses this query so an asset can become stale even
// when its producer has stopped sending samples.
func (r *PostgresRepository) ListStaleAssetIDs(ctx context.Context, cutoff time.Time) ([]string, error) {
	rows, err := dbgen.New(r.pool).ListStaleAssetIDs(ctx, pgconv.TS(cutoff))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByAsset returns telemetry for an asset newest first with a total count.
func (r *PostgresRepository) ListByAsset(ctx context.Context, assetID string, limit, offset int) ([]Sample, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListTelemetryByAsset(ctx, dbgen.ListTelemetryByAssetParams{
		AssetID:     assetID,
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountTelemetryByAsset(ctx, assetID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Sample, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

func toDomain(row dbgen.ListTelemetryByAssetRow) Sample {
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return Sample{
		ID:              row.ID,
		MessageID:       pgconv.Deref(row.MessageID),
		AssetID:         row.AssetID,
		SourceID:        pgconv.Deref(row.SourceID),
		ObservedAt:      pgconv.Time(row.ObservedAt),
		ReceivedAt:      pgconv.Time(row.ReceivedAt),
		Position:        position,
		Speed:           row.Speed,
		Heading:         row.Heading,
		Health:          pgconv.Deref(row.Health),
		ConnectionState: pgconv.Deref(row.ConnectionState),
		Payload:         pgconv.Map(row.Payload),
		CreatedAt:       pgconv.Time(row.CreatedAt),
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ Repository = (*PostgresRepository)(nil)
