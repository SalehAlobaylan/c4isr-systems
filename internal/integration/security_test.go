package integration

import (
	"net/http"
	"testing"
)

func TestComposedPublicEndpointsAndAuthenticationBoundary(t *testing.T) {
	h := newHarness(t)
	client := h.server.Client()

	for _, path := range []string{"/health", "/metrics"} {
		response, err := client.Get(h.server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, response.StatusCode)
		}
	}

	preflight, err := http.NewRequest(http.MethodOptions, h.server.URL+"/api/v1/tracks", nil)
	if err != nil {
		t.Fatal(err)
	}
	preflight.Header.Set("Origin", "http://operator.test")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response, err := client.Do(preflight)
	if err != nil {
		t.Fatalf("CORS preflight: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != "http://operator.test" {
		t.Fatalf("CORS preflight status=%d allow-origin=%q", response.StatusCode, response.Header.Get("Access-Control-Allow-Origin"))
	}

	unauthenticated, err := client.Get(h.server.URL + "/api/v1/tracks")
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request status = %d, want 401", unauthenticated.StatusCode)
	}
}

func TestComposedHealthReportsUnavailableDatabase(t *testing.T) {
	h := newHarness(t)
	h.app.Close()

	response, err := h.server.Client().Get(h.server.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health after database close: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /health status = %d, want 503", response.StatusCode)
	}
}
