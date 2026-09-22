package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
)

func TestMetricsExposeRequestsEventsAndFailures(t *testing.T) {
	m := New()
	m.ObserveRequest("GET", "/api/v1/tracks", http.StatusOK, 42, 10*time.Millisecond)
	m.ObserveEvent("observation.received")
	m.ObserveEvent("alert.created")
	m.ObserveAuthFailure()
	m.ObserveDatabaseHealth(true)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`c4isr_http_requests_total{method="GET",path="/api/v1/tracks",status="200"} 1`,
		`c4isr_http_response_bytes_total{method="GET",path="/api/v1/tracks",status="200"} 42`,
		`c4isr_domain_events_total{topic="observation.received"} 1`,
		`c4isr_platform_total{metric="auth_failures_total"} 1`,
		`c4isr_database_up 1`,
		`c4isr_http_request_duration_seconds_count 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics output missing %q:\n%s", expected, body)
		}
	}
}

func TestMetricsUseBoundedPaths(t *testing.T) {
	m := New()
	m.ObserveRequest("GET", "/api/v1/tracks/track-1", http.StatusOK, 1, 0)
	m.ObserveRequest("GET", "/api/v1/tracks/track-2", http.StatusOK, 2, 0)
	m.ObserveRequest("GET", "/api/v1/tracks/{attacker-controlled}", http.StatusOK, 5, 0)
	m.ObserveRequest("GET", "/api/v1/auth/me?include=permissions", http.StatusOK, 7, 0)
	m.ObserveRequest("GET", "/api/v1/tracks/does-not-exist", http.StatusNotFound, 6, 0)
	m.ObserveRequest("GET", "/api/v1/anything-that-is-not-a-route/one", http.StatusNotFound, 3, 0)
	m.ObserveRequest("GET", "/api/v1/anything-that-is-not-a-route/two", http.StatusNotFound, 4, 0)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if strings.Count(body, `c4isr_http_requests_total{method="GET",path="/api/v1/tracks/{id}",status="200"}`) != 1 {
		t.Fatalf("track IDs created multiple series:\n%s", body)
	}
	if strings.Count(body, `c4isr_http_requests_total{method="GET",path="unmatched",status="404"}`) != 1 {
		t.Fatalf("unknown 404 paths were not collapsed:\n%s", body)
	}
	if strings.Contains(body, `c4isr_http_requests_total{method="GET",path="/api/v1/tracks",status="404"}`) {
		t.Fatalf("404 resource misses must use the unmatched fallback:\n%s", body)
	}
	if strings.Contains(body, "attacker-controlled") {
		t.Fatalf("arbitrary template-looking input escaped metric bounding:\n%s", body)
	}
	if !strings.Contains(body, `c4isr_http_requests_total{method="GET",path="/api/v1/auth/me",status="200"} 1`) {
		t.Fatalf("static auth route was not preserved and query text was not removed:\n%s", body)
	}
	if strings.Contains(body, "include=permissions") {
		t.Fatalf("query string leaked into metric labels:\n%s", body)
	}
}

func TestMetricsBoundArbitraryMethods(t *testing.T) {
	m := New()
	m.ObserveRequest("custom-verb-1", "/api/v1/tracks", http.StatusOK, 1, 0)
	m.ObserveRequest("custom-verb-2", "/api/v1/tracks", http.StatusOK, 2, 0)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `method="OTHER",path="/api/v1/tracks",status="200"} 2`) {
		t.Fatalf("arbitrary methods were not collapsed:\n%s", body)
	}
	if strings.Contains(body, "custom-verb") {
		t.Fatalf("arbitrary method leaked into metric labels:\n%s", body)
	}
}

func TestMetricsConcurrentObservations(t *testing.T) {
	m := New()
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				m.ObserveRequest("GET", "/api/v1/tracks/track-"+string(rune('a'+worker)), http.StatusOK, i, time.Microsecond)
			}
		}(worker)
	}
	wg.Wait()
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), `c4isr_http_request_duration_seconds_count 4000`) {
		t.Fatalf("concurrent observations lost updates:\n%s", recorder.Body.String())
	}
}

func TestMetricsExposeOperationalSignals(t *testing.T) {
	m := New()
	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	m.ObserveDomainEvent(events.ObservationReceived{
		At:            base.Add(10 * time.Millisecond),
		ObservationID: "obs-1",
		ReceivedAt:    base,
	})
	m.ObserveDomainEvent(events.ObservationRejected{
		At:            base.Add(20 * time.Millisecond),
		ObservationID: "obs-2",
	})
	m.ObserveDomainEvent(events.TelemetryReceived{
		At:          base.Add(30 * time.Millisecond),
		TelemetryID: "tel-1",
		AssetID:     "asset-1",
		ObservedAt:  base,
		Stale:       true,
	})
	m.ObserveStaleEntity("asset-1", true)
	m.ObserveDomainEvent(events.TrackUpdated{
		At:         base.Add(40 * time.Millisecond),
		TrackID:    "track-1",
		ObservedAt: base,
	})
	m.ObserveDomainEvent(events.AlertCreated{At: base.Add(50 * time.Millisecond), AlertID: "alert-1"})
	m.ObserveDomainEvent(events.CommandIssued{At: base, CommandID: "cmd-1"})
	m.ObserveDomainEvent(events.CommandStatusChanged{
		At:        base.Add(60 * time.Millisecond),
		CommandID: "cmd-1",
		State:     "ACKNOWLEDGED",
	})
	m.ObserveGeofenceEvaluation(7 * time.Millisecond)
	m.ObserveWebSocketPublish(8 * time.Millisecond)
	m.ObserveDatabaseQuery(9 * time.Millisecond)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		"c4isr_observations_received_total 1",
		"c4isr_observations_rejected_total 1",
		"c4isr_telemetry_received_total 1",
		"c4isr_alerts_generated_total 1",
		"c4isr_stale_entities 1",
		"c4isr_observation_ingest_duration_seconds_count 1",
		"c4isr_telemetry_ingest_duration_seconds_count 1",
		"c4isr_track_update_duration_seconds_count 1",
		"c4isr_geofence_evaluation_duration_seconds_count 1",
		"c4isr_websocket_publish_duration_seconds_count 1",
		"c4isr_command_acknowledgment_duration_seconds_count 1",
		"c4isr_database_query_duration_seconds_count 1",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics output missing %q:\n%s", expected, body)
		}
	}
}

func TestMetricsReplaceStaleEntitiesClearsRecoveredAssets(t *testing.T) {
	m := New()
	m.ObserveStaleEntity("asset-1", true)
	m.ObserveStaleEntity("asset-2", true)
	m.ReplaceStaleEntities([]string{"asset-2", "asset-3"})

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "c4isr_stale_entities 2") {
		t.Fatalf("stale gauge after replacement = %s", recorder.Body.String())
	}
	m.ReplaceStaleEntities(nil)
	recorder = httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "c4isr_stale_entities 0") {
		t.Fatalf("stale gauge after clearing = %s", recorder.Body.String())
	}
}

func TestCommandLatencyCacheIsBoundedAndExpires(t *testing.T) {
	m := New()
	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < commandIssuedAtMax+100; i++ {
		m.ObserveDomainEvent(events.CommandIssued{
			At:        base.Add(time.Duration(i) * time.Second),
			CommandID: "cmd-" + string(rune(i)),
		})
	}
	m.commandIssuedMu.Lock()
	got := len(m.commandIssuedAt)
	m.commandIssuedMu.Unlock()
	if got > commandIssuedAtMax {
		t.Fatalf("command latency cache size = %d, want <= %d", got, commandIssuedAtMax)
	}

	short := New()
	short.ObserveDomainEvent(events.CommandIssued{At: base, CommandID: "expired"})
	short.ObserveDomainEvent(events.CommandStatusChanged{
		At:        base.Add(commandIssuedAtTTL + time.Second),
		CommandID: "other",
		State:     "ACKNOWLEDGED",
	})
	short.commandIssuedMu.Lock()
	_, exists := short.commandIssuedAt["expired"]
	short.commandIssuedMu.Unlock()
	if exists {
		t.Fatal("expired command latency entry was retained")
	}
}
