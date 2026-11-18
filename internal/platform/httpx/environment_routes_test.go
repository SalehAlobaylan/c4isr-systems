package httpx

import (
	"net/http"
	"testing"
)

func TestGeographicEnvironmentAssessmentAndSuitabilityPolicies(t *testing.T) {
	for _, test := range []struct {
		method, path, permission string
	}{
		{http.MethodGet, "/api/v1/environment-releases/georelease_x/assessments", "environments.read"},
		{http.MethodPost, "/api/v1/environment-releases/georelease_x/assessments", "environments.qualify"},
		{http.MethodGet, "/api/v1/environment-assessments/geoassess_x", "environments.read"},
		{http.MethodPost, "/api/v1/environment-assessments/geoassess_x/supersede", "environments.qualify"},
		{http.MethodPost, "/api/v1/environment-releases/georelease_x/suitability", "environments.read"},
		{http.MethodGet, "/api/v1/environment-releases/georelease_x/prepared", "environments.download"},
		{http.MethodPost, "/api/v1/environment-releases/georelease_x/release", "environments.release"},
	} {
		policy := PolicyFor(test.method, test.path)
		if !policy.Known || policy.Permission != test.permission {
			t.Fatalf("%s %s: %+v, want %s", test.method, test.path, policy, test.permission)
		}
	}
	for _, path := range []string{
		"/api/v1/environment-assessments",
		"/api/v1/environment-assessments/geoassess_x/unknown",
		"/api/v1/environment-releases/georelease_x/unknown",
	} {
		if policy := PolicyFor(http.MethodPost, path); policy.Known {
			t.Fatalf("unmounted write route %s must fail closed: %+v", path, policy)
		}
	}
}
