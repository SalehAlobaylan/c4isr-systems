package missions

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type fakeRepo struct {
	missions map[string]Mission
	tasks    map[string]Task
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		missions: map[string]Mission{},
		tasks:    map[string]Task{},
	}
}

func (f *fakeRepo) Create(_ context.Context, mission Mission) (Mission, error) {
	now := time.Now().UTC()
	mission.CreatedAt = now
	mission.UpdatedAt = now
	f.missions[mission.ID] = mission
	return mission, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (Mission, error) {
	mission, ok := f.missions[id]
	if !ok {
		return Mission{}, apperr.NotFound("mission", id)
	}
	return mission, nil
}

func (f *fakeRepo) List(_ context.Context, status string, limit, offset int) ([]Mission, int, error) {
	out := make([]Mission, 0, len(f.missions))
	for _, mission := range f.missions {
		if status != "" && string(mission.Status) != status {
			continue
		}
		out = append(out, mission)
	}
	return out, len(out), nil
}

func (f *fakeRepo) UpdateStatus(ctx context.Context, id string, status Status) (Mission, error) {
	mission, err := f.Get(ctx, id)
	if err != nil {
		return Mission{}, err
	}
	mission.Status = status
	f.missions[id] = mission
	return mission, nil
}

func (f *fakeRepo) AssignAsset(_ context.Context, missionID, assetID string) error {
	return nil
}

func (f *fakeRepo) CreateTask(_ context.Context, task Task) (Task, error) {
	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now
	f.tasks[task.ID] = task
	return task, nil
}

func (f *fakeRepo) ListTasks(_ context.Context, missionID string) ([]Task, error) {
	out := make([]Task, 0)
	for _, task := range f.tasks {
		if task.MissionID == missionID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateTaskStatus(_ context.Context, taskID string, status TaskStatus) (Task, error) {
	task, ok := f.tasks[taskID]
	if !ok {
		return Task{}, apperr.NotFound("mission task", taskID)
	}
	task.Status = status
	f.tasks[taskID] = task
	return task, nil
}

type fakeRegistry struct{ known map[string]bool }

func (f fakeRegistry) Exists(_ context.Context, id string) (bool, error) {
	return f.known[id], nil
}

func testBus() *events.Dispatcher {
	return events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateRejectsUnknownAsset(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, testBus())
	_, err := svc.Create(context.Background(), CreateInput{Name: "Patrol", Assets: []string{"ast_missing"}})
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestCreatePublishesMissionCreated(t *testing.T) {
	bus := testBus()
	var created []events.MissionCreated
	bus.Subscribe(events.TopicMissionCreated, func(_ context.Context, ev events.Event) {
		created = append(created, ev.(events.MissionCreated))
	})
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, bus)

	mission, err := svc.Create(context.Background(), CreateInput{
		Name:   "Patrol",
		Assets: []string{"ast_1"},
		Tasks:  []TaskInput{{Type: "recon"}},
		Actor:  "op_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mission.Status != StatusPlanned {
		t.Fatalf("status = %s, want %s", mission.Status, StatusPlanned)
	}
	if len(created) != 1 || created[0].MissionID != mission.ID || created[0].Actor != "op_1" {
		t.Fatalf("unexpected events: %+v", created)
	}
}

func TestUpdateStatusRejectsInvalidTransition(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, testBus())
	mission, err := svc.Create(context.Background(), CreateInput{Name: "Patrol"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateStatus(context.Background(), mission.ID, StatusCompleted, "op_1"); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("err = %v, want conflict error", err)
	}
}

func TestUpdateTaskStatusRejectsInvalidStatus(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, testBus())
	mission, err := svc.Create(context.Background(), CreateInput{Name: "Patrol"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateTaskStatus(context.Background(), mission.ID, "tsk_1", TaskStatus("BOGUS"), "op_1"); !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}
