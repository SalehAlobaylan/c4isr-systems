package missions

import (
	"context"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Service implements mission planning and execution workflows.
type Service struct {
	repo   Repository
	assets AssetRegistry
	bus    *events.Dispatcher
}

// NewService wires a mission service.
func NewService(repo Repository, assets AssetRegistry, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, assets: assets, bus: bus}
}

// Create plans a mission, assigns its assets, creates its tasks, and publishes
// mission.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Mission, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Mission{}, err
	}
	for _, assetID := range in.Assets {
		if strings.TrimSpace(assetID) == "" {
			return Mission{}, apperr.Validation("asset id is required")
		}
		exists, err := s.assets.Exists(ctx, strings.TrimSpace(assetID))
		if err != nil {
			return Mission{}, err
		}
		if !exists {
			return Mission{}, apperr.Validation("unknown asset: " + strings.TrimSpace(assetID))
		}
	}

	mission := Mission{
		ID:         ids.New("msn"),
		Name:       in.Name,
		Objective:  in.Objective,
		Priority:   in.Priority,
		Status:     StatusPlanned,
		IncidentID: in.IncidentID,
	}
	created, err := s.repo.Create(ctx, mission)
	if err != nil {
		return Mission{}, err
	}

	for _, assetID := range in.Assets {
		if err := s.repo.AssignAsset(ctx, created.ID, strings.TrimSpace(assetID)); err != nil {
			return Mission{}, err
		}
	}
	for _, task := range in.Tasks {
		if _, err := s.repo.CreateTask(ctx, Task{
			ID:          ids.New("tsk"),
			MissionID:   created.ID,
			Type:        task.Type,
			Description: task.Description,
			Status:      TaskPending,
			Target:      task.Target,
		}); err != nil {
			return Mission{}, err
		}
	}

	// Return the same hydrated representation that a subsequent GET returns.
	// Without this read-back, callers could successfully assign assets and
	// create tasks while receiving a mission with empty relation arrays.
	created, err = s.repo.Get(ctx, created.ID)
	if err != nil {
		return Mission{}, err
	}

	s.bus.Publish(ctx, events.MissionCreated{
		At:        time.Now().UTC(),
		MissionID: created.ID,
		Name:      created.Name,
		Status:    string(created.Status),
		Actor:     in.Actor,
	})
	return created, nil
}

// Get returns a mission together with its assets and tasks.
func (s *Service) Get(ctx context.Context, id string) (Mission, error) {
	return s.repo.Get(ctx, id)
}

// List returns missions newest first.
func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]Mission, int, error) {
	return s.repo.List(ctx, status, clampLimit(limit), max(offset, 0))
}

// UpdateStatus moves a mission through its lifecycle and publishes
// mission.updated.
func (s *Service) UpdateStatus(ctx context.Context, id string, status Status, actor string) (Mission, error) {
	if !validStatus(status) {
		return Mission{}, apperr.Validation("mission status must be one of PLANNED, ACTIVE, COMPLETED, ABORTED")
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return Mission{}, err
	}
	if !canTransition(current.Status, status) {
		return Mission{}, invalidTransition(current.Status, status)
	}
	updated, err := s.repo.UpdateStatus(ctx, id, status, current.Status)
	if err != nil {
		return Mission{}, err
	}
	s.publishUpdated(ctx, updated, actor)
	return updated, nil
}

// AssignAsset assigns an asset to a mission and publishes mission.updated.
func (s *Service) AssignAsset(ctx context.Context, missionID, assetID, actor string) (Mission, error) {
	missionID = strings.TrimSpace(missionID)
	assetID = strings.TrimSpace(assetID)
	if missionID == "" {
		return Mission{}, apperr.Validation("mission id is required")
	}
	if assetID == "" {
		return Mission{}, apperr.Validation("asset id is required")
	}
	current, err := s.repo.Get(ctx, missionID)
	if err != nil {
		return Mission{}, err
	}
	exists, err := s.assets.Exists(ctx, assetID)
	if err != nil {
		return Mission{}, err
	}
	if !exists {
		return Mission{}, apperr.Validation("unknown asset: " + assetID)
	}
	if err := s.repo.AssignAsset(ctx, missionID, assetID); err != nil {
		return Mission{}, err
	}
	s.publishUpdated(ctx, current, actor)
	return current, nil
}

// AddTask adds a task to a mission and publishes mission.updated.
func (s *Service) AddTask(ctx context.Context, missionID string, in TaskInput, actor string) (Task, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Task{}, err
	}
	missionID = strings.TrimSpace(missionID)
	if missionID == "" {
		return Task{}, apperr.Validation("mission id is required")
	}
	current, err := s.repo.Get(ctx, missionID)
	if err != nil {
		return Task{}, err
	}
	task, err := s.repo.CreateTask(ctx, Task{
		ID:          ids.New("tsk"),
		MissionID:   missionID,
		Type:        in.Type,
		Description: in.Description,
		Status:      TaskPending,
		Target:      in.Target,
	})
	if err != nil {
		return Task{}, err
	}
	s.publishUpdated(ctx, current, actor)
	return task, nil
}

// UpdateTaskStatus changes a task's status and publishes mission.updated.
func (s *Service) UpdateTaskStatus(ctx context.Context, missionID, taskID string, status TaskStatus, actor string) (Task, error) {
	if !validTaskStatus(status) {
		return Task{}, apperr.Validation("task status must be one of PENDING, ACTIVE, COMPLETED, FAILED, CANCELLED")
	}
	missionID = strings.TrimSpace(missionID)
	taskID = strings.TrimSpace(taskID)
	if missionID == "" {
		return Task{}, apperr.Validation("mission id is required")
	}
	if taskID == "" {
		return Task{}, apperr.Validation("task id is required")
	}
	current, err := s.repo.Get(ctx, missionID)
	if err != nil {
		return Task{}, err
	}
	task, err := s.repo.UpdateTaskStatus(ctx, missionID, taskID, status)
	if err != nil {
		return Task{}, err
	}
	s.publishUpdated(ctx, current, actor)
	return task, nil
}

func (s *Service) publishUpdated(ctx context.Context, mission Mission, actor string) {
	s.bus.Publish(ctx, events.MissionUpdated{
		At:        time.Now().UTC(),
		MissionID: mission.ID,
		Status:    string(mission.Status),
		Actor:     actor,
	})
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
