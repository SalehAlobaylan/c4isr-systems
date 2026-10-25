package imageryanalysis

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"math"
	"testing"
	"time"
)

func tileFixture() Tile {
	r, n := .2, .6
	t := Tile{Schema: "emad.multispectral-subset.v1", SceneID: "scene-before", SceneHash: "hash", Grid: Grid{"EPSG:4326", 2, 2, 39.16, 21.49, .0002, .0002, "pixel_edges_north_to_south"}, AcquiredAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), Platform: "sentinel-2b", Tile: "MGRS-37QED", Orbit: 135, ProcessingVersion: "05.11", ReflectanceUnit: "surface_reflectance", PreparationMethod: "fixture.v1", SourceHashes: map[string]string{}, Red: []*float64{&r, &r, &r, &r}, NIR: []*float64{&n, &n, &n, &n}, SCL: []int{4, 4, 9, 4}}
	for _, key := range []string{"B04_10m", "B08_10m", "SCL_20m"} {
		t.SourceHashes[key] = artifacts.Hash([]byte(key))
	}
	return t
}
func TestNDVIExactDifferenceMaskAndDeterminism(t *testing.T) {
	before := tileFixture()
	after := tileFixture()
	after.SceneID = "scene-after"
	after.AcquiredAt = before.AcquiredAt.Add(10 * 24 * time.Hour)
	low := .2
	after.NIR[0] = &low
	after.Red[1] = nil
	p := Parameters{.2, .5}
	a, e := Compare(context.Background(), before, after, p)
	if e != nil {
		t.Fatal(e)
	}
	if a.ValidCells != 2 || a.MaskedCells != 2 || a.DeclineCells != 1 || math.Abs(*a.Delta[0]+.5) > 1e-12 || a.Delta[1] != nil || a.Delta[2] != nil {
		t.Fatalf("unexpected result %+v", a)
	}
	b, e := Compare(context.Background(), before, after, p)
	if e != nil {
		t.Fatal(e)
	}
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(aa) != string(bb) {
		t.Fatal("result is not byte reproducible")
	}
}
func TestNDVIRejectsUnsuitableInputs(t *testing.T) {
	for _, name := range []string{"grid", "platform", "time", "nodata", "invalid", "offset", "source"} {
		t.Run(name, func(t *testing.T) {
			b, a := tileFixture(), tileFixture()
			a.SceneID = "different"
			a.AcquiredAt = b.AcquiredAt.Add(time.Hour)
			switch name {
			case "grid":
				a.Grid.West += .0001
			case "platform":
				a.Platform = "sentinel-2a"
			case "time":
				a.AcquiredAt = b.AcquiredAt
			case "nodata":
				a.SCL = []int{9, 9, 9, 9}
			case "invalid":
				a.Red = a.Red[:1]
			case "offset":
				v := 10000.0
				a.Red[0] = &v
			case "source":
				delete(a.SourceHashes, "B04_10m")
			}
			if _, e := Compare(context.Background(), b, a, Parameters{.2, .5}); e == nil {
				t.Fatal("unsuitable input accepted")
			}
		})
	}
}
func TestNDVICancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, a := tileFixture(), tileFixture()
	a.SceneID = "after"
	a.AcquiredAt = b.AcquiredAt.Add(time.Hour)
	if _, e := Compare(ctx, b, a, Parameters{.2, .5}); e == nil {
		t.Fatal("cancellation ignored")
	}
}
