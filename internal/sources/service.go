package sources

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Service implements source registration and lookup.
type Catalog interface {
	Summaries(context.Context, []string) (map[string]IngestionSummary, error)
}

type Service struct {
	catalog Catalog
	repo    Repository
	bus     *events.Dispatcher
}

// NewService wires a source service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, bus: bus}
}

// SetCatalog attaches the optional public-ingestion read port.
func (s *Service) SetCatalog(c Catalog) { s.catalog = c }

// Create registers a new source and publishes source.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Source, error) {
	if scopeErr := access.Require(ctx, "sources.manage"); scopeErr != nil {
		return Source{}, scopeErr
	}

	in.Normalize()
	in.ID = access.ResourceID(ctx, "source", in.ID)
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
	if scopeErr := access.Require(ctx, "sources.manage"); scopeErr != nil {
		return Source{}, false, scopeErr
	}

	in.Normalize()
	in.ID = access.ResourceID(ctx, "source", in.ID)
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
	if scopeErr := access.Require(ctx, "sources.read"); scopeErr != nil {
		return Source{}, scopeErr
	}

	source, err := s.repo.Get(ctx, id)
	if err != nil {
		return Source{}, err
	}
	if s.catalog != nil {
		summaries, err := s.catalog.Summaries(ctx, []string{id})
		if err != nil {
			return Source{}, err
		}
		if info, ok := summaries[id]; ok {
			source.Ingestion = &info
		}
	}
	resource, _ := access.ResourceFrom(ctx)
	if source.Ingestion == nil && resource.Partition == access.PublicEvidence && source.Type == TypeExternal {
		source.Ingestion = &IngestionSummary{State: "not_admitted", Meaning: "No admitted retrieval; missing records are not evidence of absence."}
	}
	return source, nil
}

// Exists reports whether a source is registered. Other modules use this to
// validate provenance without importing source internals.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	if scopeErr := access.Require(ctx, "sources.read"); scopeErr != nil {
		return false, scopeErr
	}

	return s.repo.Exists(ctx, id)
}

// List returns sources newest first.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Source, int, error) {
	if scopeErr := access.Require(ctx, "sources.read"); scopeErr != nil {
		return nil, 0, scopeErr
	}

	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	items, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if s.catalog != nil {
		keys := make([]string, 0, len(items))
		for _, item := range items {
			keys = append(keys, item.ID)
		}
		summaries, err := s.catalog.Summaries(ctx, keys)
		if err != nil {
			return nil, 0, err
		}
		for i := range items {
			if info, ok := summaries[items[i].ID]; ok {
				items[i].Ingestion = &info
			}
		}
	}
	resource, _ := access.ResourceFrom(ctx)
	if resource.Partition == access.PublicEvidence {
		for i := range items {
			if items[i].Ingestion == nil && items[i].Type == TypeExternal {
				items[i].Ingestion = &IngestionSummary{State: "not_admitted", Meaning: "No admitted retrieval; missing records are not evidence of absence."}
			}
		}
	}
	return items, total, nil
}

// UpdateStatus changes availability and publishes source.updated.
func (s *Service) UpdateStatus(ctx context.Context, id string, status Status) (Source, error) {
	if scopeErr := access.Require(ctx, "sources.manage"); scopeErr != nil {
		return Source{}, scopeErr
	}

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
