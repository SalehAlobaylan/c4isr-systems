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
	if len(second) != len(positions) {
		t.Fatalf("replay emitted %d observations, first run %d", len(second), len(positions))
	}
	for i := range positions {
		if positions[i].lat != second[i].lat || positions[i].lng != second[i].lng {
			t.Fatalf("replay diverged at observation %d: %v vs %v", i, positions[i], second[i])
		}
	}
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

func positionsForRun(t *testing.T, h *harness, runID string) []latLng {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `
		SELECT ST_Y(position::geometry), ST_X(position::geometry)
		FROM observations
		WHERE payload->>'scenarioRunId' = $1 AND position IS NOT NULL
		ORDER BY created_at ASC, id ASC`, runID)
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
