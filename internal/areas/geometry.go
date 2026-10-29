// Package areas owns immutable analyst selections, not source admission policy.
package areas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Hash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

type Component struct {
	Geometry json.RawMessage `json:"geometry"`
	Bounds   [4]float64      `json:"bounds"`
	AreaM2   float64         `json:"areaM2"`
}
type Display struct {
	State         string  `json:"state"`
	LatitudeLimit float64 `json:"latitudeLimit"`
	Message       string  `json:"message"`
}

// Unwrap each polygon independently. Holes are moved to their shell's longitude
// band before validity checks. This avoids shifting unrelated multipart islands.
// A ring winding around a pole is ambiguous in this planar contract and rejected.
func unwrap(raw json.RawMessage) (json.RawMessage, bool, error) {
	bad := func() (json.RawMessage, bool, error) {
		return nil, false, apperr.Validation("use valid 2D WGS84 Polygon/MultiPolygon geometry; ambiguous polar windings require an explicit supported boundary")
	}
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
		CRS         json.RawMessage `json:"crs"`
	}
	if len(raw) > 2<<20 || json.Unmarshal(raw, &g) != nil || len(g.CRS) > 0 {
		return bad()
	}
	var polys [][][][]float64
	switch g.Type {
	case "Polygon":
		var p [][][]float64
		if json.Unmarshal(g.Coordinates, &p) != nil {
			return bad()
		}
		polys = [][][][]float64{p}
	case "MultiPolygon":
		if json.Unmarshal(g.Coordinates, &polys) != nil {
			return bad()
		}
	default:
		return bad()
	}
	if len(polys) == 0 || len(polys) > 10000 {
		return bad()
	}
	points := 0
	changed := false
	for _, p := range polys {
		if len(p) == 0 || len(p) > 10000 {
			return bad()
		}
		shellCenter := 0.
		for ri, ring := range p {
			if len(ring) < 4 {
				return bad()
			}
			points += len(ring)
			if points > 100000 {
				return bad()
			}
			for _, v := range ring {
				if len(v) != 2 || math.IsNaN(v[0]) || math.IsNaN(v[1]) || math.IsInf(v[0], 0) || math.IsInf(v[1], 0) || v[0] < -180 || v[0] > 180 || v[1] < -90 || v[1] > 90 {
					return bad()
				}
			}
			if ring[0][0] != ring[len(ring)-1][0] || ring[0][1] != ring[len(ring)-1][1] {
				return bad()
			}
			for i := 1; i < len(ring); i++ {
				for ring[i][0]-ring[i-1][0] > 180 {
					ring[i][0] -= 360
					changed = true
				}
				for ring[i][0]-ring[i-1][0] < -180 {
					ring[i][0] += 360
					changed = true
				}
			}
			if ring[0][0] != ring[len(ring)-1][0] {
				return bad()
			}
			low, high := ring[0][0], ring[0][0]
			for _, v := range ring {
				low = math.Min(low, v[0])
				high = math.Max(high, v[0])
			}
			if high-low > 360 {
				return bad()
			}
			center := (low + high) / 2
			if ri == 0 {
				shellCenter = center
			} else {
				shift := math.Round((shellCenter-center)/360) * 360
				if shift != 0 {
					changed = true
					for _, v := range ring {
						v[0] += shift
					}
				}
			}
		}
	}
	out, _ := json.Marshal(map[string]any{"type": "MultiPolygon", "coordinates": polys})
	return out, changed, nil
}

func normalize(ctx context.Context, pool *pgxpool.Pool, raw json.RawMessage) (json.RawMessage, []Component, float64, Display, bool, error) {
	unwrapped, changed, e := unwrap(raw)
	if e != nil {
		return nil, nil, 0, Display{}, false, e
	}
	var valid bool
	if e = db.Scoped(pool).QueryRow(ctx, `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON($1),4326))`, string(unwrapped)).Scan(&valid); e != nil {
		return nil, nil, 0, Display{}, false, e
	}
	if !valid {
		return nil, nil, 0, Display{}, false, apperr.Validation("invalid area topology; boundaries and holes must not self-intersect")
	}
	var canonical, components []byte
	var area, south, north float64
	// WrapX splits at the seam without simplifying the boundary or filling holes.
	e = db.Scoped(pool).QueryRow(ctx, `WITH input AS(SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) g),normal AS(SELECT ST_Normalize(ST_Multi(ST_CollectionExtract(ST_WrapX(ST_WrapX(g,180,-360),-180,360),3))) g FROM input),parts AS(SELECT (ST_Dump(g)).geom g FROM normal) SELECT ST_AsGeoJSON(n.g,17,0),ST_Area(n.g::geography),ST_YMin(n.g),ST_YMax(n.g),(SELECT jsonb_agg(jsonb_build_object('geometry',ST_AsGeoJSON(p.g,17,0)::jsonb,'bounds',jsonb_build_array(ST_XMin(p.g),ST_YMin(p.g),ST_XMax(p.g),ST_YMax(p.g)),'areaM2',ST_Area(p.g::geography)) ORDER BY ST_XMin(p.g),ST_YMin(p.g),ST_AsEWKB(p.g)) FROM parts p) FROM normal n WHERE ST_IsValid(n.g) AND NOT ST_IsEmpty(n.g) AND ST_XMin(n.g)>=-180 AND ST_XMax(n.g)<=180 AND ST_Equals(n.g,ST_SetSRID(ST_GeomFromGeoJSON(ST_AsGeoJSON(n.g,17,0)),4326))`, string(unwrapped)).Scan(&canonical, &area, &south, &north, &components)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, 0, Display{}, false, apperr.Validation("area cannot be represented without changing its boundary at the supported coordinate precision")
	}
	if e != nil {
		return nil, nil, 0, Display{}, false, e
	}
	var parts []Component
	if e = json.Unmarshal(components, &parts); e != nil {
		return nil, nil, 0, Display{}, false, e
	}
	display := Display{"supported", 85, "WGS84 retained geometry; map display is limited to ±85° latitude."}
	if south < -85 || north > 85 {
		display.State = "partial"
		display.Message = "Part of this area lies beyond the map's ±85° display limit. Exact coordinates and tabular evidence remain available; draw a supported local subset for raster work."
	}
	return canonical, parts, area, display, changed, nil
}
