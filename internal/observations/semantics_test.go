package observations

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"testing"
	"time"
)

func TestEventMeaningIsIndependentOfPointGeometry(t *testing.T) {
	for _, typ := range []string{"event.earthquake", "detection.thermal", "earthquake", "thermal.detection"} {
		in := CreateInput{SourceID: "source", Type: typ, ObservedAt: time.Now(), Position: &geo.Point{Lat: 21, Lng: 39}}
		in.Normalize()
		if in.SemanticKind != PointEvent || in.Validate(time.Now()) != nil {
			t.Fatalf("reported point became moving: %s", typ)
		}
		in.SemanticKind = Moving
		if in.Validate(time.Now()) == nil {
			t.Fatalf("explicit moving override accepted: %s", typ)
		}
	}
	in := CreateInput{SourceID: "source", Type: "radar.plot", ObservedAt: time.Now(), Position: &geo.Point{Lat: 21, Lng: 39}, TrackHint: "legacy-track"}
	in.Normalize()
	if in.SemanticKind != Moving || in.Validate(time.Now()) != nil {
		t.Fatal("legacy motion changed")
	}
	in.SemanticKind = PointEvent
	if in.Validate(time.Now()) == nil {
		t.Fatal("event accepted a track hint")
	}
	in.TrackHint = ""
	in.GeometryRole = "measurement_footprint"
	if in.Validate(time.Now()) == nil {
		t.Fatal("a point was labelled a measured footprint")
	}
}
