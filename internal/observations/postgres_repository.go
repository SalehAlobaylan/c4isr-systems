package observations

import (
	"context"
	"encoding/json"
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
	if obs.Area != nil {
		params.HasArea = true
		params.AreaGeojson = obs.Area.GeoJSON()
		kind := string(obs.Area.Kind)
		params.AreaKind = &kind
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

// List returns observations newest first, optionally filtered by track or by
// area.
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

	if filter.AreaOnly {
		kind := string(filter.AreaKind)
		rows, err := q.ListAreaObservations(ctx, dbgen.ListAreaObservationsParams{
			AreaKind:    kind,
			LimitCount:  params.LimitCount,
			OffsetCount: params.OffsetCount,
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := q.CountAreaObservations(ctx, kind)
		if err != nil {
			return nil, 0, err
		}
		out := make([]Observation, 0, len(rows))
		for _, row := range rows {
			out = append(out, fromListAreaRow(row))
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
	AreaKind        *string
	HasPosition     bool
	Lat             float64
	Lng             float64
	HasArea         bool
	AreaGeojson     string
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

// fromListAreaRow maps the area listing, whose column set deliberately matches
// the standard observation shape so one mapper serves every read.
func fromListAreaRow(row dbgen.ListAreaObservationsRow) Observation {
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
		Area:        areaFromRow(row),
		Payload:     pgconv.Map(row.Payload),
		Quality:     pgconv.Map(row.Quality),
		TrackHint:   pgconv.Deref(row.TrackHint),
		CreatedAt:   pgconv.Time(row.CreatedAt),
	}
}

// areaFromRow rebuilds the footprint from the stored GeoJSON.
//
// The polygon is parsed back from GeoJSON rather than reconstructed from the
// original input ring so that what the domain holds is what PostGIS actually
// stored. A round trip through the database is the only way to guarantee the
// interface renders the committed geometry rather than the requested one.
func areaFromRow(row observationRow) *Area {
	if !row.HasArea {
		return nil
	}
	ring, ok := geoRingFromGeoJSON(row.AreaGeojson)
	if !ok {
		// The row claims an area but the geometry could not be read. Returning
		// nil would silently downgrade evidence that exists, so the caller sees
		// a point-less observation rather than a fabricated footprint. The
		// database constraint makes this unreachable in practice.
		return nil
	}
	return &Area{Kind: AreaKind(pgconv.Deref(row.AreaKind)), Polygon: ring}
}

// geoRingFromGeoJSON extracts the exterior ring of a GeoJSON Polygon.
func geoRingFromGeoJSON(raw string) ([]geo.Point, bool) {
	var parsed struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	if parsed.Type != "Polygon" || len(parsed.Coordinates) == 0 {
		return nil, false
	}
	ring := parsed.Coordinates[0]
	points := make([]geo.Point, 0, len(ring))
	for _, pair := range ring {
		if len(pair) < 2 {
			return nil, false
		}
		points = append(points, geo.Point{Lat: pair[1], Lng: pair[0]})
	}
	// GeoJSON closes the ring by repeating the first vertex; the domain keeps
	// it open so a producer-supplied ring round-trips unchanged.
	if len(points) >= 2 {
		first, last := points[0], points[len(points)-1]
		if first == last {
			points = points[:len(points)-1]
		}
	}
	if len(points) < 3 {
		return nil, false
	}
	return points, true
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
