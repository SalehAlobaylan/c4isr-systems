// Package environmentprep produces bounded geographic data, without exercise
// actors, network requests, fixture names or application-specific model paths.
package environmentprep

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

const Version = "emad.portable-geography.v1"

type Origin struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Height    float64 `json:"height"`
}
type Control struct {
	Longitude       float64 `json:"longitude"`
	Latitude        float64 `json:"latitude"`
	EllipsoidHeight float64 `json:"ellipsoidHeight"`
	East            float64 `json:"east"`
	North           float64 `json:"north"`
	Up              float64 `json:"up"`
	Provenance      string  `json:"provenance"`
}
type Qualification struct {
	HorizontalToleranceM float64   `json:"horizontalToleranceM"`
	VerticalToleranceM   float64   `json:"verticalToleranceM"`
	Method               string    `json:"method"`
	Controls             []Control `json:"controls"`
	MaxPixels            int       `json:"maxPixels"`
	MaxFeatures          int       `json:"maxFeatures"`
	MaxTriangles         int       `json:"maxTriangles"`
	MaxBytes             int       `json:"maxBytes"`
	MaxRAMBytes          int64     `json:"maxRamBytes"`
	MaxTempBytes         int64     `json:"maxTempBytes"`
	MaxSeconds           int       `json:"maxSeconds"`
}
type Input struct {
	DatasetIDs          []string      `json:"datasetIds"`
	GeoidID             string        `json:"geoidId,omitempty"`
	HorizontalCRS       string        `json:"horizontalCrs"`
	OutputVerticalDatum string        `json:"outputVerticalDatum"`
	Origin              Origin        `json:"origin"`
	Qualification       Qualification `json:"qualification"`
}
type Snapshot struct {
	EstimatedPixels   int                    `json:"estimatedPixels"`
	GeometryVertices  int                    `json:"geometryVertices"`
	ImageryOriginals  [][]byte               `json:"-"`
	Imagery           []imageryanalysis.Tile `json:"imagery,omitempty"`
	OutputMetadata    json.RawMessage        `json:"-"`
	NormalizedVectors json.RawMessage        `json:"normalizedVectors,omitempty"`
	ClippingVersion   string                 `json:"clippingVersion"`
	Input             Input                  `json:"input"`
	Area              areas.Revision         `json:"area"`
	Datasets          []reference.Dataset    `json:"datasets"`
	EstimatedRAM      int64                  `json:"estimatedRam"`
	EstimatedBytes    int64                  `json:"estimatedBytes"`
	Omissions         []string               `json:"omissions"`
}
type Point struct {
	Longitude float64    `json:"longitude"`
	Latitude  float64    `json:"latitude"`
	Height    float64    `json:"ellipsoidHeight"`
	ENU       [3]float64 `json:"enu"`
}
type Mesh struct {
	Points    []Point           `json:"points"`
	Triangles [][3]int          `json:"triangles"`
	Masks     map[string][]bool `json:"masks"`
}
type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type Manifest struct {
	ImageryPixels              int             `json:"imageryPixels"`
	ContentContext             json.RawMessage `json:"contentContext,omitempty"`
	NoDataPixels               int             `json:"noDataPixels"`
	Schema                     string          `json:"schema"`
	Input                      Snapshot        `json:"input"`
	Files                      []File          `json:"files"`
	HorizontalMethod           string          `json:"horizontalMethod"`
	VerticalMethod             string          `json:"verticalMethod"`
	MaxControlHorizontalErrorM float64         `json:"maxControlHorizontalErrorM"`
	MaxControlVerticalErrorM   float64         `json:"maxControlVerticalErrorM"`
	ValidPixels                int             `json:"validPixels"`
	Triangles                  int             `json:"triangles"`
	Features                   int             `json:"features"`
	Omissions                  []string        `json:"omissions"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validateOrigin(o Origin) bool {
	return finite(o.Longitude) && finite(o.Latitude) && finite(o.Height) && o.Longitude >= -180 && o.Longitude <= 180 && math.Abs(o.Latitude) <= 85 && math.Abs(o.Height) <= 12000
}
func ECEF(lon, lat, h float64) [3]float64 {
	r := math.Pi / 180
	l, p := lon*r, lat*r
	n := 6378137 / math.Sqrt(1-0.0066943799901413165*math.Sin(p)*math.Sin(p))
	return [3]float64{(n + h) * math.Cos(p) * math.Cos(l), (n + h) * math.Cos(p) * math.Sin(l), (n*(1-0.0066943799901413165) + h) * math.Sin(p)}
}
func ENU(o Origin, lon, lat, h float64) [3]float64 {
	a, b := ECEF(o.Longitude, o.Latitude, o.Height), ECEF(lon, lat, h)
	x, y, z := b[0]-a[0], b[1]-a[1], b[2]-a[2]
	l, p := o.Longitude*math.Pi/180, o.Latitude*math.Pi/180
	return [3]float64{-math.Sin(l)*x + math.Cos(l)*y, -math.Sin(p)*math.Cos(l)*x - math.Sin(p)*math.Sin(l)*y + math.Cos(p)*z, math.Cos(p)*math.Cos(l)*x + math.Cos(p)*math.Sin(l)*y + math.Sin(p)*z}
}

// Preflight runs before leasing full processing. Accuracy comes from admitted
// reference metadata; a job cannot assert a more accurate source by changing its
// requested tolerances. Independent controls are required, never round trips.
func Preflight(area areas.Revision, in Input, data []reference.Dataset) (Snapshot, error) {
	s := Snapshot{Input: in, Area: area, Datasets: data, Omissions: []string{"No actor models, runtime, navigation or collision guarantee.", "Road widths/building heights are source attributes, not inferred measurements.", "No imagery included by this grid/vector profile; imagery needs a separately georeferenced admitted input.", "Visual GLB omitted; neutral ENU mesh and georeferenced source data are authoritative.", "Interpolation between DEM samples is approximate; sparse independent controls do not establish full engineering accuracy."}}
	bad := func(msg string) (Snapshot, error) { return s, fmt.Errorf("environment preflight: %s", msg) }
	q := in.Qualification
	if in.HorizontalCRS != "EPSG:4326" || in.OutputVerticalDatum != "WGS84_ELLIPSOID" || !validateOrigin(in.Origin) || len(area.Components) != 1 || math.Abs(area.Components[0].Bounds[1]) > 85 || math.Abs(area.Components[0].Bounds[3]) > 85 || math.Abs(area.Components[0].Bounds[0]) > 180 || math.Abs(area.Components[0].Bounds[2]) > 180 || area.Components[0].Bounds[2]-area.Components[0].Bounds[0] > 2 || area.Components[0].Bounds[3]-area.Components[0].Bounds[1] > 2 {
		return bad("supported profile is one bounded WGS84 component (at most 2 degrees per side, within 85 degrees latitude), with explicit ENU origin and ellipsoid output")
	}
	if q.MaxPixels < 4 || q.MaxPixels > 131072 || q.MaxFeatures < 0 || q.MaxFeatures > 10000 || q.MaxTriangles < 2 || q.MaxTriangles > 131072 || q.MaxBytes < 1024 || q.MaxBytes > 8<<20 || q.MaxRAMBytes < 1<<20 || q.MaxRAMBytes > 128<<20 || q.MaxTempBytes < 0 || q.MaxTempBytes > 32<<20 || q.MaxSeconds < 1 || q.MaxSeconds > 120 || !finite(q.HorizontalToleranceM) || !finite(q.VerticalToleranceM) || q.HorizontalToleranceM <= 0 || q.HorizontalToleranceM > 10000 || q.VerticalToleranceM <= 0 || q.VerticalToleranceM > 1000 || len(q.Controls) < 3 || len(q.Controls) > 32 || len(q.Method) < 10 || len(q.Method) > 2000 {
		return bad("bounded qualification profile and at least three independent controls with a method are required")
	}
	if len(data) < 1 || len(data) > 16 || len(data) != len(in.DatasetIDs) {
		return bad("choose 1-16 exact retained datasets")
	}
	pixels, features, dems := 0, 0, 0
	var ha, va, geoidError float64
	var inputBytes int64
	estimatedTriangles := 0
	seen := map[string]bool{}
	var geoid *reference.Dataset
	for i := range data {
		d := &data[i]
		inputBytes += int64(len(d.Artifact))
		if seen[d.ID] || d.CRS != "EPSG:4326" || d.SHA256 != reference.Hash(d.Artifact) || d.Accuracy == nil || d.Accuracy.HorizontalM <= 0 || d.Accuracy.VerticalM < 0 || len(d.Accuracy.Provenance) < 10 {
			return bad("exact input hashes, CRS and source-backed accuracy/provenance are required")
		}
		seen[d.ID] = true
		if !d.Rights.RetainRaw {
			return bad("this portable profile retains original normalized bytes and requires retention rights")
		}
		ha = math.Max(ha, d.Accuracy.HorizontalM)
		if d.ID == in.GeoidID {
			if d.Kind != "elevation" || d.VerticalDatum != "GEOID_UNDULATION_EGM2008" {
				return bad("explicit EGM2008 undulation grid required")
			}
			var grid reference.Grid
			if json.Unmarshal(d.Artifact, &grid) != nil || grid.Width < 2 || grid.Height < 2 || grid.StepX <= 0 || grid.StepY <= 0 || len(grid.Values) != grid.Width*grid.Height {
				return bad("invalid transformation grid")
			}
			b := area.Components[0].Bounds
			if grid.West > b[0]+1e-8 || grid.South > b[1]+1e-8 || grid.West+float64(grid.Width-1)*grid.StepX < b[2]-1e-8 || grid.South+float64(grid.Height-1)*grid.StepY < b[3]-1e-8 {
				return bad("transformation grid samples outside required area of use")
			}
			pixels += grid.Width * grid.Height
			geoid = d
			geoidError = d.Accuracy.VerticalM
			continue
		}
		if d.Kind == "elevation" {
			var g reference.Grid
			if json.Unmarshal(d.Artifact, &g) != nil || g.Width < 2 || g.Height < 2 || len(g.Values) != g.Width*g.Height || g.StepX <= 0 || g.StepY <= 0 {
				return bad("invalid DEM grid")
			}
			pixels += g.Width * g.Height
			estimatedTriangles += 2 * (g.Width - 1) * (g.Height - 1)
			dems++
			va = math.Max(va, d.Accuracy.VerticalM)
			if d.VerticalDatum != "WGS84_ELLIPSOID" && d.VerticalDatum != "EGM2008" {
				return bad("unsupported vertical datum; admit a qualified transformation grid first")
			}
			if d.VerticalDatum == "EGM2008" && in.GeoidID == "" {
				return bad("EGM2008 to ellipsoid requires an admitted undulation grid")
			}
			b := area.Components[0].Bounds
			if g.West > b[0]+1e-8 || g.South > b[1]+1e-8 || g.West+float64(g.Width-1)*g.StepX < b[2]-1e-8 || g.South+float64(g.Height-1)*g.StepY < b[3]-1e-8 {
				return bad("DEM sample grid does not cover the selected area")
			}
			if d.AOI.West > b[0] || d.AOI.East < b[2] || d.AOI.South > b[1] || d.AOI.North < b[3] {
				return bad("required DEM coverage is incomplete; admit the complete bounded mosaic before submitting")
			}
		} else if d.Kind == "vector" {
			var fc reference.Collection
			if json.Unmarshal(d.Artifact, &fc) != nil {
				return bad("invalid vector input")
			}
			features += len(fc.Features)
		} else {
			return bad("unsupported reference kind")
		}
	}
	if in.GeoidID != "" {
		if geoid == nil {
			return bad("missing transformation grid")
		}
		b := area.Components[0].Bounds
		if geoid.AOI.West > b[0] || geoid.AOI.East < b[2] || geoid.AOI.South > b[1] || geoid.AOI.North < b[3] {
			return bad("transformation grid area of use does not cover the selected area")
		}
	}
	if dems != 1 || pixels > q.MaxPixels || features > q.MaxFeatures || estimatedTriangles > q.MaxTriangles || q.HorizontalToleranceM < ha || q.VerticalToleranceM < va+geoidError {
		return bad("missing DEM, impossible budget, or tolerance tighter than admitted source accuracy")
	}
	for _, c := range q.Controls {
		if len(c.Provenance) < 10 || len(c.Provenance) > 2000 || !finite(c.EllipsoidHeight) || !finite(c.East) || !finite(c.North) || !finite(c.Up) || !Inside(area.Geometry, c.Longitude, c.Latitude) {
			return bad("independent controls must be finite, provenance-qualified and inside the selected area")
		}
	}
	vertices := 0
	for _, poly := range polygons(area.Geometry) {
		for _, ring := range poly {
			vertices += len(ring)
		}
	}
	if int64(vertices)*int64(pixels) > 8000000 {
		return bad("polygon/grid complexity exceeds this profile; choose a smaller study or coarser admitted grid")
	}
	s.EstimatedPixels = pixels
	s.GeometryVertices = vertices
	s.EstimatedRAM = int64(pixels)*1024 + int64(features)*4096 + 3*inputBytes + int64(len(area.Geometry))*8 + 16<<20
	s.EstimatedBytes = int64(pixels)*256 + int64(features)*1024 + 3*inputBytes
	if s.EstimatedRAM > q.MaxRAMBytes || s.EstimatedBytes > int64(q.MaxBytes) {
		return bad("estimated RAM/output exceeds the qualification profile")
	}
	return s, nil
}

func polygons(raw []byte) [][][][]float64 {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	_ = json.Unmarshal(raw, &g)
	var p [][][][]float64
	if g.Type == "Polygon" {
		var x [][][]float64
		_ = json.Unmarshal(g.Coordinates, &x)
		p = append(p, x)
	} else if g.Type == "MultiPolygon" {
		_ = json.Unmarshal(g.Coordinates, &p)
	}
	return p
}
func ringInside(r [][]float64, x, y float64) bool {
	inside := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		a, b := r[i], r[j]
		if len(a) < 2 || len(b) < 2 {
			return false
		}
		cross := (x-a[0])*(b[1]-a[1]) - (y-a[1])*(b[0]-a[0])
		if math.Abs(cross) < 1e-12 && x >= math.Min(a[0], b[0]) && x <= math.Max(a[0], b[0]) && y >= math.Min(a[1], b[1]) && y <= math.Max(a[1], b[1]) {
			return true
		}
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}
func insidePolygons(polys [][][][]float64, x, y float64) bool {
	for _, p := range polys {
		if len(p) == 0 || !ringInside(p[0], x, y) {
			continue
		}
		hole := false
		for _, r := range p[1:] {
			hole = hole || ringInside(r, x, y)
		}
		if !hole {
			return true
		}
	}
	return false
}
func Inside(raw []byte, x, y float64) bool { return insidePolygons(polygons(raw), x, y) }

func sample(g reference.Grid, x, y float64) (float64, bool) {
	fx, fy := (x-g.West)/g.StepX, (y-g.South)/g.StepY
	if fx < 0 || fy < 0 || fx > float64(g.Width-1)+1e-8 || fy > float64(g.Height-1)+1e-8 {
		return 0, false
	}
	i, j := int(math.Min(math.Floor(fx), float64(g.Width-2))), int(math.Min(math.Floor(fy), float64(g.Height-2)))
	dx, dy := fx-float64(i), fy-float64(j)
	idx := []int{j*g.Width + i, j*g.Width + i + 1, (j+1)*g.Width + i, (j+1)*g.Width + i + 1}
	v := make([]float64, 4)
	for n, k := range idx {
		if k < 0 || k >= len(g.Values) || g.Values[k] == nil || !finite(*g.Values[k]) {
			return 0, false
		}
		v[n] = *g.Values[k]
	}
	return v[0]*(1-dx)*(1-dy) + v[1]*dx*(1-dy) + v[2]*(1-dx)*dy + v[3]*dx*dy, true
}
func Build(ctx context.Context, s Snapshot) ([]byte, Manifest, error) {
	q := s.Input.Qualification
	parsed := polygons(s.Area.Geometry)
	inside := func(x, y float64) bool { return insidePolygons(parsed, x, y) }
	ctx, cancel := context.WithTimeout(ctx, time.Duration(q.MaxSeconds)*time.Second)
	defer cancel()
	m := Manifest{ContentContext: s.OutputMetadata, Schema: Version, Input: s, Files: []File{}, HorizontalMethod: "WGS84 geodetic -> ECEF -> ENU; metres; x east, y north, z up", VerticalMethod: "identity for ellipsoid DEM; EGM2008 orthometric H + bilinear admitted undulation N = ellipsoid h", Omissions: append([]string{}, s.Omissions...)}
	mesh := Mesh{Points: []Point{}, Triangles: [][3]int{}, Masks: map[string][]bool{}}
	var geoid reference.Grid
	for _, d := range s.Datasets {
		if d.ID == s.Input.GeoidID {
			_ = json.Unmarshal(d.Artifact, &geoid)
		}
	}
	height := func(d reference.Dataset, g reference.Grid, x, y float64) (float64, bool) {
		h, ok := sample(g, x, y)
		if !ok {
			return 0, false
		}
		if d.VerticalDatum == "EGM2008" {
			n, ok := sample(geoid, x, y)
			return h + n, ok
		}
		return h, true
	}
	grids := map[string]reference.Grid{}
	for _, d := range s.Datasets {
		if d.Kind != "elevation" || d.ID == s.Input.GeoidID {
			continue
		}
		if ctx.Err() != nil {
			return nil, m, ctx.Err()
		}
		var g reference.Grid
		_ = json.Unmarshal(d.Artifact, &g)
		grids[d.ID] = g
		indices := make([]int, len(g.Values))
		mask := make([]bool, len(g.Values))
		for i := range indices {
			indices[i] = -1
		}
		for j := 0; j < g.Height; j++ {
			if e := ctx.Err(); e != nil {
				return nil, m, e
			}
			for i := 0; i < g.Width; i++ {
				k := j*g.Width + i
				x, y := g.West+float64(i)*g.StepX, g.South+float64(j)*g.StepY
				if !inside(x, y) {
					continue
				}
				h, ok := height(d, g, x, y)
				if !ok {
					m.NoDataPixels++
					continue
				}
				mask[k] = true
				indices[k] = len(mesh.Points)
				mesh.Points = append(mesh.Points, Point{x, y, h, ENU(s.Input.Origin, x, y, h)})
			}
		}
		mesh.Masks[d.ID] = mask
		for j := 0; j < g.Height-1; j++ {
			if e := ctx.Err(); e != nil {
				return nil, m, e
			}
			for i := 0; i < g.Width-1; i++ {
				a, b, c, e := j*g.Width+i, j*g.Width+i+1, (j+1)*g.Width+i, (j+1)*g.Width+i+1
				if !mask[a] || !mask[b] || !mask[c] || !mask[e] {
					continue
				}
				west, south := g.West+float64(i)*g.StepX, g.South+float64(j)*g.StepY
				// Conservative cell coverage: a boundary vertex inside the cell causes
				// omission, preventing a small hole from being bridged by a large cell.
				full := inside(west+g.StepX/2, south+g.StepY/2)
				for _, poly := range parsed {
					for _, ring := range poly {
						for k := 1; k < len(ring); k++ {
							if segmentInCell(ring[k-1], ring[k], west, south, west+g.StepX, south+g.StepY) {
								full = false
							}
						}
					}
				}
				for _, p := range parsed {
					for _, r := range p {
						for _, v := range r {
							if v[0] > west && v[0] < west+g.StepX && v[1] > south && v[1] < south+g.StepY {
								full = false
							}
						}
					}
				}
				if full {
					mesh.Triangles = append(mesh.Triangles, [3]int{indices[a], indices[b], indices[c]}, [3]int{indices[b], indices[e], indices[c]})
				}
			}
		}
	}
	if len(mesh.Points) < 4 || len(mesh.Triangles) == 0 || len(mesh.Triangles) > q.MaxTriangles {
		return nil, m, fmt.Errorf("no qualified terrain surface within the selected polygon/budget")
	}
	for _, c := range q.Controls {
		e := ENU(s.Input.Origin, c.Longitude, c.Latitude, c.EllipsoidHeight)
		he := math.Hypot(e[0]-c.East, e[1]-c.North)
		ve := math.Abs(e[2] - c.Up)
		matched := false
		for _, d := range s.Datasets {
			g, ok := grids[d.ID]
			if !ok {
				continue
			}
			h, ok := height(d, g, c.Longitude, c.Latitude)
			if ok {
				ve = math.Max(ve, math.Abs(h-c.EllipsoidHeight))
				matched = true
				break
			}
		}
		m.MaxControlHorizontalErrorM = math.Max(m.MaxControlHorizontalErrorM, he)
		m.MaxControlVerticalErrorM = math.Max(m.MaxControlVerticalErrorM, ve)
		if !matched || he > q.HorizontalToleranceM || ve > q.VerticalToleranceM {
			return nil, m, fmt.Errorf("independent control validation failed; no output published")
		}
	}
	files := map[string][]byte{}
	imagePixels := 0
	for i, original := range s.Imagery {
		if e := original.Validate(); e != nil {
			return nil, m, e
		}
		imagePixels += original.Grid.Width * original.Grid.Height
		if imagePixels+len(mesh.Points) > q.MaxPixels {
			return nil, m, fmt.Errorf("imagery and terrain exceed qualified pixel budget")
		}
		if ctx.Err() != nil {
			return nil, m, ctx.Err()
		}
		raw, _ := json.Marshal(original)
		if i < len(s.ImageryOriginals) {
			raw = s.ImageryOriginals[i]
		}
		files[fmt.Sprintf("sources/imagery-%02d.json", i)] = raw
		image := original
		image.Red = append([]*float64{}, original.Red...)
		image.NIR = append([]*float64{}, original.NIR...)
		image.SCL = append([]int{}, original.SCL...)
		mask := make([]bool, len(image.Red))
		g := image.Grid
		for y := 0; y < g.Height; y++ {
			if e := ctx.Err(); e != nil {
				return nil, m, e
			}
			for x := 0; x < g.Width; x++ {
				k := y*g.Width + x
				west, south := g.West+float64(x)*g.StepLng, g.North-float64(y+1)*g.StepLat
				selected := cellCovered(parsed, west, south, west+g.StepLng, south+g.StepLat)
				if selected && image.Red[k] != nil && image.NIR[k] != nil && image.SCL[k] != 0 {
					mask[k] = true
					m.ImageryPixels++
				} else {
					image.Red[k] = nil
					image.NIR[k] = nil
					image.SCL[k] = 0
				}
			}
		}
		files[fmt.Sprintf("imagery/%02d.json", i)], _ = json.Marshal(image)
		files[fmt.Sprintf("imagery/%02d-mask.json", i)], _ = json.Marshal(mask)
	}

	files["readme.txt"] = []byte("Portable geographic environment. Read manifest.json first. terrain-mesh.json uses x=east, y=north, z=up in metres relative to the declared WGS84 ellipsoid origin. Masks follow source south-to-north, west-to-east node order; false nodes and omitted triangles claim no surface. vectors.geojson is the clipped WGS84 output. sources/ holds original normalized references for reproducibility, including values outside the selected polygon; these are not the prepared surface. No actor, runtime, navigation or engineering guarantee. Original GeoTIFF/OSM parents identified by source receipts may remain external dependencies. Run check_portable_environment.py with PROJ/GEOS for an independent coordinate/geometry check.")
	if len(s.NormalizedVectors) > 0 {
		files["vectors.geojson"] = s.NormalizedVectors
	}
	if m.NoDataPixels > 0 {
		m.Omissions = append(m.Omissions, fmt.Sprintf("%d selected grid nodes have nodata and no surface is claimed there.", m.NoDataPixels))
	}
	files["terrain-mesh.json"], _ = json.Marshal(mesh)
	m.ValidPixels = len(mesh.Points)
	m.Triangles = len(mesh.Triangles)
	for i, d := range s.Datasets {
		if ctx.Err() != nil {
			return nil, m, ctx.Err()
		}
		files[fmt.Sprintf("sources/%02d.json", i)] = d.Artifact
		if d.Kind == "vector" {
			var fc reference.Collection
			_ = json.Unmarshal(d.Artifact, &fc)
			m.Features += len(fc.Features)
		}
	}
	names := []string{}
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		m.Files = append(m.Files, File{n, reference.Hash(files[n]), len(files[n])})
	}
	files["manifest.json"], _ = json.Marshal(m)
	raw, e := Archive(files, q.MaxBytes)
	return raw, m, e
}
func Archive(files map[string][]byte, limit int) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	names := []string{}
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if !safeName(n) {
			return nil, fmt.Errorf("unsafe package path")
		}
		h := &zip.FileHeader{Name: n, Method: zip.Store}
		h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		w, e := z.CreateHeader(h)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(files[n]); e != nil {
			return nil, e
		}
		if b.Len() > limit {
			return nil, fmt.Errorf("package exceeds byte budget")
		}
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	if b.Len() > limit {
		return nil, fmt.Errorf("package exceeds byte budget")
	}
	return b.Bytes(), nil
}
func safeName(n string) bool {
	if n == "" || len(n) > 128 || n[0] == '/' {
		return false
	}
	for _, c := range n {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '/' || c == '-' || c == '.') {
			return false
		}
	}
	for i := 0; i+1 < len(n); i++ {
		if n[i:i+2] == ".." {
			return false
		}
	}
	return true
}

func segmentInCell(a, b []float64, w, s, e, n float64) bool {
	eps := 1e-11
	w += eps
	s += eps
	e -= eps
	n -= eps
	if w >= e || s >= n {
		return false
	}
	dx, dy := b[0]-a[0], b[1]-a[1]
	lo, hi := 0., 1.
	p := []float64{-dx, dx, -dy, dy}
	q := []float64{a[0] - w, e - a[0], a[1] - s, n - a[1]}
	for i := range p {
		if p[i] == 0 {
			if q[i] < 0 {
				return false
			}
			continue
		}
		t := q[i] / p[i]
		if p[i] < 0 {
			lo = math.Max(lo, t)
		} else {
			hi = math.Min(hi, t)
		}
		if lo > hi {
			return false
		}
	}
	return hi >= lo
}

func cellCovered(p [][][][]float64, w, s, e, n float64) bool {
	for _, c := range [][2]float64{{w, s}, {e, s}, {e, n}, {w, n}, {(w + e) / 2, (s + n) / 2}} {
		if !insidePolygons(p, c[0], c[1]) {
			return false
		}
	}
	for _, poly := range p {
		for _, ring := range poly {
			for k := 1; k < len(ring); k++ {
				if segmentInCell(ring[k-1], ring[k], w, s, e, n) {
					return false
				}
			}
		}
	}
	return true
}
