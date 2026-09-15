package sources

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Service implements source registration and lookup.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
}

// NewService wires a source service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, bus: bus}
}

// Create registers a new source and publishes source.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Source, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Source{}, err
	}

	source := Source{
		ID:       in.ID,
		Name:     in.Name,
		Type:     in.Type,
		Status:   in.Status,
		Metadata: in.Metadata,
	}
	created, err := s.repo.Create(ctx, source)
	if err != nil {
		return Source{}, err
	}

	s.bus.Publish(ctx, events.SourceCreated{
		At:       time.Now().UTC(),
		SourceID: created.ID,
		Name:     created.Name,
		Type:     string(created.Type),
	})
	return created, nil
}

// Ensure idempotently registers a source, returning whether it was created.
// Scenario setup uses this so runs are repeatable.
func (s *Service) Ensure(ctx context.Context, in CreateInput) (Source, bool, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Source{}, false, err
	}

	existing, err := s.repo.Get(ctx, in.ID)
	if err == nil {
		return existing, false, nil
	}
	if !apperr.Is(err, apperr.CodeNotFound) {
		return Source{}, false, err
	}

	created, err := s.Create(ctx, in)
	if err != nil {
		return Source{}, false, err
	}
	return created, true, nil
}

// Get returns a source by id.
func (s *Service) Get(ctx context.Context, id string) (Source, error) {
	return s.repo.Get(ctx, id)
}

// Exists reports whether a source is registered. Other modules use this to
// validate provenance without importing source internals.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

// List returns sources newest first.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Source, int, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}

// UpdateStatus changes availability and publishes source.updated.
func (s *Service) UpdateStatus(ctx context.Context, id string, status Status) (Source, error) {
	switch status {
	case StatusActive, StatusDegraded, StatusOffline:
	default:
		return Source{}, apperr.Validation("source status must be one of active, degraded, offline")
	}

	updated, err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return Source{}, err
	}
	s.bus.Publish(ctx, events.SourceUpdated{
		At:       time.Now().UTC(),
		SourceID: updated.ID,
		Status:   string(updated.Status),
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
