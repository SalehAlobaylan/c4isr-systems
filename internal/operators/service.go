package operators

import (
	"context"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements operator registration and lookup.
type Service struct {
	repo Repository
}

// NewService wires an operator service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create registers an operator.
func (s *Service) Create(ctx context.Context, in CreateInput) (Operator, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Operator{}, err
	}
	if in.ID == "" {
		in.ID = ids.New("opr")
	}
	return s.repo.Create(ctx, Operator{
		ID:   in.ID,
		Name: in.Name,
		Role: in.Role,
	})
}

// Ensure idempotently registers an operator, returning whether it was created.
// Server startup uses this to guarantee the default operator exists.
func (s *Service) Ensure(ctx context.Context, in CreateInput) (Operator, bool, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Operator{}, false, err
	}

	if in.ID != "" {
		existing, err := s.repo.Get(ctx, in.ID)
		if err == nil {
			return existing, false, nil
		}
		if !apperr.Is(err, apperr.CodeNotFound) {
			return Operator{}, false, err
		}
	}

	created, err := s.Create(ctx, in)
	if err != nil {
		return Operator{}, false, err
	}
	return created, true, nil
}

// Get returns an operator by id.
func (s *Service) Get(ctx context.Context, id string) (Operator, error) {
	return s.repo.Get(ctx, id)
}

// List returns operators newest first.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Operator, int, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}

// Exists reports whether an operator is registered.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
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
