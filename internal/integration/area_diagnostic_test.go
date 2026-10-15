package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestAreaOverlapSQLIsSound isolates the spatial query from the alert pipeline.
//
// The failure this guards against is the interesting one: a hazard overlapping a
// zone must produce an alert, and the only way to know the overlap detection
// works is to ask PostGIS directly rather than infer it from a downstream alert.
func TestAreaOverlapSQLIsSound(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createSource("firms")

	if status := h.do(http.MethodPost, "/api/v1/geofences", map[string]any{
		"id": "sql-zone", "name": "SQL Zone", "type": "restricted", "severity": "high",
		"polygon": []map[string]float64{
			{"lat": 24.70, "lng": 46.67},
			{"lat": 24.72, "lng": 46.67},
			{"lat": 24.72, "lng": 46.69},
			{"lat": 24.70, "lng": 46.69},
		},
	}, nil); status != http.StatusCreated {
		t.Fatalf("create geofence: status %d", status)
	}

	// Store the footprint the same way the service does.
	overlapping := `{"type":"Polygon","coordinates":[[[46.685,24.705],[46.685,24.715],[46.695,24.715],[46.695,24.705],[46.685,24.705]]]}`
	disjoint := `{"type":"Polygon","coordinates":[[[10.0,10.0],[10.0,10.1],[10.1,10.1],[10.1,10.0],[10.0,10.0]]]}`

	cases := []struct {
		name      string
		geojson   string
		wantRows  int
		wantAreaM bool
	}{
		{name: "partially overlapping", geojson: overlapping, wantRows: 1, wantAreaM: true},
		{name: "far away", geojson: disjoint, wantRows: 0, wantAreaM: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var count int
			if err := h.pool.QueryRow(ctx, `
				SELECT count(*) FROM geofences
				WHERE active = true
				  AND ST_Intersects(geometry, ST_GeomFromGeoJSON($1))`, tc.geojson).Scan(&count); err != nil {
				t.Fatalf("intersects query: %v", err)
			}
			if count != tc.wantRows {
				t.Fatalf("intersecting zones = %d, want %d", count, tc.wantRows)
			}

			if !tc.wantAreaM {
				return
			}
			// The measured overlap must be positive and no larger than the zone,
			// which proves the footprint partially covers it rather than sitting
			// beside it. The footprint is parsed with ST_GeomFromGeoJSON because
			// a text -> geography cast uses the WKT parser, which rejects
			// GeoJSON - the exact mistake this test exists to catch.
			var overlapM2, zoneM2 float64
			if err := h.pool.QueryRow(ctx, `
				SELECT ST_Area(ST_Intersection(
				         geometry::geography,
				         ST_GeomFromGeoJSON($1::text)::geography
				       ))::float8,
				       ST_Area(geometry::geography)::float8
				FROM geofences WHERE id = 'sql-zone'`, tc.geojson).Scan(&overlapM2, &zoneM2); err != nil {
				t.Fatalf("overlap area: %v", err)
			}
			if overlapM2 <= 0 {
				t.Errorf("overlap area = %v, want a positive measurement", overlapM2)
			}
			if overlapM2 > zoneM2 {
				t.Errorf("overlap %v exceeds the zone area %v", overlapM2, zoneM2)
			}
			if overlapM2 == zoneM2 {
				t.Errorf("overlap equals the whole zone; the fixture should straddle an edge")
			}
		})
	}
}

// TestAreaObservationEventCarriesTheFootprint checks the handoff between the
// observation service and the area rule. If the footprint did not survive onto
// the event, the rule would silently evaluate nothing and no alert would fire.
func TestAreaObservationEventCarriesTheFootprint(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createSource("firms")

	if status := h.do(http.MethodPost, "/api/v1/geofences", map[string]any{
		"id": "carry-zone", "name": "Carry Zone", "type": "restricted", "severity": "high",
		"polygon": []map[string]float64{
			{"lat": 24.70, "lng": 46.67},
			{"lat": 24.72, "lng": 46.67},
			{"lat": 24.72, "lng": 46.69},
			{"lat": 24.70, "lng": 46.69},
		},
	}, nil); status != http.StatusCreated {
		t.Fatalf("create geofence: status %d", status)
	}

	if status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId":   "firms",
		"type":       "hazard.fire",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"area": map[string]any{
			"kind": "hazard",
			"polygon": []map[string]float64{
				{"lat": 24.705, "lng": 46.685},
				{"lat": 24.715, "lng": 46.685},
				{"lat": 24.715, "lng": 46.695},
				{"lat": 24.705, "lng": 46.695},
			},
		},
	}, nil); status != http.StatusCreated {
		t.Fatalf("ingest area observation: status %d", status)
	}

	// The stored geometry must be a real polygon PostGIS accepts, and must
	// actually overlap the zone. This is the state the rule reads.
	var geojson string
	var overlaps int
	if err := h.pool.QueryRow(ctx, `
		SELECT ST_AsGeoJSON(o.area::geometry)::text,
		       (SELECT count(*) FROM geofences g
		         WHERE g.active = true AND ST_Intersects(g.geometry, o.area::geometry))
		FROM observations o WHERE o.area IS NOT NULL LIMIT 1`).Scan(&geojson, &overlaps); err != nil {
		t.Fatalf("read stored area: %v", err)
	}
	t.Logf("stored area geometry: %s", geojson)
	if overlaps != 1 {
		t.Fatalf("stored footprint intersects %d zones, want 1", overlaps)
	}
	if geojson == "" {
		t.Error("stored area has no geometry")
	}

	// And the area must be retrievable through the normal observation read path,
	// since that is how an operator inspects the evidence behind the alert.
	var listed struct {
		Items []struct {
			ID   string `json:"id"`
			Area *struct {
				Kind    string  `json:"kind"`
				GeoJSON string  `json:"geojson"`
				AreaM2  float64 `json:"areaM2"`
				Center  struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"center"`
			} `json:"area"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if status := h.do(http.MethodGet, "/api/v1/observations?limit=50", nil, &listed); status != http.StatusOK {
		t.Fatalf("list observations: status %d", status)
	}
	found := false
	for _, item := range listed.Items {
		if item.Area == nil {
			continue
		}
		found = true
		if item.Area.Kind != "hazard" {
			t.Errorf("area kind = %q, want hazard", item.Area.Kind)
		}
		if item.Area.AreaM2 <= 0 {
			t.Errorf("areaM2 = %v, want positive", item.Area.AreaM2)
		}
		if item.Area.Center.Lat == 0 && item.Area.Center.Lng == 0 {
			t.Error("area center is the null island")
		}
	}
	if !found {
		t.Fatal("area observation does not report its area on the read path")
	}
}
