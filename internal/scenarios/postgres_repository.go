package scenarios

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores scenario runs in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a scenario run.
func (r *PostgresRepository) Create(ctx context.Context, run Run) (Run, error) {
	row, err := dbgen.New(r.pool).CreateScenarioRun(ctx, dbgen.CreateScenarioRunParams{
		ID:            run.ID,
		ScenarioName:  run.ScenarioName,
		Seed:          run.Seed,
		Status:        run.Status,
		PlaybackSpeed: run.PlaybackSpeed,
	})
	if err != nil {
		return Run{}, err
	}
	return toDomain(row), nil
}

// Get loads a scenario run by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Run, error) {
	row, err := dbgen.New(r.pool).GetScenarioRun(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, apperr.NotFound("scenario run", id)
		}
		return Run{}, err
	}
	return toDomain(row), nil
}

// List returns scenario runs newest first.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Run, int, error) {
	rows, err := dbgen.New(r.pool).ListScenarioRuns(ctx, dbgen.ListScenarioRunsParams{
		LimitCount:  int32(limit),
		OffsetCount: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, len(out), nil
}

// UpdateStatus transitions a run's lifecycle status.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id, status, errorMessage string) (Run, error) {
	row, err := dbgen.New(r.pool).UpdateScenarioRunStatus(ctx, dbgen.UpdateScenarioRunStatusParams{
		ID:     id,
		Status: status,
		Error:  pgconv.TextPtr(errorMessage),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, apperr.NotFound("scenario run", id)
		}
		return Run{}, err
	}
	return toDomain(row), nil
}

// UpdateSpeed changes playback speed.
func (r *PostgresRepository) UpdateSpeed(ctx context.Context, id string, speed float64) (Run, error) {
	row, err := dbgen.New(r.pool).UpdateScenarioRunSpeed(ctx, dbgen.UpdateScenarioRunSpeedParams{
		ID:            id,
		PlaybackSpeed: speed,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, apperr.NotFound("scenario run", id)
		}
		return Run{}, err
	}
	return toDomain(row), nil
}

// UpdateProgress records the run's virtual time.
func (r *PostgresRepository) UpdateProgress(ctx context.Context, id string, virtualTimeMs int64) error {
	return dbgen.New(r.pool).UpdateScenarioRunProgress(ctx, dbgen.UpdateScenarioRunProgressParams{
		ID:            id,
		VirtualTimeMs: virtualTimeMs,
	})
}

func toDomain(row dbgen.ScenarioRun) Run {
	return Run{
		ID:            row.ID,
		ScenarioName:  row.ScenarioName,
		Seed:          row.Seed,
		Status:        row.Status,
		PlaybackSpeed: row.PlaybackSpeed,
		VirtualTimeMs: row.VirtualTimeMs,
		StartedAt:     pgconv.Time(row.StartedAt),
		EndedAt:       pgconv.TimePtr(row.EndedAt),
		Error:         pgconv.Deref(row.Error),
	}
}

var _ Repository = (*PostgresRepository)(nil)
