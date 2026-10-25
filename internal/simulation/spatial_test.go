package simulation

import (
	"math"
	"testing"
	"time"
)

func height(v float64) *float64 { return &v }
func TestECEFKnownAxesAndLocalRoundTrips(t *testing.T) {
	cases := []struct {
		p    Position
		want Vector
	}{{Position{0, 0, height(0), "WGS84_ELLIPSOID"}, Vector{6378137, 0, 0}}, {Position{90, 0, height(0), "WGS84_ELLIPSOID"}, Vector{0, 6378137, 0}}, {Position{0, 90, height(0), "WGS84_ELLIPSOID"}, Vector{0, 0, 6356752.314245179}}}
	for _, c := range cases {
		v, err := ECEF(c.p)
		if err != nil || math.Abs(v.X-c.want.X) > 1e-6 || math.Abs(v.Y-c.want.Y) > 1e-6 || math.Abs(v.Z-c.want.Z) > 1e-6 {
			t.Fatalf("ECEF %+v: %+v %v", c.p, v, err)
		}
	}
	f := DefaultFrame(Position{39.5, 21.5, height(75), "WGS84_ELLIPSOID"})
	for _, p := range []Position{{39.6, 21.6, height(340), "WGS84_ELLIPSOID"}, {39.1, 21.2, height(-30), "WGS84_ELLIPSOID"}} {
		v, err := f.ToLocal(p)
		if err != nil {
			t.Fatal(err)
		}
		back, err := f.FromLocal(v)
		if err != nil || math.Abs(back.Longitude-p.Longitude) > 1e-9 || math.Abs(back.Latitude-p.Latitude) > 1e-9 || math.Abs(*back.Height-*p.Height) > 1e-5 {
			t.Fatalf("round trip %+v => %+v %v", p, back, err)
		}
	}
	equator := DefaultFrame(cases[0].p)
	p := Position{0.001, 0, height(0), "WGS84_ELLIPSOID"}
	east, _ := equator.ToLocal(p)
	if math.Abs(east.X-111.3194907876) > 1e-6 || math.Abs(east.Y) > 1e-9 {
		t.Fatalf("east axis %+v", east)
	}
	north, _ := equator.ToLocal(Position{0, 0.001, height(0), "WGS84_ELLIPSOID"})
	if north.Y < 110 || math.Abs(north.X) > 1e-9 {
		t.Fatalf("north axis %+v", north)
	}
	up, _ := equator.ToLocal(Position{0, 0, height(100), "WGS84_ELLIPSOID"})
	if up != (Vector{0, 0, 100}) {
		t.Fatalf("up axis %+v", up)
	}
	// Poles and the antimeridian remain well defined using the recorded origin longitude.
	for _, origin := range []Position{{179.99, 0, height(1), "WGS84_ELLIPSOID"}, {45, 90, height(5), "WGS84_ELLIPSOID"}, {0, -90, height(5), "WGS84_ELLIPSOID"}} {
		f := DefaultFrame(origin)
		v := Vector{25, 50, 30}
		p, err := f.FromLocal(v)
		if err != nil {
			t.Fatal(err)
		}
		back, err := f.ToLocal(p)
		if err != nil || math.Abs(back.X-v.X) > 1e-5 || math.Abs(back.Y-v.Y) > 1e-5 || math.Abs(back.Z-v.Z) > 1e-5 {
			t.Fatalf("edge round trip %+v: %+v %v", origin, back, err)
		}
	}
}
func TestSpatialRejectsUnknownHeightDatumAxesUnitsAndFidelity(t *testing.T) {
	base := Position{39, 21, height(0), "WGS84_ELLIPSOID"}
	for _, datum := range []string{"EGM2008", "MSL", "AGL", "unknown", ""} {
		p := base
		p.VerticalDatum = datum
		if _, err := ECEF(p); err == nil {
			t.Fatalf("accepted %s", datum)
		}
	}
	p := base
	p.Height = nil
	if _, err := ECEF(p); err == nil {
		t.Fatal("missing height became zero")
	}
	p = base
	p.Latitude = 200
	if _, err := ECEF(p); err == nil {
		t.Fatal("invalid latitude")
	}
	f := DefaultFrame(base)
	for _, mutate := range []func(*Frame){func(f *Frame) { f.Order = "latitude_longitude" }, func(f *Frame) { f.Axes = "north_east_down" }, func(f *Frame) { f.Units = "feet" }, func(f *Frame) { f.Transform = "unknown" }, func(f *Frame) { f.Orientation = "radians" }} {
		bad := f
		mutate(&bad)
		if _, err := bad.ToLocal(base); err == nil {
			t.Fatal("ambiguous frame accepted")
		}
	}
	if _, err := f.FromLocal(Vector{math.NaN(), 0, 0}); err == nil {
		t.Fatal("nonfinite local position")
	}
	if (Fidelity{Profile: "validated_specialized_model", ModelVersion: "1", Simplifications: []string{"test"}}).Validate() == nil {
		t.Fatal("unvalidated specialist fidelity accepted")
	}
}
func TestHistoricalTimeMappingPreservesEvidenceAndRejectsMissingAnchors(t *testing.T) {
	source := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	scenario := source.AddDate(5, 0, 0)
	mapping := TimeMap{"offset", source, scenario}
	stamp := source.Add(15 * time.Second)
	got, err := mapping.Map(stamp)
	if err != nil || !got.Equal(scenario.Add(15*time.Second)) || !stamp.Equal(source.Add(15*time.Second)) {
		t.Fatal(got, err)
	}
	if _, err = (TimeMap{Mode: "offset"}).Map(source); err == nil {
		t.Fatal("missing anchors accepted")
	}
	if _, err = mapping.Map(time.Time{}); err == nil {
		t.Fatal("missing source time accepted")
	}
}
