package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestFirstMilestoneAcceptance drives the complete C2 milestone:
//
//	Source -> Observation -> Track -> Operational Picture -> Geofence Breach
//	-> Alert -> Operator -> Incident -> Asset -> Mission -> Command -> Audit
//
// It then replays the scenario with the same seed and asserts the emitted
// observations are identical, which is the deterministic-replay guarantee.
func TestFirstMilestoneAcceptance(t *testing.T) {
	h := newHarness(t)

	// Realtime client must be connected before the run starts.
	wsCtx, cancelWS := context.WithCancel(context.Background())
	defer cancelWS()
	wsURL := "ws" + strings.TrimPrefix(h.server.URL, "http") + "/api/v1/realtime?access_token=" + integrationToken
	conn, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial realtime: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	eventTypes := make(chan string, 512)
	go func() {
		for {
			_, data, err := conn.Read(wsCtx)
			if err != nil {
				close(eventTypes)
				return
			}
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &envelope); err == nil {
				eventTypes <- envelope.Type
			}
		}
	}()

	// 1. Start the deterministic scenario at 50x so the run finishes quickly.
	var run map[string]any
	if status := h.do(http.MethodPost, "/api/v1/scenarios/restricted-area-intrusion/start",
		map[string]any{"speed": 50}, &run); status != http.StatusCreated {
		t.Fatalf("start scenario: status %d (%v)", status, run)
	}
	runID := run["id"].(string)
	resourceNamespace := run["resourceNamespace"].(string)
	sourceID := resourceNamespace + "source__simulator-01"
	assetID := resourceNamespace + "asset__patrol-01"
	geofenceID := resourceNamespace + "geofence__restricted-zone-a"
	trackReference := resourceNamespace + "track__unknown-01"

	// 2. The scenario registers its source and asset through normal services.
	var sources struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/sources", nil, &sources)
	if !containsSource(sources.Items, sourceID) {
		t.Fatal("scenario source was not registered")
	}
	var assets struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/assets", nil, &assets)
	if !containsID(assets.Items, assetID) {
		t.Fatal("scenario asset was not registered")
	}

	// 3. Wait for PostGIS-driven geofence breach to raise the alert.
	var alerts struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	waitFor(t, 60*time.Second, 250*time.Millisecond, "alert from geofence breach", func() bool {
		h.do(http.MethodGet, "/api/v1/alerts?state=ACTIVE", nil, &alerts)
		return alerts.Total >= 1
	})
	alert := alerts.Items[0]
	alertID := alert["id"].(string)
	if alert["severity"] != "high" {
		t.Fatalf("alert severity not derived from geofence: %v", alert["severity"])
	}

	// The alert must read as prose an operator can act on. The scenario
	// classifies the track as a vehicle at 0.72 confidence, which is below the
	// threshold for stating it plainly, so the wording is hedged rather than
	// asserted. An internal identifier must never reach operator-facing text.
	title, _ := alert["title"].(string)
	message, _ := alert["message"].(string)
	for _, field := range []struct{ name, value string }{{"title", title}, {"message", message}} {
		if strings.Contains(field.value, "trk_") {
			t.Errorf("alert %s leaks a track identifier: %q", field.name, field.value)
		}
	}
	if title != "Suspected vehicle entered Restricted Zone A" {
		t.Errorf("alert title = %q, want the hedged subject name", title)
	}
	if !strings.Contains(message, "Restricted Zone A") {
		t.Errorf("alert message should name the geofence: %q", message)
	}

	// 4. Track is already derived and independently inspectable.
	var tracks struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/tracks", nil, &tracks)
	track := findTrack(tracks.Items, trackReference)
	if track == nil {
		t.Fatal("track unknown-01 was not derived from observations")
	}
	trackID := track["id"].(string)

	// 5. Operator acknowledges the alert.
	var acknowledged map[string]any
	if status := h.do(http.MethodPost, "/api/v1/alerts/"+alertID+"/acknowledge", nil, &acknowledged); status != http.StatusOK {
		t.Fatalf("acknowledge alert: %d", status)
	}
	if acknowledged["state"] != "ACKNOWLEDGED" {
		t.Fatalf("alert state after acknowledge: %v", acknowledged["state"])
	}

	// 6. Operator opens an incident linked to the alert and track.
	var incident map[string]any
	status := h.do(http.MethodPost, "/api/v1/incidents", map[string]any{
		"title":    "Unknown vehicle in Restricted Zone A",
		"priority": "high",
		"alertIds": []string{alertID},
		"trackIds": []string{trackID},
	}, &incident)
	if status != http.StatusCreated {
		t.Fatalf("create incident: %d (%v)", status, incident)
	}
	incidentID := incident["id"].(string)
	var linkedAlert map[string]any
	if status := h.do(http.MethodGet, "/api/v1/alerts/"+alertID, nil, &linkedAlert); status != http.StatusOK {
		t.Fatalf("get linked alert: %d", status)
	}
	if linkedAlert["incidentId"] != incidentID {
		t.Fatalf("alert incident link = %v, want %s", linkedAlert["incidentId"], incidentID)
	}

	// 7. Operator attaches the patrol asset to the incident.
	if status := h.do(http.MethodPost, "/api/v1/incidents/"+incidentID+"/relations",
		map[string]any{"kind": "asset", "id": assetID}, nil); status != http.StatusOK {
		t.Fatalf("attach asset to incident: %d", status)
	}

	// 8. Operator creates a mission and assigns the patrol.
	var mission map[string]any
	status = h.do(http.MethodPost, "/api/v1/missions", map[string]any{
		"name":       "Respond to Restricted Zone A",
		"objective":  "Intercept the unknown vehicle",
		"priority":   "high",
		"incidentId": incidentID,
		"assets":     []string{assetID},
	}, &mission)
	if status != http.StatusCreated {
		t.Fatalf("create mission: %d (%v)", status, mission)
	}
	missionID := mission["id"].(string)
	missionAssets, ok := mission["assets"].([]any)
	if !ok || len(missionAssets) != 1 || missionAssets[0].(map[string]any)["id"] != assetID {
		t.Fatalf("mission response lost assigned asset: %v", mission["assets"])
	}

	// 9. Operator issues a command toward the asset.
	var command map[string]any
	status = h.do(http.MethodPost, "/api/v1/commands", map[string]any{
		"assetId":    assetID,
		"missionId":  missionID,
		"incidentId": incidentID,
		"type":       "move_to",
		"payload":    map[string]any{"lat": 24.7160, "lng": 46.6790},
	}, &command)
	if status != http.StatusCreated {
		t.Fatalf("issue command: %d (%v)", status, command)
	}
	commandID := command["id"].(string)
	if command["state"] != "SENT" {
		t.Fatalf("issued command should be SENT, got %v", command["state"])
	}

	// 10. Wait for the scenario to complete, including command simulation.
	waitFor(t, 60*time.Second, 250*time.Millisecond, "scenario completion", func() bool {
		var completed map[string]any
		h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &completed)
		return completed["status"] == "COMPLETED"
	})

	// 10b. Evidence remains attached and accessible after the run finished.
	var observations struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	h.do(http.MethodGet, "/api/v1/observations?track_id="+trackID+"&limit=1000", nil, &observations)
	if observations.Total < 40 {
		t.Fatalf("expected the full observation history as track evidence, got %d", observations.Total)
	}
	positions := positionsForRun(t, h, runID)
	if len(positions) < 40 {
		t.Fatalf("expected at least 40 run observations, got %d", len(positions))
	}

	// PostGIS confirms observations actually occurred inside the zone and the
	// track exited again by the end of the run.
	var insideCount int
	if err := h.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM observations o
		WHERE o.payload->>'scenarioRunId' = $1
		  AND EXISTS (
			SELECT 1 FROM geofences g
			WHERE g.id = $2
			  AND ST_Contains(g.geometry, o.position::geometry)
		)`, runID, geofenceID).Scan(&insideCount); err != nil {
		t.Fatalf("count observations inside geofence: %v", err)
	}
	if insideCount == 0 {
		t.Fatal("no observation occurred inside the restricted zone")
	}
	var inside bool
	if err := h.pool.QueryRow(context.Background(), `
		SELECT inside FROM geofence_states
		WHERE geofence_id = $1 AND track_id = $2`, geofenceID, trackID).Scan(&inside); err != nil {
		t.Fatalf("geofence state missing: %v", err)
	}
	if inside {
		t.Fatal("track should have exited the zone by the end of the scenario")
	}

	// 11. The scenario acknowledged and completed the operator command.
	var finalCommand map[string]any
	h.do(http.MethodGet, "/api/v1/commands/"+commandID, nil, &finalCommand)
	if finalCommand["state"] != "COMPLETED" {
		t.Fatalf("command did not complete through scenario simulation: %v", finalCommand["state"])
	}
	if finalCommand["acknowledgedAt"] == nil || finalCommand["completedAt"] == nil {
		t.Fatalf("command lifecycle timestamps missing: %v", finalCommand)
	}

	// 12. Incident transitions follow the explicit state machine.
	for _, state := range []string{"ACKNOWLEDGED", "INVESTIGATING", "RESPONDING", "RESOLVED"} {
		if status := h.do(http.MethodPost, "/api/v1/incidents/"+incidentID+"/status",
			map[string]any{"status": state}, nil); status != http.StatusOK {
			t.Fatalf("incident transition to %s: %d", state, status)
		}
	}

	// 13. Audit reconstructs the entire sequence.
	var audit struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/audit?limit=1000", nil, &audit)
	actions := make([]string, 0, len(audit.Items))
	for i := len(audit.Items) - 1; i >= 0; i-- {
		actions = append(actions, audit.Items[i]["action"].(string))
	}
	for _, expected := range []string{
		"source.created",
		"observation.received",
		"track.created",
		"geofence.breached",
		"alert.created",
		"alert.acknowledged",
		"incident.created",
		"mission.created",
		"command.issued",
		"command.status.changed",
	} {
		if !containsString(actions, expected) {
			t.Fatalf("audit missing %q; sequence: %v", expected, actions)
		}
	}
	if !isOrderedSubsequence(actions, []string{"observation.received", "track.created", "geofence.breached", "alert.created"}) {
		t.Fatal("audit does not preserve the evidence-to-alert ordering")
	}

	// 14. Replay the same scenario with the same seed: observations identical.
	var secondRun map[string]any
	if status := h.do(http.MethodPost, "/api/v1/scenarios/restricted-area-intrusion/start",
		map[string]any{"speed": 50}, &secondRun); status != http.StatusCreated {
		t.Fatalf("start scenario replay: %d", status)
	}
	secondRunID := secondRun["id"].(string)
	secondNamespace := secondRun["resourceNamespace"].(string)
	if secondNamespace == resourceNamespace {
		t.Fatalf("replay reused resource namespace %q", resourceNamespace)
	}
	waitFor(t, 60*time.Second, 250*time.Millisecond, "scenario replay completion", func() bool {
		var completed map[string]any
		h.do(http.MethodGet, "/api/v1/scenarios/runs/"+secondRunID, nil, &completed)
		return completed["status"] == "COMPLETED"
	})

	second := positionsForRun(t, h, secondRunID)
	assertPositionsIdentical(t, positions, second)
	assertScenarioResourcesAreIsolated(t, h, runID, resourceNamespace)
	assertScenarioResourcesAreIsolated(t, h, secondRunID, secondNamespace)

	// Realtime delivered the pipeline to connected clients.
	deadline := time.Now().Add(5 * time.Second)
	seen := map[string]bool{}
	for time.Now().Before(deadline) {
		select {
		case eventType, ok := <-eventTypes:
			if !ok {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			seen[eventType] = true
			if seen["alert.created"] && seen["track.updated"] && seen["geofence.breached"] {
				deadline = time.Now()
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	for _, expected := range []string{"observation.received", "track.updated", "geofence.breached", "alert.created"} {
		if !seen[expected] {
			t.Fatalf("realtime client never received %q", expected)
		}
	}
}

type latLng struct {
	lat float64
	lng float64
}

// positionsDiffer compares two runs and reports the first divergence.
//
// It returns the index and whether one exists, rather than failing directly,
// so that the determinism claim has one definition which both the acceptance
// test and the meta-test below can exercise. A determinism test that cannot
// fail is worse than no test at all: it reports confidence it has not earned.
func positionsDiffer(first, second []latLng) (int, bool) {
	if len(first) != len(second) {
		return 0, true
	}
	for i := range first {
		if first[i].lat != second[i].lat || first[i].lng != second[i].lng {
			return i, true
		}
	}
	return 0, false
}

// assertPositionsIdentical fails the test when a replay did not reproduce the
// first run exactly.
func assertPositionsIdentical(t *testing.T, first, second []latLng) {
	t.Helper()
	if index, differs := positionsDiffer(first, second); differs {
		if len(first) != len(second) {
			t.Fatalf("replay emitted %d observations, first run %d", len(second), len(first))
		}
		t.Fatalf("replay diverged at observation %d: %v vs %v", index, first[index], second[index])
	}
}

// positionsForRun returns a run's observed positions in emission order.
//
// The ordering key is the sequence number embedded in the observation id, not
// created_at. Scenario observations are named "<run>__observation__<ref>__<seq>"
// by the engine, and that sequence is what determinism actually means: it is
// derived from the seeded engine state and is identical across replays.
//
// created_at cannot carry that guarantee. It is wall-clock now() at insert, so
// two runs of the same scenario at the same playback speed can interleave the
// same logical observations in different orders. Comparing the two results
// index-by-index then compares *different* observations and reports a
// divergence that never happened - which is exactly how this test flaked
// intermittently at a moving index.
//
// The sequence is cast to bigint because ordering the id as text would sort
// "...__10" before "...__2".
func positionsForRun(t *testing.T, h *harness, runID string) []latLng {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `
		SELECT ST_Y(position::geometry), ST_X(position::geometry)
		FROM observations
		WHERE payload->>'scenarioRunId' = $1 AND position IS NOT NULL
		ORDER BY (regexp_match(id, '__(\d+)$'))[1]::bigint ASC, id ASC`, runID)
	if err != nil {
		t.Fatalf("query run observations: %v", err)
	}
	defer rows.Close()

	var out []latLng
	for rows.Next() {
		var p latLng
		if err := rows.Scan(&p.lat, &p.lng); err != nil {
			t.Fatalf("scan position: %v", err)
		}
		out = append(out, p)
	}
	return out
}

// TestOrderingIgnoresWallClock proves the determinism check still has teeth.
//
// The flake was fixed by ordering on the engine's sequence number instead of
// wall-clock created_at. That change could plausibly have broken the check
// entirely - e.g. by silently dropping the comparison, or by ordering both
// runs so coarsely that no difference could ever surface. This test injects
// positions that differ the way a real regression would, and requires the
// comparison to catch it.
//
// It deliberately needs no database: the property under test is that
// assertPositionsIdentical rejects divergence, which is exactly what the
// ordering fix had to preserve.
func TestOrderingIgnoresWallClock(t *testing.T) {
	t.Run("identical runs are accepted", func(t *testing.T) {
		run := []latLng{{lat: 24.7, lng: 46.6}, {lat: 24.8, lng: 46.7}, {lat: 24.9, lng: 46.8}}
		assertPositionsIdentical(t, run, append([]latLng(nil), run...))
	})

	t.Run("a shifted position is detected", func(t *testing.T) {
		first := []latLng{{lat: 24.7, lng: 46.6}, {lat: 24.8, lng: 46.7}}
		second := []latLng{{lat: 24.7, lng: 46.6}, {lat: 24.85, lng: 46.7}}
		assertDetectsDivergence(t, first, second)
	})

	t.Run("a reordered run is detected", func(t *testing.T) {
		// This is the failure mode the ordering fix exists to expose: a run
		// that emits the same positions in a different order is not a
		// replay, and must not be reported as one.
		first := []latLng{{lat: 24.7, lng: 46.6}, {lat: 24.8, lng: 46.7}}
		second := []latLng{{lat: 24.8, lng: 46.7}, {lat: 24.7, lng: 46.6}}
		assertDetectsDivergence(t, first, second)
	})

	t.Run("a different observation count is detected", func(t *testing.T) {
		first := []latLng{{lat: 24.7, lng: 46.6}, {lat: 24.8, lng: 46.7}}
		second := []latLng{{lat: 24.7, lng: 46.6}}
		assertDetectsDivergence(t, first, second)
	})

	t.Run("empty runs are accepted", func(t *testing.T) {
		assertPositionsIdentical(t, nil, nil)
	})
}

// assertDetectsDivergence requires the comparison to report a divergence.
func assertDetectsDivergence(t *testing.T, first, second []latLng) {
	t.Helper()
	if _, differs := positionsDiffer(first, second); !differs {
		t.Fatalf("divergence was not detected: %v vs %v", first, second)
	}
}

func assertScenarioResourcesAreIsolated(t *testing.T, h *harness, runID, namespace string) {
	t.Helper()
	checks := []struct {
		name  string
		query string
		args  []any
	}{
		{"sources", `SELECT count(*) FROM sources WHERE id LIKE $1`, []any{namespace + "%"}},
		{"assets", `SELECT count(*) FROM assets WHERE id LIKE $1`, []any{namespace + "%"}},
		{"geofences", `SELECT count(*) FROM geofences WHERE id LIKE $1`, []any{namespace + "%"}},
		{"observations", `SELECT count(*) FROM observations WHERE payload->>'scenarioRunId' = $1`, []any{runID}},
		{"telemetry", `SELECT count(*) FROM asset_telemetry WHERE payload->>'scenarioRunId' = $1`, []any{runID}},
		{"tracks", `SELECT count(*) FROM tracks WHERE metadata->>'scenarioRunId' = $1`, []any{runID}},
		{"commands", `SELECT count(*) FROM commands WHERE payload->>'scenarioRunId' = $1`, []any{runID}},
	}
	for _, check := range checks {
		var count int
		if err := h.pool.QueryRow(context.Background(), check.query, check.args...).Scan(&count); err != nil {
			t.Fatalf("count %s for run %s: %v", check.name, runID, err)
		}
		if count == 0 {
			t.Fatalf("run %s has no isolated %s evidence under namespace %q", runID, check.name, namespace)
		}
	}
}

func containsSource(items []map[string]any, id string) bool {
	for _, item := range items {
		if item["id"] == id {
			return true
		}
	}
	return false
}

func containsID(items []map[string]any, id string) bool {
	return containsSource(items, id)
}

func findTrack(items []map[string]any, externalRef string) map[string]any {
	for _, item := range items {
		if item["externalRef"] == externalRef {
			return item
		}
	}
	return nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func isOrderedSubsequence(sequence, subsequence []string) bool {
	index := 0
	for _, item := range sequence {
		if index < len(subsequence) && item == subsequence[index] {
			index++
		}
	}
	return index == len(subsequence)
}
