package classifications

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores classifications in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a classification hypothesis.
func (r *PostgresRepository) Create(ctx context.Context, classification Classification) (Classification, error) {
	row, err := dbgen.New(r.pool).CreateClassification(ctx, dbgen.CreateClassificationParams{
		ID:              classification.ID,
		TrackID:         classification.TrackID,
		Label:           classification.Label,
		Confidence:      classification.Confidence,
		Method:          string(classification.Method),
		SourceReference: pgconv.TextPtr(classification.SourceReference),
		CreatedBy:       pgconv.TextPtr(classification.CreatedBy),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Classification{}, apperr.BadRequest("unknown track: " + classification.TrackID)
		}
		return Classification{}, err
	}
	return toDomain(row), nil
}

// Get loads a classification by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Classification, error) {
	row, err := dbgen.New(r.pool).GetClassification(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Classification{}, apperr.NotFound("classification", id)
		}
		return Classification{}, err
	}
	return toDomain(row), nil
}

// ListByTrack returns a track's classifications newest first. The total is the
// returned page size; classifications are append-only and paging is rare.
func (r *PostgresRepository) ListByTrack(ctx context.Context, trackID string, limit, offset int) ([]Classification, int, error) {
	rows, err := dbgen.New(r.pool).ListClassificationsByTrack(ctx, dbgen.ListClassificationsByTrackParams{
		TrackID:     trackID,
		LimitCount:  int32(limit),
		OffsetCount: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Classification, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, len(out), nil
}

// LatestForTrack returns the most recent classification, or nil when the track
// has none.
func (r *PostgresRepository) LatestForTrack(ctx context.Context, trackID string) (*Classification, error) {
	row, err := dbgen.New(r.pool).LatestClassificationForTrack(ctx, trackID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	latest := toDomain(row)
	return &latest, nil
}

func toDomain(row dbgen.Classification) Classification {
	return Classification{
		ID:              row.ID,
		TrackID:         row.TrackID,
		Label:           row.Label,
		Confidence:      row.Confidence,
		Method:          Method(row.Method),
		SourceReference: pgconv.Deref(row.SourceReference),
		CreatedBy:       pgconv.Deref(row.CreatedBy),
		CreatedAt:       pgconv.Time(row.CreatedAt),
	}
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var _ Repository = (*PostgresRepository)(nil)
