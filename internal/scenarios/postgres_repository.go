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
	queries := dbgen.New(r.pool)
	rows, err := queries.ListScenarioRuns(ctx, dbgen.ListScenarioRunsParams{
		LimitCount:  int32(limit),
		OffsetCount: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := queries.CountScenarioRuns(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Run, 0, len(rows))
	for _, row := range rows {
		run := toDomain(row)
		events, eventErr := r.ListEvents(ctx, run.ID)
		if eventErr != nil {
			return nil, 0, eventErr
		}
		run.EventsTotal = len(events)
		run.EventsRun = countExecutedEvents(events)
		out = append(out, run)
	}
	return out, int(total), nil
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

// UpdateCursor persists the latest virtual-time/action cursor used by GetRun
// after the live engine has been released.
func (r *PostgresRepository) UpdateCursor(ctx context.Context, id string, virtualTimeMs int64, lastAction string, lastActionAt int64, actionError string) error {
	_, err := dbgen.New(r.pool).UpdateScenarioRunCursor(ctx, dbgen.UpdateScenarioRunCursorParams{
		ID:             id,
		VirtualTimeMs:  virtualTimeMs,
		LastAction:     pgconv.TextPtr(lastAction),
		LastActionAtMs: lastActionAt,
		ActionError:    pgconv.TextPtr(actionError),
	})
	return err
}

// CreateEvent persists or idempotently refreshes one scheduled action.
func (r *PostgresRepository) CreateEvent(ctx context.Context, runID string, event EventInspection) error {
	return dbgen.New(r.pool).CreateScenarioRunEvent(ctx, dbgen.CreateScenarioRunEventParams{
		RunID:         runID,
		Sequence:      int32(event.Sequence),
		VirtualTimeMs: event.AtMs,
		ActionName:    event.Name,
		Status:        event.Status,
		Error:         pgconv.TextPtr(event.Error),
	})
}

// UpdateEvent changes one action's durable lifecycle state.
func (r *PostgresRepository) UpdateEvent(ctx context.Context, runID string, sequence int, status, errorMessage string) error {
	return dbgen.New(r.pool).UpdateScenarioRunEvent(ctx, dbgen.UpdateScenarioRunEventParams{
		RunID:    runID,
		Sequence: int32(sequence),
		Status:   status,
		Error:    pgconv.TextPtr(errorMessage),
	})
}

// ListEvents returns events in virtual-time/sequence order.
func (r *PostgresRepository) ListEvents(ctx context.Context, runID string) ([]EventInspection, error) {
	rows, err := dbgen.New(r.pool).ListScenarioRunEvents(ctx, runID)
	if err != nil {
		return nil, err
	}
	items := make([]EventInspection, 0, len(rows))
	for _, row := range rows {
		items = append(items, EventInspection{
			Sequence: int(row.Sequence),
			AtMs:     row.VirtualTimeMs,
			Name:     row.ActionName,
			Status:   row.Status,
			Error:    pgconv.Deref(row.Error),
		})
	}
	return items, nil
}

// SkipPendingEvents makes unexecuted actions visible after cancellation or a
// failed run instead of leaving them indefinitely pending.
func (r *PostgresRepository) SkipPendingEvents(ctx context.Context, runID, errorMessage string) error {
	return dbgen.New(r.pool).SkipPendingScenarioRunEvents(ctx, dbgen.SkipPendingScenarioRunEventsParams{
		RunID: runID,
		Error: pgconv.TextPtr(errorMessage),
	})
}

func toDomain(row dbgen.ScenarioRun) Run {
	scope := NewRunScope(row.ID)
	return Run{
		ID:                row.ID,
		ResourceNamespace: scope.ResourceNamespace,
		ScenarioName:      row.ScenarioName,
		Seed:              row.Seed,
		Status:            row.Status,
		PlaybackSpeed:     row.PlaybackSpeed,
		VirtualTimeMs:     row.VirtualTimeMs,
		LastAction:        pgconv.Deref(row.LastAction),
		LastActionAt:      row.LastActionAtMs,
		ActionError:       pgconv.Deref(row.ActionError),
		StartedAt:         pgconv.Time(row.StartedAt),
		EndedAt:           pgconv.TimePtr(row.EndedAt),
		Error:             pgconv.Deref(row.Error),
	}
}

var _ Repository = (*PostgresRepository)(nil)
