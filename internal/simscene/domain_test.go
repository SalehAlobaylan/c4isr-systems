package simscene

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"math"
	"testing"
)

func TestTerrainHolesDatumAndReferenceHeightAssumptions(t *testing.T) {
	zero, ten := 0.0, 10.0
	frame := simulation.DefaultFrame(simulation.Position{Longitude: 0, Latitude: 0, Height: &zero, VerticalDatum: "WGS84_ELLIPSOID"})
	grid := reference.Grid{Width: 2, Height: 2, West: 0, South: 0, StepX: .001, StepY: .001, Values: []*float64{&zero, &zero, nil, &ten}, Registration: "point_samples"}
	raw, _ := json.Marshal(grid)
	b := contextpackage.Bundle{Manifest: contextpackage.Manifest{Assets: []contextpackage.Asset{{Kind: "elevation", SHA256: "grid", VerticalDatum: "WGS84_ELLIPSOID"}, {Kind: "vector", SHA256: "point", DatasetID: "point"}}}, Assets: map[string][]byte{"grid": raw, "point": []byte(`{"type":"FeatureCollection","features":[{"id":"north","geometry":{"type":"Point","coordinates":[0,0.001]}}]}`)}}
	g, e := GeometryFor(b, frame)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Triangles) != 0 {
		t.Fatal("missing terrain sample was filled", g.Triangles)
	}
	if g.Terrain[1].X < 111 || math.Abs(g.Terrain[1].Y) > 1e-6 || g.Terrain[3].Z < 9 {
		t.Fatal("terrain axis or height conversion failed", g.Terrain)
	}
	if len(g.Points) != 1 || !g.Points[0].HeightAssumed || g.Points[0].Local.Y < 110 {
		t.Fatal("reference height assumption or north axis lost", g.Points)
	}
	grid.Values[2] = &zero
	raw, _ = json.Marshal(grid)
	b.Assets["grid"] = raw
	g, e = GeometryFor(b, frame)
	if e != nil || len(g.Triangles) != 6 {
		t.Fatal("complete terrain failed", g, e)
	}
	b.Manifest.Assets[0].VerticalDatum = "EGM2008"
	if _, e = GeometryFor(b, frame); e == nil {
		t.Fatal("unresolved datum accepted")
	}
	if _, e = Within(frame, simulation.Position{Longitude: 1, Latitude: 0, Height: &zero, VerticalDatum: "WGS84_ELLIPSOID"}); e == nil {
		t.Fatal("10 km scene radius not enforced")
	}
}
