package observations

import (
	"context"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"go.opentelemetry.io/otel/attribute"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// SourceRegistry verifies that observations reference a registered source.
// Implemented by the sources module.
type SourceRegistry interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// Service implements observation ingestion and retrieval.
type Service struct {
	repo    Repository
	sources SourceRegistry
	bus     *events.Dispatcher
	now     func() time.Time
}

// NewService wires an observation service.
func NewService(repo Repository, sources SourceRegistry, bus *events.Dispatcher) *Service {
	return &Service{
		repo:    repo,
		sources: sources,
		bus:     bus,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// IngestResult reports the stored observation and whether the request was a
// duplicate of an already stored observation.
type IngestResult struct {
	Observation Observation
	Duplicate   bool
}

// Ingest validates, persists, and publishes a received observation. The
// observation is retained even when later processing fails, preserving the
// evidence layer.
func (s *Service) Ingest(ctx context.Context, in CreateInput) (IngestResult, error) {
	in.Normalize()
	spanCtx, span := observability.StartSpan(ctx, "c4isr.observation.ingest",
		attribute.String("observation.id", in.ID),
		attribute.String("source.id", in.SourceID),
	)
	ctx = spanCtx
	var spanErr error
	defer func() { observability.EndSpan(span, spanErr) }()

	now := s.now()
	if err := in.Validate(now); err != nil {
		spanErr = err
		s.publishRejected(ctx, in, err)
		return IngestResult{}, err
	}

	exists, err := s.sources.Exists(ctx, in.SourceID)
	if err != nil {
		spanErr = err
		s.publishRejected(ctx, in, err)
		return IngestResult{}, err
	}
	if !exists {
		err := apperr.Validation("unknown source: " + in.SourceID)
		spanErr = err
		s.publishRejected(ctx, in, err)
		return IngestResult{}, err
	}

	obs := Observation{
		ID:         in.ID,
		SourceID:   in.SourceID,
		Type:       in.Type,
		ObservedAt: in.ObservedAt,
		ReceivedAt: in.ReceivedAt,
		Position:   in.Position,
		Payload:    in.Payload,
		Quality:    in.Quality,
		TrackHint:  in.TrackHint,
	}

	created, err := s.repo.Create(ctx, obs)
	if err != nil {
		if errors.Is(err, ErrDuplicateID) {
			existing, getErr := s.repo.Get(ctx, in.ID)
			if getErr != nil {
				spanErr = getErr
				return IngestResult{}, getErr
			}
			if existing.ProcessedAt == nil && s.bus != nil {
				// The evidence insert committed before a downstream event handler
				// failed. Replay the durable observation so projection can resume.
				s.bus.Publish(ctx, events.ObservationReceived{
					At:            s.now(),
					ObservationID: existing.ID,
					SourceID:      existing.SourceID,
					Type:          existing.Type,
					ObservedAt:    existing.ObservedAt,
					ReceivedAt:    existing.ReceivedAt,
					Position:      existing.Position,
					TrackHint:     existing.TrackHint,
					Duplicate:     true,
					Retry:         true,
				})
			}
			return IngestResult{Observation: existing, Duplicate: true}, nil
		}
		spanErr = err
		s.publishRejected(ctx, in, err)
		return IngestResult{}, err
	}

	s.bus.Publish(ctx, events.ObservationReceived{
		At:            now,
		ObservationID: created.ID,
		SourceID:      created.SourceID,
		Type:          created.Type,
		ObservedAt:    created.ObservedAt,
		ReceivedAt:    created.ReceivedAt,
		Position:      created.Position,
		TrackHint:     created.TrackHint,
	})
	return IngestResult{Observation: created}, nil
}

func (s *Service) publishRejected(ctx context.Context, in CreateInput, cause error) {
	if s.bus == nil || cause == nil {
		return
	}
	s.bus.Publish(ctx, events.ObservationRejected{
		At:            s.now(),
		ObservationID: in.ID,
		SourceID:      in.SourceID,
		TrackHint:     in.TrackHint,
		Reason:        rejectionReason(cause),
	})
}

// rejectionReason is an intentional public category. The original error is
// retained in the request span/log path, while realtime and audit consumers
// receive no SQL, driver, or filesystem details from infrastructure failures.
func rejectionReason(cause error) string {
	switch apperr.CodeOf(cause) {
	case apperr.CodeBadRequest:
		return string(apperr.CodeBadRequest)
	case apperr.CodeValidation:
		return string(apperr.CodeValidation)
	case apperr.CodeNotFound:
		return string(apperr.CodeNotFound)
	case apperr.CodeConflict:
		return string(apperr.CodeConflict)
	default:
		return string(apperr.CodeInternal)
	}
}

// Get returns an observation by id.
func (s *Service) Get(ctx context.Context, id string) (Observation, error) {
	return s.repo.Get(ctx, id)
}

// List returns observations newest first.
func (s *Service) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Observation, int, error) {
	return s.repo.List(ctx, filter, ClampLimit(limit), max(offset, 0))
}

// MarkProcessed records that the platform processed an observation. Other
// modules (tracks) call this once the observation is attached as evidence.
func (s *Service) MarkProcessed(ctx context.Context, id string) error {
	return s.repo.MarkProcessed(ctx, id)
}

// ClampLimit applies the module's pagination bounds.
func ClampLimit(limit int) int {
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
