package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func seedMovingBrowser(t *testing.T, h *harness, locations publicingest.Registry, fetcher *fixtureFetcher, r access.Resource, headers map[string]string) map[string]any {
	t.Helper()
	code, s := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources", map[string]any{"name": "Synthetic OpenSky position fixture", "type": "external"}, headers)
	if code != 201 {
		t.Fatal(code, s)
	}
	source := s["id"].(string)
	worker := scopeToken(t, h, "browser-moving-connector", r, []string{"operational.read", "sources.read", "connectors.execute"}, []string{source})
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "browser-moving", SourceID: source, Resource: r, URL: publicingest.OpenSkyURL, Adapter: publicingest.OpenSkyAdapter, MaxRequestsPerDay: 10}})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range registry {
		locations[k] = v
	}
	in := publicingest.AdmissionInput{OriginalProducer: "Synthetic OpenSky fixture", UpstreamProvider: "Local native-format fixture", ConnectorID: "browser-moving", ConnectorPrincipalID: "browser-moving-connector", Dataset: publicingest.OpenSkyDataset, ProductVersion: "v1", CoverageDescription: "Synthetic bounded reports; no real provider call", RevisionPolicy: "content_hash", Enabled: true, Rights: publicingest.Rights{ID: "owned-fixture-rights", LegalBasis: "Synthetic test data owned by test; no live access", TermsURL: "https://provider.example.invalid/terms", Attribution: "Synthetic moving fixture", RetainRaw: true, AllowAnalysis: true}, Policy: publicingest.Policy{AOI: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, LookbackSeconds: 300, FreshForSeconds: 60, UnavailableAfterSeconds: 120, MinIntervalSeconds: 60, TimeoutSeconds: 5, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 1048576, MaxRequestsPerDay: 10}}
	if c, b := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources/"+source+"/admission", in, headers); c != 201 {
		t.Fatal(c, b)
	}
	at := time.Now().UTC().Add(-30 * time.Second)
	for i := 0; i < 2; i++ {
		positionTime := at.Add(time.Duration(i) * 10 * time.Second)
		fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
			raw, _ := json.Marshal(map[string]any{"time": in.Now.Unix(), "states": []any{[]any{"abcdef", "SYNTHETIC123", "Provider country inference", positionTime.Unix(), positionTime.Add(time.Second).Unix(), 39.2 + float64(i)*.001, 21.4, 8000, false, 180, 90, nil, nil, nil, nil, false, 0}}})
			p, e := publicingest.DecodeOpenSky(raw, in)
			return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
		})
		if c, b := scopeRequest(t, h, worker, "POST", "/api/v1/sources/"+source+"/connector-runs", nil, nil); c != 201 || b["newEvidence"] != float64(1) {
			t.Fatal(c, b)
		}
		if _, err := h.pool.Exec(context.Background(), `UPDATE source_connector_state SET next_attempt_at=now()-interval '1 hour' WHERE source_id=$1`, source); err != nil {
			t.Fatal(err)
		}
		if _, err := h.pool.Exec(context.Background(), `UPDATE provider_budget_state SET next_at='epoch' WHERE group_key='location:browser-moving'`); err != nil {
			t.Fatal(err)
		}
	}
	fetcher.delegate = nil
	_, list := scopeRequest(t, h, integrationToken, "GET", "/api/v1/external-tracks", nil, headers)
	for _, v := range list["items"].([]any) {
		track := v.(map[string]any)
		if track["sourceId"] == source {
			return map[string]any{"id": track["id"], "sourceId": source}
		}
	}
	t.Fatal("moving hypothesis not projected")
	return nil
}
