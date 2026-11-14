package integration

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func TestEONETRetainedFloodPilot(t *testing.T) {
	recorded := os.Getenv("EMAD_EONET_PILOT_RESPONSE")
	if recorded == "" {
		t.Skip("Set the retained regional EONET response path")
	}
	raw, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatal(err)
	}
	f := newPublicFixture(t)
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource,
		Adapter: publicingest.EONETAdapter, URL: publicingest.EONETURL, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	f.registry["fixture-location"] = registry["fixture-location"]
	f.admission.Dataset = publicingest.EONETDataset
	f.admission.ProductVersion = "v3"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.AnalystQuestion = "Which dated flood reports are available in this Middle East coverage window?"
	f.admission.Policy.AOI = publicingest.Bounds{West: 34, South: 16, East: 56, North: 33}
	f.admission.Policy.LookbackSeconds = 31 * 86400
	f.admission.Policy.MaxResponseBytes = 1 << 20
	f.admission.Policy.MaxRecordsPerPage = 100
	f.admission.Policy.MinIntervalSeconds = 900
	f.admit(t)
	provider := raw
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		p, e := publicingest.DecodeEONET(provider, in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200, Discontinuous: true, CoverageMeaning: "Returned flood reports only"}, e
	})
	path := "/api/v1/sources/" + f.source + "/connector-runs"
	window := map[string]any{"from": "2025-12-01T00:00:00Z", "to": "2025-12-31T23:59:59Z"}
	execute := func() map[string]any {
		t.Helper()
		code, run := scopeRequest(t, f.h, f.worker, "POST", path, window, nil)
		if code != 201 {
			t.Fatal(code, run)
		}
		return run
	}
	list := func() map[string]any {
		t.Helper()
		code, out := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-context/records?kind=REPORT", nil, nil)
		if code != 200 {
			t.Fatal(code, out)
		}
		return out
	}
	run := execute()
	if run["status"] != "SUCCEEDED" {
		t.Fatal(run)
	}
	find := func(out map[string]any) (string, map[string]any) {
		t.Helper()
		for _, item := range out["items"].([]any) {
			ev := item.(map[string]any)["evidence"].(map[string]any)
			if ev["providerRecordId"] == "EONET_16732" {
				return ev["id"].(string), ev["context"].(map[string]any)["report"].(map[string]any)["eventHistory"].(map[string]any)
			}
		}
		t.Fatal("Pinned flood report absent", out)
		return "", nil
	}
	id, history := find(list())
	if history["closedAt"] != "2025-12-28T00:00:00Z" || len(history["geometries"].([]any)) != 1 {
		t.Fatal(history)
	}
	f.ready(t)
	provider = bytes.Replace(raw, []byte("2025-12-28T00:00:00Z"), []byte("2025-12-29T00:00:00Z"), 1)
	execute()
	current, newHistory := find(list())
	if current == id || newHistory["closedAt"] != "2025-12-29T00:00:00Z" {
		t.Fatal(current, newHistory)
	}
	code, out := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-context/records/"+id+"/history", nil, nil)
	if code != 200 || out["total"] != float64(2) {
		t.Fatal(code, out)
	}
	f.ready(t)
	provider = []byte(`{"error":"provider unavailable"}`)
	failed := execute()
	if failed["status"] != "FAILED" || !strings.Contains(failed["failureCode"].(string), "invalid") {
		t.Fatal(failed)
	}
	f.ready(t)
	provider = []byte(`{"events":[]}`)
	if empty := execute(); empty["status"] != "SUCCEEDED" {
		t.Fatal(empty)
	}
	stillCurrent, _ := find(list())
	if stillCurrent != current {
		t.Fatal("An empty query withdrew retained provider evidence")
	}
}
