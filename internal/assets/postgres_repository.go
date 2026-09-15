package assets

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores assets in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts an asset and its state projection, returning the read model.
func (r *PostgresRepository) Create(ctx context.Context, asset Asset) (Asset, error) {
	q := dbgen.New(r.pool)
	row, err := q.CreateAsset(ctx, dbgen.CreateAssetParams{
		ID:           asset.ID,
		Name:         asset.Name,
		Type:         asset.Type,
		Status:       string(asset.Status),
		Capabilities: encodeCapabilities(asset.Capabilities),
		Metadata:     pgconv.JSONB(asset.Metadata),
	})
	if err != nil {
		return Asset{}, err
	}
	if err := q.EnsureAssetState(ctx, row.ID); err != nil {
		return Asset{}, err
	}
	return r.Get(ctx, row.ID)
}

// Get loads an asset with its current state by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Asset, error) {
	row, err := dbgen.New(r.pool).GetAssetDetail(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, apperr.NotFound("asset", id)
		}
		return Asset{}, err
	}
	return fromDetailRow(row), nil
}

// Exists reports whether an asset id is registered.
func (r *PostgresRepository) Exists(ctx context.Context, id string) (bool, error) {
	return dbgen.New(r.pool).AssetExists(ctx, id)
}

// List returns assets ordered by name with a total count.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Asset, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListAssetsWithState(ctx, dbgen.ListAssetsWithStateParams{
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountAssets(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Asset, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromListRow(row))
	}
	return out, int(total), nil
}

// UpdateStatus changes asset availability and returns the read model.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id string, status Status) (Asset, error) {
	_, err := dbgen.New(r.pool).UpdateAssetStatus(ctx, dbgen.UpdateAssetStatusParams{
		Status: string(status),
		ID:     id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, apperr.NotFound("asset", id)
		}
		return Asset{}, err
	}
	return r.Get(ctx, id)
}

type assetStateRow struct {
	ID              string
	Name            string
	Type            string
	Status          string
	Capabilities    []byte
	Metadata        []byte
	CreatedAt       pgtype.Timestamptz
	UpdatedAt       pgtype.Timestamptz
	Health          *string
	ConnectionState *string
	LastSeenAt      pgtype.Timestamptz
	StateSpeed      *float64
	StateHeading    *float64
	HasPosition     bool
	Lat             float64
	Lng             float64
}

func fromDetailRow(row dbgen.GetAssetDetailRow) Asset {
	return fromStateRow(assetStateRow(row))
}

func fromListRow(row dbgen.ListAssetsWithStateRow) Asset {
	return fromStateRow(assetStateRow(row))
}

func fromStateRow(row assetStateRow) Asset {
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return Asset{
		ID:              row.ID,
		Name:            row.Name,
		Type:            row.Type,
		Status:          Status(row.Status),
		Capabilities:    decodeCapabilities(row.Capabilities),
		Metadata:        pgconv.Map(row.Metadata),
		CreatedAt:       pgconv.Time(row.CreatedAt),
		UpdatedAt:       pgconv.Time(row.UpdatedAt),
		Position:        position,
		Speed:           row.StateSpeed,
		Heading:         row.StateHeading,
		Health:          pgconv.Deref(row.Health),
		ConnectionState: pgconv.Deref(row.ConnectionState),
		LastSeenAt:      pgconv.TimePtr(row.LastSeenAt),
	}
}

func encodeCapabilities(capabilities []string) []byte {
	if capabilities == nil {
		return []byte("[]")
	}
	return pgconv.JSONBArray(capabilities)
}

func decodeCapabilities(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		return []string{}
	}
	return out
}

var _ Repository = (*PostgresRepository)(nil)
