package telemetry

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

// FreshnessSignal is the minimal per-asset input the monitor needs. It is a
// projection of the asset and its state rather than the full asset, so a tick
// stays cheap regardless of how many assets exist.
type FreshnessSignal struct {
	AssetID         string
	Status          string
	ConnectionState string
	LastSeenAt      *time.Time
}

// TrackFreshnessSignal is the equivalent projection for a track. Tracks have no
// connection state or administrative status beyond their lifecycle status, which
// is why this is separate rather than a widened FreshnessSignal: the zero
// values of the asset-only fields would otherwise classify a track as
// "unavailable" and count every one of them.
type TrackFreshnessSignal struct {
	TrackID    string
	Status     string
	LastSeenAt time.Time
}

// FreshnessObserver receives transitions the monitor detects. The metrics
// surface uses it to maintain the stale gauge.
type FreshnessObserver interface {
	ObserveStaleEntity(entityID string, stale bool)
}

// FreshnessMonitor periodically reclassifies every asset and publishes only
// genuine transitions.
//
// This exists because freshness computed at read time is invisible to a client
// that is already holding a rendered picture. Without a transition signal, an
// asset whose feed stopped stays on the map looking live until some unrelated
// event causes a refetch - which, in a quiet picture, is exactly when it
// matters most. Recovery is published too: an asset that resumes reporting
// returns to live without waiting for the operator to reload.
type FreshnessMonitor struct {
	repo     Repository
	bus      *events.Dispatcher
	observer FreshnessObserver
	policy   freshness.Policy
	now      func() time.Time

	mu       sync.Mutex
	previous map[string]freshness.State
}

// NewFreshnessMonitor wires the monitor. The observer may be nil.
func NewFreshnessMonitor(repo Repository, bus *events.Dispatcher, observer FreshnessObserver, policy freshness.Policy) *FreshnessMonitor {
	return &FreshnessMonitor{
		repo:     repo,
		bus:      bus,
		observer: observer,
		policy:   policy.Normalized(),
		now:      func() time.Time { return time.Now().UTC() },
		previous: map[string]freshness.State{},
	}
}

// Reconcile classifies every asset and track once and publishes the transitions
// since the previous tick. The first tick reports every entity, which is
// correct: a client that connected during the run still learns the current
// picture, and a gauge rebuilt from empty would otherwise read zero on a
// picture full of silent contacts.
func (m *FreshnessMonitor) Reconcile(ctx context.Context) error {
	assetSignals, err := m.repo.ListFreshnessSignals(ctx)
	if err != nil {
		return err
	}
	trackSignals, err := m.repo.ListTrackFreshnessSignals(ctx)
	if err != nil {
		return err
	}

	now := m.now()
	current := make(map[string]freshness.State, len(assetSignals)+len(trackSignals))
	for _, signal := range assetSignals {
		current[entityKey(EntityTypeAsset, signal.AssetID)] = m.policy.ForAsset(freshness.AssetSignal{
			LastSeenAt:      signal.LastSeenAt,
			ConnectionState: signal.ConnectionState,
			Status:          signal.Status,
		}, now)
	}
	for _, signal := range trackSignals {
		current[entityKey(EntityTypeTrack, signal.TrackID)] = m.policy.ForTrack(freshness.TrackSignal{
			LastSeenAt: signal.LastSeenAt,
			Status:     signal.Status,
		}, now)
	}

	m.mu.Lock()
	previous := m.previous
	m.previous = current
	m.mu.Unlock()

	for key, state := range current {
		if previous[key] == state {
			continue
		}
		m.report(ctx, key, state, previous[key])
	}

	// An entity that disappeared was most likely deleted or closed. Clear its
	// gauge so a stale count cannot outlive the entity that produced it.
	for key := range previous {
		if _, stillPresent := current[key]; stillPresent {
			continue
		}
		m.report(ctx, key, freshness.StateUnknown, previous[key])
	}
	return nil
}

// entityKey namespaces an entity id by type so an asset and a track can never
// collide in the monitor's map or the metrics set, even if a deployment issues
// ids that are not prefixed apart.
func entityKey(entityType, id string) string {
	return entityType + ":" + id
}

func (m *FreshnessMonitor) report(ctx context.Context, key string, state, previous freshness.State) {
	entityType, id := splitEntityKey(key)
	if m.observer != nil {
		m.observer.ObserveStaleEntity(key, !state.Trustworthy())
	}
	if m.bus == nil {
		return
	}
	m.bus.Publish(ctx, events.EntityFreshnessChanged{
		At:         m.now(),
		EntityType: entityType,
		EntityID:   id,
		Freshness:  string(state),
		Previous:   string(previous),
	})
}

func splitEntityKey(key string) (entityType, id string) {
	if index := strings.IndexByte(key, ':'); index >= 0 {
		return key[:index], key[index+1:]
	}
	return EntityTypeAsset, key
}

// Snapshot returns the last classification the monitor computed. It exists for
// diagnostics and tests; the API always re-derives freshness per read so a
// paused or missed tick cannot leave the interface showing a stale answer.
//
// Keys are entity-type-namespaced (see entityKey) so a caller inspecting the
// snapshot cannot mistake an asset id for a track id.
func (m *FreshnessMonitor) Snapshot() map[string]freshness.State {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]freshness.State, len(m.previous))
	for key, state := range m.previous {
		out[key] = state
	}
	return out
}

// Entity kinds carried by freshness transitions. Both are published and both
// feed the stale gauge: a contact that has gone quiet and a platform that has
// gone quiet are the same operational failure to the operator holding the
// picture, and counting only one of them would understate it.
const (
	EntityTypeAsset = "asset"
	EntityTypeTrack = "track"
)
