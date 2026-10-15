package observations

import (
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

func timeFixture() time.Time {
	return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
}

func nowFixture() time.Time {
	return time.Date(2026, 10, 13, 12, 1, 0, 0, time.UTC)
}

func hazardArea() *Area {
	// The ring is deliberately open. A producer supplies distinct vertices and
	// the encoder closes it; a closed ring here would encode a doubled vertex.
	return &Area{
		Kind: AreaHazard,
		Polygon: []geo.Point{
			{Lat: 24.70, Lng: 46.60},
			{Lat: 24.80, Lng: 46.60},
			{Lat: 24.80, Lng: 46.70},
			{Lat: 24.70, Lng: 46.70},
		},
	}
}

func TestAreaValidateAcceptsAWellFormedFootprint(t *testing.T) {
	if err := hazardArea().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestAreaValidateRejectsUnusableFootprints(t *testing.T) {
	cases := []struct {
		name string
		area Area
	}{
		{
			name: "unknown kind",
			area: Area{Kind: "unclear", Polygon: hazardArea().Polygon},
		},
		{
			name: "too few points",
			area: Area{Kind: AreaHazard, Polygon: []geo.Point{{Lat: 1, Lng: 1}, {Lat: 2, Lng: 2}}},
		},
		{
			name: "out of range coordinate",
			area: Area{Kind: AreaHazard, Polygon: []geo.Point{
				{Lat: 91, Lng: 46.6}, {Lat: 24.8, Lng: 46.6}, {Lat: 24.8, Lng: 46.7},
			}},
		},
		{
			name: "degenerate collinear ring",
			area: Area{Kind: AreaHazard, Polygon: []geo.Point{
				{Lat: 24.7, Lng: 46.6}, {Lat: 24.7, Lng: 46.7}, {Lat: 24.7, Lng: 46.8},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.area.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want a rejection for %s", tc.name)
			}
		})
	}
}

func TestAreaAndTrackHintAreMutuallyExclusive(t *testing.T) {
	in := CreateInput{
		SourceID:   "firms",
		Type:       "hazard.fire",
		ObservedAt: timeFixture(),
		Area:       hazardArea(),
		TrackHint:  "track-unknown-01",
	}
	if err := in.Validate(nowFixture()); err == nil {
		t.Fatal("an area observation carrying a trackHint must be rejected")
	}
}

func TestAreaAndPositionAreMutuallyExclusive(t *testing.T) {
	in := CreateInput{
		SourceID:   "firms",
		Type:       "hazard.fire",
		ObservedAt: timeFixture(),
		Area:       hazardArea(),
		Position:   &geo.Point{Lat: 24.75, Lng: 46.65},
	}
	// The footprint has extent; a separate point adds a second, contradictory
	// answer to "where is it" and the point could sit outside the footprint.
	if err := in.Validate(nowFixture()); err == nil {
		t.Fatal("an area observation carrying a position must be rejected")
	}
}

func TestPointObservationWithoutAnAreaRemainsValid(t *testing.T) {
	in := CreateInput{
		SourceID:   "radar-01",
		Type:       "track.observation",
		ObservedAt: timeFixture(),
		Position:   &geo.Point{Lat: 24.7, Lng: 46.6},
		TrackHint:  "track-unknown-01",
	}
	if err := in.Validate(nowFixture()); err != nil {
		t.Fatalf("a point observation must still be valid: %v", err)
	}
}

func TestAreaGeoJSONClosesTheRing(t *testing.T) {
	raw := hazardArea().GeoJSON()
	// The ring is closed by repeating the first vertex, which is what PostGIS
	// and the browser both expect from a GeoJSON polygon.
	if want := `"coordinates":[[[46.6,24.7],[46.6,24.8],[46.7,24.8],[46.7,24.7],[46.6,24.7]]]`; !contains(raw, want) {
		t.Fatalf("GeoJSON = %s, want a closed ring containing %s", raw, want)
	}
}

func TestAreaRoundTripsThroughGeoJSON(t *testing.T) {
	original := hazardArea()
	ring, ok := geoRingFromGeoJSON(original.GeoJSON())
	if !ok {
		t.Fatal("a generated GeoJSON polygon must parse back")
	}
	if len(ring) != len(original.Polygon) {
		t.Fatalf("ring length = %d, want %d", len(ring), len(original.Polygon))
	}
	// The closing vertex is dropped so a producer-supplied ring round-trips
	// unchanged; keeping it would silently grow the polygon on every cycle.
	for i := range ring {
		if ring[i] != original.Polygon[i] {
			t.Fatalf("ring[%d] = %v, want %v", i, ring[i], original.Polygon[i])
		}
	}
}

func TestGeoRingRejectsUnusableInput(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "not json", raw: `{`},
		{name: "not a polygon", raw: `{"type":"Point","coordinates":[46.6,24.7]}`},
		{name: "too few vertices", raw: `{"type":"Polygon","coordinates":[[[46.6,24.7],[46.7,24.8]]]}`},
		{name: "malformed pair", raw: `{"type":"Polygon","coordinates":[[[46.6]]]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := geoRingFromGeoJSON(tc.raw); ok {
				t.Fatalf("geoRingFromGeoJSON(%s) accepted an unusable geometry", tc.name)
			}
		})
	}
}

func TestAreaCentroidAndSize(t *testing.T) {
	area := hazardArea()
	if area.AreaMeters2() <= 0 {
		t.Fatalf("area = %v, want a positive footprint", area.AreaMeters2())
	}
	center := area.Centroid()
	if center.Lat < 24.70 || center.Lat > 24.80 || center.Lng < 46.60 || center.Lng > 46.70 {
		t.Fatalf("centroid = %v, want it inside the footprint", center)
	}
}

func TestIsAreaDistinguishesTheTwoShapes(t *testing.T) {
	if !(Observation{Area: hazardArea()}).IsArea() {
		t.Error("an observation with an area must report IsArea")
	}
	if (Observation{Position: &geo.Point{Lat: 1, Lng: 1}}).IsArea() {
		t.Error("a point observation must not report IsArea")
	}
}

func TestNormalizeTrimsTheAreaKind(t *testing.T) {
	in := CreateInput{Area: &Area{Kind: AreaKind("  hazard  ")}}
	in.Normalize()
	if in.Area.Kind != AreaHazard {
		t.Fatalf("kind = %q, want %q", in.Area.Kind, AreaHazard)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
