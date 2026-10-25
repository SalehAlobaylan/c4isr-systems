package scenarios

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
)

// DefaultLimit and MaxLimit bound run list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// StartOptions overrides scenario defaults at launch.
type StartOptions struct {
	Speed          float64
	Seed           *int64
	frozen         *FrozenRun
	definition     *Scenario
	contextBundles []contextpackage.Bundle
}

// Service is the application layer for scenario control. The runner itself is
// an external source of information; runs are supervised and controllable but
// never bypass domain services.
type Service struct {
	NativeRun func(context.Context, string) (Run, error)
	repo      Repository
	loader    *Loader
	deps      EngineDeps
	bus       *events.Dispatcher

	mu sync.Mutex
	// lifecycleMu serializes startup and shutdown. Without it, a Start that is
	// still setting up domain resources could register an engine after Close
	// had already waited for the previous engine set.
	lifecycleMu sync.Mutex
	engines     map[string]*engine
	cancels     map[string]context.CancelFunc
	locks       map[string]*sync.Mutex
	closed      bool
	wg          sync.WaitGroup
}

// NewService wires the scenario application service.
func NewService(repo Repository, loader *Loader, deps EngineDeps, bus *events.Dispatcher) *Service {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if bus == nil {
		bus = events.NewDispatcher(deps.Logger)
	}
	s := &Service{
		repo:    repo,
		loader:  loader,
		deps:    deps,
		bus:     bus,
		engines: map[string]*engine{},
		cancels: map[string]context.CancelFunc{},
		locks:   map[string]*sync.Mutex{},
	}
	bus.Subscribe(events.TopicCommandIssued, s.handleCommandIssued)
	return s
}

// ListScenarios returns the scenario definitions available on disk.
func (s *Service) ListScenarios(ctx context.Context) ([]Summary, error) {
	if err := access.Require(ctx, "sim.scenarios.author"); err != nil {
		return nil, err
	}
	return s.loader.List()
}

// GetScenario returns one parsed scenario definition.
func (s *Service) GetScenario(ctx context.Context, name string) (*Scenario, error) {
	if err := access.Require(ctx, "sim.scenarios.author"); err != nil {
		return nil, err
	}
	return s.loader.Load(name)
}

// Start registers scenario entities through the normal application services,
// then begins executing the deterministic timeline.
func (s *Service) Start(ctx context.Context, name string, opts StartOptions) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.startLocked(ctx, name, opts)
}

func (s *Service) startLocked(ctx context.Context, name string, opts StartOptions) (Run, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Run{}, apperr.Conflict("scenario service is closed")
	}
	s.mu.Unlock()
	sc := opts.definition
	var err error
	if sc == nil {
		sc, err = s.loader.Load(name)
		if err != nil {
			return Run{}, err
		}
	}

	seed := sc.Seed
	if opts.Seed != nil {
		seed = *opts.Seed
	}
	speed := sc.PlaybackSpeed
	if speed <= 0 {
		speed = defaultPlaybackSpeed
	}
	if math.IsNaN(opts.Speed) || math.IsInf(opts.Speed, 0) || opts.Speed < 0 {
		return Run{}, apperr.Validation("speed must be finite and non-negative")
	}
	if opts.Speed > 0 {
		speed = opts.Speed
	}

	clock := defaultClock()
	if sc.Clock != nil {
		clock = *sc.Clock
	}
	frozen := FrozenRun{RuntimeBuild: runtimeBuild(), Schema: "emad.run.v1", Runtime: RuntimeVersion, Seed: seed, InitialScale: speed, Clock: clock}
	if opts.frozen != nil {
		frozen = *opts.frozen
		frozen.InitialScale = speed
		seed = frozen.Seed
		clock = frozen.Clock
	}
	frozenBundles := opts.contextBundles
	if frozen.VersionID == "" {
		repo, ok := s.repo.(versionRepository)
		if !ok {
			return Run{}, apperr.Internal(fmt.Errorf("version repository unavailable"))
		}
		bundles := []contextpackage.Bundle{}
		if len(sc.ContextPackages) > 0 {
			if s.deps.ContextPackages == nil {
				return Run{}, apperr.Conflict("context package store unavailable")
			}
			resource, _ := access.ResourceFrom(ctx)
			resource.Partition = access.PublicEvidence
			resource.RunID = ""
			publicCtx, err := access.Rebind(ctx, resource)
			if err != nil {
				return Run{}, err
			}
			for _, id := range sc.ContextPackages {
				b, err := s.deps.ContextPackages.Load(publicCtx, id)
				if err != nil {
					return Run{}, err
				}
				if err = b.Validate(time.Now(), true); err != nil {
					return Run{}, apperr.Validation(err.Error())
				}
				bundles = append(bundles, b)
			}
		}
		frozenBundles = bundles
		frozen.ContextPackageIDs = append([]string{}, sc.ContextPackages...)
		id, err := repo.SaveVersion(ctx, Version{RuntimeBuild: runtimeBuild(), Schema: "emad.scenario.v1", Definition: sc, Runtime: RuntimeVersion, Context: bundles})
		if err != nil {
			return Run{}, err
		}
		frozen.VersionID = id
	}
	runID := ids.New("scn")
	scope := NewRunScope(runID)
	ctx, err = access.NewRun(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	run, err := s.repo.Create(ctx, Run{
		Frozen:            frozen,
		ID:                runID,
		ResourceNamespace: scope.ResourceNamespace,
		ScenarioName:      sc.Name,
		Seed:              seed,
		Status:            StatusRunning,
		PlaybackSpeed:     speed,
	})
	if err != nil {
		return Run{}, err
	}
	resource, _ := access.ResourceFrom(ctx)
	scopedCtx := runctx.WithScope(access.Internal(ctx, resource, "scenario-runner"), runctx.Scope{RunID: scope.RunID, ResourceNamespace: scope.ResourceNamespace})
	scopedCtx = freshness.WithClock(scopedCtx, s.ClockNow)
	s.bus.Publish(scopedCtx, events.ScenarioRunStarted{
		At:           time.Now().UTC(),
		RunID:        run.ID,
		ScenarioName: sc.Name,
		Seed:         seed,
	})

	if err := s.setup(scopedCtx, sc, scope); err != nil {
		s.failRun(scopedCtx, run.ID, scope, err)
		return Run{}, err
	}

	progress := func(ms int64) {
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(scopedCtx), 2*time.Second)
		defer cancel()
		_ = s.repo.UpdateProgress(updateCtx, run.ID, ms)
	}
	eng := newEngine(run.ID, scope, sc, seed, speed, s.deps, s.repo, progress)
	eng.epoch = clock.Epoch
	eng.versionID = frozen.VersionID
	eng.contextBundles = frozenBundles
	if err := eng.build(); err != nil {
		s.failRun(scopedCtx, run.ID, scope, err)
		return Run{}, err
	}
	if err := eng.persistStaticEvents(scopedCtx); err != nil {
		s.failRun(scopedCtx, run.ID, scope, err)
		return Run{}, err
	}

	// The run outlives the HTTP request that starts it; it is controlled via
	// pause/resume/speed/stop and cancelled on process shutdown.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(scopedCtx))
	runCtx = runctx.WithScope(runCtx, runctx.Scope{RunID: scope.RunID, ResourceNamespace: scope.ResourceNamespace})
	s.mu.Lock()
	s.engines[run.ID] = eng
	s.cancels[run.ID] = cancel
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		runErr := eng.run(runCtx)

		status := StatusCompleted
		message := ""
		switch {
		case eng.IsStopped():
			status = StatusStopped
		case runErr != nil:
			status = StatusFailed
			message = runErr.Error()
		}

		if _, err := s.finalizeRun(runCtx, run.ID, scope, status, message); err != nil {
			s.deps.Logger.Error("finalize scenario run", "run_id", run.ID, "error", err)
		}

		s.mu.Lock()
		delete(s.engines, run.ID)
		delete(s.cancels, run.ID)
		s.mu.Unlock()
	}()

	snapshot := eng.snapshot()
	run.ResourceNamespace = scope.ResourceNamespace
	run.EventsRun = snapshot.EventsRun
	run.EventsTotal = snapshot.EventsTotal
	if err := access.Activate(ctx); err != nil {
		return Run{}, err
	}
	return run, nil
}

// Pause freezes a running scenario.
func (s *Service) Pause(ctx context.Context, runID string) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if terminalStatus(run.Status) {
		return Run{}, apperr.Conflict("scenario run is terminal")
	}
	eng, err := s.activeEngine(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	run, err = s.repo.UpdateStatus(ctx, runID, StatusPaused, "")
	if err != nil {
		return Run{}, err
	}
	eng.Pause()
	if err := s.repo.UpdateProgress(ctx, runID, eng.VirtualTimeMs()); err != nil {
		return Run{}, err
	}
	s.bus.Publish(runctx.WithScope(ctx, runctx.Scope{RunID: runID, ResourceNamespace: NewRunScope(runID).ResourceNamespace}), events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return s.responseRun(ctx, run), nil
}

// Resume continues a paused scenario.
func (s *Service) Resume(ctx context.Context, runID string) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if terminalStatus(run.Status) {
		return Run{}, apperr.Conflict("scenario run is terminal")
	}
	eng, err := s.activeEngine(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	run, err = s.repo.UpdateStatus(ctx, runID, StatusRunning, "")
	if err != nil {
		return Run{}, err
	}
	eng.Resume()
	s.bus.Publish(runctx.WithScope(ctx, runctx.Scope{RunID: runID, ResourceNamespace: NewRunScope(runID).ResourceNamespace}), events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return s.responseRun(ctx, run), nil
}

// SetSpeed changes playback speed.
func (s *Service) SetSpeed(ctx context.Context, runID string, speed float64) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return Run{}, apperr.Validation("speed must be positive")
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if terminalStatus(run.Status) {
		return Run{}, apperr.Conflict("scenario run is terminal")
	}
	eng, err := s.activeEngine(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	run, err = s.repo.UpdateSpeed(ctx, runID, speed)
	if err != nil {
		return Run{}, err
	}
	eng.SetSpeed(speed)
	s.bus.Publish(runctx.WithScope(ctx, runctx.Scope{RunID: runID, ResourceNamespace: NewRunScope(runID).ResourceNamespace}), events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return s.responseRun(ctx, run), nil
}

// Stop cancels a running scenario.
func (s *Service) Stop(ctx context.Context, runID string) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if terminalStatus(run.Status) {
		return Run{}, apperr.Conflict("scenario run is terminal")
	}
	s.mu.Lock()
	eng, ok := s.engines[runID]
	cancel := s.cancels[runID]
	s.mu.Unlock()
	if !ok {
		return Run{}, apperr.Conflict("scenario run is not active")
	}
	eng.Stop()
	if cancel != nil {
		cancel()
	}
	scope := NewRunScope(runID)
	finalized, err := s.finalizeRunLocked(ctx, runID, scope, StatusStopped, "")
	if err != nil {
		return Run{}, err
	}
	return s.responseRun(ctx, finalized), nil
}

// GetRun returns persisted run state merged with live engine state.
func (s *Service) GetRun(ctx context.Context, runID string) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.read"); scopeErr != nil {
		return Run{}, scopeErr
	}

	run, err := s.repo.Get(ctx, runID)
	if apperr.Is(err, apperr.CodeNotFound) && s.NativeRun != nil {
		return s.NativeRun(ctx, runID)
	}
	if err != nil {
		return Run{}, err
	}
	return s.responseRun(ctx, run), nil
}

func (s *Service) responseRun(ctx context.Context, run Run) Run {
	s.mu.Lock()
	eng := s.engines[run.ID]
	s.mu.Unlock()
	if eng != nil && !terminalStatus(run.Status) {
		run.VirtualTimeMs = eng.VirtualTimeMs()
		snapshot := eng.snapshot()
		run.LastAction = snapshot.LastAction
		run.LastActionAt = snapshot.LastActionAt
		run.ActionError = snapshot.ActionError
		run.EventsRun = snapshot.EventsRun
		run.EventsTotal = snapshot.EventsTotal
		if eng.IsPaused() {
			run.Status = StatusPaused
		}
		return run
	}
	events, eventErr := s.repo.ListEvents(ctx, run.ID)
	if eventErr == nil && len(events) > 0 {
		run.EventsTotal = len(events)
		run.EventsRun = countExecutedEvents(events)
	}
	return run
}

// InspectEvents returns the runner's scheduled event view. For completed or
// stopped runs the persisted lifecycle status is used to provide a useful
// terminal summary even after the in-memory engine has been released.
func (s *Service) InspectEvents(ctx context.Context, runID string) ([]EventInspection, error) {
	if scopeErr := access.Require(ctx, "sim.scenarios.author"); scopeErr != nil {
		return nil, scopeErr
	}

	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	eng := s.engines[runID]
	s.mu.Unlock()
	if eng != nil && !terminalStatus(run.Status) {
		return eng.snapshotEvents(), nil
	}
	if persisted, err := s.repo.ListEvents(ctx, runID); err == nil && len(persisted) > 0 {
		return persisted, nil
	}
	version, err := s.Version(ctx, run.ID)
	sc := version.Definition
	if err != nil {
		return nil, err
	}
	temporary := newEngine(run.ID, NewRunScope(run.ID), sc, run.Seed, run.PlaybackSpeed, EngineDeps{Logger: slog.Default()}, nil, nil)
	if err := temporary.build(); err != nil {
		return nil, err
	}
	items := temporary.snapshotEvents()
	for index := range items {
		if run.Status == StatusCompleted {
			items[index].Status = EventCompleted
		} else if run.Status == StatusStopped || run.Status == StatusFailed {
			items[index].Status = EventSkipped
		}
	}
	return items, nil
}

// Restart stops an active run, then starts a fresh run with the original
// scenario and seed. A new run id makes replay lineage explicit in audit and
// prevents mutable state from masquerading as a replay.
func (s *Service) Restart(ctx context.Context, runID string) (Run, error) {
	if scopeErr := access.Require(ctx, "scenarios.control"); scopeErr != nil {
		return Run{}, scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Run{}, apperr.Conflict("scenario service is closed")
	}
	s.mu.Unlock()
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	previous, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	if previous.Frozen.VersionID == "" {
		return Run{}, apperr.Conflict("legacy run has no frozen inputs; start a new scenario explicitly")
	}
	repo, ok := s.repo.(versionRepository)
	if !ok {
		return Run{}, apperr.Internal(fmt.Errorf("version repository unavailable"))
	}
	version, err := repo.LoadVersion(ctx, previous.Frozen.VersionID)
	if err != nil {
		return Run{}, err
	}
	if version.Runtime != RuntimeVersion || version.RuntimeBuild != runtimeBuild() {
		return Run{}, apperr.Conflict("re-simulation requires the original runtime build; recorded evidence remains available")
	}
	s.mu.Lock()
	_, active := s.engines[runID]
	s.mu.Unlock()
	if active {
		s.mu.Lock()
		eng := s.engines[runID]
		cancel := s.cancels[runID]
		s.mu.Unlock()
		eng.Stop()
		if cancel != nil {
			cancel()
		}
		if _, err := s.finalizeRunLocked(ctx, runID, NewRunScope(runID), StatusStopped, ""); err != nil {
			return Run{}, err
		}
	} else if !terminalStatus(previous.Status) {
		// A process restart or an earlier failed cleanup can leave a persisted
		// RUNNING/PAUSED row without an in-memory engine. Close that lineage
		// before creating the fresh namespace so restart never leaves a ghost
		// active run behind.
		if _, err := s.finalizeRunLocked(ctx, runID, NewRunScope(runID), StatusStopped, ""); err != nil {
			return Run{}, err
		}
	}
	frozen := previous.Frozen
	frozen.RestartOf = previous.ID
	return s.startLocked(ctx, previous.ScenarioName, StartOptions{Speed: previous.PlaybackSpeed, Seed: &previous.Seed, frozen: &frozen, definition: version.Definition, contextBundles: version.Context})
}

// ListRuns returns scenario runs newest first.
func (s *Service) ListRuns(ctx context.Context, limit, offset int) ([]Run, int, error) {
	if scopeErr := access.Require(ctx, "scenarios.read"); scopeErr != nil {
		return nil, 0, scopeErr
	}

	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	items, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 && s.NativeRun != nil {
		r, _ := access.ResourceFrom(ctx)
		run, e := s.NativeRun(ctx, r.RunID)
		if e == nil {
			if offset > 0 {
				return []Run{}, 1, nil
			}
			return []Run{run}, 1, nil
		}
		if !apperr.Is(e, apperr.CodeNotFound) {
			return nil, 0, e
		}
	}
	for i := range items {
		s.mu.Lock()
		active := s.engines[items[i].ID] != nil
		s.mu.Unlock()
		if active && !terminalStatus(items[i].Status) {
			items[i] = s.responseRun(ctx, items[i])
		}
	}
	return items, total, nil
}

func (s *Service) activeEngine(ctx context.Context, runID string) (*engine, error) {
	s.mu.Lock()
	eng, ok := s.engines[runID]
	s.mu.Unlock()
	if ok {
		return eng, nil
	}
	if _, err := s.repo.Get(ctx, runID); err != nil {
		return nil, err
	}
	return nil, apperr.Conflict("scenario run is not active")
}

func (s *Service) runLock(runID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lock := s.locks[runID]; lock != nil {
		return lock
	}
	lock := &sync.Mutex{}
	s.locks[runID] = lock
	return lock
}

func (s *Service) handleCommandIssued(ctx context.Context, ev events.Event) {
	issued, ok := ev.(events.CommandIssued)
	if !ok {
		return
	}
	s.mu.Lock()
	engines := make([]*engine, 0, len(s.engines))
	for _, eng := range s.engines {
		engines = append(engines, eng)
	}
	s.mu.Unlock()

	for _, eng := range engines {
		if eng.ownsAsset(issued.AssetID) {
			eng.simulateCommand(ctx, issued)
			return
		}
	}
}

func (s *Service) setup(ctx context.Context, sc *Scenario, scope RunScope) error {
	for _, src := range sc.Sources {
		if s.deps.Sources == nil {
			return apperr.Internal(fmt.Errorf("scenario source dependency is not configured"))
		}
		name := src.Name
		if name == "" {
			name = src.ID
		}
		sourceType := src.Type
		if sourceType == "" {
			sourceType = string(sources.TypeSynthetic)
		}
		created, isNew, err := s.deps.Sources.Ensure(ctx, sources.CreateInput{
			ID:   scope.Source(src.ID),
			Name: name,
			Type: sources.Type(sourceType),
			Metadata: map[string]any{
				"scenarioRunId": scope.RunID, "resourceNamespace": scope.ResourceNamespace, "scenarioEntityId": src.ID,
			},
		})
		if err != nil {
			return err
		}
		if !isNew {
			if err := verifyScenarioResource("source", scope.Source(src.ID), created.Metadata, scope); err != nil {
				return err
			}
		}
	}

	for _, a := range sc.Assets {
		if s.deps.Assets == nil {
			return apperr.Internal(fmt.Errorf("scenario asset dependency is not configured"))
		}
		name := a.Name
		if name == "" {
			name = a.ID
		}
		assetType := a.Type
		if assetType == "" {
			assetType = "vehicle"
		}
		created, isNew, err := s.deps.Assets.Ensure(ctx, assets.CreateInput{
			ID:   scope.Asset(a.ID),
			Name: name,
			Type: assetType,
			Metadata: map[string]any{
				"scenarioRunId": scope.RunID, "resourceNamespace": scope.ResourceNamespace, "scenarioEntityId": a.ID,
			},
		})
		if err != nil {
			return err
		}
		if !isNew {
			if err := verifyScenarioResource("asset", scope.Asset(a.ID), created.Metadata, scope); err != nil {
				return err
			}
		}
	}

	for _, gf := range sc.Geofences {
		if s.deps.Geofences == nil {
			return apperr.Internal(fmt.Errorf("scenario geofence dependency is not configured"))
		}
		polygon := make([]geo.Point, 0, len(gf.Polygon))
		for _, p := range gf.Polygon {
			polygon = append(polygon, p.Point())
		}
		created, isNew, err := s.deps.Geofences.Ensure(ctx, geospatial.CreateInput{
			ID:       scope.Geofence(gf.ID),
			Name:     gf.Name,
			Type:     geospatial.Type(gf.Type),
			Severity: geospatial.Severity(gf.Severity),
			Polygon:  polygon,
			Metadata: map[string]any{
				"scenarioRunId": scope.RunID, "resourceNamespace": scope.ResourceNamespace, "scenarioEntityId": gf.ID,
			},
		})
		if err != nil {
			return err
		}
		if !isNew {
			if err := verifyScenarioResource("geofence", scope.Geofence(gf.ID), created.Metadata, scope); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyScenarioResource(kind, id string, metadata map[string]any, scope RunScope) error {
	runID, runOK := metadata["scenarioRunId"].(string)
	namespace, namespaceOK := metadata["resourceNamespace"].(string)
	if !runOK || !namespaceOK || strings.TrimSpace(runID) != scope.RunID || strings.TrimSpace(namespace) != scope.ResourceNamespace {
		return apperr.Conflict(fmt.Sprintf("%s %q already exists outside scenario run scope", kind, id))
	}
	return nil
}

func (s *Service) failRun(ctx context.Context, runID string, scope RunScope, cause error) {
	if _, err := s.finalizeRun(ctx, runID, scope, StatusFailed, cause.Error()); err != nil {
		s.deps.Logger.Error("mark scenario run failed", "run_id", runID, "error", err)
	}
}

func (s *Service) finalizeRun(ctx context.Context, runID string, scope RunScope, status, message string) (Run, error) {
	guard := s.runLock(runID)
	guard.Lock()
	defer guard.Unlock()
	return s.finalizeRunLocked(ctx, runID, scope, status, message)
}

func (s *Service) finalizeRunLocked(ctx context.Context, runID string, scope RunScope, status, message string) (Run, error) {
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	current, err := s.repo.Get(finalizeCtx, runID)
	if err != nil {
		return Run{}, err
	}
	if terminalStatus(current.Status) {
		// A prior process may have persisted terminal status before its final
		// event sweep completed. Make retrying finalization repair that state
		// without publishing a duplicate terminal event.
		if err := s.repo.SkipPendingEvents(finalizeCtx, runID, current.Error); err != nil {
			return Run{}, err
		}
		return current, nil
	}
	s.mu.Lock()
	eng := s.engines[runID]
	s.mu.Unlock()
	if eng != nil {
		if err := s.repo.UpdateProgress(finalizeCtx, runID, eng.VirtualTimeMs()); err != nil {
			return Run{}, err
		}
	}
	if err := s.repo.SkipPendingEvents(finalizeCtx, runID, message); err != nil {
		return Run{}, err
	}
	updated, err := s.repo.UpdateStatus(finalizeCtx, runID, status, message)
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(runctx.WithScope(finalizeCtx, runctx.Scope{RunID: scope.RunID, ResourceNamespace: scope.ResourceNamespace}), events.ScenarioRunEnded{
		At: time.Now().UTC(), RunID: runID, Status: status,
	})
	return updated, nil
}

func terminalStatus(status string) bool {
	return status == StatusCompleted || status == StatusStopped || status == StatusFailed
}

func countExecutedEvents(items []EventInspection) int {
	count := 0
	for _, item := range items {
		if item.Status == EventRunning || item.Status == EventCompleted || item.Status == EventFailed {
			count++
		}
	}
	return count
}

// Close stops all active engines and waits for their terminal persistence to
// finish. App.Close calls this before closing the database pool.
func (s *Service) Close(ctx context.Context) error {
	if scopeErr := access.Require(ctx, "platform.maintenance"); scopeErr != nil {
		return scopeErr
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	s.closed = true
	engines := make([]*engine, 0, len(s.engines))
	cancels := make([]context.CancelFunc, 0, len(s.engines))
	for runID, eng := range s.engines {
		engines = append(engines, eng)
		if cancel := s.cancels[runID]; cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	s.mu.Unlock()
	for _, eng := range engines {
		eng.Stop()
	}
	// Mark engines stopped before cancelling their contexts. The runner uses
	// the stopped bit to distinguish an intentional shutdown from an
	// unexpected context failure when it chooses the terminal run status.
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
