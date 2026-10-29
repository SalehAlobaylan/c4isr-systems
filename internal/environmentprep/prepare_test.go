package environmentprep

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

func fixture() (areas.Revision, Input, []reference.Dataset) {
	raw := json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[46,24],[47,24],[47,25],[46,25],[46,24]]]]}`)
	area := areas.Revision{Geometry: raw, Components: []areas.Component{{Geometry: raw, Bounds: [4]float64{46, 24, 47, 25}}}}
	values := []*float64{}
	for i := 0; i < 81; i++ {
		v := 100.
		values = append(values, &v)
	}
	grid := reference.Grid{Width: 9, Height: 9, West: 46, South: 24, StepX: .125, StepY: .125, Registration: "point_samples", Values: values}
	artifact, _ := json.Marshal(grid)
	d := reference.Dataset{Input: reference.Input{Kind: "elevation", CRS: "EPSG:4326", VerticalDatum: "WGS84_ELLIPSOID", AOI: publicingest.Bounds{West: 46, South: 24, East: 47, North: 25}, Artifact: artifact, SHA256: reference.Hash(artifact), Rights: publicingest.Rights{RetainRaw: true}, Accuracy: &reference.Accuracy{HorizontalM: 10, VerticalM: 2, Provenance: "Synthetic admitted accuracy for protocol testing"}}, ID: "terrain"}
	// These ENU values were computed independently with PROJ topocentric, not
	// with this package's ECEF/ENU implementation. They are not survey controls.
	controls := []Control{{46.5, 24.5, 100, 0, 0, 100, "Independent PROJ coordinate fixture"}, {46.5001, 24.5, 100, 10.135637192357791, .0000036677303683, 99.99999195185877, "Independent PROJ coordinate fixture"}, {46.5, 24.5001, 100, 0, 11.076724262114205, 99.99999033398097, "Independent PROJ coordinate fixture"}}
	in := Input{DatasetIDs: []string{d.ID}, HorizontalCRS: "EPSG:4326", OutputVerticalDatum: "WGS84_ELLIPSOID", Origin: Origin{46.5, 24.5, 0}, Qualification: Qualification{HorizontalToleranceM: 10, VerticalToleranceM: 2, Method: "Independent PROJ controls; synthetic protocol evidence", Controls: controls, MaxPixels: 1024, MaxFeatures: 10, MaxTriangles: 2048, MaxBytes: 2 << 20, MaxRAMBytes: 32 << 20, MaxTempBytes: 0, MaxSeconds: 5}}
	return area, in, []reference.Dataset{d}
}
func TestPortableTerrainIndependentControlsAndDeterminism(t *testing.T) {
	area, in, data := fixture()
	s, e := Preflight(area, in, data)
	if e != nil {
		t.Fatal(e)
	}
	a, m, e := Build(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := Build(context.Background(), s)
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("same frozen input changed output", e)
	}
	if m.MaxControlHorizontalErrorM > 1e-6 || m.MaxControlVerticalErrorM > 1e-6 {
		t.Fatal("independent frame control mismatch", m)
	}
	z, e := zip.NewReader(bytes.NewReader(a), int64(len(a)))
	if e != nil {
		t.Fatal(e)
	}
	if len(z.File) != 4 {
		t.Fatal("unexpected package content", len(z.File))
	}
	for _, f := range z.File {
		if !safeName(f.Name) {
			t.Fatal("unsafe generated path")
		}
	}
}
func TestPortablePreflightAndVerticalControlsFailClosed(t *testing.T) {
	area, in, data := fixture()
	in.Qualification.HorizontalToleranceM = 1
	if _, e := Preflight(area, in, data); e == nil {
		t.Fatal("claimed accuracy exceeded admitted accuracy")
	}
	in.Qualification.HorizontalToleranceM = 10
	data[0].VerticalDatum = "EGM2008"
	if _, e := Preflight(area, in, data); e == nil {
		t.Fatal("missing vertical grid accepted")
	}
	data[0].VerticalDatum = "WGS84_ELLIPSOID"
	in.Qualification.Controls[1].East += 100
	s, e := Preflight(area, in, data)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = Build(context.Background(), s); e == nil {
		t.Fatal("wrong independent control accepted")
	}
	in.Qualification.Controls[1].East -= 100
	in.Qualification.MaxBytes = 1024
	if _, e = Preflight(area, in, data); e == nil {
		t.Fatal("impossible resource estimate accepted")
	}
}
func TestPortableHoleMaskAndBoundarySegments(t *testing.T) {
	area, in, data := fixture()
	area.Geometry = json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[46,24],[47,24],[47,25],[46,25],[46,24]],[[46.01,24.14],[46.99,24.14],[46.99,24.16],[46.01,24.16],[46.01,24.14]]]]}`)
	s, e := Preflight(area, in, data)
	if e != nil {
		t.Fatal(e)
	}
	raw, _, e := Build(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	z, _ := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	for _, f := range z.File {
		if f.Name != "terrain-mesh.json" {
			continue
		}
		r, _ := f.Open()
		var mesh Mesh
		_ = json.NewDecoder(r).Decode(&mesh)
		_ = r.Close()
		for _, tri := range mesh.Triangles {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, i := range tri {
				lo = math.Min(lo, mesh.Points[i].Latitude)
				hi = math.Max(hi, mesh.Points[i].Latitude)
			}
			if lo < 24.14 && hi > 24.16 {
				t.Fatal("surface bridged an omitted strip")
			}
		}
	}
}
func TestUnsafeArchivePathsRejected(t *testing.T) {
	for _, name := range []string{"../escape", "/root", "x\\file", "file:stream", "a/../../b"} {
		if _, e := Archive(map[string][]byte{name: []byte("bad")}, 1024); e == nil {
			t.Fatal("unsafe path accepted", name)
		}
	}
}

func TestQualifiedGeoidConversionAndImageryMasks(t *testing.T) {
	area, in, data := fixture()
	data[0].VerticalDatum = "EGM2008"
	geoid := data[0]
	geoid.ID = "geoid"
	geoid.VerticalDatum = "GEOID_UNDULATION_EGM2008"
	geoid.Accuracy = &reference.Accuracy{HorizontalM: 10, VerticalM: 1, Provenance: "Synthetic undulation-grid uncertainty fixture"}
	var grid reference.Grid
	_ = json.Unmarshal(geoid.Artifact, &grid)
	for i := range grid.Values {
		v := 30.
		grid.Values[i] = &v
	}
	geoid.Artifact, _ = json.Marshal(grid)
	geoid.SHA256 = reference.Hash(geoid.Artifact)
	data = append(data, geoid)
	in.DatasetIDs = append(in.DatasetIDs, geoid.ID)
	in.GeoidID = geoid.ID
	in.Qualification.VerticalToleranceM = 3
	for i := range in.Qualification.Controls {
		in.Qualification.Controls[i].EllipsoidHeight += 30
		in.Qualification.Controls[i].Up += 30
	}
	snap, e := Preflight(area, in, data)
	if e != nil {
		t.Fatal(e)
	}
	v := .2
	tile := imageryanalysis.Tile{Schema: "emad.multispectral-subset.v1", SceneID: "synthetic", SceneHash: strings.Repeat("a", 64), AcquiredAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), PreparationMethod: "Owned synthetic qualified imagery fixture", ReflectanceUnit: "surface_reflectance", Grid: imageryanalysis.Grid{CRS: "EPSG:4326", Width: 2, Height: 2, West: 46.499, North: 24.501, StepLng: .0005, StepLat: .0005, Registration: "pixel_edges_north_to_south"}, Red: []*float64{&v, &v, &v, &v}, NIR: []*float64{&v, &v, &v, &v}, SCL: []int{4, 4, 4, 4}, SourceHashes: map[string]string{"B04_10m": strings.Repeat("a", 64), "B08_10m": strings.Repeat("b", 64), "SCL_20m": strings.Repeat("c", 64)}}
	original, _ := json.MarshalIndent(tile, "", " ")
	snap.Imagery = []imageryanalysis.Tile{tile}
	snap.ImageryOriginals = [][]byte{original}
	raw, manifest, e := Build(context.Background(), snap)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.ImageryPixels != 4 || manifest.MaxControlVerticalErrorM > 0.001 {
		t.Fatal("incorrect H+N or image grid", manifest.ImageryPixels, manifest.MaxControlVerticalErrorM)
	}
	z, _ := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	found := false
	for _, file := range z.File {
		if file.Name == "sources/imagery-00.json" {
			r, _ := file.Open()
			var b bytes.Buffer
			_, _ = b.ReadFrom(r)
			_ = r.Close()
			if !bytes.Equal(original, b.Bytes()) {
				t.Fatal("source image bytes changed")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("source imagery omitted")
	}
	geoid.AOI.East = 46.6
	data[1] = geoid
	if _, e := Preflight(area, in, data); e == nil {
		t.Fatal("out-of-area vertical grid accepted")
	}
}
