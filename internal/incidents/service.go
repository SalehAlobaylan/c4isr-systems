package incidents

import (
	"context"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Service implements incident coordination workflows.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
}

// NewService wires an incident service.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, bus: bus}
}

// Create opens an incident, attaches the supplied evidence relations, and
// publishes incident.created.
func (s *Service) Create(ctx context.Context, in CreateInput) (Incident, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Incident{}, err
	}
	relations := []struct {
		kind string
		ids  []string
	}{
		{"alert", in.AlertIDs},
		{"track", in.TrackIDs},
		{"asset", in.AssetIDs},
		{"observation", in.ObservationIDs},
		{"assessment", in.AssessmentIDs},
	}
	for _, relation := range relations {
		for _, id := range relation.ids {
			if strings.TrimSpace(id) == "" {
				return Incident{}, apperr.Validation(relation.kind + " id is required")
			}
		}
	}

	incident := Incident{
		ID:               ids.New("inc"),
		Title:            in.Title,
		Description:      in.Description,
		Priority:         in.Priority,
		Status:           StatusOpen,
		AssignedOperator: in.Actor,
	}
	created, err := s.repo.Create(ctx, incident)
	if err != nil {
		return Incident{}, err
	}

	for _, id := range in.AlertIDs {
		if err := s.repo.AttachAlert(ctx, created.ID, strings.TrimSpace(id)); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range in.TrackIDs {
		if err := s.repo.AttachTrack(ctx, created.ID, strings.TrimSpace(id)); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range in.AssetIDs {
		if err := s.repo.AttachAsset(ctx, created.ID, strings.TrimSpace(id)); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range in.ObservationIDs {
		if err := s.repo.AttachObservation(ctx, created.ID, strings.TrimSpace(id)); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range in.AssessmentIDs {
		if err := s.repo.AttachAssessment(ctx, created.ID, strings.TrimSpace(id)); err != nil {
			return Incident{}, err
		}
	}

	s.bus.Publish(ctx, events.IncidentCreated{
		At:         time.Now().UTC(),
		IncidentID: created.ID,
		Title:      created.Title,
		Priority:   string(created.Priority),
		Status:     string(created.Status),
		Actor:      in.Actor,
	})
	return created, nil
}

// Get returns an incident together with its attached evidence.
func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	return s.repo.GetDetail(ctx, id)
}

// List returns incidents newest first.
func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]Incident, int, error) {
	return s.repo.List(ctx, status, clampLimit(limit), max(offset, 0))
}

// UpdateStatus moves an incident through its lifecycle and publishes
// incident.updated.
func (s *Service) UpdateStatus(ctx context.Context, id string, status Status, actor string) (Incident, error) {
	if !validStatus(status) {
		return Incident{}, apperr.Validation("incident status must be one of OPEN, ACKNOWLEDGED, INVESTIGATING, RESPONDING, RESOLVED, CLOSED")
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return Incident{}, err
	}
	if !canTransition(current.Status, status) {
		return Incident{}, invalidTransition(current.Status, status)
	}
	updated, err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return Incident{}, err
	}
	s.publishUpdated(ctx, updated, actor)
	return updated, nil
}

// Update edits incident metadata and publishes incident.updated.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput, actor string) (Incident, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Incident{}, err
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return Incident{}, err
	}
	if in.Title != "" {
		current.Title = in.Title
	}
	if in.Description != "" {
		current.Description = in.Description
	}
	if in.Priority != "" {
		current.Priority = in.Priority
	}
	if in.AssignedOperator != "" {
		current.AssignedOperator = in.AssignedOperator
	}
	updated, err := s.repo.Update(ctx, current)
	if err != nil {
		return Incident{}, err
	}
	s.publishUpdated(ctx, updated, actor)
	return updated, nil
}

// AttachAlert links an alert to an incident and publishes incident.updated.
func (s *Service) AttachAlert(ctx context.Context, incidentID, alertID, actor string) (Incident, error) {
	return s.attach(ctx, incidentID, "alert", alertID, actor, s.repo.AttachAlert)
}

// AttachTrack links a track to an incident and publishes incident.updated.
func (s *Service) AttachTrack(ctx context.Context, incidentID, trackID, actor string) (Incident, error) {
	return s.attach(ctx, incidentID, "track", trackID, actor, s.repo.AttachTrack)
}

// AttachAsset links an asset to an incident and publishes incident.updated.
func (s *Service) AttachAsset(ctx context.Context, incidentID, assetID, actor string) (Incident, error) {
	return s.attach(ctx, incidentID, "asset", assetID, actor, s.repo.AttachAsset)
}

// AttachObservation links an observation to an incident and publishes
// incident.updated.
func (s *Service) AttachObservation(ctx context.Context, incidentID, observationID, actor string) (Incident, error) {
	return s.attach(ctx, incidentID, "observation", observationID, actor, s.repo.AttachObservation)
}

// AttachAssessment links an assessment to an incident and publishes
// incident.updated.
func (s *Service) AttachAssessment(ctx context.Context, incidentID, assessmentID, actor string) (Incident, error) {
	return s.attach(ctx, incidentID, "assessment", assessmentID, actor, s.repo.AttachAssessment)
}

func (s *Service) attach(
	ctx context.Context,
	incidentID, kind, targetID, actor string,
	attach func(ctx context.Context, incidentID, targetID string) error,
) (Incident, error) {
	if strings.TrimSpace(incidentID) == "" {
		return Incident{}, apperr.Validation("incident id is required")
	}
	if strings.TrimSpace(targetID) == "" {
		return Incident{}, apperr.Validation(kind + " id is required")
	}
	current, err := s.repo.Get(ctx, strings.TrimSpace(incidentID))
	if err != nil {
		return Incident{}, err
	}
	if err := attach(ctx, current.ID, strings.TrimSpace(targetID)); err != nil {
		return Incident{}, err
	}
	s.publishUpdated(ctx, current, actor)
	return current, nil
}

func (s *Service) publishUpdated(ctx context.Context, incident Incident, actor string) {
	s.bus.Publish(ctx, events.IncidentUpdated{
		At:         time.Now().UTC(),
		IncidentID: incident.ID,
		Status:     string(incident.Status),
		Priority:   string(incident.Priority),
		Actor:      actor,
	})
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
