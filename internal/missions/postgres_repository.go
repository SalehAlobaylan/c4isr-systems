package missions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores missions in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a mission.
func (r *PostgresRepository) Create(ctx context.Context, mission Mission) (Mission, error) {
	row, err := dbgen.New(r.pool).CreateMission(ctx, dbgen.CreateMissionParams{
		ID:         mission.ID,
		Name:       mission.Name,
		Objective:  mission.Objective,
		Priority:   string(mission.Priority),
		Status:     string(mission.Status),
		IncidentID: pgconv.TextPtr(mission.IncidentID),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Mission{}, apperr.BadRequest("unknown incident: " + mission.IncidentID)
		}
		return Mission{}, err
	}
	return toDomain(row), nil
}

// Get loads a mission together with its assets and tasks.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Mission, error) {
	q := dbgen.New(r.pool)
	row, err := q.GetMission(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Mission{}, apperr.NotFound("mission", id)
		}
		return Mission{}, err
	}
	return r.withRelations(ctx, toDomain(row))
}

func (r *PostgresRepository) withRelations(ctx context.Context, mission Mission) (Mission, error) {
	q := dbgen.New(r.pool)
	assets, err := q.ListMissionAssets(ctx, mission.ID)
	if err != nil {
		return Mission{}, err
	}
	mission.Assets = make([]RelatedAsset, 0, len(assets))
	for _, asset := range assets {
		mission.Assets = append(mission.Assets, toRelatedAsset(asset))
	}

	tasks, err := q.ListMissionTasks(ctx, mission.ID)
	if err != nil {
		return Mission{}, err
	}
	mission.Tasks = make([]Task, 0, len(tasks))
	for _, task := range tasks {
		mission.Tasks = append(mission.Tasks, taskFromList(task))
	}

	return mission, nil
}

// List returns missions newest first with an optional status filter.
func (r *PostgresRepository) List(ctx context.Context, status string, limit, offset int) ([]Mission, int, error) {
	q := dbgen.New(r.pool)
	rows, err := q.ListMissions(ctx, dbgen.ListMissionsParams{
		Status:      pgconv.TextPtr(status),
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountMissions(ctx, pgconv.TextPtr(status))
	if err != nil {
		return nil, 0, err
	}
	out := make([]Mission, 0, len(rows))
	for _, row := range rows {
		mission, err := r.withRelations(ctx, toDomain(row))
		if err != nil {
			return nil, 0, err
		}
		out = append(out, mission)
	}
	return out, int(total), nil
}

// UpdateStatus changes mission lifecycle status.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id string, status Status) (Mission, error) {
	row, err := dbgen.New(r.pool).UpdateMissionStatus(ctx, dbgen.UpdateMissionStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Mission{}, apperr.NotFound("mission", id)
		}
		return Mission{}, err
	}
	return toDomain(row), nil
}

// AssignAsset assigns an asset to a mission.
func (r *PostgresRepository) AssignAsset(ctx context.Context, missionID, assetID string) error {
	err := dbgen.New(r.pool).AssignAssetToMission(ctx, dbgen.AssignAssetToMissionParams{
		MissionID: missionID,
		AssetID:   assetID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return apperr.BadRequest("unknown asset: " + assetID)
		}
		return err
	}
	return nil
}

// CreateTask inserts a mission task.
func (r *PostgresRepository) CreateTask(ctx context.Context, task Task) (Task, error) {
	params := dbgen.CreateMissionTaskParams{
		ID:          task.ID,
		MissionID:   task.MissionID,
		Type:        task.Type,
		Description: task.Description,
		Status:      string(task.Status),
	}
	if task.Target != nil {
		params.HasTarget = true
		params.Lat = task.Target.Lat
		params.Lng = task.Target.Lng
	}
	row, err := dbgen.New(r.pool).CreateMissionTask(ctx, params)
	if err != nil {
		if isForeignKeyViolation(err) {
			return Task{}, apperr.BadRequest("unknown mission: " + task.MissionID)
		}
		return Task{}, err
	}
	return taskFromCreate(row), nil
}

// ListTasks returns the tasks of a mission oldest first.
func (r *PostgresRepository) ListTasks(ctx context.Context, missionID string) ([]Task, error) {
	rows, err := dbgen.New(r.pool).ListMissionTasks(ctx, missionID)
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromList(row))
	}
	return out, nil
}

// UpdateTaskStatus changes a task's status.
func (r *PostgresRepository) UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) (Task, error) {
	row, err := dbgen.New(r.pool).UpdateMissionTaskStatus(ctx, dbgen.UpdateMissionTaskStatusParams{
		ID:     taskID,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, apperr.NotFound("mission task", taskID)
		}
		return Task{}, err
	}
	return taskFromUpdate(row), nil
}

type missionTaskRow struct {
	ID          string
	MissionID   string
	Type        string
	Description string
	Status      string
	CreatedAt   pgtype.Timestamptz
	UpdatedAt   pgtype.Timestamptz
	HasPosition bool
	Lat         float64
	Lng         float64
}

func toDomain(row dbgen.Mission) Mission {
	return Mission{
		ID:         row.ID,
		Name:       row.Name,
		Objective:  row.Objective,
		Priority:   Priority(row.Priority),
		Status:     Status(row.Status),
		IncidentID: pgconv.Deref(row.IncidentID),
		CreatedAt:  pgconv.Time(row.CreatedAt),
		UpdatedAt:  pgconv.Time(row.UpdatedAt),
		StartedAt:  pgconv.TimePtr(row.StartedAt),
		EndedAt:    pgconv.TimePtr(row.EndedAt),
	}
}

func toRelatedAsset(row dbgen.ListMissionAssetsRow) RelatedAsset {
	var position *geo.Point
	if row.HasPosition {
		position = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return RelatedAsset{
		ID:              row.ID,
		Name:            row.Name,
		Type:            row.Type,
		Status:          row.Status,
		Position:        position,
		ConnectionState: pgconv.Deref(row.ConnectionState),
	}
}

func toTaskDomain(row missionTaskRow) Task {
	var target *geo.Point
	if row.HasPosition {
		target = &geo.Point{Lat: row.Lat, Lng: row.Lng}
	}
	return Task{
		ID:          row.ID,
		MissionID:   row.MissionID,
		Type:        row.Type,
		Description: row.Description,
		Status:      TaskStatus(row.Status),
		Target:      target,
		CreatedAt:   pgconv.Time(row.CreatedAt),
		UpdatedAt:   pgconv.Time(row.UpdatedAt),
	}
}

func taskFromCreate(row dbgen.CreateMissionTaskRow) Task {
	return toTaskDomain(missionTaskRow(row))
}

func taskFromList(row dbgen.ListMissionTasksRow) Task {
	return toTaskDomain(missionTaskRow(row))
}

func taskFromUpdate(row dbgen.UpdateMissionTaskStatusRow) Task {
	return toTaskDomain(missionTaskRow(row))
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var _ Repository = (*PostgresRepository)(nil)
