package integration

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"os"
	"testing"
)

// Public metadata only, explicit opt-in and disposable source/database.
func TestCDSEAuthorizedLiveCatalogueSmoke(t *testing.T) {
	if os.Getenv("C4ISR_TEST_CDSE_LIVE") != "1" {
		t.Skip("set C4ISR_TEST_CDSE_LIVE=1 for bounded public catalogue acceptance")
	}
	f := newPublicFixture(t)
	entries, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.STACURL, Adapter: publicingest.STACAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range entries {
		f.registry[k] = v
	}
	f.fetcher.delegate = publicingest.NewHTTPFetcher()
	f.admission.OriginalProducer = "Copernicus Sentinel-2"
	f.admission.UpstreamProvider = "CDSE native STAC"
	f.admission.Dataset = publicingest.STACDataset
	f.admission.ProductVersion = "stac-1.1-l2a"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.CoverageDescription = "Bounded Jeddah historical metadata query; no raster acquisition or change claim"
	f.admission.Rights.ID = "cdse-catalogue-reviewed-v1"
	f.admission.Rights.LegalBasis = "Copernicus Sentinel data legal notice; catalogue discovery only"
	f.admission.Rights.TermsURL = "https://dataspace.copernicus.eu/terms-and-conditions"
	f.admission.Rights.Attribution = "Contains modified Copernicus Sentinel data (2025); CDSE STAC metadata"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39.15, South: 21.45, East: 39.2, North: 21.5}
	f.admission.Policy.LookbackSeconds = 31 * 86400
	f.admission.Policy.MinIntervalSeconds = 5
	f.admission.Policy.TimeoutSeconds = 10
	f.admission.Policy.MaxRecordsPerPage = 4
	f.admission.Policy.MaxResponseBytes = 1024 * 1024
	f.admit(t)
	code, run := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", map[string]string{"from": "2025-02-01T00:00:00Z", "to": "2025-02-28T23:59:59Z"}, nil)
	if code != 201 || (run["status"] != "PARTIAL" && run["status"] != "SUCCEEDED") || run["newEvidence"].(float64) < 1 {
		t.Fatal(code, run)
	}
	code, scenes := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/imagery/scenes", nil, nil)
	if code != 200 || scenes["total"].(float64) < 1 {
		t.Fatal(code, scenes)
	}
	_, tracks := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/tracks", nil, nil)
	if tracks["total"] != float64(0) {
		t.Fatal("catalogue created tracks")
	}
	t.Logf("live CDSE metadata status=%v requests=%v sceneRevisions=%v; no rasters fetched", run["status"], run["requests"], scenes["total"])
}
