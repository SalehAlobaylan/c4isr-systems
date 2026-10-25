package simruntime

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"math"
	"testing"
)

func TestWaypointClockSpawnDespawnAndDeterminism(t *testing.T) {
	h := 10.0
	frame := simulation.DefaultFrame(simulation.Position{Longitude: 39.2, Latitude: 21.5, Height: &h, VerticalDatum: "WGS84_ELLIPSOID"})
	point := func(v simulation.Vector) simulation.Position {
		p, e := frame.FromLocal(v)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	scene := simscene.Frozen{Environment: modelassets.EnvironmentAsset{Frame: frame}, Input: simscene.Input{Actors: []simscene.Placement{{ID: "a", Position: point(simulation.Vector{}), Heading: 270, SpeedMPS: 10, Waypoints: []simulation.Position{point(simulation.Vector{X: 100}), point(simulation.Vector{X: 100, Y: 100})}, SpawnMs: 1000, DespawnMs: 30000}, {ID: "b", Position: point(simulation.Vector{Z: 5}), Heading: 20, SpawnMs: 0, DespawnMs: 5000}}}}
	w, e := NewWorld(scene)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		elapsed int64
		x, y    float64
		active  bool
	}{{0, 0, 0, false}, {1000, 0, 0, true}, {6000, 50, 0, true}, {16000, 100, 50, true}, {25000, 100, 100, true}, {30000, 100, 100, false}}
	for _, c := range cases {
		a, e := w.At(c.elapsed)
		if e != nil || math.Abs(a[0].Local.X-c.x) > 1e-5 || math.Abs(a[0].Local.Y-c.y) > 1e-5 || a[0].Active != c.active {
			t.Fatalf("at %d: %+v %v", c.elapsed, a, e)
		}
	}
	first, _ := w.At(16000)
	_, _ = w.At(30000)
	second, _ := w.At(16000)
	if first[0] != second[0] {
		t.Fatal("evaluation depended on tick order")
	}
	if _, e = w.At(-1); e == nil {
		t.Fatal("negative time accepted")
	}
	if _, e = w.At(3600001); e == nil {
		t.Fatal("unbounded time accepted")
	}
}
func TestRuntimeTransportRequiresCredentialSafeOrigin(t *testing.T) {
	for _, u := range []string{"http://remote.example", "https://u:p@remote.example", "https://remote.example?token=x", "https://remote.example/path", "file:///tmp/x"} {
		if _, e := NewClient(u, "private"); e == nil {
			t.Fatalf("unsafe origin %s accepted", u)
		}
	}
	for _, u := range []string{"http://127.0.0.1:8080", "http://[::1]:8080", "https://runtime.example"} {
		if _, e := NewClient(u, "private"); e != nil {
			t.Fatal(e)
		}
	}
}
