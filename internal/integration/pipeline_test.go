package integration

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
)

func TestConcurrencyGuardMigrationRepairsLegacyDuplicates(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `
		DROP INDEX alerts_active_geofence_track_key;
		DROP INDEX tracks_initial_observation_id_key;
		INSERT INTO tracks (id, first_seen_at, last_seen_at, metadata)
		VALUES ('migration-track', now() - interval '2 minutes', now() - interval '2 minutes', '{}'::jsonb);
		INSERT INTO tracks (id, first_seen_at, last_seen_at, metadata)
		VALUES
			('migration-track-old', now() - interval '2 minutes', now() - interval '2 minutes', '{"initialObservationId":"legacy-observation"}'::jsonb),
			('migration-track-new', now() - interval '1 minute', now() - interval '1 minute', '{"initialObservationId":"legacy-observation"}'::jsonb);
		INSERT INTO geofences (id, name, type, geometry)
		VALUES ('migration-geofence', 'Migration fence', 'restricted', ST_GeomFromText('POLYGON((0 0, 1 0, 1 1, 0 1, 0 0))', 4326));
		INSERT INTO alerts (id, type, severity, state, title, track_id, geofence_id, created_at)
		VALUES
			('migration-alert-old', 'geofence.breach', 'high', 'ACTIVE', 'old', 'migration-track', 'migration-geofence', now() - interval '2 minutes'),
			('migration-alert-new', 'geofence.breach', 'high', 'ACTIVE', 'new', 'migration-track', 'migration-geofence', now() - interval '1 minute');
	`); err != nil {
		t.Fatalf("seed legacy duplicate alerts: %v", err)
	}
	raw, err := os.ReadFile("../../db/migrations/00017_concurrency_guards.sql")
	if err != nil {
		t.Fatalf("read concurrency migration: %v", err)
	}
	if _, err := h.pool.Exec(ctx, extractUpSection(string(raw))); err != nil {
		t.Fatalf("apply concurrency migration to duplicate data: %v", err)
	}
	var unresolved, migratedResolved, retainedOwners int
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE state <> 'RESOLVED'),
		       count(*) FILTER (WHERE resolved_by = 'migration:00017'),
		       (SELECT count(*) FROM tracks WHERE metadata->>'initialObservationId' = 'legacy-observation')
		FROM alerts
		WHERE geofence_id = 'migration-geofence' AND track_id = 'migration-track'
	`).Scan(&unresolved, &migratedResolved, &retainedOwners); err != nil {
		t.Fatalf("check repaired alerts: %v", err)
	}
	if unresolved != 1 || migratedResolved != 1 || retainedOwners != 1 {
		t.Fatalf("repaired legacy data = unresolved %d, migrated resolved %d, retained track owners %d; want 1, 1, 1", unresolved, migratedResolved, retainedOwners)
	}
}

// integrationFreshnessObserver records every gauge update so a test can assert
// on the whole sequence, including an asset recovering contact.
type integrationFreshnessObserver struct {
	calls []string
}

func (o *integrationFreshnessObserver) ObserveStaleEntity(assetID string, stale bool) {
	state := "live"
	if stale {
		state = "not-current"
	}
	o.calls = append(o.calls, assetID+":"+state)
}

func newIntegrationMonitor(t *testing.T, h *harness, observer telemetry.FreshnessObserver) *telemetry.FreshnessMonitor {
	t.Helper()
	repo := telemetry.NewPostgresRepository(h.pool)
	return telemetry.NewFreshnessMonitor(repo, events.NewDispatcher(slog.New(slog.DiscardHandler)), observer, freshness.DefaultPolicy())
}

func TestFreshnessMonitorCoversNeverSeenAssetsAndRecoversAfterTelemetry(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createSource("stale-source")
	if status := h.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"id": "never-seen", "name": "Never Seen", "type": "vehicle",
	}, nil); status != http.StatusCreated {
		t.Fatalf("create never-seen asset: status %d", status)
	}

	observer := &integrationFreshnessObserver{}
	monitor := newIntegrationMonitor(t, h, observer)

	// An asset that has never reported has no data at all, which is distinct
	// from being stale: nothing is known rather than something being old.
	if err := monitor.Reconcile(ctx); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	// Snapshot and gauge keys are type-namespaced so an asset and a track sharing
	// an id cannot collapse into one another.
	if got := monitor.Snapshot()["asset:never-seen"]; got != freshness.StateUnknown {
		t.Fatalf("never-seen freshness = %q, want %q", got, freshness.StateUnknown)
	}
	if len(observer.calls) == 0 || observer.calls[0] != "asset:never-seen:not-current" {
		t.Fatalf("gauge updates = %v, want never-seen marked not-current", observer.calls)
	}

	if status := h.do(http.MethodPost, "/api/v1/telemetry", map[string]any{
		"messageId":  "never-seen-recovered",
		"assetId":    "never-seen",
		"sourceId":   "stale-source",
		"observedAt": time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano),
	}, nil); status != http.StatusCreated {
		t.Fatalf("recover never-seen asset: status %d", status)
	}

	if err := monitor.Reconcile(ctx); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if got := monitor.Snapshot()["asset:never-seen"]; got != freshness.StateLive {
		t.Fatalf("recovered freshness = %q, want %q", got, freshness.StateLive)
	}
	last := observer.calls[len(observer.calls)-1]
	if last != "asset:never-seen:live" {
		t.Fatalf("final gauge update = %q, want never-seen:live", last)
	}
}

func TestFreshnessIsExposedOnAssetAndTrackReads(t *testing.T) {
	h := newHarness(t)
	// ingestObservation posts against this shared synthetic source.
	h.createSource("simulator-test")
	h.createSource("freshness-source")

	if status := h.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"id": "freshness-asset", "name": "Freshness Asset", "type": "vehicle",
	}, nil); status != http.StatusCreated {
		t.Fatalf("create asset: status %d", status)
	}
	if status := h.do(http.MethodPost, "/api/v1/telemetry", map[string]any{
		"messageId":  "freshness-asset-1",
		"assetId":    "freshness-asset",
		"sourceId":   "freshness-source",
		"observedAt": time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano),
	}, nil); status != http.StatusCreated {
		t.Fatalf("ingest telemetry: status %d", status)
	}

	// A track built from a current observation is live, and its position must
	// not be presented as though it were unqualified.
	h.ingestObservation("freshness-track", 24.7, 46.6, time.Now().UTC().Add(-time.Second))

	var assets struct {
		Items []struct {
			ID         string `json:"id"`
			Freshness  string `json:"freshness"`
			Label      string `json:"freshnessLabel"`
			LastSeenAt string `json:"lastSeenAt"`
		} `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/assets?limit=200", nil, &assets); status != http.StatusOK {
		t.Fatalf("list assets: status %d", status)
	}
	found := false
	for _, item := range assets.Items {
		if item.ID != "freshness-asset" {
			continue
		}
		found = true
		if item.Freshness != string(freshness.StateLive) {
			t.Errorf("asset freshness = %q, want %q", item.Freshness, freshness.StateLive)
		}
		if item.Label != freshness.StateLive.Label() {
			t.Errorf("asset freshnessLabel = %q, want %q", item.Label, freshness.StateLive.Label())
		}
		if item.LastSeenAt == "" {
			t.Error("asset lastSeenAt must be populated for freshness to be auditable")
		}
	}
	if !found {
		t.Fatal("freshness-asset missing from asset list")
	}

	var tracks struct {
		Items []struct {
			ID          string `json:"id"`
			ExternalRef string `json:"externalRef"`
			Freshness   string `json:"freshness"`
		} `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/tracks?limit=200", nil, &tracks); status != http.StatusOK {
		t.Fatalf("list tracks: status %d", status)
	}
	found = false
	for _, item := range tracks.Items {
		if item.ExternalRef != "freshness-track" {
			continue
		}
		found = true
		if item.Freshness != string(freshness.StateLive) {
			t.Errorf("track freshness = %q, want %q", item.Freshness, freshness.StateLive)
		}
	}
	if !found {
		t.Fatal("freshness-track missing from track list")
	}
}

func TestObservationIngestionPreservesEvidenceAndTime(t *testing.T) {
	h := newHarness(t)
	h.createSource("simulator-test")

	observedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Millisecond)
	created := h.ingestObservation("", 24.7, 46.6, observedAt)

	observationID, _ := created["id"].(string)
	if observationID == "" {
		t.Fatal("observation id missing from response")
	}
	if created["receivedAt"] == nil {
		t.Fatal("receivedAt must be recorded independently of observedAt")
	}

	var fetched map[string]any
	if status := h.do(http.MethodGet, "/api/v1/observations/"+observationID, nil, &fetched); status != http.StatusOK {
		t.Fatalf("get observation: status %d", status)
	}
	if fetched["sourceId"] != "simulator-test" {
		t.Fatalf("source provenance lost: %v", fetched["sourceId"])
	}
	position, ok := fetched["position"].(map[string]any)
	if !ok || position["lat"].(float64) != 24.7 || position["lng"].(float64) != 46.6 {
		t.Fatalf("position not preserved: %v", fetched["position"])
	}

	// Re-submitting the same client id is an idempotent duplicate.
	var duplicate map[string]any
	status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"id":         observationID,
		"sourceId":   "simulator-test",
		"type":       "track.observation",
		"observedAt": observedAt.Format(time.RFC3339Nano),
	}, &duplicate)
	if status != http.StatusOK {
		t.Fatalf("duplicate should return 200, got %d", status)
	}
	if duplicate["duplicate"] != true {
		t.Fatalf("duplicate flag missing: %v", duplicate)
	}

	// Unknown sources are rejected with validation semantics.
	status = h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId":   "does-not-exist",
		"type":       "track.observation",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("unknown source should be rejected, got %d", status)
	}

	// Invalid coordinates are rejected.
	status = h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId":   "simulator-test",
		"type":       "track.observation",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"position":   map[string]any{"lat": 123.0, "lng": 46.0},
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid coordinates should be rejected, got %d", status)
	}
}

func TestTrackCorrelationKeepsEvidenceAndIgnoresStaleState(t *testing.T) {
	h := newHarness(t)
	h.createSource("simulator-test")

	base := time.Now().UTC().Truncate(time.Millisecond)
	first := h.ingestObservation("unknown-01", 24.7000, 46.6000, base)
	firstID, _ := first["id"].(string)

	// A late (stale) observation attaches as evidence but must not regress
	// current state.
	staleAt := base.Add(-90 * time.Second)
	h.ingestObservation("unknown-01", 24.9000, 46.9000, staleAt)

	// A newer observation advances state.
	newerAt := base.Add(15 * time.Second)
	h.ingestObservation("unknown-01", 24.7010, 46.6010, newerAt)

	var tracks struct {
		Items []map[string]any `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/tracks", nil, &tracks); status != http.StatusOK {
		t.Fatalf("list tracks: %d", status)
	}
	if len(tracks.Items) != 1 {
		t.Fatalf("expected one track from one hint, got %d", len(tracks.Items))
	}
	track := tracks.Items[0]
	position := track["position"].(map[string]any)
	if position["lat"].(float64) < 24.7005 || position["lat"].(float64) > 24.7015 {
		t.Fatalf("track state regressed or did not update: %v", position)
	}
	if track["observationCount"] != float64(3) {
		t.Fatalf("track observation count = %v, want 3", track["observationCount"])
	}

	trackID := track["id"].(string)

	var observations struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if status := h.do(http.MethodGet, "/api/v1/observations?track_id="+trackID, nil, &observations); status != http.StatusOK {
		t.Fatalf("list observations: %d", status)
	}
	if observations.Total != 3 {
		t.Fatalf("all observations must remain attached as evidence, got %d", observations.Total)
	}

	var history struct {
		Items []map[string]any `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/tracks/"+trackID+"/history", nil, &history); status != http.StatusOK {
		t.Fatalf("list history: %d", status)
	}
	if len(history.Items) != 2 {
		t.Fatalf("stale observation must not append history, got %d points", len(history.Items))
	}

	// The first observation must be linked as evidence to the track.
	found := false
	for _, obs := range observations.Items {
		if obs["id"] == firstID {
			found = true
		}
	}
	if !found {
		t.Fatal("first observation missing from track evidence")
	}
}

func TestTelemetryStaleDuplicateAndOutOfOrder(t *testing.T) {
	h := newHarness(t)
	h.createSource("simulator-test")

	status := h.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"id": "patrol-test", "name": "Patrol Test", "type": "vehicle",
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("create asset: status %d", status)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	send := func(messageID string, at time.Time, lat, lng float64) int {
		return h.do(http.MethodPost, "/api/v1/telemetry", map[string]any{
			"messageId":  messageID,
			"assetId":    "patrol-test",
			"sourceId":   "simulator-test",
			"observedAt": at.Format(time.RFC3339Nano),
			"position":   map[string]any{"lat": lat, "lng": lng},
		}, nil)
	}

	if status := send("msg-1", now, 24.7000, 46.6000); status != http.StatusCreated {
		t.Fatalf("first sample: status %d", status)
	}

	// Duplicate message id is acknowledged but not stored twice.
	var duplicate map[string]any
	status = h.do(http.MethodPost, "/api/v1/telemetry", map[string]any{
		"messageId":  "msg-1",
		"assetId":    "patrol-test",
		"sourceId":   "simulator-test",
		"observedAt": now.Format(time.RFC3339Nano),
		"position":   map[string]any{"lat": 24.7000, "lng": 46.6000},
	}, &duplicate)
	if status != http.StatusOK || duplicate["duplicate"] != true {
		t.Fatalf("duplicate telemetry: status %d body %v", status, duplicate)
	}

	// Stale sample: history only, no state regression.
	send("msg-2", now.Add(-120*time.Second), 24.9000, 46.9000)

	var asset map[string]any
	h.do(http.MethodGet, "/api/v1/assets/patrol-test", nil, &asset)
	position := asset["position"].(map[string]any)
	if position["lat"].(float64) != 24.7000 {
		t.Fatalf("stale telemetry regressed asset state: %v", position)
	}

	// Out-of-order but newer sample advances state.
	send("msg-3", now.Add(5*time.Second), 24.7100, 46.6100)
	h.do(http.MethodGet, "/api/v1/assets/patrol-test", nil, &asset)
	position = asset["position"].(map[string]any)
	if position["lat"].(float64) != 24.7100 {
		t.Fatalf("newer telemetry did not advance state: %v", position)
	}

	var history struct {
		Total int `json:"total"`
	}
	h.do(http.MethodGet, "/api/v1/assets/patrol-test/telemetry", nil, &history)
	if history.Total != 3 {
		t.Fatalf("telemetry history should preserve accepted samples, got %d", history.Total)
	}
}

func TestGeofenceBreachCreatesSingleAlertAndSpatialQueries(t *testing.T) {
	h := newHarness(t)
	h.createSource("simulator-test")

	// Restricted zone around (24.71, 46.61).
	status := h.do(http.MethodPost, "/api/v1/geofences", map[string]any{
		"id": "zone-test", "name": "Test Zone", "type": "restricted", "severity": "high",
		"polygon": []map[string]any{
			{"lat": 24.7120, "lng": 46.6080},
			{"lat": 24.7120, "lng": 46.6120},
			{"lat": 24.7080, "lng": 46.6120},
			{"lat": 24.7080, "lng": 46.6080},
		},
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("create geofence: status %d", status)
	}

	// An asset outside the zone for proximity queries.
	h.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"id": "patrol-near", "name": "Patrol Near", "type": "vehicle",
	}, nil)
	h.do(http.MethodPost, "/api/v1/telemetry", map[string]any{
		"assetId":    "patrol-near",
		"sourceId":   "simulator-test",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"position":   map[string]any{"lat": 24.7060, "lng": 46.6060},
	}, nil)

	base := time.Now().UTC()
	h.ingestObservation("unknown-zone", 24.7000, 46.6000, base)                    // outside
	h.ingestObservation("unknown-zone", 24.7100, 46.6100, base.Add(1*time.Second)) // inside -> breach

	var alerts struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	waitFor(t, 10*time.Second, 200*time.Millisecond, "alert creation", func() bool {
		h.do(http.MethodGet, "/api/v1/alerts?state=ACTIVE", nil, &alerts)
		return alerts.Total >= 1
	})
	if alerts.Total != 1 {
		t.Fatalf("expected exactly one alert, got %d", alerts.Total)
	}
	alert := alerts.Items[0]
	if alert["geofenceId"] != "zone-test" {
		t.Fatalf("alert lost geofence provenance: %v", alert["geofenceId"])
	}
	if alert["trackId"] == nil || alert["sourceReference"] == nil {
		t.Fatalf("alert persisted without rule provenance: %v", alert)
	}

	// Further inside-observations must not spam duplicate alerts.
	h.ingestObservation("unknown-zone", 24.7115, 46.6110, base.Add(2*time.Second))
	time.Sleep(500 * time.Millisecond)
	h.do(http.MethodGet, "/api/v1/alerts?state=ACTIVE", nil, &alerts)
	if alerts.Total != 1 {
		t.Fatalf("duplicate breach produced extra alerts: %d", alerts.Total)
	}

	// PostGIS-backed spatial queries.
	var containing struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/geospatial/geofences-containing?lat=24.7100&lng=46.6100", nil, &containing)
	if len(containing.Items) != 1 {
		t.Fatalf("geofences-containing returned %d geofences", len(containing.Items))
	}

	var within struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/geospatial/assets-within?lat=24.7100&lng=46.6100&radius_m=1000", nil, &within)
	if len(within.Items) != 1 || within.Items[0]["assetId"] != "patrol-near" {
		t.Fatalf("assets-within did not use PostGIS distance: %v", within.Items)
	}
	if within.Items[0]["distanceM"].(float64) <= 0 {
		t.Fatalf("distance missing: %v", within.Items[0])
	}

	var nearest struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/geospatial/nearest-assets?lat=24.7100&lng=46.6100&limit=1", nil, &nearest)
	if len(nearest.Items) != 1 || nearest.Items[0]["assetId"] != "patrol-near" {
		t.Fatalf("nearest-assets returned %v", nearest.Items)
	}

	// Acknowledge transition is auditable and reflected in state.
	alertID := alert["id"].(string)
	var acknowledged map[string]any
	if status := h.do(http.MethodPost, "/api/v1/alerts/"+alertID+"/acknowledge", nil, &acknowledged); status != http.StatusOK {
		t.Fatalf("acknowledge: status %d", status)
	}
	if acknowledged["state"] != "ACKNOWLEDGED" || acknowledged["acknowledgedBy"] != "operator-01" {
		t.Fatalf("acknowledgement not attributed: %v", acknowledged)
	}

	var audit struct {
		Items []map[string]any `json:"items"`
	}
	h.do(http.MethodGet, "/api/v1/audit?action=alert.acknowledged", nil, &audit)
	if len(audit.Items) != 1 || audit.Items[0]["actorType"] != "OPERATOR" {
		t.Fatalf("acknowledgement missing from audit: %v", audit.Items)
	}
}

var _ = context.Background
