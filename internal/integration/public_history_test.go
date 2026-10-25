package integration

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

type nativeFixtureFunc func(context.Context, publicingest.Location, publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error)

func (f nativeFixtureFunc) Fetch(ctx context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
	return f(ctx, l, in)
}

// M1 uses native provider decoders and the real source admission, publication,
// PostGIS, history, rights and HTTP paths. NASA's published example is explicitly
// mutated to this test's clock/AOI: these are synthetic reports, not Saudi fires.
func TestM1NativeFIRMSUSGSReferenceAndBitemporalEvidence(t *testing.T) {
	f := newPublicFixture(t)
	t.Setenv("EMAD_TEST_FIRMS_KEY", "fixture_key_not_a_real_credential")
	at := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Minute)
	raw, err := os.ReadFile("../publicingest/testdata/firms/noaa20-published-example.csv")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	confidence, malformed := "n", false
	withdrawn := false
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		if l.Adapter == publicingest.USGSAdapter {
			status := "reviewed"
			updated := at.Add(time.Minute)
			var geometry any = map[string]any{"type": "Point", "coordinates": []float64{39.5, 21.5, 12}}
			if withdrawn {
				status = "deleted"
				updated = at.Add(2 * time.Minute)
				geometry = nil
			}
			document, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "metadata": map[string]any{"status": 200, "count": 1}, "features": []any{map[string]any{"type": "Feature", "id": "synthetic-quake", "geometry": geometry, "properties": map[string]any{"time": at.UnixMilli(), "updated": updated.UnixMilli(), "status": status, "type": "earthquake", "title": "Synthetic USGS protocol fixture", "mag": 3.5}}}})
			page, e := publicingest.DecodeUSGS(document, in)
			return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200, ResponseHash: fmt.Sprintf("%x", sha256.Sum256(document))}, e
		}
		day := in.From.UTC().Truncate(24 * time.Hour)
		if in.Checkpoint != "" {
			var c struct {
				Day      time.Time
				Complete bool
			}
			_ = json.Unmarshal([]byte(in.Checkpoint), &c)
			if !c.Complete {
				day = c.Day
			}
		}
		var body strings.Builder
		writer := csv.NewWriter(&body)
		_ = writer.Write(rows[0])
		if day.Equal(at.Truncate(24 * time.Hour)) {
			row := append([]string{}, rows[1]...)
			row[0] = "21.5"
			row[1] = "39.2"
			row[5] = at.Format("2006-01-02")
			row[6] = at.Format("1504")
			row[9] = confidence
			if malformed {
				row[9] = "not-a-provider-class"
			}
			_ = writer.Write(row)
		}
		writer.Flush()
		document := []byte(body.String())
		page, e := publicingest.DecodeFIRMS(document, in)
		return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200, ResponseHash: fmt.Sprintf("%x", sha256.Sum256(document))}, e
	})
	register := func(source, id, adapter, endpoint, key string) {
		t.Helper()
		entries, e := publicingest.NewRegistry([]publicingest.Location{{ID: id, SourceID: source, Resource: f.resource, URL: endpoint, Adapter: adapter, AuthTokenEnv: key, MaxRequestsPerDay: 20}})
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range entries {
			f.registry[k] = v
		}
	}
	register(f.source, "fixture-location", publicingest.FIRMSAdapter, publicingest.FIRMSAreaURL, "EMAD_TEST_FIRMS_KEY")
	f.admission.Dataset = publicingest.FIRMSDataset
	f.admission.ProductVersion = publicingest.FIRMSProductVersion
	f.admission.RevisionPolicy = "content_hash"
	f.admission.OriginalProducer = "NASA LANCE FIRMS synthetic fixture"
	f.admission.UpstreamProvider = "FIRMS NOAA-20 protocol fixture"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}
	f.admission.Policy.LookbackSeconds = 3600
	f.admission.Policy.MinIntervalSeconds = 60
	f.admission.Policy.MaxResponseBytes = 65536
	f.admit(t)
	executeComplete := func(target *publicFixture) (int, int) {
		t.Helper()
		fresh, duplicates := 0, 0
		for i := 0; i < 3; i++ {
			target.ready(t)
			run := target.execute(t)
			fresh += int(run["newEvidence"].(float64))
			duplicates += int(run["duplicates"].(float64))
			if run["status"] == "SUCCEEDED" {
				return fresh, duplicates
			}
			if run["status"] != "PARTIAL" {
				t.Fatalf("native feed: %v", run)
			}
		}
		t.Fatal("bounded calendar-day cursor did not finish")
		return 0, 0
	}
	if n, _ := executeComplete(f); n != 1 {
		t.Fatalf("FIRMS publication count %d", n)
	}
	query := func(parameters url.Values, want int) map[string]any {
		t.Helper()
		status, body := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events?"+parameters.Encode(), nil, nil)
		if status != 200 || body["total"] != float64(want) {
			t.Fatalf("query %v: %d %v", parameters, status, body)
		}
		return body
	}
	first := query(url.Values{}, 1)["items"].([]any)[0].(map[string]any)
	oldID := first["evidenceId"].(string)
	reported := first["reported"].(map[string]any)
	if reported["type"] != "thermal.detection" || reported["area"] != nil || first["qualityClass"] != "nominal" || first["effectiveBasis"] != "reported_observation_no_revision_time" {
		t.Fatalf("thermal semantics: %v", first)
	}
	if n, d := executeComplete(f); n != 0 || d != 1 {
		t.Fatalf("overlap dedup: new=%d duplicates=%d", n, d)
	}
	var cutoff time.Time
	if err = f.h.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&cutoff); err != nil {
		t.Fatal(err)
	}
	confidence = "l"
	if n, _ := executeComplete(f); n != 1 {
		t.Fatalf("quality correction missing %d", n)
	}
	// A second source arrives after cutoff, but its origin/revision was earlier.
	usgs := *f
	status, source := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources", map[string]any{"name": "Synthetic USGS", "type": "external"}, nil)
	if status != 201 {
		t.Fatal(source)
	}
	usgs.source = source["id"].(string)
	usgs.worker = scopeToken(t, f.h, "m1-usgs-worker", f.resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{usgs.source})
	usgs.admission = f.admission
	usgs.admission.OriginalProducer = "USGS synthetic fixture"
	usgs.admission.UpstreamProvider = "Synthetic ComCat protocol fixture"
	usgs.admission.ConnectorID = "m1-usgs"
	usgs.admission.ConnectorPrincipalID = "m1-usgs-worker"
	usgs.admission.Dataset = "usgs-earthquakes"
	usgs.admission.ProductVersion = "geojson-summary-v1"
	usgs.admission.RevisionPolicy = "provider_version"
	usgs.admission.Rights.ID = "m1-usgs-rights"
	register(usgs.source, "m1-usgs", publicingest.USGSAdapter, publicingest.USGSQueryURL, "")
	usgs.admit(t)
	if n, _ := executeComplete(&usgs); n != 1 {
		t.Fatalf("USGS publication count %d", n)
	}
	query(url.Values{}, 2)
	// Latest is selected BEFORE quality/geometry/time filters; corrections must
	// never resurrect an old matching report.
	query(url.Values{"quality": {"nominal"}}, 0)
	query(url.Values{"quality": {"low"}, "phenomenon": {"thermal.detection"}, "source_id": {f.source}, "bbox": {"39,21,39.3,22"}, "observed_from": {at.Add(-time.Second).Format(time.RFC3339Nano)}, "observed_to": {at.Add(time.Second).Format(time.RFC3339Nano)}}, 1)
	query(url.Values{"quality": {"reviewed"}}, 1)
	query(url.Values{"bbox": {"39.8,21.8,40,22"}}, 0)
	query(url.Values{"revisions": {"all"}}, 3)
	paged := query(url.Values{"limit": {"1"}, "offset": {"99"}}, 2)
	if len(paged["items"].([]any)) != 0 {
		t.Fatal("offset query lost total or returned extra rows")
	}
	known := url.Values{"view": {"known_at"}, "as_of": {cutoff.UTC().Format(time.RFC3339Nano)}}
	knownList := query(known, 1)
	if knownList["items"].([]any)[0].(map[string]any)["evidenceId"] != oldID {
		t.Fatal("known-at view leaked later correction")
	}
	effective := url.Values{"view": {"effective_at"}, "as_of": {cutoff.UTC().Format(time.RFC3339Nano)}}
	query(effective, 2)
	current := query(url.Values{"source_id": {f.source}}, 1)["items"].([]any)[0].(map[string]any)
	currentID := current["evidenceId"].(string)
	if s, _ := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events/"+currentID+"?"+known.Encode(), nil, nil); s != 404 {
		t.Fatalf("future detail leaked %d", s)
	}
	if s, hist := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events/"+oldID+"/revisions?"+known.Encode(), nil, nil); s != 200 || hist["total"] != float64(1) {
		t.Fatalf("future history leaked %d %v", s, hist)
	}
	if s, hist := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events/"+currentID+"/revisions?"+known.Encode(), nil, nil); s != 200 || hist["total"] != float64(0) {
		t.Fatalf("invisible anchor leaked history %d %v", s, hist)
	}
	// The retained CSV header and original row have a separate exact-byte hash
	// from parsed content and the full retrieval response.
	req, _ := http.NewRequest("GET", f.h.server.URL+"/api/v1/sources/"+f.source+"/evidence/"+currentID+"/original", nil)
	req.Header.Set("Authorization", "Bearer "+f.manager)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := io.ReadAll(response.Body)
	response.Body.Close()
	_, evidence := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/evidence/"+currentID, nil, nil)
	if err != nil || response.StatusCode != 200 || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/csv") || !strings.Contains(response.Header.Get("Content-Disposition"), ".csv") || evidence["rawHash"] != fmt.Sprintf("%x", sha256.Sum256(exact)) || !strings.Contains(string(exact), ",VIIRS,l,2.0NRT,") {
		t.Fatalf("exact CSV provenance: %d %v %v", response.StatusCode, evidence, err)
	}
	if evidence["originalMediaType"] != "text/csv" || evidence["originalRecord"] != nil || evidence["retrieval"].(map[string]any)["responseHash"] == "" {
		t.Fatal("CSV represented as invented JSON or response hash missing")
	}
	// Real permitted reference versions cover the same single AOI. No observation
	// or identity is synthesized from reference geography.
	for _, name := range []string{"places", "elevation"} {
		raw, err := os.ReadFile("../../testdata/reference/jeddah-makkah/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var in reference.Input
		if json.Unmarshal(raw, &in) != nil {
			t.Fatal("reference fixture")
		}
		in.SourceID = f.source
		if s, b := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", in, nil); s != 201 {
			t.Fatalf("same-AOI reference: %d %v", s, b)
		}
	}
	for _, path := range []string{"tracks"} {
		s, b := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/"+path, nil, nil)
		if s != 200 || b["total"] != float64(0) {
			t.Fatalf("event became controllable or tracked %s %d %v", path, s, b)
		}
	}
	restricted := scopeToken(t, f.h, "m1-usgs-only", f.resource, []string{"operational.read", "observations.read", "sources.read"}, []string{usgs.source})
	if s, b := scopeRequest(t, f.h, restricted, "GET", "/api/v1/public-events", nil, nil); s != 200 || b["total"] != float64(1) {
		t.Fatalf("source RLS leak %d %v", s, b)
	}
	if s, _ := scopeRequest(t, f.h, restricted, "GET", "/api/v1/public-events/"+oldID, nil, nil); s != 404 {
		t.Fatalf("restricted revision leaked %d", s)
	}
	if s, _ := scopeRequest(t, f.h, restricted, "GET", "/api/v1/public-events?source_id="+url.QueryEscape(f.source), nil, nil); s != 403 {
		t.Fatalf("source filter bypass %d", s)
	}
	// Malformed responses do not erase published coverage or immutable revisions.
	malformed = true
	f.ready(t)
	if run := f.execute(t); run["status"] != "FAILED" || run["failureCode"] != "invalid_response" {
		t.Fatalf("malformed response accepted %v", run)
	}
	query(url.Values{}, 2)
	if health := f.health(t); health["state"] != "stale" || health["lastSuccessAt"] == nil {
		t.Fatalf("outage ambiguity %v", health)
	}
	var beforeWithdrawal time.Time
	if err = f.h.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&beforeWithdrawal); err != nil {
		t.Fatal(err)
	}
	withdrawn = true
	if n, _ := executeComplete(&usgs); n != 1 {
		t.Fatalf("withdrawal revision missing %d", n)
	}
	query(url.Values{"source_id": {usgs.source}, "quality": {"reviewed"}}, 0)
	query(url.Values{"phenomenon": {"event.earthquake"}, "bbox": {"39,21,40,22"}}, 0)
	past := query(url.Values{"source_id": {usgs.source}, "view": {"known_at"}, "as_of": {beforeWithdrawal.UTC().Format(time.RFC3339Nano)}}, 1)
	if past["items"].([]any)[0].(map[string]any)["reported"].(map[string]any)["withdrawn"] == true {
		t.Fatal("withdrawal leaked backward in known-time history")
	}
	for _, parameters := range []string{"view=known_at", "view=latest&as_of=2020-01-01T00:00:00Z", "quality=confirmed_fire", "bbox=NaN,21,40,22", "source_id=a&source_id=b"} {
		if s, _ := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events?"+parameters, nil, nil); s != 400 {
			t.Fatalf("ambiguous query accepted %s %d", parameters, s)
		}
	}
	var assets int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM assets`).Scan(&assets); err != nil || assets != 0 {
		t.Fatalf("public events created assets %d %v", assets, err)
	}
}
