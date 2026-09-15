package scenarios

import (
	"context"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
)

// DefaultLimit and MaxLimit bound run list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// StartOptions overrides scenario defaults at launch.
type StartOptions struct {
	Speed float64
	Seed  *int64
}

// Service is the application layer for scenario control. The runner itself is
// an external source of information; runs are supervised and controllable but
// never bypass domain services.
type Service struct {
	repo   Repository
	loader *Loader
	deps   EngineDeps
	bus    *events.Dispatcher

	mu      sync.Mutex
	engines map[string]*engine
	cancels map[string]context.CancelFunc
}

// NewService wires the scenario application service.
func NewService(repo Repository, loader *Loader, deps EngineDeps, bus *events.Dispatcher) *Service {
	s := &Service{
		repo:    repo,
		loader:  loader,
		deps:    deps,
		bus:     bus,
		engines: map[string]*engine{},
		cancels: map[string]context.CancelFunc{},
	}
	bus.Subscribe(events.TopicCommandIssued, s.handleCommandIssued)
	return s
}

// ListScenarios returns the scenario definitions available on disk.
func (s *Service) ListScenarios() ([]Summary, error) {
	return s.loader.List()
}

// GetScenario returns one parsed scenario definition.
func (s *Service) GetScenario(name string) (*Scenario, error) {
	return s.loader.Load(name)
}

// Start registers scenario entities through the normal application services,
// then begins executing the deterministic timeline.
func (s *Service) Start(ctx context.Context, name string, opts StartOptions) (Run, error) {
	sc, err := s.loader.Load(name)
	if err != nil {
		return Run{}, err
	}

	seed := sc.Seed
	if opts.Seed != nil {
		seed = *opts.Seed
	}
	speed := sc.PlaybackSpeed
	if speed <= 0 {
		speed = defaultPlaybackSpeed
	}
	if opts.Speed > 0 {
		speed = opts.Speed
	}

	run, err := s.repo.Create(ctx, Run{
		ID:            ids.New("scn"),
		ScenarioName:  sc.Name,
		Seed:          seed,
		Status:        StatusRunning,
		PlaybackSpeed: speed,
	})
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(ctx, events.ScenarioRunStarted{
		At:           time.Now().UTC(),
		RunID:        run.ID,
		ScenarioName: sc.Name,
		Seed:         seed,
	})

	if err := s.setup(ctx, sc); err != nil {
		s.failRun(ctx, run.ID, err)
		return Run{}, err
	}

	progress := func(ms int64) {
		updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.repo.UpdateProgress(updateCtx, run.ID, ms)
	}
	eng := newEngine(run.ID, sc, seed, speed, s.deps, progress)
	if err := eng.build(); err != nil {
		s.failRun(ctx, run.ID, err)
		return Run{}, err
	}

	// The run outlives the HTTP request that starts it; it is controlled via
	// pause/resume/speed/stop and cancelled on process shutdown.
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.engines[run.ID] = eng
	s.cancels[run.ID] = cancel
	s.mu.Unlock()

	go func() {
		runErr := eng.run(runCtx)

		status := StatusCompleted
		message := ""
		switch {
		case runErr != nil:
			status = StatusFailed
			message = runErr.Error()
		case eng.IsStopped():
			status = StatusStopped
		}

		endCtx, endCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer endCancel()
		if _, err := s.repo.UpdateStatus(endCtx, run.ID, status, message); err != nil {
			s.deps.Logger.Error("update scenario run status", "run_id", run.ID, "error", err)
		}
		s.bus.Publish(context.Background(), events.ScenarioRunEnded{
			At:     time.Now().UTC(),
			RunID:  run.ID,
			Status: status,
		})

		s.mu.Lock()
		delete(s.engines, run.ID)
		delete(s.cancels, run.ID)
		s.mu.Unlock()
	}()

	return run, nil
}

// Pause freezes a running scenario.
func (s *Service) Pause(ctx context.Context, runID string) (Run, error) {
	eng, err := s.engine(runID)
	if err != nil {
		return Run{}, err
	}
	eng.Pause()
	run, err := s.repo.UpdateStatus(ctx, runID, StatusPaused, "")
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(ctx, events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return run, nil
}

// Resume continues a paused scenario.
func (s *Service) Resume(ctx context.Context, runID string) (Run, error) {
	eng, err := s.engine(runID)
	if err != nil {
		return Run{}, err
	}
	eng.Resume()
	run, err := s.repo.UpdateStatus(ctx, runID, StatusRunning, "")
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(ctx, events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return run, nil
}

// SetSpeed changes playback speed.
func (s *Service) SetSpeed(ctx context.Context, runID string, speed float64) (Run, error) {
	if speed <= 0 {
		return Run{}, apperr.Validation("speed must be positive")
	}
	eng, err := s.engine(runID)
	if err != nil {
		return Run{}, err
	}
	eng.SetSpeed(speed)
	run, err := s.repo.UpdateSpeed(ctx, runID, speed)
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(ctx, events.ScenarioRunUpdated{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return run, nil
}

// Stop cancels a running scenario.
func (s *Service) Stop(ctx context.Context, runID string) (Run, error) {
	s.mu.Lock()
	eng, ok := s.engines[runID]
	cancel := s.cancels[runID]
	s.mu.Unlock()
	if !ok {
		return Run{}, apperr.NotFound("scenario run", runID)
	}
	eng.Stop()
	if cancel != nil {
		cancel()
	}
	run, err := s.repo.UpdateStatus(ctx, runID, StatusStopped, "")
	if err != nil {
		return Run{}, err
	}
	s.bus.Publish(ctx, events.ScenarioRunEnded{At: time.Now().UTC(), RunID: runID, Status: run.Status})
	return run, nil
}

// GetRun returns persisted run state merged with live engine state.
func (s *Service) GetRun(ctx context.Context, runID string) (Run, error) {
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	s.mu.Lock()
	eng := s.engines[runID]
	s.mu.Unlock()
	if eng != nil {
		run.VirtualTimeMs = eng.VirtualTimeMs()
		if eng.IsPaused() {
			run.Status = StatusPaused
		}
	}
	return run, nil
}

// ListRuns returns scenario runs newest first.
func (s *Service) ListRuns(ctx context.Context, limit, offset int) ([]Run, int, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) engine(runID string) (*engine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eng, ok := s.engines[runID]
	if !ok {
		return nil, apperr.Conflict("scenario run is not active")
	}
	return eng, nil
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
			eng.simulateCommand(issued)
			return
		}
	}
}

func (s *Service) setup(ctx context.Context, sc *Scenario) error {
	for _, src := range sc.Sources {
		name := src.Name
		if name == "" {
			name = src.ID
		}
		sourceType := src.Type
		if sourceType == "" {
			sourceType = string(sources.TypeSynthetic)
		}
		if _, _, err := s.deps.Sources.Ensure(ctx, sources.CreateInput{
			ID:   src.ID,
			Name: name,
			Type: sources.Type(sourceType),
		}); err != nil {
			return err
		}
	}

	for _, a := range sc.Assets {
		name := a.Name
		if name == "" {
			name = a.ID
		}
		assetType := a.Type
		if assetType == "" {
			assetType = "vehicle"
		}
		if _, _, err := s.deps.Assets.Ensure(ctx, assets.CreateInput{
			ID:   a.ID,
			Name: name,
			Type: assetType,
		}); err != nil {
			return err
		}
	}

	for _, gf := range sc.Geofences {
		polygon := make([]geo.Point, 0, len(gf.Polygon))
		for _, p := range gf.Polygon {
			polygon = append(polygon, p.Point())
		}
		if _, _, err := s.deps.Geofences.Ensure(ctx, geospatial.CreateInput{
			ID:       gf.ID,
			Name:     gf.Name,
			Type:     geospatial.Type(gf.Type),
			Severity: geospatial.Severity(gf.Severity),
			Polygon:  polygon,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) failRun(ctx context.Context, runID string, cause error) {
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.repo.UpdateStatus(failCtx, runID, StatusFailed, cause.Error()); err != nil {
		s.deps.Logger.Error("mark scenario run failed", "run_id", runID, "error", err)
	}
	s.bus.Publish(ctx, events.ScenarioRunEnded{At: time.Now().UTC(), RunID: runID, Status: StatusFailed})
}
