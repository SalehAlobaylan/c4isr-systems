package observations

import (
	"context"
	"errors"
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

// ErrDuplicateID is returned when a client-supplied observation id exists.
var ErrDuplicateID = errors.New("duplicate observation id")

// PostgresRepository stores observations in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts an observation, distinguishing duplicate ids.
func (r *PostgresRepository) Create(ctx context.Context, obs Observation) (Observation, error) {
	params := dbgen.CreateObservationParams{
		ID:              obs.ID,
		SourceID:        obs.SourceID,
		ObservationType: obs.Type,
		ObservedAt:      pgconv.TS(obs.ObservedAt),
		ReceivedAt:      pgconv.TS(obs.ReceivedAt),
		ProcessedAt:     tsOrNull(obs.ProcessedAt),
		Payload:         pgconv.JSONB(obs.Payload),
		TrackHint:       pgconv.TextPtr(obs.TrackHint),
	}
	if obs.Position != nil {
		params.HasPosition = true
		params.Lat = obs.Position.Lat
		params.Lng = obs.Position.Lng
	}
	if len(obs.Quality) > 0 {
		params.Quality = pgconv.JSONB(obs.Quality)
	}

	row, err := dbgen.New(r.pool).CreateObservation(ctx, params)
	if err != nil {
		if isUniqueViolation(err) {
			return Observation{}, ErrDuplicateID
		}
		if isForeignKeyViolation(err) {
			return Observation{}, apperr.BadRequest("unknown source: " + obs.SourceID)
		}
		return Observation{}, err
	}
	return toDomain(row), nil
}

// Get loads an observation by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Observation, error) {
	row, err := dbgen.New(r.pool).GetObservation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Observation{}, apperr.NotFound("observation", id)
		}
		return Observation{}, err
	}
	return fromGetRow(row), nil
}

// List returns observations newest first, optionally filtered by track.
func (r *PostgresRepository) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Observation, int, error) {
	q := dbgen.New(r.pool)
	params := dbgen.ListObservationsParams{LimitCount: int32(limit), OffsetCount: int32(offset)}

	if filter.TrackID != "" {
		rows, err := q.ListObservationsByTrack(ctx, dbgen.ListObservationsByTrackParams{
			TrackID:     filter.TrackID,
			LimitCount:  params.LimitCount,
			OffsetCount: params.OffsetCount,
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := q.CountObservationsByTrack(ctx, filter.TrackID)
		if err != nil {
			return nil, 0, err
		}
		out := make([]Observation, 0, len(rows))
		for _, row := range rows {
			out = append(out, fromListByTrackRow(row))
		}
		return out, int(total), nil
	}

	rows, err := q.ListObservations(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountObservations(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Observation, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromListRow(row))
	}
	return out, int(total), nil
}

// MarkProcessed records that the platform processed the observation.
func (r *PostgresRepository) MarkProcessed(ctx context.Context, id string) error {
	return dbgen.New(r.pool).MarkObservationProcessed(ctx, id)
}

// observationRow normalizes the generated row shapes (identical columns across
// observation queries) before domain conversion.
type observationRow struct {
	ID              string
	SourceID        string
	ObservationType string
	ObservedAt      pgtype.Timestamptz
	ReceivedAt      pgtype.Timestamptz
	ProcessedAt     pgtype.Timestamptz
	Payload         []byte
	Quality         []byte
	TrackHint       *string
	CreatedAt       pgtype.Timestamptz
	HasPosition     bool
	Lat             float64
	Lng             float64
}

func toDomain(row dbgen.CreateObservationRow) Observation {
	return fromRow(observationRow(row))
}

func fromGetRow(row dbgen.GetObservationRow) Observation {
	return fromRow(observationRow(row))
}

func fromListRow(row dbgen.ListObservationsRow) Observation {
	return fromRow(observationRow(row))
}

func fromListByTrackRow(row dbgen.ListObservationsByTrackRow) Observation {
	return fromRow(observationRow(row))
}

func fromRow(row observationRow) Observation {
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return Observation{
		ID:          row.ID,
		SourceID:    row.SourceID,
		Type:        row.ObservationType,
		ObservedAt:  pgconv.Time(row.ObservedAt),
		ReceivedAt:  pgconv.Time(row.ReceivedAt),
		ProcessedAt: pgconv.TimePtr(row.ProcessedAt),
		Position:    position,
		Payload:     pgconv.Map(row.Payload),
		Quality:     pgconv.Map(row.Quality),
		TrackHint:   pgconv.Deref(row.TrackHint),
		CreatedAt:   pgconv.Time(row.CreatedAt),
	}
}

func tsOrNull(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgconv.TS(*t)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var _ Repository = (*PostgresRepository)(nil)
