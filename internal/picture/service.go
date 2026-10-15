package picture

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

// Service reconstructs the operational picture at a requested instant.
type Service struct {
	repo  Repository
	fresh freshness.Policy
	now   func() time.Time
}

// NewService wires the picture service.
func NewService(repo Repository, policy freshness.Policy) *Service {
	return &Service{
		repo:  repo,
		fresh: policy.Normalized(),
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// Request asks for the picture as it stood at an instant.
//
// At is the instant to reconstruct. A zero At means "now", which is the live
// picture and is served identically so the interface has one code path.
type Request struct {
	At          time.Time
	ScopePrefix string
	Limit       int
	// Live forces the snapshot to be treated as the current moment even when
	// At is set to the same instant the service resolves for now. Without it
	// the service could only infer intent by comparing instants, which makes an
	// explicit "return me to now" indistinguishable from a replay that happens
	// to land on now.
	Live bool
}

// effectiveAt returns the instant to reconstruct, defaulting to now.
func (r Request) effectiveAt(now time.Time) time.Time {
	if r.At.IsZero() {
		return now
	}
	return r.At
}

// Snapshot reconstructs the picture.
//
// The returned freshness is computed against the requested instant, not the
// wall clock. This is the difference between an honest replay and a misleading
// one: at T+23s, an entity whose last observation was 6 virtual seconds earlier
// was current then, and painting it in today's colours would splice two
// different moments into one picture.
//
// An instant outside the evidence window is clamped rather than refused. The
// operator asked for a moment that does not exist in the data; returning the
// nearest real one with an honest At is more useful than an error, and is
// visible in the interface rather than silently substituted.
func (s *Service) Snapshot(ctx context.Context, req Request) (Snapshot, error) {
	if req.Limit <= 0 {
		req.Limit = DefaultLimit
	}
	if req.Limit > MaxLimit {
		req.Limit = MaxLimit
	}

	now := s.now()
	window, err := s.repo.Window(ctx, req.ScopePrefix)
	if err != nil {
		return Snapshot{}, err
	}

	requested := req.effectiveAt(now)
	live := req.Live || requested.Equal(now)
	at := window.Clamp(requested)

	tracks, err := s.repo.TracksAt(ctx, at, req.ScopePrefix)
	if err != nil {
		return Snapshot{}, err
	}
	for i := range tracks {
		observedAt := tracks[i].ObservedAt
		tracks[i].Freshness = s.fresh.ForTrack(freshness.TrackSignal{
			LastSeenAt: observedAt,
			Status:     tracks[i].Status,
		}, at)
	}

	assets, err := s.repo.AssetsAt(ctx, at, req.ScopePrefix)
	if err != nil {
		return Snapshot{}, err
	}
	for i := range assets {
		observedAt := assets[i].ObservedAt
		assets[i].Freshness = s.fresh.ForAsset(freshness.AssetSignal{
			LastSeenAt:      &observedAt,
			ConnectionState: assets[i].ConnectionState,
			Status:          assets[i].Status,
		}, at)
	}

	alerts, err := s.repo.AlertsAt(ctx, at, req.Limit)
	if err != nil {
		return Snapshot{}, err
	}
	geofences, err := s.repo.GeofencesAt(ctx, req.Limit)
	if err != nil {
		return Snapshot{}, err
	}

	if tracks == nil {
		tracks = []Track{}
	}
	if assets == nil {
		assets = []Asset{}
	}
	if alerts == nil {
		alerts = []Alert{}
	}
	if geofences == nil {
		geofences = []Geofence{}
	}

	return Snapshot{
		At:              at,
		Live:            live,
		Tracks:          tracks,
		Assets:          assets,
		Alerts:          alerts,
		Geofences:       geofences,
		Window:          window,
		TruncatedAlerts: len(alerts) >= req.Limit,
	}, nil
}

// Window returns the span of evidence available for scrubbing.
func (s *Service) Window(ctx context.Context, scopePrefix string) (Window, error) {
	window, err := s.repo.Window(ctx, scopePrefix)
	if err != nil {
		return Window{}, err
	}
	if window.Empty() {
		return Window{}, apperr.NotFound("picture", "window")
	}
	return window, nil
}

// DefaultLimit and MaxLimit bound reconstruction queries. The picture is read
// whole rather than paged, because a partial picture is a lie: omitting the
// entity that breached the zone would invert the meaning of the replay.
const (
	DefaultLimit = 500
	MaxLimit     = 5000
)
