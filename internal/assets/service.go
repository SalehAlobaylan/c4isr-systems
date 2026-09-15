package assets

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements asset registration and lookup.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
}

// NewService wires an asset service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, bus: bus}
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
	return created, nil
}

// Ensure idempotently registers an asset, returning whether it was created.
func (s *Service) Ensure(ctx context.Context, in CreateInput) (Asset, bool, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Asset{}, false, err
	}

	existing, err := s.repo.Get(ctx, in.ID)
	if err == nil {
		return existing, false, nil
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
	return s.repo.Get(ctx, id)
}

// Exists reports whether an asset is registered. Other modules use this to
// validate references without importing asset internals.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

// List returns assets with their current state.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Asset, int, error) {
	return s.repo.List(ctx, clampLimit(limit), max(offset, 0))
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
	return updated, nil
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
