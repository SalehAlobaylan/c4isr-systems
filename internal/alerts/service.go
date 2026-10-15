package alerts

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
	"go.opentelemetry.io/otel/attribute"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

const (
	geofenceBreachType = "geofence.breach"
	// areaOverlapType is raised when an area observation - a hazard, a weather
	// cell, an infrastructure failure - overlaps a geofence. It has no track,
	// so the alert links to the observation instead.
	areaOverlapType = "geofence.area.overlap"
)

// Service implements alert raising and lifecycle transitions.
type Service struct {
	repo       Repository
	bus        *events.Dispatcher
	classifier ClassifierLabeler
	now        func() time.Time
}

// NewService wires an alert service and subscribes it to geofence breaches.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	return NewServiceWithClassifier(repo, bus, nil)
}

// NewServiceWithClassifier wires an alert service that can name the subject of
// an alert in operator language instead of by identifier. A nil classifier is
// allowed and yields the honest "Unidentified contact" wording, which is still
// better than printing a ULID at an operator.
func NewServiceWithClassifier(repo Repository, bus *events.Dispatcher, classifier ClassifierLabeler) *Service {
	s := &Service{
		repo:       repo,
		bus:        bus,
		classifier: classifier,
		now:        func() time.Time { return time.Now().UTC() },
	}
	bus.Subscribe(events.TopicGeofenceBreached, func(ctx context.Context, ev events.Event) {
		if breach, ok := ev.(events.GeofenceBreached); ok {
			s.HandleGeofenceBreached(ctx, breach)
		}
	})
	bus.Subscribe(events.TopicAreaGeofenceOverlapped, func(ctx context.Context, ev events.Event) {
		if overlap, ok := ev.(events.AreaGeofenceOverlapped); ok {
			s.HandleAreaGeofenceOverlapped(ctx, overlap)
		}
	})
	return s
}

// HandleAreaGeofenceOverlapped raises an alert when an area observation
// overlaps a geofence.
//
// This is the whole point of the area model: a hazard that overlaps a
// restricted zone demands attention immediately, and there is no track to
// drive that detection. The alert links to the observation, so the evidence
// trail from alert back to source observation stays intact - the same
// provenance guarantee a track-derived alert gives.
func (s *Service) HandleAreaGeofenceOverlapped(ctx context.Context, ev events.AreaGeofenceOverlapped) {
	spanCtx, span := observability.StartSpan(ctx, "c4isr.alert.generate",
		attribute.String("alert.type", areaOverlapType),
		attribute.String("observation.id", ev.ObservationID),
		attribute.String("geofence.id", ev.GeofenceID),
	)
	ctx = spanCtx
	var spanErr error
	defer func() { observability.EndSpan(span, spanErr) }()

	// Suppress repeats while an unresolved alert for the same observation and
	// zone stands. Public hazard feeds report the same fire repeatedly, and
	// without this the alert centre becomes noise that an operator learns to
	// ignore - which is worse than having no alert at all.
	existing, err := s.repo.FindUnresolvedForObservationGeofence(ctx, ev.ObservationID, ev.GeofenceID)
	if err != nil {
		spanErr = err
		slog.Default().Error("alerts: find unresolved area overlap alert",
			"observation_id", ev.ObservationID, "geofence_id", ev.GeofenceID, "error", err)
		return
	}
	if existing != nil {
		return
	}

	subject := areaSubject(ev.AreaKind)
	sourceReference := map[string]any{
		"rule":         areaOverlapType,
		"geofenceId":   ev.GeofenceID,
		"geofenceName": ev.GeofenceName,
		"geofenceType": ev.GeofenceType,
		// The observation, not a track, is the evidence. Recording the
		// observation id here is what keeps the chain inspectable.
		"observationId": ev.ObservationID,
		"sourceId":      ev.SourceID,
		"areaKind":      ev.AreaKind,
		"overlapM2":     ev.OverlapM2,
		"areaM2":        ev.AreaM2,
		"center":        map[string]any{"lat": ev.Center.Lat, "lng": ev.Center.Lng},
		"subjectLabel":  subject.Label,
	}
	if scope, ok := runctx.ScopeFrom(ctx); ok {
		sourceReference["scenarioRunId"] = scope.RunID
		sourceReference["resourceNamespace"] = scope.ResourceNamespace
	}

	alert := Alert{
		ID:              ids.New("alr"),
		Type:            areaOverlapType,
		Severity:        normalizeSeverity(ev.Severity),
		State:           StateActive,
		Title:           subject.OverlapTitle(ev.GeofenceName),
		Message:         subject.OverlapMessage(ev.AreaKind, ev.GeofenceName),
		SourceReference: sourceReference,
		// No TrackID: an area is not a track, and pointing at one would be a
		// fabricated relationship.
		GeofenceID: ev.GeofenceID,
	}

	created, err := s.repo.Create(ctx, alert)
	if err != nil {
		if errors.Is(err, ErrDuplicateAreaOverlap) {
			return
		}
		spanErr = err
		slog.Default().Error("alerts: create area overlap alert",
			"observation_id", ev.ObservationID, "geofence_id", ev.GeofenceID, "error", err)
		return
	}

	s.bus.Publish(ctx, events.AlertCreated{
		At:         s.now(),
		AlertID:    created.ID,
		Type:       created.Type,
		Severity:   string(created.Severity),
		Title:      created.Title,
		AssetID:    created.AssetID,
		GeofenceID: created.GeofenceID,
	})
}

// HandleGeofenceBreached raises an alert for a breach, suppressing duplicates
// while an alert for the same geofence and track remains unresolved.
func (s *Service) HandleGeofenceBreached(ctx context.Context, ev events.GeofenceBreached) {
	spanCtx, span := observability.StartSpan(ctx, "c4isr.alert.generate",
		attribute.String("geofence.id", ev.GeofenceID),
		attribute.String("track.id", ev.TrackID),
	)
	ctx = spanCtx
	var spanErr error
	defer func() { observability.EndSpan(span, spanErr) }()

	existing, err := s.repo.FindUnresolvedForGeofenceTrack(ctx, ev.GeofenceID, ev.TrackID)
	if err != nil {
		spanErr = err
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

	// The subject is resolved once, here, and the resulting words are stored on
	// the alert. Re-deriving them on read would mean an alert silently rewrote
	// itself as the track was reclassified, destroying the record of what the
	// system actually believed when it raised the alarm.
	subject := DescribeTrack(ctx, ev.TrackID, s.classifier)
	sourceReference["subjectLabel"] = subject.Label
	sourceReference["subjectConfirmed"] = subject.Confirmed

	alert := Alert{
		ID:              ids.New("alr"),
		Type:            geofenceBreachType,
		Severity:        normalizeSeverity(ev.Severity),
		State:           StateActive,
		Title:           subject.BreachTitle(ev.GeofenceName),
		Message:         subject.BreachMessage(ev.GeofenceType, ev.GeofenceName),
		SourceReference: sourceReference,
		TrackID:         ev.TrackID,
		GeofenceID:      ev.GeofenceID,
	}
	created, err := s.repo.Create(ctx, alert)
	if err != nil {
		if errors.Is(err, ErrDuplicateGeofenceBreach) {
			return
		}
		spanErr = err
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
