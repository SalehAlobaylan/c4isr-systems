package tracks

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// An area observation must never become a track.
//
// This is the single most important property of the area model. If an area
// reached correlation, the platform would invent a track for a wildfire,
// complete with a velocity derived from how the fire moved between reports -
// a number with no physical meaning that would then drive geofence containment
// and alerting. The test asserts the absence of any repository write, which is
// stronger than asserting the absence of a resulting track.
func TestAreaObservationIsNeverCorrelatedIntoATrack(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil, testDispatcher())

	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		ObservationID: "obs_fire_1",
		SourceID:      "firms",
		Type:          "hazard.fire",
		ObservedAt:    time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
		AreaGeoJSON:   `{"type":"Polygon","coordinates":[[[46.6,24.7],[46.7,24.7],[46.7,24.8],[46.6,24.7]]]}`,
		AreaKind:      "hazard",
		AreaMeters2:   1_000_000,
	})

	if len(repo.tracks) != 0 {
		t.Fatalf("an area observation created %d tracks, want none", len(repo.tracks))
	}
	if len(repo.attached) != 0 {
		t.Fatalf("an area observation attached %d pieces of evidence to a track, want none", len(repo.attached))
	}
	if len(repo.history) != 0 {
		t.Fatalf("an area observation wrote %d history points, want none", len(repo.history))
	}
	if len(repo.states) != 0 {
		t.Fatalf("an area observation wrote %d track states, want none", len(repo.states))
	}
}

// An area observation that also carries a retry flag must still be skipped.
// Retry exists to finish an interrupted projection; it must not become a way to
// smuggle an area into the track pipeline.
func TestRetriedAreaObservationIsStillNotCorrelated(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil, testDispatcher())

	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		ObservationID: "obs_fire_1",
		Duplicate:     true,
		Retry:         true,
		AreaGeoJSON:   `{"type":"Polygon","coordinates":[[[46.6,24.7],[46.7,24.7],[46.7,24.8],[46.6,24.7]]]}`,
		AreaKind:      "hazard",
	})

	if len(repo.tracks) != 0 {
		t.Fatalf("a retried area observation created %d tracks, want none", len(repo.tracks))
	}
}

// A point observation must still be correlated. Without this the area guard
// could pass simply by breaking correlation entirely.
func TestPointObservationIsStillCorrelated(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil, testDispatcher())

	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		ObservationID: "obs_point_1",
		SourceID:      "radar-01",
		Type:          "track.observation",
		ObservedAt:    time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
		Position:      &geo.Point{Lat: 24.7, Lng: 46.6},
		TrackHint:     "track-unknown-01",
	})

	if len(repo.tracks) == 0 {
		t.Fatal("a point observation must still be correlated into a track")
	}
}
