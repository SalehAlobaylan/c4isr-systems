package publicingest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func firmsFixture(t *testing.T) ([]byte, FetchRequest) {
	t.Helper()
	raw, err := os.ReadFile("testdata/firms/noaa20-published-example.csv")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2025, 6, 4, 0, 0, 0, 0, time.UTC)
	return raw, FetchRequest{Now: from.Add(24 * time.Hour), From: from, To: from.Add(24*time.Hour - time.Nanosecond), Admission: Admission{SourceID: "firms", AdmissionInput: AdmissionInput{OriginalProducer: "NASA LANCE FIRMS", Dataset: FIRMSDataset, ProductVersion: FIRMSProductVersion, RevisionPolicy: "content_hash", Rights: Rights{RetainRaw: true}, Policy: Policy{AOI: Bounds{West: 99, South: 5, East: 102, North: 14}, MaxRecordsPerPage: 20, MaxResponseBytes: 65536}}}}
}
func TestFIRMSPublishedExampleTimeQualityIdentityAndOriginalCSV(t *testing.T) {
	raw, in := firmsFixture(t)
	page, err := DecodeFIRMS(raw, in)
	if err != nil || len(page.Records) != 5 || page.HasMore {
		t.Fatalf("decode %d %v", len(page.Records), err)
	}
	first := page.Records[0]
	o := first.Observation
	if o.ObservedAt.Hour() != 6 || o.ObservedAt.Minute() != 31 || o.ObservedAt.Location() != time.UTC || o.GeometryRole != "reported_location" || o.Area != nil || o.Quality["providerConfidence"] != "nominal" || o.Properties["scanKm"] != 0.4 {
		t.Fatalf("semantics %+v", o)
	}
	if page.Records[2].Observation.Quality["providerConfidence"] != "low" {
		t.Fatal("low confidence was erased")
	}
	e, err := normalizeRecord(first, in.Admission, "run", in.Now)
	if err != nil {
		t.Fatal(err)
	}
	if e.OriginalMediaType != "text/csv" || e.RawHash != firmsHash(first.OriginalBytes) || len(e.OriginalRecord) != 0 || !bytes.Equal(e.OriginalBytes, first.OriginalBytes) {
		t.Fatal("exact original CSV retention/hash changed")
	}
	retry, err := DecodeFIRMS(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := normalizeRecord(retry.Records[0], in.Admission, "retry", in.Now.Add(time.Hour))
	if e.ID != again.ID || e.OriginRecordKey != again.OriginRecordKey {
		t.Fatal("overlap/retry changed source identity")
	}
	changed := bytes.Replace(raw, []byte(",VIIRS,n,2.0NRT"), []byte(",VIIRS,h,2.0NRT"), 1)
	correction, err := DecodeFIRMS(changed, in)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := normalizeRecord(correction.Records[0], in.Admission, "next", in.Now.Add(time.Hour))
	if next.ProviderRecordID != e.ProviderRecordID || next.ProviderRevision == e.ProviderRevision {
		t.Fatal("content revision lost pixel lineage")
	}
	if page.Records[1].ProviderRecordID == page.Records[2].ProviderRecordID {
		t.Fatal("distinct overlapping pixels merged")
	}
	in.From = in.From.Add(6*time.Hour + 32*time.Minute)
	filtered, err := DecodeFIRMS(raw, in)
	if err != nil || len(filtered.Records) != 4 || filtered.ExcludedRecords != 1 {
		t.Fatal("exact frozen interval selection failed", err)
	}
}
func TestFIRMSRejectsMalformedProviderRowsAndUnreviewedProduct(t *testing.T) {
	raw, in := firmsFixture(t)
	for name, mutate := range map[string]func([]byte) []byte{
		"invalid_time":       func(b []byte) []byte { return bytes.Replace(b, []byte(",631,"), []byte(",2561,"), 1) },
		"missing_column":     func(b []byte) []byte { return bytes.Replace(b, []byte("latitude,"), []byte("unexpected,"), 1) },
		"invalid_coordinate": func(b []byte) []byte { return bytes.Replace(b, []byte("5.95080,"), []byte("95.95080,"), 1) },
		"unknown_quality":    func(b []byte) []byte { return bytes.Replace(b, []byte(",VIIRS,n,"), []byte(",VIIRS,confirmed,"), 1) },
		"wrong_satellite":    func(b []byte) []byte { return bytes.Replace(b, []byte(",N20,"), []byte(",N,"), 1) },
		"wrong_product":      func(b []byte) []byte { return bytes.Replace(b, []byte(",2.0NRT,"), []byte(",9.0NRT,"), 1) },
		"broken_csv":         func(b []byte) []byte { return append(b, []byte("\"bad\n")...) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFIRMS(mutate(append([]byte{}, raw...)), in); err == nil {
				t.Fatal("malformed row accepted")
			}
		})
	}
	in.Admission.Policy.MaxRecordsPerPage = 2
	if _, err := DecodeFIRMS(raw, in); err == nil {
		t.Fatal("row budget silently truncated data")
	}
}
func TestFIRMSNativeURLCredentialIsolationDayCursorAndEmptyResult(t *testing.T) {
	raw, in := firmsFixture(t)
	in.To = in.From.Add(36 * time.Hour)
	location := Location{ID: "firms", URL: FIRMSAreaURL, Adapter: FIRMSAdapter, token: "fixture_key_1234567890"}
	fetcher := NewHTTPFetcher()
	calls := 0
	fetcher.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Authorization") != "" || req.URL.RawQuery != "" || !strings.Contains(req.URL.Path, "/VIIRS_NOAA20_NRT/99,5,102,14/1/2025-06-04") || req.Header.Get("Accept") != "text/csv" {
			t.Fatal("native request/credential contract changed")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
	page, meta, err := fetcher.Fetch(context.Background(), location, in)
	if err != nil || calls != 1 || !page.HasMore || meta.ResponseHash != firmsHash(raw) {
		t.Fatal("native fetch failed", err)
	}
	wire, _ := json.Marshal(meta)
	if bytes.Contains(wire, []byte(location.token)) {
		t.Fatal("credential leaked in retrieval metadata")
	}
	in.Checkpoint = page.Checkpoint
	request, err := firmsURL(location, in)
	if err != nil || !strings.HasSuffix(request, "2025-06-05") {
		t.Fatal("day cursor did not advance", err)
	}
	header := raw[:bytes.IndexByte(raw, '\n')+1]
	empty, err := DecodeFIRMS(header, in)
	if err != nil || empty.HasMore || len(empty.Records) != 0 {
		t.Fatal("header-only day handling", err)
	}
	in.From = in.From.Add(time.Second)
	if _, err = firmsURL(location, in); err == nil {
		t.Fatal("partial cursor accepted a different frozen window")
	}
}

func TestFIRMSCSVRetentionRightsAndByteBudget(t *testing.T) {
	raw, in := firmsFixture(t)
	page, err := DecodeFIRMS(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Admission.Rights.RetainRaw = false
	e, err := normalizeRecord(page.Records[0], in.Admission, "run", in.Now)
	if err != nil || len(e.OriginalBytes) != 0 || len(e.OriginalRecord) != 0 || e.RawHash == "" {
		t.Fatalf("reference-only CSV retained original: %+v %v", e, err)
	}
	in.Admission.Policy.MaxResponseBytes = len(raw) - 1
	if _, err = DecodeFIRMS(raw, in); err == nil {
		t.Fatal("decoder byte bound ignored")
	}
}
