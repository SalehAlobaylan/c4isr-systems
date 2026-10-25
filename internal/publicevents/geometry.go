package publicevents

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"math"
)

// ValidateStudyGeometry bounds analyst study polygons and measurement lines;
// it does not admit a jurisdiction or change any connector retrieval policy.
func ValidateStudyGeometry(raw string, allowLine bool) error {
	bad := func() error {
		return apperr.Validation("use a WGS84 study polygon (3–64 vertices) or measurement line within 400 square degrees; split dateline crossings")
	}
	if len(raw) < 1 || len(raw) > 8192 {
		return bad()
	}
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
		CRS         json.RawMessage `json:"crs"`
	}
	if json.Unmarshal([]byte(raw), &g) != nil || len(g.CRS) > 0 {
		return bad()
	}
	var points [][]float64
	if g.Type == "Polygon" {
		var rings [][][]float64
		if json.Unmarshal(g.Coordinates, &rings) != nil || len(rings) != 1 {
			return bad()
		}
		points = rings[0]
		if len(points) < 4 || len(points) > 65 || len(points[0]) != 2 || len(points[len(points)-1]) != 2 || points[0][0] != points[len(points)-1][0] || points[0][1] != points[len(points)-1][1] {
			return bad()
		}
	} else if g.Type == "LineString" && allowLine {
		if json.Unmarshal(g.Coordinates, &points) != nil || len(points) < 2 || len(points) > 64 {
			return bad()
		}
	} else {
		return bad()
	}
	w, s, e, n := 180., 90., -180., -90.
	for _, p := range points {
		if len(p) != 2 || math.IsNaN(p[0]) || math.IsNaN(p[1]) || math.IsInf(p[0], 0) || math.IsInf(p[1], 0) || p[0] < -180 || p[0] > 180 || p[1] < -85 || p[1] > 85 {
			return bad()
		}
		w = math.Min(w, p[0])
		e = math.Max(e, p[0])
		s = math.Min(s, p[1])
		n = math.Max(n, p[1])
	}
	for i := 1; i < len(points); i++ {
		if math.Abs(points[i][0]-points[i-1][0]) > 180 {
			return bad()
		}
	}
	if (e-w)*(n-s) > 400 {
		return bad()
	}
	if g.Type == "Polygon" {
		area := 0.
		cross := func(a, b, c []float64) float64 { return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0]) }
		on := func(a, b, c []float64) bool {
			return math.Abs(cross(a, b, c)) < 1e-12 && c[0] >= math.Min(a[0], b[0])-1e-12 && c[0] <= math.Max(a[0], b[0])+1e-12 && c[1] >= math.Min(a[1], b[1])-1e-12 && c[1] <= math.Max(a[1], b[1])+1e-12
		}
		for i := 0; i < len(points)-1; i++ {
			a, b := points[i], points[i+1]
			if a[0] == b[0] && a[1] == b[1] {
				return bad()
			}
			area += a[0]*b[1] - b[0]*a[1]
			for j := i + 2; j < len(points)-1; j++ {
				if i == 0 && j == len(points)-2 {
					continue
				}
				c, d := points[j], points[j+1]
				if cross(a, b, c)*cross(a, b, d) < 0 && cross(c, d, a)*cross(c, d, b) < 0 || on(a, b, c) || on(a, b, d) || on(c, d, a) || on(c, d, b) {
					return bad()
				}
			}
		}
		if math.Abs(area) < 1e-12 {
			return bad()
		}
	}
	return nil
}
