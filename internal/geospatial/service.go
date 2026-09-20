package geospatial

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements geofence registration and containment tracking.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
	now  func() time.Time
}

// NewService wires a geospatial service and subscribes it to track updates.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	s := &Service{
		repo: repo,
		bus:  bus,
		now:  func() time.Time { return time.Now().UTC() },
	}
	bus.Subscribe(events.TopicTrackUpdated, func(ctx context.Context, ev events.Event) {
		if track, ok := ev.(events.TrackUpdated); ok {
			s.HandleTrackUpdated(ctx, track)
		}
	})
	return s
}

// HandleTrackUpdated derives breach and exit events from real containment
// transitions, persisting state before publishing.
func (s *Service) HandleTrackUpdated(ctx context.Context, ev events.TrackUpdated) {
	if ev.Position == nil {
		return
	}
	now := s.now()

	containing, err := s.repo.ContainingPoint(ctx, *ev.Position)
	if err != nil {
		slog.Default().Error("geospatial: list containing geofences", "track_id", ev.TrackID, "error", err)
		return
	}
	containing = scopedGeofences(ctx, containing)
	states, err := s.repo.StatesForTrack(ctx, ev.TrackID)
	if err != nil {
		slog.Default().Error("geospatial: list geofence states", "track_id", ev.TrackID, "error", err)
		return
	}
	states = scopedStates(ctx, states)

	containingSet := make(map[string]Geofence, len(containing))
	for _, geofence := range containing {
		containingSet[geofence.ID] = geofence
	}

	for _, geofence := range containing {
		wasInside := states[geofence.ID]
		if err := s.repo.SetState(ctx, geofence.ID, ev.TrackID, true); err != nil {
			slog.Default().Error("geospatial: persist geofence state",
				"geofence_id", geofence.ID, "track_id", ev.TrackID, "error", err)
			continue
		}
		if wasInside {
			continue
		}
		s.bus.Publish(ctx, events.GeofenceBreached{
			At:           now,
			GeofenceID:   geofence.ID,
			GeofenceName: geofence.Name,
			GeofenceType: string(geofence.Type),
			Severity:     string(geofence.Severity),
			TrackID:      ev.TrackID,
			Position:     *ev.Position,
		})
	}

	exited := make([]string, 0, len(states))
	for id, inside := range states {
		if !inside {
			continue
		}
		if _, ok := containingSet[id]; ok {
			continue
		}
		exited = append(exited, id)
	}
	sort.Strings(exited)
	for _, id := range exited {
		if err := s.repo.SetState(ctx, id, ev.TrackID, false); err != nil {
			slog.Default().Error("geospatial: persist geofence state",
				"geofence_id", id, "track_id", ev.TrackID, "error", err)
			continue
		}
		s.bus.Publish(ctx, events.GeofenceExited{
			At:         now,
			GeofenceID: id,
			TrackID:    ev.TrackID,
			Position:   *ev.Position,
		})
	}
}

// scopedGeofences keeps scenario-owned tracks inside their own synthetic
// world. Geofence containment is normally global for operator and external
// tracks, but a scenario event carries a run scope and must not evaluate
// against a previous run's or an operator-created geofence with the same
// polygon.
func scopedGeofences(ctx context.Context, geofences []Geofence) []Geofence {
	scope, ok := runctx.ScopeFrom(ctx)
	if !ok {
		return geofences
	}
	prefix := scope.ResourceNamespace + "geofence__"
	out := make([]Geofence, 0, len(geofences))
	for _, geofence := range geofences {
		if strings.HasPrefix(geofence.ID, prefix) {
			out = append(out, geofence)
		}
	}
	return out
}

func scopedStates(ctx context.Context, states map[string]bool) map[string]bool {
	scope, ok := runctx.ScopeFrom(ctx)
	if !ok {
		return states
	}
	prefix := scope.ResourceNamespace + "geofence__"
	out := make(map[string]bool, len(states))
	for id, inside := range states {
		if strings.HasPrefix(id, prefix) {
			out[id] = inside
		}
	}
	return out
}

// Create registers a geofence and publishes geofence.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Geofence, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Geofence{}, err
	}

	geofence := Geofence{
		ID:       in.ID,
		Name:     in.Name,
		Type:     in.Type,
		Severity: in.Severity,
		Active:   *in.Active,
		GeoJSON:  in.GeoJSON(),
		Metadata: in.Metadata,
	}
	created, err := s.repo.Create(ctx, geofence)
	if err != nil {
		return Geofence{}, err
	}

	s.bus.Publish(ctx, events.GeofenceCreated{
		At:         s.now(),
		GeofenceID: created.ID,
		Name:       created.Name,
		Type:       string(created.Type),
	})
	return created, nil
}

// Ensure idempotently registers a geofence, returning whether it was created.
// Scenario setup uses this so runs are repeatable.
func (s *Service) Ensure(ctx context.Context, in CreateInput) (Geofence, bool, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Geofence{}, false, err
	}

	existing, err := s.repo.Get(ctx, in.ID)
	if err == nil {
		return existing, false, nil
	}
	if !apperr.Is(err, apperr.CodeNotFound) {
		return Geofence{}, false, err
	}

	created, err := s.Create(ctx, in)
	if err != nil {
		return Geofence{}, false, err
	}
	return created, true, nil
}

// Get returns a geofence by id.
func (s *Service) Get(ctx context.Context, id string) (Geofence, error) {
	return s.repo.Get(ctx, id)
}

// List returns geofences newest first.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Geofence, int, error) {
	return s.repo.List(ctx, clampLimit(limit), clampOffset(offset))
}

// SetActive enables or disables a geofence.
func (s *Service) SetActive(ctx context.Context, id string, active bool) (Geofence, error) {
	return s.repo.SetActive(ctx, id, active)
}

// ContainingPoint lists active geofences containing a point.
func (s *Service) ContainingPoint(ctx context.Context, point geo.Point) ([]Geofence, error) {
	if !point.Valid() {
		return nil, apperr.Validation("point must be a valid WGS84 coordinate")
	}
	return s.repo.ContainingPoint(ctx, point)
}

// AssetsWithinRadius lists assets inside a radius, nearest first.
func (s *Service) AssetsWithinRadius(ctx context.Context, center geo.Point, radiusM float64, limit int) ([]AssetDistance, error) {
	if !center.Valid() {
		return nil, apperr.Validation("center must be a valid WGS84 coordinate")
	}
	if radiusM <= 0 {
		return nil, apperr.Validation("radius_m must be greater than zero")
	}
	return s.repo.AssetsWithinRadius(ctx, center, radiusM, clampLimit(limit))
}

// NearestAssets lists assets by distance from a point, nearest first.
func (s *Service) NearestAssets(ctx context.Context, center geo.Point, limit int) ([]AssetDistance, error) {
	if !center.Valid() {
		return nil, apperr.Validation("center must be a valid WGS84 coordinate")
	}
	return s.repo.NearestAssets(ctx, center, clampLimit(limit))
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

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
