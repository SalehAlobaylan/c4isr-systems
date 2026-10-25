package publicingest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUSGSRecordedRevisionsWithdrawalsAndBoundedQuery(t *testing.T) {
	rawFixture, err := os.ReadFile("testdata/usgs/recorded.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Features []struct {
			Properties struct {
				Updated int64 `json:"updated"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err = json.Unmarshal(rawFixture, &fixture); err != nil {
		t.Fatal(err)
	}
	now := time.UnixMilli(fixture.Features[0].Properties.Updated).Add(2 * time.Minute).UTC()
	in := FetchRequest{Now: now, From: now.Add(-48 * time.Hour), To: now, Admission: Admission{SourceID: "usgs", AdmissionInput: AdmissionInput{OriginalProducer: "USGS ComCat", Dataset: "earthquakes", ProductVersion: "geojson-v1", RevisionPolicy: "provider_version", Rights: Rights{RetainRaw: true}, Policy: Policy{AOI: Bounds{West: -68, East: -64, South: 17, North: 20}, MaxRecordsPerPage: 10, MaxResponseBytes: 65536}}}}
	location := Location{ID: "usgs", URL: USGSQueryURL, Adapter: USGSAdapter}
	var first, corrected Evidence
	for _, name := range []string{"recorded", "corrected", "withdrawn", "invalid"} {
		raw, err := os.ReadFile("testdata/usgs/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		fetcher := NewHTTPFetcher()
		fetcher.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			if q.Get("minlongitude") != "-68" || q.Get("includedeleted") != "true" || q.Get("limit") != "10" || q.Get("offset") != "1" || q.Get("eventtype") != "earthquake" {
				t.Fatalf("unbounded/native query changed: %v", q)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
		})
		page, _, err := fetcher.Fetch(context.Background(), location, in)
		if name == "invalid" {
			if err == nil {
				t.Fatal("malformed geometry accepted")
			}
			continue
		}
		if err != nil || len(page.Records) != 1 {
			t.Fatalf("%s: %v %+v", name, err, page)
		}
		e, err := normalizeRecord(page.Records[0], in.Admission, "run", now)
		if err != nil {
			t.Fatal(err)
		}
		if name == "recorded" {
			first = e
			if e.Observation.Properties["depthKm"] != 26.34 || e.Observation.Quality["reviewStatus"] != "reviewed" {
				t.Fatalf("depth/quality lost: %+v", e.Observation)
			}
		}
		if name == "corrected" {
			corrected = e
			if first.ID == e.ID || first.ProviderRecordID != e.ProviderRecordID || !e.ProviderUpdatedAt.After(*first.ProviderUpdatedAt) {
				t.Fatal("correction identity lost")
			}
		}
		if name == "withdrawn" && !e.Observation.Withdrawn {
			t.Fatal("explicit withdrawal lost")
		}
		if name == "withdrawn" && e.Observation.Position != nil {
			t.Fatal("withdrawal manufactured coordinates")
		}
	}
	if corrected.ContentHash == first.ContentHash {
		t.Fatal("corrected raw record not retained")
	}
	in.Admission.Policy.MaxRecordsPerPage = 1
	raw, _ := os.ReadFile("testdata/usgs/recorded.json")
	page, err := DecodeUSGS(raw, in)
	if err != nil || !page.HasMore {
		t.Fatalf("pagination: %v", err)
	}
	in.Checkpoint = page.Checkpoint
	query, err := usgsURL(location, in)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(query)
	if u.Query().Get("offset") != "2" {
		t.Fatal("cursor did not resume")
	}
	var cursor usgsCursor
	json.Unmarshal([]byte(in.Checkpoint), &cursor)
	cursor.Complete = true
	c, _ := json.Marshal(cursor)
	in.Checkpoint = string(c)
	query, _ = usgsURL(location, in)
	u, _ = url.Parse(query)
	if u.Query().Get("offset") != "1" {
		t.Fatal("completed watermark reused a page offset")
	}
}

func TestUSGSUnavailableIsNotAnEmptyPage(t *testing.T) {
	fetcher := NewHTTPFetcher()
	fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": {"60"}}, Body: io.NopCloser(strings.NewReader("provider outage"))}, nil
	})
	_, _, err := fetcher.Fetch(context.Background(), Location{URL: USGSQueryURL, Adapter: USGSAdapter}, FetchRequest{Now: time.Now(), Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{MaxRecordsPerPage: 10}}}})
	if err == nil || err.Error() != "provider_unavailable" {
		t.Fatalf("outage became empty coverage: %v", err)
	}
}

func TestUSGSQueryMetadataMayOmitFeedCount(t *testing.T) {
	raw, err := os.ReadFile("testdata/usgs/query-recorded.json")
	if err != nil {
		t.Fatal(err)
	}
	page, err := DecodeUSGS(raw, FetchRequest{Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{AOI: Bounds{West: -68, East: -64, South: 17, North: 20}, MaxRecordsPerPage: 20}}}})
	if err != nil || len(page.Records) != 5 {
		t.Fatalf("native catalogue metadata rejected: %v records=%d", err, len(page.Records))
	}
}
