package integration

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"os"
	"testing"
)

// Explicit opt-in: a human-reviewed test admission, source-bound service
// credential, production HTTPS adapter and disposable database. No developer
// database or long-running provider schedule is created.
func TestUSGSAuthorizedLiveSmoke(t *testing.T) {
	if os.Getenv("C4ISR_TEST_USGS_LIVE") != "1" {
		t.Skip("set C4ISR_TEST_USGS_LIVE=1 for bounded live USGS acceptance")
	}
	f := newPublicFixture(t)
	entries, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.USGSQueryURL, Adapter: publicingest.USGSAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range entries {
		f.registry[key] = value
	}
	f.fetcher.delegate = publicingest.NewHTTPFetcher()
	f.admission.OriginalProducer = "USGS ComCat"
	f.admission.UpstreamProvider = "USGS earthquake catalogue"
	f.admission.Dataset = "usgs-earthquakes"
	f.admission.ProductVersion = "geojson-summary-v1"
	f.admission.CoverageDescription = "Bounded Puerto Rico query for explicit live adapter acceptance; no absence inference"
	f.admission.Rights.ID = "usgs-comcat-reviewed-v1"
	f.admission.Rights.LegalBasis = "USGS data use policy and provider network attribution retained in originals"
	f.admission.Rights.TermsURL = "https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits"
	f.admission.Rights.Attribution = "U.S. Geological Survey, ComCat; contributing network retained per report"
	f.admission.Policy.AOI = publicingest.Bounds{West: -68, East: -64, South: 17, North: 20}
	f.admission.Policy.TimeoutSeconds = 10
	f.admission.Policy.MaxRecordsPerPage = 20
	f.admission.Policy.MaxResponseBytes = 1024 * 1024
	f.admit(t)
	run := f.execute(t)
	if run["status"] != "SUCCEEDED" && run["status"] != "PARTIAL" {
		t.Fatalf("live USGS query: %v", run)
	}
	if run["newEvidence"].(float64) < 1 {
		t.Fatalf("live smoke expected at least one provider record: %v", run)
	}
	if h := f.health(t); h["pendingPublications"] != float64(0) {
		t.Fatalf("live projection not complete: %v", h)
	}
	status, events := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events", nil, nil)
	if status != 200 || events["total"].(float64) < 1 {
		t.Fatalf("live observations missing: %d %v", status, events)
	}
	_, tracks := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/tracks", nil, nil)
	if tracks["total"] != float64(0) {
		t.Fatal("live earthquakes created tracks")
	}
	t.Logf("live USGS result status=%v requests=%v evidence=%v events=%v", run["status"], run["requests"], run["newEvidence"], events["total"])
}
