package classifications

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements classification recording and lookup.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
	now  func() time.Time
}

// NewService wires a classification service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{
		repo: repo,
		bus:  bus,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Create records a classification hypothesis and publishes
// classification.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Classification, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Classification{}, err
	}

	created, err := s.repo.Create(ctx, Classification{
		ID:              ids.New("cls"),
		TrackID:         in.TrackID,
		Label:           in.Label,
		Confidence:      in.Confidence,
		Method:          in.Method,
		SourceReference: in.SourceReference,
		CreatedBy:       in.CreatedBy,
	})
	if err != nil {
		return Classification{}, err
	}

	s.bus.Publish(ctx, events.ClassificationCreated{
		At:               s.now(),
		ClassificationID: created.ID,
		TrackID:          created.TrackID,
		Label:            created.Label,
		Confidence:       created.Confidence,
		Method:           string(created.Method),
	})
	return created, nil
}

// Get returns a classification by id.
func (s *Service) Get(ctx context.Context, id string) (Classification, error) {
	return s.repo.Get(ctx, id)
}

// ListByTrack returns a track's classifications newest first.
func (s *Service) ListByTrack(ctx context.Context, trackID string, limit, offset int) ([]Classification, int, error) {
	return s.repo.ListByTrack(ctx, trackID, clampLimit(limit), max(offset, 0))
}

// LatestForTrack returns the most recent classification for a track, if any.
func (s *Service) LatestForTrack(ctx context.Context, trackID string) (*Classification, error) {
	return s.repo.LatestForTrack(ctx, trackID)
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
