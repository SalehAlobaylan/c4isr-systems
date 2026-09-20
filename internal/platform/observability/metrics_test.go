package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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
	m.ObserveRequest("GET", "/api/v1/tracks/does-not-exist", http.StatusNotFound, 6, 0)
	m.ObserveRequest("GET", "/api/v1/anything-that-is-not-a-route/one", http.StatusNotFound, 3, 0)
	m.ObserveRequest("GET", "/api/v1/anything-that-is-not-a-route/two", http.StatusNotFound, 4, 0)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if strings.Count(body, `c4isr_http_requests_total{method="GET",path="/api/v1/tracks",status="200"}`) != 1 {
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
