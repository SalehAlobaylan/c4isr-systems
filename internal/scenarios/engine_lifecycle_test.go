package scenarios

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

type memoryScenarioRepository struct {
	mu     sync.Mutex
	runs   map[string]Run
	events map[string]map[int]EventInspection
}

func newMemoryScenarioRepository() *memoryScenarioRepository {
	return &memoryScenarioRepository{
		runs:   map[string]Run{},
		events: map[string]map[int]EventInspection{},
	}
}

func (r *memoryScenarioRepository) Create(_ context.Context, run Run) (Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[run.ID]; exists {
		return Run{}, errors.New("duplicate run")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.ResourceNamespace == "" {
		run.ResourceNamespace = NewRunScope(run.ID).ResourceNamespace
	}
	r.runs[run.ID] = run
	r.events[run.ID] = map[int]EventInspection{}
	return run, nil
}

func (r *memoryScenarioRepository) Get(_ context.Context, id string) (Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return Run{}, apperr.NotFound("scenario run", id)
	}
	return run, nil
}

func (r *memoryScenarioRepository) List(_ context.Context, limit, offset int) ([]Run, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]Run, 0, len(r.runs))
	for _, run := range r.runs {
		items = append(items, run)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.After(items[j].StartedAt) })
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]Run(nil), items[offset:end]...), len(items), nil
}

func (r *memoryScenarioRepository) UpdateStatus(_ context.Context, id, status, message string) (Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return Run{}, apperr.NotFound("scenario run", id)
	}
	run.Status = status
	run.Error = message
	if terminalStatus(status) {
		ended := time.Now().UTC()
		run.EndedAt = &ended
	}
	r.runs[id] = run
	return run, nil
}

func (r *memoryScenarioRepository) UpdateSpeed(_ context.Context, id string, speed float64) (Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return Run{}, apperr.NotFound("scenario run", id)
	}
	run.PlaybackSpeed = speed
	r.runs[id] = run
	return run, nil
}

func (r *memoryScenarioRepository) UpdateProgress(_ context.Context, id string, virtualTimeMs int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.NotFound("scenario run", id)
	}
	run.VirtualTimeMs = virtualTimeMs
	r.runs[id] = run
	return nil
}

func (r *memoryScenarioRepository) UpdateCursor(_ context.Context, id string, virtualTimeMs int64, lastAction string, lastActionAt int64, actionError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.NotFound("scenario run", id)
	}
	run.VirtualTimeMs = virtualTimeMs
	run.LastAction = lastAction
	run.LastActionAt = lastActionAt
	run.ActionError = actionError
	r.runs[id] = run
	return nil
}

func (r *memoryScenarioRepository) CreateEvent(_ context.Context, runID string, event EventInspection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[runID]; !ok {
		return apperr.NotFound("scenario run", runID)
	}
	r.events[runID][event.Sequence] = event
	return nil
}

func (r *memoryScenarioRepository) UpdateEvent(_ context.Context, runID string, sequence int, status, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.events[runID][sequence]
	if !ok {
		return apperr.NotFound("scenario event", ids.New("missing"))
	}
	event.Status = status
	event.Error = message
	r.events[runID][sequence] = event
	return nil
}

func (r *memoryScenarioRepository) ListEvents(_ context.Context, runID string) ([]EventInspection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[runID]; !ok {
		return nil, apperr.NotFound("scenario run", runID)
	}
	items := make([]EventInspection, 0, len(r.events[runID]))
	for _, event := range r.events[runID] {
		items = append(items, event)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AtMs != items[j].AtMs {
			return items[i].AtMs < items[j].AtMs
		}
		return items[i].Sequence < items[j].Sequence
	})
	return items, nil
}

func (r *memoryScenarioRepository) SkipPendingEvents(_ context.Context, runID, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sequence, event := range r.events[runID] {
		if event.Status != EventPending && event.Status != EventRunning {
			continue
		}
		event.Status = EventSkipped
		if message != "" {
			event.Error = message
		}
		r.events[runID][sequence] = event
	}
	return nil
}

func TestEngineFailedActionIsCountedAndPendingActionsAreSkipped(t *testing.T) {
	repo := newMemoryScenarioRepository()
	scope := NewRunScope("run-test")
	runID := "run-test"
	if _, err := repo.Create(context.Background(), Run{ID: runID, ScenarioName: "failure", Status: StatusRunning}); err != nil {
		t.Fatal(err)
	}
	engine := newEngine(runID, scope, &Scenario{
		Name: "failure",
		Events: []EventSpec{
			{Action: ActionIncident, After: "0s", IncidentRef: "first"},
			{Action: ActionIncident, After: "1s", IncidentRef: "second"},
		},
	}, 1, 1000, EngineDeps{}, repo, nil)
	if err := engine.build(); err != nil {
		t.Fatal(err)
	}
	if err := engine.persistStaticEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.run(context.Background()); err == nil {
		t.Fatal("run should fail when the incident dependency is missing")
	}
	snapshot := engine.snapshot()
	if snapshot.EventsRun != 1 || snapshot.EventsRun > snapshot.EventsTotal {
		t.Fatalf("snapshot = %#v, want one executed action and bounded totals", snapshot)
	}
	items, err := repo.ListEvents(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Status != EventFailed || items[1].Status != EventSkipped {
		t.Fatalf("events = %#v, want failed then skipped", items)
	}
}

func TestEngineDynamicActionsArePersistedAndOrdered(t *testing.T) {
	repo := newMemoryScenarioRepository()
	runID := "run-dynamic"
	if _, err := repo.Create(context.Background(), Run{ID: runID, ScenarioName: "dynamic", Status: StatusRunning}); err != nil {
		t.Fatal(err)
	}
	engine := newEngine(runID, NewRunScope(runID), &Scenario{Name: "dynamic"}, 1, 1000, EngineDeps{}, repo, nil)
	engine.lastResume = time.Now()
	var order []string
	var orderMu sync.Mutex
	if err := engine.scheduleAfterMany(context.Background(), []dynamicAction{
		{Name: "first", DelayMs: 0, Run: func(context.Context) error {
			orderMu.Lock()
			order = append(order, "first")
			orderMu.Unlock()
			return nil
		}},
		{Name: "second", DelayMs: 0, Run: func(context.Context) error {
			orderMu.Lock()
			order = append(order, "second")
			orderMu.Unlock()
			return nil
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"first", "second"}; len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("execution order = %#v, want %#v", order, want)
	}
	snapshot := engine.snapshot()
	if snapshot.EventsTotal != 2 || snapshot.EventsRun != 2 || snapshot.EventsRun > snapshot.EventsTotal {
		t.Fatalf("snapshot = %#v, want two executed dynamic actions", snapshot)
	}
	items, err := repo.ListEvents(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Status != EventCompleted {
			t.Fatalf("dynamic event = %#v, want completed", item)
		}
	}
}

func TestScenarioRestartUsesFreshNamespaceAndCloseFinalizesOnce(t *testing.T) {
	dir := t.TempDir()
	content := []byte("scenario: lifecycle\ndescription: lifecycle test\nseed: 7\nplayback_speed: 1\nevents:\n  - after: 1h\n    action: incident\n    incident_ref: delayed\n")
	if err := os.WriteFile(filepath.Join(dir, "lifecycle.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	repo := newMemoryScenarioRepository()
	bus := events.NewDispatcher(nil)
	var endedMu sync.Mutex
	ended := map[string]int{}
	bus.Subscribe(events.TopicScenarioRunEnded, func(_ context.Context, event events.Event) {
		runEnded := event.(events.ScenarioRunEnded)
		endedMu.Lock()
		ended[runEnded.RunID]++
		endedMu.Unlock()
	})
	service := NewService(repo, NewLoader(dir), EngineDeps{}, bus)
	first, err := service.Start(context.Background(), "lifecycle", StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Restart(context.Background(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.ResourceNamespace == second.ResourceNamespace {
		t.Fatalf("restart reused run identity: first=%#v second=%#v", first, second)
	}
	old, err := repo.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != StatusStopped {
		t.Fatalf("old run status = %s, want STOPPED", old.Status)
	}
	oldEvents, err := repo.ListEvents(context.Background(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(oldEvents) != 1 || oldEvents[0].Status != EventSkipped {
		t.Fatalf("old events = %#v, want one skipped event", oldEvents)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	endedMu.Lock()
	defer endedMu.Unlock()
	if ended[first.ID] != 1 || ended[second.ID] != 1 {
		t.Fatalf("terminal events = %#v, want exactly one per run", ended)
	}
	newRun, err := repo.Get(context.Background(), second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if newRun.Status != StatusStopped {
		t.Fatalf("new run status = %s, want STOPPED after close", newRun.Status)
	}
}

func TestScenarioStartAndCloseCannotLeaveRunningRun(t *testing.T) {
	dir := t.TempDir()
	content := []byte("scenario: race\ndescription: lifecycle race\nseed: 1\nplayback_speed: 1\nevents:\n  - after: 1h\n    action: incident\n    incident_ref: delayed\n")
	if err := os.WriteFile(filepath.Join(dir, "race.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 20; iteration++ {
		repo := newMemoryScenarioRepository()
		service := NewService(repo, NewLoader(dir), EngineDeps{}, events.NewDispatcher(nil))
		startDone := make(chan struct{})
		go func() {
			_, _ = service.Start(context.Background(), "race", StartOptions{})
			close(startDone)
		}()
		closeDone := make(chan error, 1)
		go func() { closeDone <- service.Close(context.Background()) }()
		<-startDone
		if err := <-closeDone; err != nil {
			t.Fatal(err)
		}
		items, _, err := repo.List(context.Background(), 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range items {
			if !terminalStatus(run.Status) {
				t.Fatalf("iteration %d left run %#v non-terminal", iteration, run)
			}
		}
	}
}

func TestScenarioControlsReturnStableLifecycleErrors(t *testing.T) {
	repo := newMemoryScenarioRepository()
	if _, err := repo.Create(context.Background(), Run{ID: "terminal", ScenarioName: "terminal", Status: StatusCompleted}); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, NewLoader(t.TempDir()), EngineDeps{}, events.NewDispatcher(nil))

	terminalControls := []struct {
		name string
		call func() error
	}{
		{name: "pause", call: func() error { _, err := service.Pause(context.Background(), "terminal"); return err }},
		{name: "resume", call: func() error { _, err := service.Resume(context.Background(), "terminal"); return err }},
		{name: "speed", call: func() error { _, err := service.SetSpeed(context.Background(), "terminal", 2); return err }},
		{name: "stop", call: func() error { _, err := service.Stop(context.Background(), "terminal"); return err }},
	}
	for _, control := range terminalControls {
		t.Run(control.name+" terminal", func(t *testing.T) {
			if err := control.call(); !apperr.Is(err, apperr.CodeConflict) {
				t.Fatalf("error = %v, want conflict", err)
			}
		})
	}

	missingControls := []struct {
		name string
		call func() error
	}{
		{name: "pause", call: func() error { _, err := service.Pause(context.Background(), "missing"); return err }},
		{name: "resume", call: func() error { _, err := service.Resume(context.Background(), "missing"); return err }},
		{name: "speed", call: func() error { _, err := service.SetSpeed(context.Background(), "missing", 2); return err }},
		{name: "stop", call: func() error { _, err := service.Stop(context.Background(), "missing"); return err }},
	}
	for _, control := range missingControls {
		t.Run(control.name+" missing", func(t *testing.T) {
			if err := control.call(); !apperr.Is(err, apperr.CodeNotFound) {
				t.Fatalf("error = %v, want not found", err)
			}
		})
	}
}

func TestScenarioSetupDoesNotAdoptAnUnscopedResource(t *testing.T) {
	scope := NewRunScope("run-scope")
	if err := verifyScenarioResource("asset", scope.Asset("a"), map[string]any{
		"scenarioRunId":     "another-run",
		"resourceNamespace": "another-run__",
	}, scope); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("unscoped resource error = %v, want conflict", err)
	}
	if err := verifyScenarioResource("asset", scope.Asset("a"), map[string]any{
		"scenarioRunId":     scope.RunID,
		"resourceNamespace": scope.ResourceNamespace,
	}, scope); err != nil {
		t.Fatalf("matching resource metadata should be accepted: %v", err)
	}
}

func TestScenarioRestartFinalizesOrphanedRunBeforeStartingFreshNamespace(t *testing.T) {
	dir := t.TempDir()
	content := []byte("scenario: orphan\ndescription: orphan test\nseed: 9\nplayback_speed: 1\nevents:\n  - after: 1h\n    action: incident\n    incident_ref: delayed\n")
	if err := os.WriteFile(filepath.Join(dir, "orphan.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	repo := newMemoryScenarioRepository()
	if _, err := repo.Create(context.Background(), Run{ID: "orphaned", ScenarioName: "orphan", Status: StatusRunning}); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, NewLoader(dir), EngineDeps{}, events.NewDispatcher(nil))
	fresh, err := service.Restart(context.Background(), "orphaned")
	if err != nil {
		t.Fatal(err)
	}
	old, err := repo.Get(context.Background(), "orphaned")
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != StatusStopped {
		t.Fatalf("orphaned run status = %s, want STOPPED", old.Status)
	}
	if fresh.ID == old.ID || fresh.ResourceNamespace == old.ResourceNamespace {
		t.Fatalf("restart reused orphan namespace: old=%#v fresh=%#v", old, fresh)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
