package publicingest

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOptionalGDACSRSSCapture(t *testing.T) {
	path := os.Getenv("EMAD_GDACS_CAPTURE_XML")
	if path == "" {
		t.Skip("explicit retained public RSS capture required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	in := FetchRequest{Now: now, From: now.Add(-7 * 24 * time.Hour), To: now, Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{AOI: Bounds{West: 34, South: 16, East: 56, North: 33}, MaxRecordsPerPage: 100, MaxResponseBytes: 1048576, TimeoutSeconds: 10}}}}
	page, err := DecodeGDACSRSS(raw, in)
	if err != nil {
		t.Fatal("retained official RSS decode:", err)
	}
	t.Log("retained capture decoded", len(page.Records), "regional records,", page.ExcludedRecords, "excluded")
	if os.Getenv("EMAD_GDACS_NATIVE_FETCH") != "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, meta, err := NewHTTPFetcher().Fetch(ctx, Location{URL: GDACSRSSURL, Adapter: GDACSRSSAdapter}, in)
	if err != nil {
		t.Fatal("native official RSS retrieval:", meta.HTTPStatus, err)
	}
	t.Log("native RSS decoded", len(page.Records), "regional records,", page.ExcludedRecords, "excluded")
}
