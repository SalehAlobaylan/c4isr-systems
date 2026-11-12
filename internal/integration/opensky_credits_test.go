package integration

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func TestOpenSkyCreditExhaustionIsDurableAndUnknownIsNotAnAllowance(t *testing.T) {
	f := newPublicFixture(t)
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.OpenSkyURL, Adapter: publicingest.OpenSkyAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for key, location := range registry {
		f.registry[key] = location
	}
	f.admission.Dataset = publicingest.OpenSkyDataset
	f.admission.ProductVersion = "v1"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.LookbackSeconds = 300
	f.admission.Policy.MinIntervalSeconds = 60
	f.admission.Policy.MaxRecordsPerPage = 10
	f.admission.Policy.MaxResponseBytes = 4096
	f.admit(t)
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		zero := int64(0)
		return publicingest.Page{}, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 429, CreditsRemaining: &zero}, &publicingest.RetrievalError{Code: "provider_rate_limit", RetryAt: in.Now.Add(time.Hour)}
	})
	run := f.execute(t)
	if run["status"] != "FAILED" || run["failureCode"] != "provider_rate_limit" || run["retrieval"].(map[string]any)["creditsRemaining"] != float64(0) {
		t.Fatal("provider failure lost its reported zero balance", run)
	}
	if got := f.health(t)["nextAttemptAt"].(string); got < time.Now().Add(59*time.Minute).UTC().Format(time.RFC3339) {
		t.Fatal("provider retry was shortened", got)
	}
	code, activation := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/activation", nil, nil)
	if code != 200 || activation["providerCredits"].(map[string]any)["remaining"] != float64(0) {
		t.Fatal("activation lost durable zero balance", activation)
	}
	f.ready(t)
	code, blocked := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil)
	if code != 201 || blocked["failureCode"] != "account_credits_exhausted" || blocked["requests"] != float64(0) {
		t.Fatal("clearing pacing must not replenish credits", code, blocked)
	}
	// Advance the fixture across the configured UTC credit reset, then
	// confirm that a response without a balance is still reported as unknown.
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE provider_budget_state SET day=day-1`); err != nil {
		t.Fatal(err)
	}
	f.ready(t)
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		return publicingest.Page{Records: []publicingest.Record{}, CoverageFrom: in.From, CoverageTo: in.To}, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, nil
	})
	run = f.execute(t)
	if run["status"] != "SUCCEEDED" {
		t.Fatal(run)
	}
	code, activation = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/activation", nil, nil)
	if code != 200 || activation["providerCredits"].(map[string]any)["remaining"] != nil {
		t.Fatal("unknown balance was filled from a prior response or allowance", activation)
	}
}
