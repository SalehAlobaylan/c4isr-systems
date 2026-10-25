package reference

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

func (h *Handler) mountSpatial(r chi.Router) {
	r.Post("/map/surrounding-area", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		var in struct {
			Longitude *float64 `json:"longitude"`
			Latitude  *float64 `json:"latitude"`
			RadiusM   float64  `json:"radiusM"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.Longitude == nil || in.Latitude == nil || *in.Longitude < -180 || *in.Longitude > 180 || *in.Latitude < -85 || *in.Latitude > 85 || in.RadiusM < 100 || in.RadiusM > 50000 {
			httpx.Error(w, apperr.Validation("surrounding context requires WGS84 coordinates and a radius between 100 and 50000 metres"))
			return
		}
		var raw []byte
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `WITH area AS(SELECT ST_Buffer(ST_SetSRID(ST_MakePoint($1,$2),4326)::geography,$3,'quad_segs=16')::geometry g) SELECT ST_AsGeoJSON(g,7) FROM area`, *in.Longitude, *in.Latitude, in.RadiusM).Scan(&raw)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if e = publicevents.ValidateStudyGeometry(string(raw), false); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"type": "Feature", "geometry": json.RawMessage(raw), "properties": map[string]any{"name": "Surrounding context", "radiusM": in.RadiusM, "meaning": "Analyst context buffer; not a maritime or administrative jurisdiction"}})
	})
	r.Post("/map/measure", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		var in struct {
			Geometry json.RawMessage `json:"geometry"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<10))
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
			httpx.Error(w, apperr.Validation("invalid measurement geometry"))
			return
		}
		if e := publicevents.ValidateStudyGeometry(string(in.Geometry), true); e != nil {
			httpx.Error(w, e)
			return
		}
		var valid bool
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON($1),4326))`, string(in.Geometry)).Scan(&valid)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if !valid {
			httpx.Error(w, apperr.Validation("study polygon intersects itself or is degenerate"))
			return
		}
		var area, perimeter, length float64
		e = db.Scoped(h.s.pool).QueryRow(r.Context(), `WITH g AS(SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) g) SELECT CASE WHEN ST_GeometryType(g)='ST_Polygon' THEN ST_Area(g::geography) ELSE 0 END,CASE WHEN ST_GeometryType(g)='ST_Polygon' THEN ST_Perimeter(g::geography) ELSE 0 END,CASE WHEN ST_GeometryType(g)='ST_LineString' THEN ST_Length(g::geography) ELSE 0 END FROM g`, string(in.Geometry)).Scan(&area, &perimeter, &length)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"areaM2": area, "perimeterM": perimeter, "lengthM": length, "method": "PostGIS WGS84 geography", "meaning": "Analyst study/measurement geometry, not an admitted jurisdiction"})
	})
}
