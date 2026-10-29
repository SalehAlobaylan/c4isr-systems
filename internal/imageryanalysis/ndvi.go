// Package imageryanalysis is a deterministic bounded subset processor. It is
// called by the independent worker executable, never by an HTTP handler.
package imageryanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"math"
	"time"
)

const Version = "emad.ndvi-change.v1"
const MaxCells = 65536

// Pixels are north-to-south rows, west-to-east columns. Coordinates describe
// pixel edges in EPSG:4326; angular spacing is NOT asserted to be metre GSD.
type Grid struct {
	CRS          string  `json:"crs"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	West         float64 `json:"west"`
	North        float64 `json:"north"`
	StepLng      float64 `json:"stepLng"`
	StepLat      float64 `json:"stepLat"`
	Registration string  `json:"registration"`
}

func (g Grid) Bounds() publicingest.Bounds {
	return publicingest.Bounds{West: g.West, South: g.North - float64(g.Height)*g.StepLat, East: g.West + float64(g.Width)*g.StepLng, North: g.North}
}

type Tile struct {
	Schema            string            `json:"schema"`
	SceneID           string            `json:"sceneId"`
	SceneHash         string            `json:"sceneHash"`
	Grid              Grid              `json:"grid"`
	AcquiredAt        time.Time         `json:"acquiredAt"`
	Platform          string            `json:"platform"`
	Tile              string            `json:"tile"`
	Orbit             int               `json:"orbit"`
	ProcessingVersion string            `json:"processingVersion"`
	ReflectanceUnit   string            `json:"reflectanceUnit"`
	Red               []*float64        `json:"red"`
	NIR               []*float64        `json:"nir"`
	SCL               []int             `json:"scl"`
	SourceHashes      map[string]string `json:"sourceHashes"`
	PreparationMethod string            `json:"preparationMethod"`
	Synthetic         bool              `json:"synthetic"`
}

func Decode(raw []byte) (Tile, error) {
	var t Tile
	if len(raw) > artifacts.MaxBytes || json.Unmarshal(raw, &t) != nil {
		return t, errors.New("invalid_subset")
	}
	return t, t.Validate()
}
func (t Tile) Validate() error {
	g := t.Grid
	n := g.Width * g.Height
	if t.Schema != "emad.multispectral-subset.v1" || t.SceneID == "" || t.SceneHash == "" || t.AcquiredAt.IsZero() || t.PreparationMethod == "" || len(t.PreparationMethod) > 2000 || t.ReflectanceUnit != "surface_reflectance" || g.CRS != "EPSG:4326" || g.Registration != "pixel_edges_north_to_south" || g.Width < 1 || g.Height < 1 || g.Width > 256 || g.Height > 256 || n > MaxCells || len(t.Red) != n || len(t.NIR) != n || len(t.SCL) != n || g.StepLng < 1e-6 || g.StepLat < 1e-6 || g.StepLng > .001 || g.StepLat > .001 || g.Bounds().Validate() != nil {
		return errors.New("invalid_subset")
	}
	if len(t.SourceHashes) != 3 {
		return errors.New("source_hashes_missing")
	}
	for _, band := range []string{"B04_10m", "B08_10m", "SCL_20m"} {
		h := t.SourceHashes[band]
		if len(h) != 64 {
			return errors.New("source_hashes_missing")
		}
		for _, c := range h {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return errors.New("source_hashes_missing")
			}
		}
	}
	for i := range t.Red {
		if t.SCL[i] < 0 || t.SCL[i] > 11 {
			return errors.New("invalid_quality_mask")
		}
		for _, v := range []*float64{t.Red[i], t.NIR[i]} {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < -.2 || *v > 2) {
				return errors.New("invalid_reflectance")
			}
		}
	}
	return nil
}
func valid(t Tile, i int) (float64, bool) {
	if t.SCL[i] != 4 && t.SCL[i] != 5 && t.SCL[i] != 6 {
		return 0, false
	}
	r, n := t.Red[i], t.NIR[i]
	if r == nil || n == nil || *r < 0 || *n < 0 || *r+*n < 1e-8 {
		return 0, false
	}
	return (*n - *r) / (*n + *r), true
}

type Parameters struct {
	DeclineThreshold float64 `json:"declineThreshold"`
	MinValidFraction float64 `json:"minValidFraction"`
}

func (p Parameters) Validate() error {
	if math.IsNaN(p.DeclineThreshold) || p.DeclineThreshold < .05 || p.DeclineThreshold > 1 || math.IsNaN(p.MinValidFraction) || p.MinValidFraction < .1 || p.MinValidFraction > 1 {
		return errors.New("invalid_analysis_parameters")
	}
	return nil
}

type Result struct {
	GeometryProcessor string          `json:"geometryProcessor"`
	Schema            string          `json:"schema"`
	Algorithm         string          `json:"algorithm"`
	Parameters        Parameters      `json:"parameters"`
	Grid              Grid            `json:"grid"`
	BeforeScene       string          `json:"beforeScene"`
	AfterScene        string          `json:"afterScene"`
	BeforeNDVI        []*float64      `json:"beforeNdvi"`
	AfterNDVI         []*float64      `json:"afterNdvi"`
	Delta             []*float64      `json:"delta"`
	ValidCells        int             `json:"validCells"`
	MaskedCells       int             `json:"maskedCells"`
	DeclineCells      int             `json:"declineCells"`
	MeanDelta         float64         `json:"meanDelta"`
	FindingGeometry   json.RawMessage `json:"findingGeometry"`
	Limitations       []string        `json:"limitations"`
	Synthetic         bool            `json:"synthetic"`
}

func Compare(ctx context.Context, before, after Tile, p Parameters) (Result, error) {
	var result Result
	if e := before.Validate(); e != nil {
		return result, e
	}
	if e := after.Validate(); e != nil {
		return result, e
	}
	if e := p.Validate(); e != nil {
		return result, e
	}
	if before.Grid != after.Grid {
		return result, errors.New("grid_alignment_mismatch")
	}
	if before.SceneID == after.SceneID || !after.AcquiredAt.After(before.AcquiredAt) || after.AcquiredAt.Sub(before.AcquiredAt) > 90*24*time.Hour {
		return result, errors.New("scene_time_incomparable")
	}
	if before.Platform != after.Platform || before.Tile != after.Tile || before.Orbit != after.Orbit || before.ProcessingVersion != after.ProcessingVersion {
		return result, errors.New("radiometric_or_geometry_incomparable")
	}
	result = Result{Schema: "emad.ndvi-result.v1", Algorithm: Version, Parameters: p, Grid: before.Grid, BeforeScene: before.SceneID, AfterScene: after.SceneID, BeforeNDVI: make([]*float64, len(before.Red)), AfterNDVI: make([]*float64, len(before.Red)), Delta: make([]*float64, len(before.Red)), Synthetic: before.Synthetic || after.Synthetic, Limitations: []string{"NDVI difference is an environmental indicator, not verified vegetation loss or a causal assessment.", "Cloud/shadow/snow/saturated/unclassified and invalid pixels are excluded using supplied SCL; mask errors remain possible.", "Same grid, platform, tile, orbit and processing baseline required; seasonal/illumination/atmospheric differences remain.", "Angular output pixels do not imply constant ground resolution or a measured affected area.", "Retained prepared subsets and their input hashes reproduce this calculation; source-file hash claims are supplied by the preparer."}}
	var polygons [][][][]float64
	sum := 0.0
	for i := range before.Red {
		if i%before.Grid.Width == 0 {
			if e := ctx.Err(); e != nil {
				return Result{}, e
			}
		}
		a, aok := valid(before, i)
		b, bok := valid(after, i)
		if aok {
			v := a
			result.BeforeNDVI[i] = &v
		}
		if bok {
			v := b
			result.AfterNDVI[i] = &v
		}
		if !aok || !bok {
			result.MaskedCells++
			continue
		}
		result.ValidCells++
		delta := b - a
		result.Delta[i] = &delta
		sum += delta
		if delta <= -p.DeclineThreshold {
			result.DeclineCells++
			g := before.Grid
			row, col := i/g.Width, i%g.Width
			w, n := g.West+float64(col)*g.StepLng, g.North-float64(row)*g.StepLat
			e, s := w+g.StepLng, n-g.StepLat
			polygons = append(polygons, [][][]float64{{{w, s}, {e, s}, {e, n}, {w, n}, {w, s}}})
		}
	}
	if float64(result.ValidCells)/float64(len(before.Red)) < p.MinValidFraction {
		return Result{}, errors.New("insufficient_common_clear_pixels")
	}
	if len(polygons) > 4096 {
		return Result{}, errors.New("finding_geometry_exceeds_budget")
	}
	result.MeanDelta = sum / float64(result.ValidCells)
	if len(polygons) > 0 {
		result.FindingGeometry, _ = json.Marshal(map[string]any{"type": "MultiPolygon", "coordinates": polygons})
	} else {
		result.FindingGeometry = json.RawMessage("null")
	}
	if result.Synthetic {
		result.Limitations = append(result.Limitations, "SYNTHETIC FIXTURE: these pixel values are not observed environmental evidence.")
	}
	return result, nil
}
func TileSummary(t Tile) string {
	return fmt.Sprintf("%dx%d aligned angular cells; prepared using %s", t.Grid.Width, t.Grid.Height, t.PreparationMethod)
}

// CommonClearCells uses the same reflectance and SCL eligibility as Compare.
// It permits a preflight to explain insufficient coverage without a result.
func CommonClearCells(a, b Tile) (int, int) {
	if a.Grid != b.Grid || len(a.Red) != len(b.Red) {
		return 0, 0
	}
	count := 0
	for i := range a.Red {
		_, okA := valid(a, i)
		_, okB := valid(b, i)
		if okA && okB {
			count++
		}
	}
	return count, len(a.Red)
}
