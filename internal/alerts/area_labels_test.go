package alerts

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

func TestAreaSubjectNamesThePhenomenon(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{kind: "hazard", want: "Hazard area"},
		{kind: "weather", want: "Weather cell"},
		{kind: "environmental", want: "Environmental condition"},
		{kind: "infrastructure", want: "Infrastructure disruption"},
		{kind: "crowd", want: "Crowd area"},
		// An unrecognised kind is still an event that reached a zone. Saying so
		// keeps the alert actionable without inventing a classification.
		{kind: "something-new", want: "Unidentified area event"},
		{kind: "", want: "Unidentified area event"},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			subject := areaSubject(tc.kind)
			if subject.Label != tc.want {
				t.Fatalf("label = %q, want %q", subject.Label, tc.want)
			}
			// An area has no classification, so nothing about it is hedged: the
			// system knows a hazard-shaped thing reached the zone.
			if !subject.Confirmed {
				t.Error("an area subject must not be hedged; there is no hypothesis to hedge")
			}
		})
	}
}

func TestOverlapTitleAndMessageNameTheZone(t *testing.T) {
	subject := areaSubject("hazard")

	if got := subject.OverlapTitle("Restricted Zone A"); got != "Hazard area overlaps Restricted Zone A" {
		t.Fatalf("title = %q", got)
	}
	message := subject.OverlapMessage("hazard", "Restricted Zone A")
	if message == "" {
		t.Fatal("message must not be empty")
	}
	for _, want := range []string{"hazard", "Restricted Zone A"} {
		if !contains(message, want) {
			t.Errorf("message %q does not name %q", message, want)
		}
	}
}

func TestOverlapTitleFallsBackForAnUnnamedZone(t *testing.T) {
	subject := areaSubject("weather")
	if got := subject.OverlapTitle(""); got != "Weather cell overlaps a restricted zone" {
		t.Fatalf("title = %q, want a generic zone reference", got)
	}
}

func TestAreaOverlapAlertLinksTheObservationNotATrack(t *testing.T) {
	repo := &fakeRepository{}
	bus := events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := NewServiceWithClassifier(repo, bus, nil)

	svc.HandleAreaGeofenceOverlapped(context.Background(), events.AreaGeofenceOverlapped{
		ObservationID: "obs_fire_1",
		SourceID:      "firms",
		AreaKind:      "hazard",
		GeofenceID:    "geo_1",
		GeofenceName:  "Restricted Zone A",
		GeofenceType:  "restricted",
		Severity:      "high",
		OverlapM2:     4_000_000,
		AreaM2:        9_000_000,
		Center:        geo.Point{Lat: 24.75, Lng: 46.65},
	})

	if len(repo.createdCalls) != 1 {
		t.Fatalf("created %d alerts, want 1", len(repo.createdCalls))
	}
	created := repo.createdCalls[0]

	// The evidence is the observation. Pointing at a track would fabricate a
	// relationship the platform never established.
	if created.TrackID != "" {
		t.Errorf("area alert carries track id %q; an area has no track", created.TrackID)
	}
	if created.GeofenceID != "geo_1" {
		t.Errorf("geofence = %q, want geo_1", created.GeofenceID)
	}
	if created.Severity != SeverityHigh {
		t.Errorf("severity = %q, want it taken from the zone", created.Severity)
	}
	if created.Type != areaOverlapType {
		t.Errorf("type = %q, want %q", created.Type, areaOverlapType)
	}
	if created.Title != "Hazard area overlaps Restricted Zone A" {
		t.Errorf("title = %q", created.Title)
	}

	// Provenance must explain the alert without a second lookup.
	ref := created.SourceReference
	if ref["observationId"] != "obs_fire_1" {
		t.Errorf("sourceReference observationId = %v, want the reporting observation", ref["observationId"])
	}
	if ref["sourceId"] != "firms" {
		t.Errorf("sourceReference sourceId = %v, want firms", ref["sourceId"])
	}
	if ref["areaKind"] != "hazard" {
		t.Errorf("sourceReference areaKind = %v, want hazard", ref["areaKind"])
	}
	// The overlap and total areas are what let an operator judge severity
	// instead of taking "it overlaps" on faith.
	if ref["overlapM2"] != 4_000_000.0 {
		t.Errorf("overlapM2 = %v, want the measured overlap", ref["overlapM2"])
	}
	if ref["areaM2"] != 9_000_000.0 {
		t.Errorf("areaM2 = %v, want the total footprint", ref["areaM2"])
	}
	if center, ok := ref["center"].(map[string]any); !ok || center["lat"] != 24.75 {
		t.Errorf("center = %v, want the footprint centroid", ref["center"])
	}
}

func TestAreaOverlapSuppressesRepeatForTheSameObservation(t *testing.T) {
	existing := Alert{
		ID:              "alr_existing",
		GeofenceID:      "geo_1",
		SourceReference: map[string]any{"observationId": "obs_fire_1"},
	}
	repo := &fakeRepository{areaUnresolved: &existing}
	svc, _ := newTestService(repo)

	svc.HandleAreaGeofenceOverlapped(context.Background(), events.AreaGeofenceOverlapped{
		ObservationID: "obs_fire_1",
		GeofenceID:    "geo_1",
		GeofenceName:  "Restricted Zone A",
		Severity:      "high",
	})

	// Public hazard feeds report the same fire repeatedly. Without this the
	// alert centre fills with duplicates and the operator learns to ignore it.
	if len(repo.createdCalls) != 0 {
		t.Fatalf("created %d alerts for an already-alerted observation, want none", len(repo.createdCalls))
	}
}

func TestAreaOverlapStillAlertsAfterResolution(t *testing.T) {
	// Once an operator has resolved the first alert, the next report of the same
	// observation overlapping the zone is new information, not a duplicate.
	repo := &fakeRepository{}
	svc, _ := newTestService(repo)

	svc.HandleAreaGeofenceOverlapped(context.Background(), events.AreaGeofenceOverlapped{
		ObservationID: "obs_fire_1",
		GeofenceID:    "geo_1",
		GeofenceName:  "Restricted Zone A",
		Severity:      "high",
	})

	if len(repo.createdCalls) != 1 {
		t.Fatalf("created %d alerts, want 1", len(repo.createdCalls))
	}
}

func TestAreaOverlapDoesNotCollideWithATrackBreach(t *testing.T) {
	// An area overlap and a track breach on the same zone are different facts and
	// must both be visible. Sharing a dedup key would hide one of them.
	repo := &fakeRepository{areaUnresolved: &Alert{
		ID:              "alr_track",
		GeofenceID:      "geo_1",
		TrackID:         "trk_1",
		SourceReference: map[string]any{},
	}}
	svc, _ := newTestService(repo)

	svc.HandleAreaGeofenceOverlapped(context.Background(), events.AreaGeofenceOverlapped{
		ObservationID: "obs_fire_1",
		GeofenceID:    "geo_1",
		GeofenceName:  "Restricted Zone A",
		Severity:      "high",
	})

	if len(repo.createdCalls) != 1 {
		t.Fatalf("created %d alerts, want the area alert to be independent of a track breach", len(repo.createdCalls))
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOfSub(haystack, needle) >= 0)
}

func indexOfSub(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
