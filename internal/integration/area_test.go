package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// An area observation is the answer to a real modelling problem: weather, fire
// and earthquake data describe *areas* at an instant, and forcing them through
// the track pipeline produces either a nonsense track or an alert that never
// fires. These tests drive the whole path against real PostGIS and assert the
// two properties that make it correct: a footprint raises an alert on overlap,
// and it never becomes a track.
func TestAreaObservationRaisesAlertOnOverlapWithoutCreatingATrack(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createSource("firms")

	// A restricted zone, and a hazard footprint that overlaps it partially.
	// Partial matters: ST_Contains would silently drop a fire burning across a
	// boundary, which is the most operationally interesting case there is.
	if status := h.do(http.MethodPost, "/api/v1/geofences", map[string]any{
		"id": "area-zone", "name": "Restricted Area A", "type": "restricted", "severity": "high",
		"polygon": []map[string]float64{
			{"lat": 24.70, "lng": 46.67},
			{"lat": 24.72, "lng": 46.67},
			{"lat": 24.72, "lng": 46.69},
			{"lat": 24.70, "lng": 46.69},
		},
	}, nil); status != http.StatusCreated {
		t.Fatalf("create geofence: status %d", status)
	}

	// The footprint straddles the zone's eastern edge.
	var created map[string]any
	status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId":   "firms",
		"type":       "hazard.fire",
		"observedAt": time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano),
		"area": map[string]any{
			"kind": "hazard",
			"polygon": []map[string]float64{
				{"lat": 24.705, "lng": 46.685},
				{"lat": 24.715, "lng": 46.685},
				{"lat": 24.715, "lng": 46.695},
				{"lat": 24.705, "lng": 46.695},
			},
		},
		"payload": map[string]any{"brightness": 312.4, "confidence": "high"},
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("ingest area observation: status %d (%v)", status, created)
	}
	observationID, _ := created["id"].(string)
	if observationID == "" {
		t.Fatalf("area observation has no id: %v", created)
	}

	// The footprint survives the round trip with its geometry intact.
	area, ok := created["area"].(map[string]any)
	if !ok {
		t.Fatalf("area observation does not report an area: %v", created)
	}
	if area["kind"] != "hazard" {
		t.Errorf("area kind = %v, want hazard", area["kind"])
	}
	if area["geojson"] == "" || area["geojson"] == nil {
		t.Errorf("area geojson is empty: %v", area)
	}
	if size, _ := area["areaM2"].(float64); size <= 0 {
		t.Errorf("areaM2 = %v, want a positive footprint", area["areaM2"])
	}

	// No track. An area has no trajectory, and inventing one would produce a
	// velocity for a wildfire that has no physical meaning.
	var trackCount int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM tracks`).Scan(&trackCount); err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 0 {
		t.Errorf("an area observation produced %d tracks, want none", trackCount)
	}
	var evidenceLinks int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM track_observations`).Scan(&evidenceLinks); err != nil {
		t.Fatalf("count track evidence: %v", err)
	}
	if evidenceLinks != 0 {
		t.Errorf("an area observation attached %d evidence links to a track, want none", evidenceLinks)
	}

	// An alert, because the footprint overlaps the restricted zone.
	var alerts struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	waitFor(t, 15*time.Second, 100*time.Millisecond, "alert from area overlap", func() bool {
		h.do(http.MethodGet, "/api/v1/alerts?state=ACTIVE", nil, &alerts)
		return alerts.Total >= 1
	})
	alert := alerts.Items[0]
	if alert["type"] != "geofence.area.overlap" {
		t.Fatalf("alert type = %v, want geofence.area.overlap", alert["type"])
	}
	if alert["geofenceId"] != "area-zone" {
		t.Errorf("alert geofence = %v, want area-zone", alert["geofenceId"])
	}
	if track, ok := alert["trackId"].(string); ok && track != "" {
		t.Errorf("area alert carries track id %q; an area has no track", track)
	}

	// The evidence trail runs from the alert to the observation that reported it.
	reference, _ := alert["sourceReference"].(map[string]any)
	if reference["observationId"] != observationID {
		t.Errorf("sourceReference observationId = %v, want %s", reference["observationId"], observationID)
	}
	if reference["sourceId"] != "firms" {
		t.Errorf("sourceReference sourceId = %v, want firms", reference["sourceId"])
	}
	// PostGIS measured the overlap; the alert reports what it measured rather
	// than asserting "it overlaps" without a number.
	if overlap, _ := reference["overlapM2"].(float64); overlap <= 0 {
		t.Errorf("overlapM2 = %v, want the measured overlap", reference["overlapM2"])
	}
	if alert["title"] == "" || alert["title"] == nil {
		t.Error("area alert has no title")
	}
}

func TestAreaObservationAwayFromAnyZoneRaisesNothing(t *testing.T) {
	h := newHarness(t)
	h.createSource("firms")

	if status := h.do(http.MethodPost, "/api/v1/geofences", map[string]any{
		"id": "far-zone", "name": "Far Away", "type": "restricted", "severity": "high",
		"polygon": []map[string]float64{
			{"lat": 10.0, "lng": 10.0},
			{"lat": 10.1, "lng": 10.0},
			{"lat": 10.1, "lng": 10.1},
			{"lat": 10.0, "lng": 10.1},
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
				{"lat": 24.70, "lng": 46.67},
				{"lat": 24.71, "lng": 46.67},
				{"lat": 24.71, "lng": 46.68},
				{"lat": 24.70, "lng": 46.68},
			},
		},
	}, nil); status != http.StatusCreated {
		t.Fatalf("ingest area observation: status %d", status)
	}

	// Give the rule a moment, then confirm it stayed quiet. Silence is the
	// correct outcome: an area nowhere near a zone is not an alert.
	time.Sleep(2 * time.Second)
	var alerts struct {
		Total int `json:"total"`
	}
	h.do(http.MethodGet, "/api/v1/alerts", nil, &alerts)
	if alerts.Total != 0 {
		t.Errorf("an area overlapping nothing produced %d alerts", alerts.Total)
	}
}

func TestAreaObservationContradictionsAreRejected(t *testing.T) {
	h := newHarness(t)
	h.createSource("firms")

	polygon := []map[string]float64{
		{"lat": 24.70, "lng": 46.67},
		{"lat": 24.71, "lng": 46.67},
		{"lat": 24.71, "lng": 46.68},
	}

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			// An area is never correlated, so a track hint contradicts it.
			name: "area with a trackHint",
			body: map[string]any{
				"sourceId": "firms", "type": "hazard.fire",
				"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
				"area":       map[string]any{"kind": "hazard", "polygon": polygon},
				"trackHint":  "track-unknown-01",
			},
		},
		{
			// The footprint has extent; a separate point is a second, possibly
			// contradictory, answer to where it is.
			name: "area with a position",
			body: map[string]any{
				"sourceId": "firms", "type": "hazard.fire",
				"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
				"area":       map[string]any{"kind": "hazard", "polygon": polygon},
				"position":   map[string]float64{"lat": 24.705, "lng": 46.675},
			},
		},
		{
			name: "area with an unknown kind",
			body: map[string]any{
				"sourceId": "firms", "type": "hazard.fire",
				"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
				"area":       map[string]any{"kind": "meteorological", "polygon": polygon},
			},
		},
		{
			name: "area with too few vertices",
			body: map[string]any{
				"sourceId": "firms", "type": "hazard.fire",
				"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
				"area": map[string]any{"kind": "hazard", "polygon": []map[string]float64{
					{"lat": 24.70, "lng": 46.67}, {"lat": 24.71, "lng": 46.68},
				}},
			},
		},
		{
			name: "area with an out-of-range coordinate",
			body: map[string]any{
				"sourceId": "firms", "type": "hazard.fire",
				"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
				"area": map[string]any{"kind": "hazard", "polygon": []map[string]float64{
					{"lat": 91.0, "lng": 46.67}, {"lat": 24.71, "lng": 46.67}, {"lat": 24.71, "lng": 46.68},
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := h.do(http.MethodPost, "/api/v1/observations", tc.body, nil); status != http.StatusBadRequest &&
				status != http.StatusUnprocessableEntity {
				t.Errorf("POST /api/v1/observations for %s = %d, want a rejection", tc.name, status)
			}
		})
	}

	// Nothing contradictory may have reached the evidence layer.
	var stored int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM observations WHERE area IS NOT NULL`).Scan(&stored); err != nil {
		t.Fatalf("count area observations: %v", err)
	}
	if stored != 0 {
		t.Errorf("%d malformed area observations were persisted, want 0", stored)
	}
}

func TestPointObservationsStillCorrelateWithTheAreaModelInPlace(t *testing.T) {
	h := newHarness(t)
	// ingestObservation posts against this shared synthetic source.
	h.createSource("simulator-test")
	// The area model must be additive. A point observation has to keep creating
	// tracks exactly as before, or the existing C2 pipeline has been broken to
	// accommodate event data.
	h.ingestObservation("still-a-track", 24.712, 46.676, time.Now().UTC())

	var tracks struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/tracks", nil, &tracks)
	if len(tracks.Items) != 1 {
		t.Fatalf("point observation produced %d tracks, want 1", len(tracks.Items))
	}

	var areaCount int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM observations WHERE area IS NOT NULL`).Scan(&areaCount); err != nil {
		t.Fatalf("count area observations: %v", err)
	}
	if areaCount != 0 {
		t.Errorf("a point observation was stored as an area")
	}
}
