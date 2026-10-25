// Package simruntime implements the first independent, deterministic kinematic
// runtime. World evaluation happens in emad-runtime, never in an HTTP handler.
package simruntime

import (
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"math"
)

type ActorState struct {
	ID      string            `json:"id"`
	Local   simulation.Vector `json:"local"`
	Heading float64           `json:"heading"`
	Active  bool              `json:"active"`
}
type Snapshot struct {
	RunID        string       `json:"runId"`
	SceneID      string       `json:"sceneId"`
	RuntimeBuild string       `json:"runtimeBuild"`
	Sequence     int          `json:"sequence"`
	ElapsedMs    int64        `json:"elapsedMs"`
	Actors       []ActorState `json:"actors"`
}
type path struct {
	placement simscene.Placement
	points    []simulation.Vector
	holds     []hold
}
type hold struct{ from, to int64 }

func (w *World) Control(id, action string, elapsed int64) bool {
	for i := range w.paths {
		p := &w.paths[i]
		if p.placement.ID != id {
			continue
		}
		if elapsed < p.placement.SpawnMs || elapsed >= p.placement.DespawnMs {
			return false
		}
		active := len(p.holds) > 0 && p.holds[len(p.holds)-1].to == 0
		if action == "HOLD" && !active {
			p.holds = append(p.holds, hold{from: elapsed})
		}
		if action == "RESUME_ROUTE" && active {
			p.holds[len(p.holds)-1].to = elapsed
		}
		return true
	}
	return false
}

type World struct{ paths []path }

func NewWorld(f simscene.Frozen) (*World, error) {
	w := &World{}
	for _, a := range f.Input.Actors {
		p := path{placement: a}
		for _, position := range append([]simulation.Position{a.Position}, a.Waypoints...) {
			v, e := simscene.Within(f.Environment.Frame, position)
			if e != nil {
				return nil, e
			}
			p.points = append(p.points, v)
		}
		w.paths = append(w.paths, p)
	}
	return w, nil
}

// At is a pure function of frozen paths and integer virtual milliseconds. The
// declared model has no forces, collision, terrain following, sensors or weapons.
func (w *World) At(elapsed int64) ([]ActorState, error) {
	if elapsed < 0 || elapsed > 3600000 {
		return nil, fmt.Errorf("virtual time outside runtime budget")
	}
	out := make([]ActorState, 0, len(w.paths))
	for _, p := range w.paths {
		a := p.placement
		state := ActorState{ID: a.ID, Local: p.points[0], Heading: a.Heading, Active: elapsed >= a.SpawnMs && elapsed < a.DespawnMs}
		t := elapsed
		if t < a.SpawnMs {
			t = a.SpawnMs
		}
		if t > a.DespawnMs {
			t = a.DespawnMs
		}
		paused := int64(0)
		for _, h := range p.holds {
			if h.from >= t {
				continue
			}
			end := h.to
			if end == 0 || end > t {
				end = t
			}
			paused += end - h.from
		}
		distance := float64(t-a.SpawnMs-paused) * a.SpeedMPS / 1000
		for i := 1; i < len(p.points); i++ {
			from, to := p.points[i-1], p.points[i]
			dx, dy, dz := to.X-from.X, to.Y-from.Y, to.Z-from.Z
			length := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if length == 0 {
				continue
			}
			state.Heading = math.Mod(math.Atan2(dx, dy)*180/math.Pi+360, 360)
			fraction := math.Min(1, distance/length)
			state.Local = simulation.Vector{X: from.X + dx*fraction, Y: from.Y + dy*fraction, Z: from.Z + dz*fraction}
			if distance < length {
				break
			}
			distance -= length
		}
		out = append(out, state)
	}
	return out, nil
}
