// Package arearule routes area observations to spatial rules.
//
// It exists as a separate concern from the geospatial module for one reason:
// an area observation must raise attention without ever becoming a track. If
// this logic lived inside the track pipeline, an area could leak into
// correlation and produce exactly the nonsense the area model exists to avoid -
// a "track" for a wildfire, complete with a speed the platform invented.
//
// The module is deliberately thin. It evaluates one question - does this
// footprint overlap a zone that matters? - and publishes an event. Deciding
// whether that event deserves an operator is the alerts module's job, not this
// one's.
package arearule

import (
	"context"
	"log/slog"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"go.opentelemetry.io/otel/attribute"
)

// OverlapQuerier answers which active zones a footprint overlaps. Implemented by
// the geospatial module, which delegates the decision to PostGIS.
type OverlapQuerier interface {
	IntersectingArea(ctx context.Context, area Overlap) ([]Geofence, error)
}

// Overlap is the footprint submitted for evaluation, in GeoJSON form.
type Overlap struct {
	GeoJSON string
}

// Geofence is a zone an area may overlap. It is a structural subset of the
// geospatial type so this package does not import geospatial internals, and the
// adapter below converts between the two.
type Geofence struct {
	ID       string
	Name     string
	Type     string
	Severity string
	Metadata map[string]any
}

// overlapMeters2 reads the overlap area the geospatial layer recorded.
func (g Geofence) overlapMeters2() float64 {
	if g.Metadata == nil {
		return 0
	}
	switch value := g.Metadata["overlapM2"].(type) {
	case float64:
		return value
	case int64:
		return float64(value)
	case int:
		return float64(value)
	default:
		return 0
	}
}

// Service evaluates area observations against geofences.
type Service struct {
	overlaps OverlapQuerier
	bus      *events.Dispatcher
	now      func() time.Time
	// observer receives evaluation durations so the operational picture stays
	// measurable on the same axis as track-driven containment.
	observer EvaluationObserver
	// failures is notified when an evaluation cannot complete, so a broken
	// spatial query is visible rather than indistinguishable from calm.
	failures FailureObserver
}

// EvaluationObserver receives one duration per area evaluation.
type EvaluationObserver interface {
	ObserveGeofenceEvaluation(time.Duration)
}

// FailureObserver is notified when a footprint could not be evaluated.
//
// A containment query that fails leaves the operator with no alert and no
// visible reason, which is indistinguishable from "nothing is happening". The
// counter makes that silence observable so it can be alerted on.
type FailureObserver interface {
	ObserveAreaEvaluationFailure()
}

// NewService wires the area rule and subscribes it to observation events.
func NewService(overlaps OverlapQuerier, bus *events.Dispatcher) *Service {
	s := &Service{
		overlaps: overlaps,
		bus:      bus,
		now:      func() time.Time { return time.Now().UTC() },
	}
	bus.Subscribe(events.TopicObservationReceived, func(ctx context.Context, ev events.Event) {
		if received, ok := ev.(events.ObservationReceived); ok {
			s.HandleObservationReceived(ctx, received)
		}
	})
	return s
}

// SetObserver attaches an optional evaluation observer.
func (s *Service) SetObserver(observer EvaluationObserver) {
	s.observer = observer
}

// SetFailureObserver attaches an optional failure observer.
func (s *Service) SetFailureObserver(observer FailureObserver) {
	s.failures = observer
}

// HandleObservationReceived evaluates an area observation against geofences.
//
// Point observations return immediately: they belong to the track pipeline and
// must not be evaluated twice, or a track's containment would be re-derived
// from its source observation and produce duplicate breaches.
func (s *Service) HandleObservationReceived(ctx context.Context, ev events.ObservationReceived) {
	if ev.AreaKind == "" || ev.AreaGeoJSON == "" {
		return
	}

	started := time.Now()
	spanCtx, span := observability.StartSpan(ctx, "c4isr.area.evaluate",
		attribute.String("observation.id", ev.ObservationID),
		attribute.String("area.kind", ev.AreaKind),
		attribute.String("geofence.evaluation", "intersection"),
	)
	ctx = spanCtx
	var spanErr error
	defer func() {
		if s.observer != nil {
			s.observer.ObserveGeofenceEvaluation(time.Since(started))
		}
		observability.EndSpan(span, spanErr)
	}()

	zones, err := s.overlaps.IntersectingArea(ctx, Overlap{GeoJSON: ev.AreaGeoJSON})
	if err != nil {
		spanErr = err
		if s.failures != nil {
			s.failures.ObserveAreaEvaluationFailure()
		}
		slog.Default().Error("arearule: evaluate area overlap", "error", err,
			"observation_id", ev.ObservationID)
		return
	}

	now := s.now()
	for _, zone := range zones {
		// A duplicate replay must not re-announce the same overlap. A retry is
		// the exception: it means a prior publish failed and the evidence is
		// being re-driven, so the operator is owed the alert.
		if ev.Duplicate && !ev.Retry {
			continue
		}
		s.bus.Publish(ctx, events.AreaGeofenceOverlapped{
			At:            now,
			ObservationID: ev.ObservationID,
			SourceID:      ev.SourceID,
			AreaKind:      ev.AreaKind,
			GeofenceID:    zone.ID,
			GeofenceName:  zone.Name,
			GeofenceType:  zone.Type,
			Severity:      zone.Severity,
			OverlapM2:     zone.overlapMeters2(),
			AreaM2:        ev.AreaMeters2,
			Center:        ev.AreaCenter,
		})
	}
}
