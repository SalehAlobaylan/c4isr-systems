// Package simscene owns immutable authoring snapshots shared by Studio and the
// independent runtime. It contains no renderer, clock loop, or operational asset.
package simscene

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

const Schema = "emad.scene.v1"
const RuntimeVersion = "emad-kinematic-v1"
const MaxBytes = 4 * 1024 * 1024
const Radius = 10000.0

type Placement struct {
	ID         string                `json:"id"`
	TemplateID string                `json:"templateId"`
	Position   simulation.Position   `json:"position"`
	Heading    float64               `json:"heading"`
	SpeedMPS   float64               `json:"speedMps"`
	Waypoints  []simulation.Position `json:"waypoints"`
	SpawnMs    int64                 `json:"spawnMs"`
	DespawnMs  int64                 `json:"despawnMs"`
}
type Input struct {
	Sensors          []Sensor     `json:"sensors,omitempty"`
	Controlled       []Controlled `json:"controlled,omitempty"`
	Name             string       `json:"name"`
	ContextPackageID string       `json:"contextPackageId"`
	EnvironmentID    string       `json:"environmentId"`
	Actors           []Placement  `json:"actors"`
}
type Frozen struct {
	SensorPresets []SensorPreset               `json:"sensorPresets,omitempty"`
	Schema        string                       `json:"schema"`
	Runtime       string                       `json:"runtime"`
	Input         Input                        `json:"input"`
	Context       contextpackage.Bundle        `json:"context"`
	Environment   modelassets.EnvironmentAsset `json:"environment"`
	Templates     []modelassets.ActorTemplate  `json:"templates"`
	Models        []modelassets.ModelAsset     `json:"models"`
	Geometry      Geometry                     `json:"geometry"`
}
type Scene struct {
	ID     string `json:"id"`
	Frozen Frozen `json:"frozen"`
}
type ReferencePoint struct {
	ID            string            `json:"id"`
	Local         simulation.Vector `json:"local"`
	HeightAssumed bool              `json:"heightAssumed"`
}
type Geometry struct {
	Terrain   []simulation.Vector `json:"terrain"`
	Triangles []int               `json:"triangles"`
	Points    []ReferencePoint    `json:"points"`
	Omissions []string            `json:"omissions"`
}

var actorID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Within(f simulation.Frame, p simulation.Position) (simulation.Vector, error) {
	v, e := f.ToLocal(p)
	if e != nil {
		return v, e
	}
	if math.Sqrt(v.X*v.X+v.Y*v.Y+v.Z*v.Z) > Radius {
		return v, fmt.Errorf("scene radius exceeds 10 km; choose a smaller context subset")
	}
	return v, nil
}
func (s Frozen) Validate(now time.Time) error {
	fail := func() error { return fmt.Errorf("invalid frozen scene contract") }
	if s.Schema != Schema || s.Runtime != s.Input.Runtime() || s.Environment.Frame.Validate() != nil || s.Input.EnvironmentID != s.Environment.ID || s.Input.ContextPackageID != s.Context.ID || len(s.Input.Name) < 1 || len(s.Input.Name) > 200 || len(s.Input.Actors) < 1 || len(s.Input.Actors) > 16 || len(s.Models) < 1 || len(s.Models) > 8 || len(s.Templates) < 1 || len(s.Templates) > 16 {
		return fail()
	}
	if e := s.Context.Validate(now, true); e != nil {
		return e
	}
	if e := s.Input.ValidateSensors(); e != nil {
		return e
	}
	if e := s.ValidatePresets(); e != nil {
		return e
	}
	templates := map[string]modelassets.ActorTemplate{}
	models := map[string]bool{}
	var vertices, triangles, pixels int
	var bytes int64
	for _, m := range s.Models {
		if m.ID == "" || models[m.ID] || m.Upload.State != "READY" || !m.Upload.License.AllowSimulation || m.Validation.Fidelity != "visual_only" {
			return fail()
		}
		models[m.ID] = true
		vertices += m.Validation.Vertices
		triangles += m.Validation.Triangles
		pixels += m.Validation.DecodedTexturePixels
		bytes += m.Upload.Bytes
	}
	if vertices > 500000 || triangles > 500000 || pixels > 32*1024*1024 || bytes > 32*1024*1024 || !models[s.Environment.ModelVersionID] {
		return fmt.Errorf("aggregate scene model budget exceeded")
	}
	for _, t := range s.Templates {
		if templates[t.ID].ID != "" || !models[t.ModelVersionID] || (t.Behavior != "STATIC" && t.Behavior != "WAYPOINT_KINEMATIC") {
			return fail()
		}
		templates[t.ID] = t
	}
	seen := map[string]bool{}
	for _, a := range s.Input.Actors {
		t, ok := templates[a.TemplateID]
		if !ok || !actorID.MatchString(a.ID) || seen[a.ID] || !finite(a.Heading) || a.Heading < 0 || a.Heading >= 360 || !finite(a.SpeedMPS) || a.SpeedMPS < 0 || a.SpeedMPS > t.MaxSpeedMPS || a.SpawnMs < 0 || a.DespawnMs <= a.SpawnMs || a.DespawnMs > 3600000 || len(a.Waypoints) > 32 {
			return fail()
		}
		if t.Behavior == "STATIC" && (a.SpeedMPS != 0 || len(a.Waypoints) != 0) || t.Behavior == "WAYPOINT_KINEMATIC" && (a.SpeedMPS <= 0 || len(a.Waypoints) < 1) {
			return fail()
		}
		seen[a.ID] = true
		for _, p := range append([]simulation.Position{a.Position}, a.Waypoints...) {
			if _, e := Within(s.Environment.Frame, p); e != nil {
				return e
			}
		}
	}
	_, e := GeometryFor(s.Context, s.Environment.Frame)
	return e
}

// GeometryFor uses the accepted S0 transform. Unknown vertical datums are never
// reinterpreted as ellipsoidal heights. Reference points without elevation are
// explicitly projected onto the origin-height surface, not asserted as truth.
func GeometryFor(b contextpackage.Bundle, f simulation.Frame) (Geometry, error) {
	g := Geometry{Terrain: []simulation.Vector{}, Triangles: []int{}, Points: []ReferencePoint{}, Omissions: []string{}}
	for _, asset := range b.Manifest.Assets {
		raw := b.Assets[asset.SHA256]
		switch asset.Kind {
		case "elevation":
			if asset.VerticalDatum != "WGS84_ELLIPSOID" {
				return g, fmt.Errorf("terrain requires a versioned WGS84_ELLIPSOID conversion; source datum: %s", asset.VerticalDatum)
			}
			var grid reference.Grid
			if json.Unmarshal(raw, &grid) != nil || grid.Width < 2 || grid.Height < 2 || grid.Width > 256 || grid.Height > 256 || grid.Width*grid.Height > 4096 || len(grid.Values) != grid.Width*grid.Height || grid.Registration != "point_samples" || grid.StepX <= 0 || grid.StepY <= 0 {
				return g, fmt.Errorf("terrain grid exceeds the 4096-sample scene profile")
			}
			offset := len(g.Terrain)
			valid := make([]bool, len(grid.Values))
			if offset+len(grid.Values) > 4096 {
				return g, fmt.Errorf("aggregate terrain budget exceeded")
			}
			for i, h := range grid.Values {
				v := simulation.Vector{}
				if h != nil {
					var e error
					v, e = Within(f, simulation.Position{Longitude: grid.West + float64(i%grid.Width)*grid.StepX, Latitude: grid.South + float64(i/grid.Width)*grid.StepY, Height: h, VerticalDatum: asset.VerticalDatum})
					if e != nil {
						return g, e
					}
					valid[i] = true
				}
				g.Terrain = append(g.Terrain, v)
			}
			for y := 0; y < grid.Height-1; y++ {
				for x := 0; x < grid.Width-1; x++ {
					a := y*grid.Width + x
					c := a + grid.Width
					for _, tri := range [][3]int{{a, a + 1, c}, {a + 1, c + 1, c}} {
						if valid[tri[0]] && valid[tri[1]] && valid[tri[2]] {
							for _, i := range tri {
								g.Triangles = append(g.Triangles, offset+i)
							}
						}
					}
				}
			}
		case "vector":
			var collection reference.Collection
			if e := json.Unmarshal(raw, &collection); e != nil {
				return g, e
			}
			if len(collection.Features) > 1000 {
				return g, fmt.Errorf("reference feature budget exceeded")
			}
			for _, feature := range collection.Features {
				var geom struct {
					Type        string          `json:"type"`
					Coordinates json.RawMessage `json:"coordinates"`
				}
				if json.Unmarshal(feature.Geometry, &geom) != nil {
					return g, fmt.Errorf("invalid reference geometry")
				}
				if geom.Type != "Point" {
					g.Omissions = append(g.Omissions, "Non-point reference feature not rendered: "+feature.ID)
					continue
				}
				var coords []float64
				if json.Unmarshal(geom.Coordinates, &coords) != nil || len(coords) < 2 || len(coords) > 3 {
					return g, fmt.Errorf("invalid reference point")
				}
				h := *f.Origin.Height
				assumed := true
				if len(coords) == 3 && asset.VerticalDatum == "WGS84_ELLIPSOID" {
					h = coords[2]
					assumed = false
				}
				v, e := Within(f, simulation.Position{Longitude: coords[0], Latitude: coords[1], Height: &h, VerticalDatum: "WGS84_ELLIPSOID"})
				if e != nil {
					return g, e
				}
				if len(g.Points) >= 1000 {
					return g, fmt.Errorf("reference point budget exceeded")
				}
				g.Points = append(g.Points, ReferencePoint{asset.DatasetID + ":" + feature.ID, v, assumed})
			}
		default:
			g.Omissions = append(g.Omissions, "Retained lineage only; scene profile does not render "+asset.Kind)
		}
	}
	return g, nil
}
func Freeze(f Frozen) (Scene, []byte, error) {
	if e := f.Validate(time.Now()); e != nil {
		return Scene{}, nil, e
	}
	g, e := GeometryFor(f.Context, f.Environment.Frame)
	if e != nil {
		return Scene{}, nil, e
	}
	f.Geometry = g
	raw, e := json.Marshal(f)
	if e != nil {
		return Scene{}, nil, e
	}
	if len(raw) > MaxBytes {
		return Scene{}, nil, fmt.Errorf("scene snapshot exceeds 4 MiB")
	}
	return Scene{"scene_" + artifacts.Hash(raw), f}, raw, nil
}
