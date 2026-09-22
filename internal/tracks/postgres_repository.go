package tracks

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
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores tracks in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a track.
func (r *PostgresRepository) Create(ctx context.Context, track Track) (Track, error) {
	row, err := dbgen.New(r.pool).CreateTrack(ctx, dbgen.CreateTrackParams{
		ID:          track.ID,
		ExternalRef: pgconv.TextPtr(track.ExternalRef),
		Status:      string(track.Status),
		FirstSeenAt: pgconv.TS(track.FirstSeenAt),
		LastSeenAt:  pgconv.TS(track.LastSeenAt),
		Metadata:    pgconv.JSONB(track.Metadata),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tracks_external_ref_key" {
			return Track{}, ErrDuplicateExternalRef
		}
		return Track{}, err
	}
	return toDomain(row), nil
}

// Get loads a track by id, including its observation count and supporting
// source ids.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Track, error) {
	q := dbgen.New(r.pool)
	row, err := q.GetTrackDetail(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Track{}, apperr.NotFound("track", id)
		}
		return Track{}, err
	}
	track := fromGetRow(row)

	counts, err := q.CountObservationsBySourceForTrack(ctx, id)
	if err != nil {
		return Track{}, err
	}
	track.SourceIDs = make([]string, 0, len(counts))
	for _, count := range counts {
		track.SourceIDs = append(track.SourceIDs, count.SourceID)
	}
	return track, nil
}

// FindByExternalRef loads the track carrying an external correlation hint.
func (r *PostgresRepository) FindByExternalRef(ctx context.Context, ref string) (Track, error) {
	row, err := dbgen.New(r.pool).FindTrackByExternalRef(ctx, pgconv.TextPtr(ref))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Track{}, apperr.NotFound("track", ref)
		}
		return Track{}, err
	}
	return fromFindRow(row), nil
}

// FindByObservationID resolves the track that already owns an observation.
// Retry events use this durable evidence link so an unhinted observation is
// replayed into its original track instead of creating a second track.
func (r *PostgresRepository) FindByObservationID(ctx context.Context, observationID string) (Track, error) {
	row, err := dbgen.New(r.pool).FindTrackByObservationID(ctx, observationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Track{}, apperr.NotFound("track for observation", observationID)
		}
		return Track{}, err
	}
	return fromObservationRow(row), nil
}

// List returns tracks ordered by last seen descending with a total count.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Track, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListTrackDetails(ctx, dbgen.ListTrackDetailsParams{
		LimitCount:  int32(limit),
		OffsetCount: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountTracks(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Track, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromListRow(row))
	}
	return out, int(total), nil
}

// ApplyObservation advances the track watermark and state in one conditional
// database operation. The row-level update lock makes out-of-order concurrent
// observations deterministic: only the newest timestamp (or first projection)
// can win.
func (r *PostgresRepository) ApplyObservation(ctx context.Context, update StateUpdate, observedAt, expectedLastSeenAt time.Time) (bool, error) {
	params := dbgen.ApplyTrackObservationParams{
		TrackID:            update.TrackID,
		ObservedAt:         pgconv.TS(observedAt),
		ExpectedLastSeenAt: pgconv.TS(expectedLastSeenAt),
		Speed:              update.Speed,
		Heading:            update.Heading,
	}
	if update.Position != nil {
		params.HasPosition = true
		params.Lat = update.Position.Lat
		params.Lng = update.Position.Lng
	}
	return dbgen.New(r.pool).ApplyTrackObservation(ctx, params)
}

// ApplyObservationWithHistory commits a projection, its history point, and
// the observation process marker in one transaction. The caller only publishes
// a track update after this transaction succeeds.
func (r *PostgresRepository) ApplyObservationWithHistory(ctx context.Context, update StateUpdate, observedAt, expectedLastSeenAt time.Time, history *HistoryPoint, observationID string) (bool, error) {
	var applied bool
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		params := dbgen.ApplyTrackObservationParams{
			TrackID:            update.TrackID,
			ObservedAt:         pgconv.TS(observedAt),
			ExpectedLastSeenAt: pgconv.TS(expectedLastSeenAt),
			Speed:              update.Speed,
			Heading:            update.Heading,
		}
		if update.Position != nil {
			params.HasPosition = true
			params.Lat = update.Position.Lat
			params.Lng = update.Position.Lng
		}
		var err error
		applied, err = q.ApplyTrackObservation(ctx, params)
		if err != nil || !applied {
			return err
		}
		if history != nil {
			historyParams := dbgen.CreateTrackHistoryEntryParams{
				ID:         history.ID,
				TrackID:    history.TrackID,
				ObservedAt: pgconv.TS(history.ObservedAt),
				Speed:      history.Speed,
				Heading:    history.Heading,
			}
			if history.Position != nil {
				historyParams.HasPosition = true
				historyParams.Lat = history.Position.Lat
				historyParams.Lng = history.Position.Lng
			}
			if err := q.CreateTrackHistoryEntry(ctx, historyParams); err != nil {
				return err
			}
		}
		return q.MarkObservationProcessed(ctx, observationID)
	})
	return applied, err
}

// Touch advances the last-seen watermark without regressing it.
func (r *PostgresRepository) Touch(ctx context.Context, id string, observedAt time.Time) error {
	return dbgen.New(r.pool).TouchTrack(ctx, dbgen.TouchTrackParams{
		ObservedAt: pgconv.TS(observedAt),
		ID:         id,
	})
}

// Close marks a track closed.
func (r *PostgresRepository) Close(ctx context.Context, id string) (Track, error) {
	row, err := dbgen.New(r.pool).CloseTrack(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Track{}, apperr.NotFound("track", id)
		}
		return Track{}, err
	}
	return toDomain(row), nil
}

// UpsertState writes the current projected state of a track.
func (r *PostgresRepository) UpsertState(ctx context.Context, update StateUpdate) error {
	params := dbgen.UpsertTrackStateParams{
		TrackID: update.TrackID,
		Speed:   update.Speed,
		Heading: update.Heading,
	}
	if update.Position != nil {
		params.HasPosition = true
		params.Lat = update.Position.Lat
		params.Lng = update.Position.Lng
	}
	return dbgen.New(r.pool).UpsertTrackState(ctx, params)
}

// AttachObservation links evidence to a track. It returns false when the
// observation was already attached.
func (r *PostgresRepository) AttachObservation(ctx context.Context, trackID, observationID string) (bool, error) {
	affected, err := dbgen.New(r.pool).AttachObservationToTrack(ctx, dbgen.AttachObservationToTrackParams{
		TrackID:       trackID,
		ObservationID: observationID,
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// AddHistoryEntry appends a track history point.
func (r *PostgresRepository) AddHistoryEntry(ctx context.Context, entry HistoryPoint) error {
	params := dbgen.CreateTrackHistoryEntryParams{
		ID:         entry.ID,
		TrackID:    entry.TrackID,
		ObservedAt: pgconv.TS(entry.ObservedAt),
		Speed:      entry.Speed,
		Heading:    entry.Heading,
	}
	if entry.Position != nil {
		params.HasPosition = true
		params.Lat = entry.Position.Lat
		params.Lng = entry.Position.Lng
	}
	return dbgen.New(r.pool).CreateTrackHistoryEntry(ctx, params)
}

// ListHistory returns a track's history oldest first.
func (r *PostgresRepository) ListHistory(ctx context.Context, trackID string, limit int) ([]HistoryPoint, error) {
	rows, err := dbgen.New(r.pool).ListTrackHistory(ctx, dbgen.ListTrackHistoryParams{
		TrackID:    trackID,
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]HistoryPoint, 0, len(rows))
	for _, row := range rows {
		point := HistoryPoint{
			ID:         row.ID,
			TrackID:    trackID,
			ObservedAt: pgconv.Time(row.ObservedAt),
			Speed:      row.Speed,
			Heading:    row.Heading,
		}
		if row.HasPosition {
			point.Position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
		}
		out = append(out, point)
	}
	return out, nil
}

// CountObservations returns how many observations support a track.
func (r *PostgresRepository) CountObservations(ctx context.Context, trackID string) (int, error) {
	total, err := dbgen.New(r.pool).CountTrackObservations(ctx, trackID)
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

// CountObservationsBySource groups a track's supporting observations by
// source.
func (r *PostgresRepository) CountObservationsBySource(ctx context.Context, trackID string) ([]SourceCount, error) {
	rows, err := dbgen.New(r.pool).CountObservationsBySourceForTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	out := make([]SourceCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, SourceCount{SourceID: row.SourceID, Count: int(row.ObservationCount)})
	}
	return out, nil
}

// trackDetailRow normalizes the generated row shapes (identical columns across
// the track detail queries) before domain conversion.
type trackDetailRow struct {
	ID             string
	ExternalRef    *string
	Status         string
	FirstSeenAt    pgtype.Timestamptz
	LastSeenAt     pgtype.Timestamptz
	Metadata       []byte
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
	ClosedAt       pgtype.Timestamptz
	Speed          *float64
	Heading        *float64
	StateUpdatedAt pgtype.Timestamptz
	HasPosition    bool
	Lat            float64
	Lng            float64
}

func fromGetRow(row dbgen.GetTrackDetailRow) Track {
	track := fromDetailRow(trackDetailRow{
		ID:             row.ID,
		ExternalRef:    row.ExternalRef,
		Status:         row.Status,
		FirstSeenAt:    row.FirstSeenAt,
		LastSeenAt:     row.LastSeenAt,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		ClosedAt:       row.ClosedAt,
		Speed:          row.Speed,
		Heading:        row.Heading,
		StateUpdatedAt: row.StateUpdatedAt,
		HasPosition:    row.HasPosition,
		Lat:            row.Lat,
		Lng:            row.Lng,
	})
	track.ObservationCount = int(row.ObservationCount)
	return track
}

func fromFindRow(row dbgen.FindTrackByExternalRefRow) Track {
	return fromDetailRow(trackDetailRow{
		ID:             row.ID,
		ExternalRef:    row.ExternalRef,
		Status:         row.Status,
		FirstSeenAt:    row.FirstSeenAt,
		LastSeenAt:     row.LastSeenAt,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		ClosedAt:       row.ClosedAt,
		Speed:          row.Speed,
		Heading:        row.Heading,
		StateUpdatedAt: row.StateUpdatedAt,
		HasPosition:    row.HasPosition,
		Lat:            row.Lat,
		Lng:            row.Lng,
	})
}

func fromObservationRow(row dbgen.FindTrackByObservationIDRow) Track {
	return fromDetailRow(trackDetailRow{
		ID:             row.ID,
		ExternalRef:    row.ExternalRef,
		Status:         row.Status,
		FirstSeenAt:    row.FirstSeenAt,
		LastSeenAt:     row.LastSeenAt,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		ClosedAt:       row.ClosedAt,
		Speed:          row.Speed,
		Heading:        row.Heading,
		StateUpdatedAt: row.StateUpdatedAt,
		HasPosition:    row.HasPosition,
		Lat:            row.Lat,
		Lng:            row.Lng,
	})
}

func fromListRow(row dbgen.ListTrackDetailsRow) Track {
	track := fromDetailRow(trackDetailRow{
		ID:             row.ID,
		ExternalRef:    row.ExternalRef,
		Status:         row.Status,
		FirstSeenAt:    row.FirstSeenAt,
		LastSeenAt:     row.LastSeenAt,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		ClosedAt:       row.ClosedAt,
		Speed:          row.Speed,
		Heading:        row.Heading,
		StateUpdatedAt: row.StateUpdatedAt,
		HasPosition:    row.HasPosition,
		Lat:            row.Lat,
		Lng:            row.Lng,
	})
	track.ObservationCount = int(row.ObservationCount)
	return track
}

func fromDetailRow(row trackDetailRow) Track {
	track := Track{
		ID:          row.ID,
		ExternalRef: pgconv.Deref(row.ExternalRef),
		Status:      Status(row.Status),
		FirstSeenAt: pgconv.Time(row.FirstSeenAt),
		LastSeenAt:  pgconv.Time(row.LastSeenAt),
		Speed:       row.Speed,
		Heading:     row.Heading,
		Metadata:    pgconv.Map(row.Metadata),
		CreatedAt:   pgconv.Time(row.CreatedAt),
		UpdatedAt:   pgconv.Time(row.UpdatedAt),
		ClosedAt:    pgconv.TimePtr(row.ClosedAt),
	}
	if row.HasPosition {
		track.Position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return track
}

func toDomain(row dbgen.Track) Track {
	return Track{
		ID:          row.ID,
		ExternalRef: pgconv.Deref(row.ExternalRef),
		Status:      Status(row.Status),
		FirstSeenAt: pgconv.Time(row.FirstSeenAt),
		LastSeenAt:  pgconv.Time(row.LastSeenAt),
		Metadata:    pgconv.Map(row.Metadata),
		CreatedAt:   pgconv.Time(row.CreatedAt),
		UpdatedAt:   pgconv.Time(row.UpdatedAt),
		ClosedAt:    pgconv.TimePtr(row.ClosedAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)
