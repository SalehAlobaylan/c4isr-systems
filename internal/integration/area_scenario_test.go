package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// The area scenario is the demonstration of the event/area path: it must run
// deterministically, raise alerts from footprints, and produce no tracks at
// all. A scenario that only loads but never fires would leave the feature
// unproven.
// The map fetches areas with a dedicated filter rather than paging the general
// observation list. That distinction has to hold server-side: on a live system
// the newest N observations are almost entirely radar pings, so a client-side
// filter would find no hazards at all.
func TestAreaFilterReturnsOnlyFootprints(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createSource("firms")

	// A hazard footprint.
	if status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId": "firms", "type": "hazard.fire",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"area": map[string]any{"kind": "hazard", "polygon": []map[string]float64{
			{"lat": 24.70, "lng": 46.67}, {"lat": 24.71, "lng": 46.67}, {"lat": 24.71, "lng": 46.68},
		}},
	}, nil); status != http.StatusCreated {
		t.Fatalf("ingest hazard: status %d", status)
	}
	// A weather cell.
	if status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId": "firms", "type": "weather.cell",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"area": map[string]any{"kind": "weather", "polygon": []map[string]float64{
			{"lat": 24.80, "lng": 46.77}, {"lat": 24.81, "lng": 46.77}, {"lat": 24.81, "lng": 46.78},
		}},
	}, nil); status != http.StatusCreated {
		t.Fatalf("ingest weather: status %d", status)
	}
	// Point observations, which the filter must exclude.
	h.createSource("simulator-test")
	for i := 0; i < 3; i++ {
		h.ingestObservation("point", 24.712, 46.676, time.Now().UTC().Add(time.Duration(i)*time.Millisecond))
	}

	var areas struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if status := h.do(http.MethodGet, "/api/v1/observations?area=true", nil, &areas); status != http.StatusOK {
		t.Fatalf("list areas: status %d", status)
	}
	if areas.Total != 2 {
		t.Fatalf("area filter returned %d observations, want 2", areas.Total)
	}
	for _, item := range areas.Items {
		if _, ok := item["area"].(map[string]any); !ok {
			t.Errorf("area filter returned an observation with no area: %v", item["id"])
		}
	}

	// Narrowing by kind must select exactly that class.
	var hazards struct {
		Total int              `json:"total"`
		Items []map[string]any `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/observations?area=true&area_kind=hazard", nil, &hazards); status != http.StatusOK {
		t.Fatalf("list hazards: status %d", status)
	}
	if hazards.Total != 1 {
		t.Fatalf("hazard filter returned %d, want 1", hazards.Total)
	}
	if kind := hazards.Items[0]["area"].(map[string]any)["kind"]; kind != "hazard" {
		t.Errorf("kind = %v, want hazard", kind)
	}

	// An unrecognised kind is a client error, not an empty result. An empty
	// list would read as "no hazards here", which is the one reading that must
	// never be mistaken for a broken filter.
	var bad map[string]any
	if status := h.do(http.MethodGet, "/api/v1/observations?area=true&area_kind=meteorological", nil, &bad); status != http.StatusBadRequest &&
		status != http.StatusUnprocessableEntity {
		t.Errorf("unknown area_kind = %d, want a rejection rather than an empty list", status)
	}

	// The general list still contains the point observations, proving the
	// filter narrows rather than replaces.
	var all struct {
		Total int `json:"total"`
	}
	h.do(http.MethodGet, "/api/v1/observations?limit=1", nil, &all)
	if all.Total != 5 {
		t.Errorf("unfiltered list returned %d observations, want all 5", all.Total)
	}

	var stored int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM observations`).Scan(&stored); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if stored != 5 {
		t.Errorf("stored %d observations, want 5", stored)
	}
}

func TestAreaScenarioRunsAndRaisesFootprintAlerts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	run := h.startScenario(t, "area-hazard-intrusion")
	runID := run["id"].(string)

	var completed map[string]any
	waitFor(t, 90*time.Second, 250*time.Millisecond, "area scenario completion", func() bool {
		h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &completed)
		return completed["status"] == "COMPLETED" || completed["status"] == "FAILED"
	})
	if completed["status"] != "COMPLETED" {
		t.Fatalf("area scenario ended %v: %v", completed["status"], completed["error"])
	}

	// The defining property: an area scenario must not create tracks. Every
	// entity in it is a footprint, and footprints have no trajectory.
	var tracks int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM tracks`).Scan(&tracks); err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if tracks != 0 {
		t.Fatalf("area scenario produced %d tracks, want none", tracks)
	}

	// The repeated fire reports are distinct observations of one footprint,
	// which is what a public hazard feed actually does.
	var areas int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM observations WHERE area IS NOT NULL`).Scan(&areas); err != nil {
		t.Fatalf("count area observations: %v", err)
	}
	if areas == 0 {
		t.Fatal("area scenario produced no area observations")
	}

	// Alerts come from footprints overlapping zones, and each links to the
	// observation that reported it.
	var alerts struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	h.do(http.MethodGet, "/api/v1/alerts", nil, &alerts)
	if alerts.Total == 0 {
		t.Fatal("area scenario produced no alerts")
	}

	areaAlerts := 0
	for _, alert := range alerts.Items {
		if alert["type"] != "geofence.area.overlap" {
			t.Errorf("alert type = %v, want geofence.area.overlap", alert["type"])
			continue
		}
		areaAlerts++
		reference, _ := alert["sourceReference"].(map[string]any)
		observationID, _ := reference["observationId"].(string)
		if observationID == "" {
			t.Errorf("area alert has no observation reference: %v", alert)
			continue
		}
		// The evidence link must resolve: an alert pointing at an observation
		// that does not exist is an unusable trail.
		var evidence int
		if err := h.pool.QueryRow(ctx,
			`SELECT count(*) FROM observations WHERE id = $1 AND area IS NOT NULL`, observationID).Scan(&evidence); err != nil {
			t.Fatalf("resolve alert evidence: %v", err)
		}
		if evidence != 1 {
			t.Errorf("area alert references observation %q which is not a stored area", observationID)
		}
	}
	if areaAlerts == 0 {
		t.Fatal("no area overlap alerts were raised")
	}

	// An area that overlaps nothing - the weather cell sitting north-east of
	// the airfield - must not have raised anything. Alert fatigue is a real
	// failure mode, not a theoretical one.
	var weatherAlerts int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM alerts WHERE source_reference->>'areaKind' = 'weather'`).Scan(&weatherAlerts); err != nil {
		t.Fatalf("count weather alerts: %v", err)
	}
	if weatherAlerts != 0 {
		t.Errorf("a weather cell overlapping nothing raised %d alerts", weatherAlerts)
	}
}

// The area scenario must replay identically, like every other scenario. The
// determinism guarantee has to cover hazard data too.
func TestAreaScenarioReplaysIdentically(t *testing.T) {
	h := newHarness(t)

	run := h.startScenario(t, "area-hazard-intrusion")
	runID := run["id"].(string)
	waitFor(t, 90*time.Second, 250*time.Millisecond, "first area scenario run", func() bool {
		var run map[string]any
		h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &run)
		return run["status"] == "COMPLETED" || run["status"] == "FAILED"
	})
	first := areaGeometriesForRun(t, h, runID)
	if len(first) == 0 {
		t.Fatal("first run produced no area geometries")
	}

	second := h.startScenario(t, "area-hazard-intrusion")
	secondID := second["id"].(string)
	waitFor(t, 90*time.Second, 250*time.Millisecond, "replayed area scenario run", func() bool {
		var run map[string]any
		h.do(http.MethodGet, "/api/v1/scenarios/runs/"+secondID, nil, &run)
		return run["status"] == "COMPLETED" || run["status"] == "FAILED"
	})
	replay := areaGeometriesForRun(t, h, secondID)

	if len(first) != len(replay) {
		t.Fatalf("replay produced %d areas, first run %d", len(replay), len(first))
	}
	// Ordered by the engine's sequence suffix, which is derived from the seeded
	// RNG rather than wall clock - the same ordering discipline the track
	// determinism test uses.
	for i := range first {
		if first[i] != replay[i] {
			t.Fatalf("replay diverged at area %d: %s vs %s", i, first[i], replay[i])
		}
	}
}

// areaGeometriesForRun returns the run's footprints in emission order.
func areaGeometriesForRun(t *testing.T, h *harness, runID string) []string {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `
		SELECT ST_AsGeoJSON(area::geometry)::text
		FROM observations
		WHERE payload->>'scenarioRunId' = $1 AND area IS NOT NULL
		ORDER BY (regexp_match(id, '__(\d+)$'))[1]::bigint ASC, id ASC`, runID)
	if err != nil {
		t.Fatalf("query run areas: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var geometry string
		if err := rows.Scan(&geometry); err != nil {
			t.Fatalf("scan area geometry: %v", err)
		}
		out = append(out, geometry)
	}
	return out
}
