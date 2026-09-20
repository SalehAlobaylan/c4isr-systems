package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

const geofenceBreachType = "geofence.breach"

// Service implements alert raising and lifecycle transitions.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
	now  func() time.Time
}

// NewService wires an alert service and subscribes it to geofence breaches.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	s := &Service{
		repo: repo,
		bus:  bus,
		now:  func() time.Time { return time.Now().UTC() },
	}
	bus.Subscribe(events.TopicGeofenceBreached, func(ctx context.Context, ev events.Event) {
		if breach, ok := ev.(events.GeofenceBreached); ok {
			s.HandleGeofenceBreached(ctx, breach)
		}
	})
	return s
}

// HandleGeofenceBreached raises an alert for a breach, suppressing duplicates
// while an alert for the same geofence and track remains unresolved.
func (s *Service) HandleGeofenceBreached(ctx context.Context, ev events.GeofenceBreached) {
	existing, err := s.repo.FindUnresolvedForGeofenceTrack(ctx, ev.GeofenceID, ev.TrackID)
	if err != nil {
		slog.Default().Error("alerts: find unresolved breach alert",
			"geofence_id", ev.GeofenceID, "track_id", ev.TrackID, "error", err)
		return
	}
	if existing != nil {
		return
	}

	now := s.now()
	sourceReference := map[string]any{
		"rule":         geofenceBreachType,
		"geofenceId":   ev.GeofenceID,
		"geofenceName": ev.GeofenceName,
		"geofenceType": ev.GeofenceType,
		"trackId":      ev.TrackID,
	}
	if scope, ok := runctx.ScopeFrom(ctx); ok {
		sourceReference["scenarioRunId"] = scope.RunID
		sourceReference["resourceNamespace"] = scope.ResourceNamespace
	}

	alert := Alert{
		ID:              ids.New("alr"),
		Type:            geofenceBreachType,
		Severity:        normalizeSeverity(ev.Severity),
		State:           StateActive,
		Title:           fmt.Sprintf("Track %s entered %s", ev.TrackID, ev.GeofenceName),
		Message:         fmt.Sprintf("Track %s entered %s geofence %s", ev.TrackID, ev.GeofenceType, ev.GeofenceName),
		SourceReference: sourceReference,
		TrackID:         ev.TrackID,
		GeofenceID:      ev.GeofenceID,
	}
	created, err := s.repo.Create(ctx, alert)
	if err != nil {
		slog.Default().Error("alerts: create breach alert",
			"geofence_id", ev.GeofenceID, "track_id", ev.TrackID, "error", err)
		return
	}

	s.bus.Publish(ctx, events.AlertCreated{
		At:         now,
		AlertID:    created.ID,
		Type:       created.Type,
		Severity:   string(created.Severity),
		Title:      created.Title,
		TrackID:    created.TrackID,
		AssetID:    created.AssetID,
		GeofenceID: created.GeofenceID,
	})
}

// Get returns an alert by id.
func (s *Service) Get(ctx context.Context, id string) (Alert, error) {
	return s.repo.Get(ctx, id)
}

// List returns alerts newest first.
func (s *Service) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Alert, int, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, filter, limit, offset)
}

// Acknowledge marks an ACTIVE alert as seen by an operator.
func (s *Service) Acknowledge(ctx context.Context, id, operator string) (Alert, error) {
	updated, err := s.repo.Acknowledge(ctx, id, operator)
	if err != nil {
		return Alert{}, err
	}
	s.bus.Publish(ctx, events.AlertAcknowledged{
		At:       s.now(),
		AlertID:  updated.ID,
		Operator: operator,
	})
	return updated, nil
}

// Resolve closes an alert.
func (s *Service) Resolve(ctx context.Context, id, operator string) (Alert, error) {
	updated, err := s.repo.Resolve(ctx, id, operator)
	if err != nil {
		return Alert{}, err
	}
	s.bus.Publish(ctx, events.AlertResolved{
		At:       s.now(),
		AlertID:  updated.ID,
		Operator: operator,
	})
	return updated, nil
}

// SetIncident links an alert to an incident workspace.
func (s *Service) SetIncident(ctx context.Context, alertID, incidentID string) error {
	return s.repo.SetIncident(ctx, alertID, incidentID)
}

func normalizeSeverity(raw string) Severity {
	switch Severity(raw) {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return Severity(raw)
	default:
		return SeverityMedium
	}
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
