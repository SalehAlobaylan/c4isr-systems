package arearule

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeOverlaps struct {
	zones []Geofence
	err   error
	calls []string
}

func (f *fakeOverlaps) IntersectingArea(_ context.Context, area Overlap) ([]Geofence, error) {
	f.calls = append(f.calls, area.GeoJSON)
	return f.zones, f.err
}

// fakeFailures counts notified evaluation failures.
type fakeFailures struct{ count int }

func (f *fakeFailures) ObserveAreaEvaluationFailure() { f.count++ }

func newTestService(t *testing.T, overlaps OverlapQuerier) (*Service, *recorder) {
	t.Helper()
	log := &recorder{}
	bus := events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	bus.Subscribe(events.TopicAreaGeofenceOverlapped, log.record)
	return NewService(overlaps, bus), log
}

type recorder struct {
	events []events.AreaGeofenceOverlapped
}

func (r *recorder) record(_ context.Context, ev events.Event) {
	if overlapped, ok := ev.(events.AreaGeofenceOverlapped); ok {
		r.events = append(r.events, overlapped)
	}
}

func areaObservation() events.ObservationReceived {
	return events.ObservationReceived{
		ObservationID: "obs_fire_1",
		SourceID:      "firms",
		Type:          "hazard.fire",
		AreaGeoJSON:   `{"type":"Polygon","coordinates":[[[46.6,24.7],[46.7,24.7],[46.7,24.8],[46.6,24.7]]]}`,
		AreaKind:      "hazard",
		AreaMeters2:   1_234_567,
		AreaCenter:    geo.Point{Lat: 24.75, Lng: 46.65},
	}
}

func TestAreaOverlapPublishesForEveryIntersectingZone(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{
		{ID: "geo_1", Name: "Restricted Zone A", Type: "restricted", Severity: "high"},
		{ID: "geo_2", Name: "Airport", Type: "protected", Severity: "critical",
			Metadata: map[string]any{"overlapM2": 4_200_000.0}},
	}}
	svc, log := newTestService(t, overlaps)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	if len(log.events) != 2 {
		t.Fatalf("published %d events, want one per intersecting zone", len(log.events))
	}

	first := log.events[0]
	if first.GeofenceID != "geo_1" || first.GeofenceName != "Restricted Zone A" {
		t.Fatalf("first event = %+v", first)
	}
	if first.Severity != "high" {
		t.Errorf("severity = %q, want high", first.Severity)
	}
	// The overlap area is what lets a rule tell a graze from a headline event.
	if log.events[1].OverlapM2 != 4_200_000 {
		t.Errorf("overlap area = %v, want the value the spatial layer reported", log.events[1].OverlapM2)
	}

	// Every event carries the observation, because that is the evidence. Without
	// it the alert has no trail back to the source that reported the fire.
	for _, ev := range log.events {
		if ev.ObservationID != "obs_fire_1" {
			t.Errorf("observation id = %q, want the reporting observation", ev.ObservationID)
		}
		if ev.SourceID != "firms" {
			t.Errorf("source id = %q, want firms", ev.SourceID)
		}
		if ev.AreaKind != "hazard" {
			t.Errorf("area kind = %q, want hazard", ev.AreaKind)
		}
		if ev.AreaM2 != 1_234_567 {
			t.Errorf("area size = %v, want the reported footprint", ev.AreaM2)
		}
	}
}

func TestPointObservationsAreNotEvaluatedByTheAreaRule(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone", Severity: "high"}}}
	svc, log := newTestService(t, overlaps)

	// A point observation belongs to the track pipeline. Evaluating it here as
	// well would re-derive a track's containment from its source observation and
	// raise a duplicate breach for the same movement.
	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		ObservationID: "obs_point_1",
		SourceID:      "radar-01",
		Position:      &geo.Point{Lat: 24.7, Lng: 46.6},
	})

	if len(log.events) != 0 {
		t.Fatalf("point observation produced %d area events, want none", len(log.events))
	}
	if len(overlaps.calls) != 0 {
		t.Fatalf("point observation was evaluated against geofences %d times, want none", len(overlaps.calls))
	}
}

func TestDuplicateReplayDoesNotReannounceAnOverlap(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone", Severity: "high"}}}
	svc, log := newTestService(t, overlaps)

	replayed := areaObservation()
	replayed.Duplicate = true
	svc.HandleObservationReceived(context.Background(), replayed)

	if len(log.events) != 0 {
		t.Fatalf("duplicate re-announced %d events, want none", len(log.events))
	}
}

func TestRetryReplaysAPreviouslyFailedOverlap(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone", Severity: "high"}}}
	svc, log := newTestService(t, overlaps)

	// A retry means a previous publish failed and the evidence is being
	// re-driven. The operator is owed the alert, so it must be published.
	retry := areaObservation()
	retry.Duplicate = true
	retry.Retry = true
	svc.HandleObservationReceived(context.Background(), retry)

	if len(log.events) != 1 {
		t.Fatalf("retry published %d events, want 1", len(log.events))
	}
}

func TestNoOverlapPublishesNothing(t *testing.T) {
	overlaps := &fakeOverlaps{zones: nil}
	svc, log := newTestService(t, overlaps)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	if len(log.events) != 0 {
		t.Fatalf("published %d events for an area overlapping nothing", len(log.events))
	}
	if len(overlaps.calls) != 1 {
		t.Fatalf("evaluated %d times, want once", len(overlaps.calls))
	}
}

func TestSpatialFailurePublishesNothingRatherThanGuessing(t *testing.T) {
	overlaps := &fakeOverlaps{err: errors.New("postgis unavailable")}
	failures := &fakeFailures{}
	svc, log := newTestService(t, overlaps)
	svc.SetFailureObserver(failures)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	// A failed containment query is not evidence of no overlap. Publishing
	// nothing leaves the operator with silence; guessing would be worse.
	if len(log.events) != 0 {
		t.Fatalf("published %d events after a spatial failure", len(log.events))
	}
	// Silence is only safe if it is visible. Without this the failure is
	// indistinguishable from "nothing is happening" until someone goes looking.
	if failures.count != 1 {
		t.Fatalf("failure observer notified %d times, want 1", failures.count)
	}
}

func TestSuccessfulEvaluationDoesNotReportAFailure(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone", Severity: "high"}}}
	failures := &fakeFailures{}
	svc, _ := newTestService(t, overlaps)
	svc.SetFailureObserver(failures)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	// A counter that fires on success is a counter nobody trusts.
	if failures.count != 0 {
		t.Fatalf("failure observer notified %d times on a successful evaluation", failures.count)
	}
}

func TestAreaWithoutGeometryIsIgnored(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone"}}}
	svc, log := newTestService(t, overlaps)

	// A kind without geometry cannot be evaluated. Treating it as an area would
	// submit an empty polygon to PostGIS and produce a confusing error.
	ev := areaObservation()
	ev.AreaGeoJSON = ""
	svc.HandleObservationReceived(context.Background(), ev)

	if len(log.events) != 0 {
		t.Fatalf("published %d events for an area with no geometry", len(log.events))
	}
}

func TestOverlappingZoneIsPassedThroughWithItsMetadata(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{
		{ID: "geo_1", Name: "Zone", Severity: "high",
			Metadata: map[string]any{"overlapM2": 1_500.5}},
	}}
	svc, log := newTestService(t, overlaps)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	// A fractional overlap proves the value survived as a float rather than
	// being truncated to an int somewhere in the adapter.
	if got := log.events[0].OverlapM2; got != 1_500.5 {
		t.Fatalf("overlap = %v, want 1500.5", got)
	}
}

func TestMissingOverlapMetadataReadsAsZero(t *testing.T) {
	overlaps := &fakeOverlaps{zones: []Geofence{{ID: "geo_1", Name: "Zone"}}}
	svc, log := newTestService(t, overlaps)

	svc.HandleObservationReceived(context.Background(), areaObservation())

	// A zone with no recorded overlap must not panic; zero is honest.
	if got := log.events[0].OverlapM2; got != 0 {
		t.Fatalf("overlap = %v, want 0 when unreported", got)
	}
}
