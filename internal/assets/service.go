package assets

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements asset registration and lookup.
type Service struct {
	repo  Repository
	bus   *events.Dispatcher
	fresh freshness.Policy
	now   func() time.Time
}

// NewService wires an asset service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return NewServiceWithFreshness(repo, bus, freshness.DefaultPolicy())
}

// NewServiceWithFreshness wires an asset service that classifies how current
// each asset's reported state is. The policy is applied on read so every
// consumer - API, realtime, and any future integration - receives the same
// answer to "can I trust this position?".
func NewServiceWithFreshness(repo Repository, bus *events.Dispatcher, policy freshness.Policy) *Service {
	return &Service{
		repo:  repo,
		bus:   bus,
		fresh: policy.Normalized(),
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// classify stamps the freshness state derived from signals already present on
// the asset. It never touches the database, which is why a stale asset cannot
// be persisted as fresh or vice versa.
func (s *Service) classify(asset Asset) Asset {
	asset.Freshness = s.fresh.ForAsset(freshness.AssetSignal{
		LastSeenAt:      asset.LastSeenAt,
		ConnectionState: asset.ConnectionState,
		Status:          string(asset.Status),
	}, s.now())
	return asset
}

// Create registers a new asset and publishes asset.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Asset, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Asset{}, err
	}

	created, err := s.repo.Create(ctx, Asset{
		ID:           in.ID,
		Name:         in.Name,
		Type:         in.Type,
		Status:       in.Status,
		Capabilities: in.Capabilities,
		Metadata:     in.Metadata,
	})
	if err != nil {
		return Asset{}, err
	}

	s.bus.Publish(ctx, events.AssetCreated{
		At:      time.Now().UTC(),
		AssetID: created.ID,
		Name:    created.Name,
		Type:    created.Type,
	})
	return s.classify(created), nil
}

// Ensure idempotently registers an asset, returning whether it was created.
func (s *Service) Ensure(ctx context.Context, in CreateInput) (Asset, bool, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Asset{}, false, err
	}

	existing, err := s.repo.Get(ctx, in.ID)
	if err == nil {
		return s.classify(existing), false, nil
	}
	if !apperr.Is(err, apperr.CodeNotFound) {
		return Asset{}, false, err
	}

	created, err := s.Create(ctx, in)
	if err != nil {
		return Asset{}, false, err
	}
	return created, true, nil
}

// Get returns an asset with its current state by id.
func (s *Service) Get(ctx context.Context, id string) (Asset, error) {
	asset, err := s.repo.Get(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	return s.classify(asset), nil
}

// Exists reports whether an asset is registered. Other modules use this to
// validate references without importing asset internals.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

// List returns assets with their current state.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Asset, int, error) {
	items, total, err := s.repo.List(ctx, clampLimit(limit), max(offset, 0))
	if err != nil {
		return nil, 0, err
	}
	now := s.now()
	for i := range items {
		items[i].Freshness = s.fresh.ForAsset(freshness.AssetSignal{
			LastSeenAt:      items[i].LastSeenAt,
			ConnectionState: items[i].ConnectionState,
			Status:          string(items[i].Status),
		}, now)
	}
	return items, total, nil
}

// UpdateStatus changes availability and publishes asset.updated.
func (s *Service) UpdateStatus(ctx context.Context, id string, status Status) (Asset, error) {
	if !validStatus(status) {
		return Asset{}, apperr.Validation("asset status must be one of available, assigned, unavailable, offline, maintenance")
	}

	updated, err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return Asset{}, err
	}
	s.bus.Publish(ctx, events.AssetUpdated{
		At:      time.Now().UTC(),
		AssetID: updated.ID,
		Status:  string(updated.Status),
	})
	return s.classify(updated), nil
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
