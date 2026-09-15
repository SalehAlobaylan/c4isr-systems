package commands

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

// PostgresRepository stores commands in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a command.
func (r *PostgresRepository) Create(ctx context.Context, command Command) (Command, error) {
	row, err := dbgen.New(r.pool).CreateCommand(ctx, dbgen.CreateCommandParams{
		ID:            command.ID,
		AssetID:       command.AssetID,
		MissionID:     pgconv.TextPtr(command.MissionID),
		IncidentID:    pgconv.TextPtr(command.IncidentID),
		Type:          command.Type,
		Payload:       pgconv.JSONB(command.Payload),
		State:         string(command.State),
		CreatedBy:     pgconv.TextPtr(command.CreatedBy),
		CorrelationID: pgconv.TextPtr(command.CorrelationID),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Command{}, apperr.BadRequest("command references an unknown asset, mission, or incident")
		}
		return Command{}, err
	}
	created := toDomain(row)
	if created.State == StateSent && created.SentAt == nil {
		return r.Transition(ctx, created.ID, StateSent, "")
	}
	return created, nil
}

// Get loads a command by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Command, error) {
	row, err := dbgen.New(r.pool).GetCommand(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Command{}, apperr.NotFound("command", id)
		}
		return Command{}, err
	}
	return toDomain(row), nil
}

// List returns commands newest first with optional filters.
func (r *PostgresRepository) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Command, int, error) {
	q := dbgen.New(r.pool)
	params := dbgen.ListCommandsParams{
		AssetID:     pgconv.TextPtr(filter.AssetID),
		State:       pgconv.TextPtr(filter.State),
		MissionID:   pgconv.TextPtr(filter.MissionID),
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	}
	rows, err := q.ListCommands(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountCommands(ctx, dbgen.CountCommandsParams{
		AssetID:   pgconv.TextPtr(filter.AssetID),
		State:     pgconv.TextPtr(filter.State),
		MissionID: pgconv.TextPtr(filter.MissionID),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Command, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

// Transition moves a command to a new state.
func (r *PostgresRepository) Transition(ctx context.Context, id string, state State, failureReason string) (Command, error) {
	row, err := dbgen.New(r.pool).TransitionCommand(ctx, dbgen.TransitionCommandParams{
		ID:            id,
		State:         string(state),
		FailureReason: pgconv.TextPtr(failureReason),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Command{}, apperr.NotFound("command", id)
		}
		return Command{}, err
	}
	return toDomain(row), nil
}

func toDomain(row dbgen.Command) Command {
	return Command{
		ID:             row.ID,
		AssetID:        row.AssetID,
		MissionID:      pgconv.Deref(row.MissionID),
		IncidentID:     pgconv.Deref(row.IncidentID),
		Type:           row.Type,
		Payload:        pgconv.Map(row.Payload),
		State:          State(row.State),
		CreatedBy:      pgconv.Deref(row.CreatedBy),
		CorrelationID:  pgconv.Deref(row.CorrelationID),
		CreatedAt:      pgconv.Time(row.CreatedAt),
		UpdatedAt:      pgconv.Time(row.UpdatedAt),
		QueuedAt:       pgconv.TimePtr(row.QueuedAt),
		SentAt:         pgconv.TimePtr(row.SentAt),
		AcknowledgedAt: pgconv.TimePtr(row.AcknowledgedAt),
		CompletedAt:    pgconv.TimePtr(row.CompletedAt),
		FailureReason:  pgconv.Deref(row.FailureReason),
	}
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var _ Repository = (*PostgresRepository)(nil)
