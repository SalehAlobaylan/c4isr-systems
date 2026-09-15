package audit

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores audit entries in PostgreSQL.
type PostgresRepository struct {
	q *dbgen.Queries
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{q: dbgen.New(pool)}
}

// Append inserts an audit entry.
func (r *PostgresRepository) Append(ctx context.Context, entry Entry) (Entry, error) {
	row, err := r.q.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		ID:            entry.ID,
		OccurredAt:    pgconv.TS(entry.OccurredAt),
		ActorType:     string(entry.ActorType),
		ActorID:       pgconv.TextPtr(entry.ActorID),
		Action:        entry.Action,
		SubjectType:   pgconv.TextPtr(entry.SubjectType),
		SubjectID:     pgconv.TextPtr(entry.SubjectID),
		CorrelationID: pgconv.TextPtr(entry.CorrelationID),
		Data:          pgconv.JSONB(entry.Data),
	})
	if err != nil {
		return Entry{}, err
	}
	return toDomain(row), nil
}

// List returns audit entries matching the filter, newest first, with a total
// count.
func (r *PostgresRepository) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Entry, int, error) {
	since := pgconv.TS(derefTime(filter.Since))

	rows, err := r.q.ListAuditEvents(ctx, dbgen.ListAuditEventsParams{
		SubjectType: pgconv.TextPtr(filter.SubjectType),
		SubjectID:   pgconv.TextPtr(filter.SubjectID),
		Action:      pgconv.TextPtr(filter.Action),
		Since:       since,
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountAuditEvents(ctx, dbgen.CountAuditEventsParams{
		SubjectType: pgconv.TextPtr(filter.SubjectType),
		SubjectID:   pgconv.TextPtr(filter.SubjectID),
		Action:      pgconv.TextPtr(filter.Action),
		Since:       since,
	})
	if err != nil {
		return nil, 0, err
	}

	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

func toDomain(row dbgen.AuditEvent) Entry {
	return Entry{
		ID:            row.ID,
		OccurredAt:    pgconv.Time(row.OccurredAt),
		ActorType:     ActorType(row.ActorType),
		ActorID:       pgconv.Deref(row.ActorID),
		Action:        row.Action,
		SubjectType:   pgconv.Deref(row.SubjectType),
		SubjectID:     pgconv.Deref(row.SubjectID),
		CorrelationID: pgconv.Deref(row.CorrelationID),
		Data:          pgconv.Map(row.Data),
	}
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

var _ Repository = (*PostgresRepository)(nil)
