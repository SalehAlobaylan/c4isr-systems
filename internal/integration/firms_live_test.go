package integration

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"os"
	"testing"
)

// Opt-in only. Uses the actual restricted HTTPS fetcher and a disposable DB.
// A successful empty bounded query is valid; it makes no absence assertion.
func TestFIRMSAuthorizedLiveSmoke(t *testing.T) {
	if os.Getenv("C4ISR_TEST_FIRMS_LIVE") != "1" {
		t.Skip("set C4ISR_TEST_FIRMS_LIVE=1 and FIRMS_MAP_KEY for bounded live FIRMS acceptance")
	}
	if os.Getenv("FIRMS_MAP_KEY") == "" {
		t.Fatal("FIRMS_MAP_KEY must be set in the server environment")
	}
	f := newPublicFixture(t)
	entries, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.FIRMSAreaURL, Adapter: publicingest.FIRMSAdapter, AuthTokenEnv: "FIRMS_MAP_KEY", MaxRequestsPerDay: 5}})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range entries {
		f.registry[k] = v
	}
	f.fetcher.delegate = publicingest.NewHTTPFetcher()
	f.admission.Dataset = publicingest.FIRMSDataset
	f.admission.ProductVersion = publicingest.FIRMSProductVersion
	f.admission.RevisionPolicy = "content_hash"
	f.admission.OriginalProducer = "NASA LANCE FIRMS"
	f.admission.UpstreamProvider = "NASA FIRMS NOAA-20 VIIRS NRT"
	f.admission.CoverageDescription = "Explicit live smoke over one bounded region and rolling day; not an absence assertion"
	f.admission.Rights.ID = "firms-noaa20-open-attribution-v1"
	f.admission.Rights.LegalBasis = "NASA open data policy and FIRMS acknowledgement"
	f.admission.Rights.TermsURL = "https://www.earthdata.nasa.gov/engage/open-data-services-software/data-use-policy"
	f.admission.Rights.Attribution = "NASA FIRMS, NASA LANCE; NOAA-20 VIIRS"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}
	f.admission.Policy.MinIntervalSeconds = 60
	f.admission.Policy.MaxRequestsPerDay = 5
	f.admission.Policy.TimeoutSeconds = 10
	f.admission.Policy.MaxRecordsPerPage = 1000
	f.admission.Policy.MaxResponseBytes = 1024 * 1024
	f.admit(t)
	// Rolling-day bounds can span two UTC calendar days. Test-only ready resets
	// the persisted minimum interval between the two disposable invocations.
	for i := 0; i < 2; i++ {
		f.ready(t)
		run := f.execute(t)
		if run["status"] == "SUCCEEDED" {
			if h := f.health(t); h["pendingPublications"] != float64(0) {
				t.Fatal(h)
			}
			return
		}
		if run["status"] != "PARTIAL" {
			t.Fatalf("FIRMS live result %v", run)
		}
	}
	t.Fatal("FIRMS day cursor did not complete its bounded live query")
}
