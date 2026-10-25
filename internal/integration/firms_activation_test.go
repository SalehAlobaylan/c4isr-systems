package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func TestFIRMSPartialBackfillDoesNotClaimCompleteCoverage(t *testing.T) {
	f := newPublicFixture(t)
	t.Setenv("EMAD_FIRMS_ACTIVATION_KEY", "fixture_map_key_not_a_real_credential")
	entries, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.FIRMSAreaURL, Adapter: publicingest.FIRMSAdapter, AuthTokenEnv: "EMAD_FIRMS_ACTIVATION_KEY", MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for key, location := range entries {
		f.registry[key] = location
	}
	f.admission.Dataset = publicingest.FIRMSDataset
	f.admission.ProductVersion = publicingest.FIRMSProductVersion
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.MinIntervalSeconds = 3600
	f.admit(t)
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, location publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		return publicingest.Page{Records: []publicingest.Record{}, CoverageFrom: in.From, CoverageTo: in.To, HasMore: true, Checkpoint: "next-day"}, publicingest.Retrieval{LocationID: location.ID, HTTPStatus: 200}, nil
	})
	run := f.execute(t)
	if run["status"] != "PARTIAL" || run["failureCode"] != "page_budget" {
		t.Fatal(run)
	}
	code, activation := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/activation", nil, nil)
	if code != 200 || activation["state"] != "CONFIGURED" || !strings.Contains(activation["reason"].(string), "backfilled") {
		t.Fatal(activation)
	}
	health := activation["health"].(map[string]any)
	if health["lastSuccessAt"] != nil || health["coverageTo"] != nil || health["state"] != "unknown" || health["nextAttemptAt"] == nil {
		t.Fatal("partial page claimed a complete successful query", health)
	}
}
