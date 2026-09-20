package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPermissionForOperationalRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/v1/tracks", "tracks.read"},
		{http.MethodPost, "/api/v1/alerts/alr_1/acknowledge", "alerts.acknowledge"},
		{http.MethodPost, "/api/v1/incidents", "incidents.create"},
		{http.MethodPost, "/api/v1/commands/cmd_1/transition", "commands.transition"},
		{http.MethodPost, "/api/v1/scenarios/runs/run_1/pause", "scenarios.control"},
		{http.MethodGet, "/api/v1/auth/me", ""},
	}
	for _, tt := range tests {
		if got := PermissionFor(tt.method, tt.path); got != tt.want {
			t.Errorf("PermissionFor(%s, %s) = %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestPolicyFailsClosedAndRespectsPathSegments(t *testing.T) {
	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/unknown"},
		{http.MethodDelete, "/api/v1/tracks/track-1"},
		{http.MethodGet, "/api/v1/sources-admin"},
		{http.MethodGet, "/api/v1//sources"},
		{http.MethodGet, "/api/v1/sources//"},
		{http.MethodPost, "/api/v1/sources-admin"},
		{http.MethodGet, "/api/v1/geospatial/not-a-query"},
		{http.MethodGet, "/api/v1/classifications"},
		{http.MethodPost, "/api/v1/scenarios/runs/run-1/not-a-control"},
	} {
		if policy := PolicyFor(tt.method, tt.path); policy.Known {
			t.Errorf("PolicyFor(%s, %s) = %#v, want unknown", tt.method, tt.path, policy)
		}
	}
	if policy := PolicyFor(http.MethodGet, "/api/v1/auth/me"); !policy.Known || policy.Permission != "" {
		t.Fatalf("auth/me policy = %#v, want known with no extra permission", policy)
	}
	if policy := PolicyFor(http.MethodGet, "/api/v1/realtime"); !policy.Known || policy.Permission != "operational.read" {
		t.Fatalf("realtime policy = %#v", policy)
	}
	if policy := PolicyFor(http.MethodPost, "/api/v1/realtime"); policy.Known {
		t.Fatalf("POST realtime must be unknown: %#v", policy)
	}
}

func TestAuthorizationPublicRequestsAndRoleFailures(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := Authorization(next)

	for _, path := range []string{"/health", "/metrics"} {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusNoContent || !nextCalled {
			t.Fatalf("public %s status=%d called=%v", path, resp.Code, nextCalled)
		}
	}

	nextCalled = false
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/unknown", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("preflight status=%d called=%v", resp.Code, nextCalled)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	req = req.WithContext(withOperatorRole(context.Background(), "operator"))
	req = req.WithContext(withOperatorID(req.Context(), "operator-01"))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unknown route status=%d, want 403", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/sources", nil)
	req = req.WithContext(withOperatorRole(context.Background(), "operator"))
	req = req.WithContext(withOperatorID(req.Context(), "operator-01"))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("insufficient role status=%d, want 403", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated auth/me status=%d, want 401", resp.Code)
	}
}

func TestStatusRecorderFlushRecordsImplicitSuccess(t *testing.T) {
	underlying := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: underlying, status: http.StatusOK}
	recorder.Flush()
	if recorder.status != http.StatusOK || !recorder.wroteHeader {
		t.Fatalf("flush recorder = %#v, want an implicit 200 header", recorder)
	}
}

func TestAllowsRoleMatrix(t *testing.T) {
	if !Allows("operator", "alerts.acknowledge") {
		t.Fatal("operator should acknowledge alerts")
	}
	if Allows("operator", "admin.manage") {
		t.Fatal("operator must not manage administrators")
	}
	if Allows("operator", "sources.manage") {
		t.Fatal("operator must not manage source configuration")
	}
	if !Allows("supervisor", "assessments.create") {
		t.Fatal("supervisor should create assessments")
	}
	if !Allows("supervisor", "sources.manage") {
		t.Fatal("supervisor should manage source configuration")
	}
	if Allows("analyst", "commands.issue") {
		t.Fatal("analyst must not issue commands")
	}
	if !Allows("administrator", "anything.at.all") {
		t.Fatal("administrator should have full access")
	}
}

func TestAuthenticateUsesBearerSubjectAndIgnoresSpoofedHeader(t *testing.T) {
	lookup := func(_ contextT, id string) (Identity, error) {
		return Identity{ID: id, Name: "Operator", Role: "operator"}, nil
	}
	called := false
	handler := Authenticate(AuthOptions{
		Required:        true,
		Tokens:          map[string]string{"secret-token": "operator-01"},
		DefaultOperator: "operator-01",
		Lookup:          lookup,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if GetOperatorID(r.Context()) != "operator-01" {
			t.Errorf("operator id = %q", GetOperatorID(r.Context()))
		}
		if GetOperatorRole(r.Context()) != "operator" {
			t.Errorf("operator role = %q", GetOperatorRole(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
	req.Header.Set(AuthorizationHeader, "Bearer secret-token")
	req.Header.Set(OperatorHeader, "administrator")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent || !called {
		t.Fatalf("authenticated request status=%d called=%v", resp.Code, called)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
	req.Header.Set(OperatorHeader, "operator-01")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer token status=%d, want 401", resp.Code)
	}
}

func TestAuthenticateAllowsLegacyHeaderOnlyWhenDisabled(t *testing.T) {
	handler := Authenticate(AuthOptions{
		DefaultOperator: "operator-01",
		Lookup: func(_ contextT, id string) (Identity, error) {
			return Identity{ID: id, Role: "operator"}, nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
	req.Header.Set(OperatorHeader, "operator-01")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("legacy request status=%d, want 204", resp.Code)
	}
}
