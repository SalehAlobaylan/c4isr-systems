package imagery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
	"time"
)

type ProposalInput struct {
	Scope   analystscope.Input   `json:"scope"`
	SceneID string               `json:"sceneId"`
	AOI     *publicingest.Bounds `json:"aoi,omitempty"`
}
type Proposal struct {
	Scope         analystscope.Snapshot `json:"scope"`
	SceneID       string                `json:"sceneId"`
	SceneGeometry json.RawMessage       `json:"sceneGeometry"`
	SuggestedAOI  *publicingest.Bounds  `json:"suggestedAoi,omitempty"`
	ProposedAOI   *publicingest.Bounds  `json:"proposedAoi,omitempty"`
	Supported     bool                  `json:"supported"`
	Valid         bool                  `json:"valid"`
	Reasons       []string              `json:"reasons"`
	Meaning       string                `json:"meaning"`
}

func (s *Service) CaptureProposal(ctx context.Context, in ProposalInput) (Proposal, error) {
	out := Proposal{SceneID: in.SceneID, ProposedAOI: in.AOI, Reasons: []string{}, Meaning: "Selected area, retained provider footprint and proposed local rectangle are separate. Adopt a suggested rectangle deliberately; exact containment and current rights are rechecked at submission."}
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return out, e
	}
	scope, e := (analystscope.Service{Pool: s.Pool}).Resolve(ctx, in.Scope)
	if e != nil {
		return out, e
	}
	out.Scope = scope
	scene, e := s.Scene(ctx, in.SceneID)
	if e != nil {
		return out, e
	}
	if e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_AsGeoJSON(geometry,17) FROM imagery_scenes WHERE id=$1`, scene.ID).Scan(&out.SceneGeometry); e != nil {
		return out, e
	}
	out.Supported = scene.Collection == publicingest.EarthSearchDataset
	if !out.Supported {
		out.Reasons = append(out.Reasons, "This catalog product has no supported automatic raster path. Choose an admitted Collection 1 scene or use an explicitly supported retained import.")
	}
	policy, e := s.Artifacts.Policy(ctx, scene.SourceID, scene.AdmissionID)
	if e != nil || !scene.Rights.RetainRaw || !scene.Rights.AllowAnalysis || !policy.Rights.RetainRaw || !policy.Rights.AllowAnalysis {
		out.Supported = false
		out.Reasons = append(out.Reasons, "Original scene/profile retention and analysis rights are required.")
	}
	if len(scope.Area.Geometry) == 0 {
		out.Reasons = append(out.Reasons, "Choose a bounded selected area before acquiring a local rectangle.")
		return out, nil
	}
	if scope.Capabilities["captures"].State != "supported" || !analystscope.ContainsTime(scope, scene.AcquiredAt) {
		out.Supported = false
		out.Reasons = append(out.Reasons, "Scene acquisition is outside the selected supported period.")
	}
	// A representative point proposes a local rectangle only. Repeated exact
	// coverage checks shrink it around holes/coasts; it never becomes a buffer
	// or a replacement polygon and is not adopted automatically.
	var x, y *float64
	e = db.Scoped(s.Pool).QueryRow(ctx, `WITH part AS(SELECT ST_Intersection(geometry,ST_SetSRID(ST_GeomFromGeoJSON($2),4326))g FROM imagery_scenes WHERE id=$1),point AS(SELECT ST_PointOnSurface(g)g FROM part WHERE NOT ST_IsEmpty(g) AND ST_Area(g)>0) SELECT ST_X(g),ST_Y(g) FROM point`, scene.ID, scope.Area.Geometry).Scan(&x, &y)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if e == nil && x != nil && y != nil && out.Supported {
		for half := .025; half >= .0001; half /= 2 {
			b := publicingest.Bounds{West: *x - half, South: *y - half, East: *x + half, North: *y + half}
			var covered bool
			e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_Covers(geometry,ST_MakeEnvelope($2,$3,$4,$5,4326)) AND ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON($6),4326),ST_MakeEnvelope($2,$3,$4,$5,4326)) FROM imagery_scenes WHERE id=$1`, scene.ID, b.West, b.South, b.East, b.North, scope.Area.Geometry).Scan(&covered)
			if e != nil {
				return out, e
			}
			if covered && b.Validate() == nil {
				out.SuggestedAOI = &b
				break
			}
		}
	}
	if out.SuggestedAOI == nil {
		out.Reasons = append(out.Reasons, "No supported inscribed rectangle was found. Draw a smaller local selection or choose a covering scene.")
	}
	if in.AOI != nil {
		b := *in.AOI
		if b.Validate() != nil || b.East-b.West > .1 || b.North-b.South > .1 {
			out.Reasons = append(out.Reasons, "The capture rectangle must be WGS84 longitude/latitude and at most 0.1 degrees per side.")
		} else {
			var covered bool
			e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_Covers(geometry,ST_MakeEnvelope($2,$3,$4,$5,4326)) AND ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON($6),4326),ST_MakeEnvelope($2,$3,$4,$5,4326)) FROM imagery_scenes WHERE id=$1`, scene.ID, b.West, b.South, b.East, b.North, scope.Area.Geometry).Scan(&covered)
			if e != nil {
				return out, e
			}
			out.Valid = out.Supported && covered
			if !covered {
				out.Reasons = append(out.Reasons, "The proposed rectangle crosses the exact selected boundary, a hole, or the retained scene footprint.")
			}
		}
	}
	return out, nil
}

type PairInput struct {
	Scope      analystscope.Input         `json:"scope"`
	BeforeID   string                     `json:"beforeId"`
	AfterID    string                     `json:"afterId"`
	Parameters imageryanalysis.Parameters `json:"parameters"`
}
type PairCheck struct {
	Compatible    bool      `json:"compatible"`
	Reasons       []string  `json:"reasons"`
	ValidCells    int       `json:"validCells"`
	TotalCells    int       `json:"totalCells"`
	ClearFraction float64   `json:"clearFraction"`
	Limitations   []string  `json:"limitations"`
	EvaluatedAt   time.Time `json:"evaluatedAt"`
}

func (s *Service) CheckPair(ctx context.Context, in PairInput) (PairCheck, error) {
	out := PairCheck{Reasons: []string{}, Limitations: []string{}, EvaluatedAt: time.Now().UTC()}
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return out, e
	}
	scope, e := (analystscope.Service{Pool: s.Pool}).Resolve(ctx, in.Scope)
	if e != nil {
		return out, e
	}
	var tiles []imageryanalysis.Tile
	profile, source := "", ""
	for _, id := range []string{in.BeforeID, in.AfterID} {
		job, e := s.Job(ctx, id)
		if e != nil {
			return out, e
		}
		if job.Status != "SUCCEEDED" || job.Input.Kind != "PREPARE" || job.Result == nil || job.Result.Tile == nil || len(job.Input.Artifacts) != 1 || len(job.Input.Scenes) != 1 {
			out.Reasons = append(out.Reasons, "Choose two completed retained preparations.")
			return out, nil
		}
		scene := job.Input.Scenes[0]
		policy, e := s.Artifacts.Policy(ctx, scene.SourceID, scene.AdmissionID)
		if e != nil {
			return out, e
		}
		if !policy.Rights.AllowAnalysis || !scene.Rights.AllowAnalysis || source != "" && source != scene.SourceID || profile != "" && profile != policy.ProfileKey {
			out.Reasons = append(out.Reasons, "Both preparations require their original analysis rights and the same approved retrieval profile.")
			return out, nil
		}
		profile = policy.ProfileKey
		source = scene.SourceID
		if !analystscope.ContainsTime(scope, scene.AcquiredAt) || scope.Capabilities["imagery"].State != "supported" {
			out.Reasons = append(out.Reasons, "Both acquisition dates must be within the selected supported period.")
			return out, nil
		}
		if e = analystscope.ValidateBounds(ctx, s.Pool, scope, job.Result.Tile.Grid.Bounds()); e != nil {
			out.Reasons = append(out.Reasons, "Both retained subsets must be inside the exact selected area.")
			return out, nil
		}
		_, raw, e := s.Artifacts.Read(ctx, job.Input.Artifacts[0].ID)
		if e != nil {
			return out, e
		}
		tile, e := imageryanalysis.Decode(raw)
		if e != nil {
			return out, e
		}
		tiles = append(tiles, tile)
	}
	if in.BeforeID == in.AfterID {
		return out, apperr.Validation("choose different preparations in acquisition order")
	}
	out.ValidCells, out.TotalCells = imageryanalysis.CommonClearCells(tiles[0], tiles[1])
	if out.TotalCells > 0 {
		out.ClearFraction = float64(out.ValidCells) / float64(out.TotalCells)
	}
	result, e := imageryanalysis.Compare(ctx, tiles[0], tiles[1], in.Parameters)
	if e != nil {
		reason := map[string]string{
			"grid_alignment_mismatch":              "The retained grids differ. Capture both dates using the same local rectangle and grid size.",
			"scene_time_incomparable":              "Choose different acquisitions in chronological order and no more than 90 days apart.",
			"radiometric_or_geometry_incomparable": "Choose the same platform, tile, orbit and processing baseline.",
			"insufficient_common_clear_pixels":     "Too few jointly clear pixels meet the selected minimum. Inspect the retained masks and choose a clearer compatible pair.",
			"finding_geometry_exceeds_budget":      "The change geometry exceeds the local processing budget. Choose a smaller retained subset."}[e.Error()]
		if reason == "" {
			reason = "These retained inputs cannot be compared under the selected processing rules."
		}
		out.Reasons = append(out.Reasons, reason)
		return out, nil
	}
	out.Compatible = true
	out.ValidCells = result.ValidCells
	out.TotalCells = len(tiles[0].Red)
	out.ClearFraction = float64(result.ValidCells) / float64(out.TotalCells)
	out.Limitations = result.Limitations
	return out, nil
}
func (h Handler) mountPreflight(r chi.Router) {
	r.Post("/imagery/capture-proposal", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		var in ProposalInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := h.Service.CaptureProposal(ctx, in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/imagery/comparison-check", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		var in PairInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := h.Service.CheckPair(ctx, in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}
