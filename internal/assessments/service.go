package assessments

import (
	"context"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service implements assessment recording and lookup.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
}

// NewService wires an assessment service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, bus: bus}
}

// Create records an assessment and publishes assessment.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Assessment, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Assessment{}, err
	}

	now := time.Now().UTC()
	evidence := make([]Evidence, len(in.Evidence))
	for i, e := range in.Evidence {
		if e.AddedAt.IsZero() {
			e.AddedAt = now
		}
		evidence[i] = e
	}

	created, err := s.repo.Create(ctx, Assessment{
		ID:          ids.New("asm"),
		SubjectType: in.SubjectType,
		SubjectID:   in.SubjectID,
		Type:        in.Type,
		Conclusion:  in.Conclusion,
		Confidence:  in.Confidence,
		Method:      in.Method,
		CreatedBy:   in.CreatedBy,
		Evidence:    evidence,
	})
	if err != nil {
		return Assessment{}, err
	}

	s.bus.Publish(ctx, events.AssessmentCreated{
		At:           time.Now().UTC(),
		AssessmentID: created.ID,
		SubjectType:  string(created.SubjectType),
		SubjectID:    created.SubjectID,
		Type:         created.Type,
		Method:       string(created.Method),
	})
	return created, nil
}

// Get returns an assessment by id.
func (s *Service) Get(ctx context.Context, id string) (Assessment, error) {
	return s.repo.Get(ctx, id)
}

// List returns assessments matching an optional subject, newest first.
func (s *Service) List(ctx context.Context, subjectType, subjectID string, limit, offset int) ([]Assessment, int, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, strings.TrimSpace(subjectType), strings.TrimSpace(subjectID), limit, offset)
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
