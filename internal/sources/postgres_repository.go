package sources

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores sources in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a source.
func (r *PostgresRepository) Create(ctx context.Context, source Source) (Source, error) {
	row, err := dbgen.New(r.pool).CreateSource(ctx, dbgen.CreateSourceParams{
		ID:       source.ID,
		Name:     source.Name,
		Type:     string(source.Type),
		Status:   string(source.Status),
		Metadata: pgconv.JSONB(source.Metadata),
	})
	if err != nil {
		return Source{}, err
	}
	return toDomain(row), nil
}

// Get loads a source by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Source, error) {
	row, err := dbgen.New(r.pool).GetSource(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Source{}, apperr.NotFound("source", id)
		}
		return Source{}, err
	}
	return toDomain(row), nil
}

// Exists reports whether a source id is registered.
func (r *PostgresRepository) Exists(ctx context.Context, id string) (bool, error) {
	return dbgen.New(r.pool).SourceExists(ctx, id)
}

// List returns sources ordered newest first with a total count.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Source, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListSources(ctx, dbgen.ListSourcesParams{LimitCount: int32(limit), OffsetCount: int32(offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountSources(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

// UpdateStatus changes source availability.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id string, status Status) (Source, error) {
	row, err := dbgen.New(r.pool).UpdateSourceStatus(ctx, dbgen.UpdateSourceStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Source{}, apperr.NotFound("source", id)
		}
		return Source{}, err
	}
	return toDomain(row), nil
}

func toDomain(row dbgen.Source) Source {
	return Source{
		ID:        row.ID,
		Name:      row.Name,
		Type:      Type(row.Type),
		Status:    Status(row.Status),
		Metadata:  pgconv.Map(row.Metadata),
		CreatedAt: pgconv.Time(row.CreatedAt),
		UpdatedAt: pgconv.Time(row.UpdatedAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)
