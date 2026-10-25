package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGeographicMapExactScopesHistoryAggregatesAndIsolation(t *testing.T) {
	f := newPublicFixture(t)
	f.admission.Policy.MaxRecordsPerPage = 1000
	f.admission.Policy.MaxResponseBytes = 4 * 1024 * 1024
	f.admit(t)
	polygon := json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[46,25],[46,24]]]}`)
	raw, _ := json.Marshal(reference.Collection{Type: "FeatureCollection", Features: []reference.Feature{{Type: "Feature", Geometry: polygon, Properties: map[string]any{"key": "country:TEST", "parentKey": "", "name": "Synthetic test jurisdiction", "kind": "country", "aliases": "اختبار"}}, {Type: "Feature", Geometry: polygon, Properties: map[string]any{"key": "region:TEST-1", "parentKey": "country:TEST", "name": "Test region", "kind": "region"}}}})
	in := reference.GeographyInput{SourceID: f.source, Label: "Isolated synthetic navigation", Version: "v1", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"synthetic":true,"retrievedAt":"2026-01-01T00:00:00Z"}`), Artifact: raw, SHA256: reference.Hash(raw)}
	svc := reference.NewService(f.h.pool)
	admitted, duplicate, e := svc.AdmitGeography(scopeContext(t, f.h, f.resource), in)
	if e != nil || duplicate || admitted.Features != 2 {
		t.Fatalf("geography admission: %+v %v", admitted, e)
	}
	if _, duplicate, e = svc.AdmitGeography(scopeContext(t, f.h, f.resource), in); e != nil || !duplicate {
		t.Fatalf("geography retry: %v %v", duplicate, e)
	}
	code, b := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/geography/regions?kind=country", nil, nil)
	if code != 200 || len(b["features"].([]any)) != 1 {
		t.Fatalf("countries: %d %v", code, b)
	}
	region := b["features"].([]any)[0].(map[string]any)["id"].(string)
	code, b = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/geography/regions?parent=country:TEST", nil, nil)
	if code != 200 || len(b["features"].([]any)) != 1 {
		t.Fatalf("children: %d %v", code, b)
	}
	code, b = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/geography/regions?q="+url.QueryEscape("اختبار"), nil, nil)
	if code != 200 || len(b["features"].([]any)) != 1 {
		t.Fatalf("local-name search: %d %v", code, b)
	}
	for _, badQuery := range []string{"offset=invalid", "offset=-1", "id=" + url.QueryEscape(string(make([]byte, 260)))} {
		if code, _ := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/geography/regions?"+badQuery, nil, nil); code != 400 {
			t.Fatalf("invalid geography query %s: %d", badQuery, code)
		}
	}
	bad := in
	bad.SHA256 = "invalid"
	if _, _, e = svc.AdmitGeography(scopeContext(t, f.h, f.resource), bad); e == nil {
		t.Fatal("wrong retained hash admitted")
	}
	bad = in
	bad.Artifact = []byte(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[46,24],[47,25],[47,24],[46,25],[46,24]]]},"properties":{"key":"country:INVALID","parentKey":"","kind":"country","name":"Invalid self intersection"}}]}`)
	bad.SHA256 = reference.Hash(bad.Artifact)
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/editions", bad, nil); code != 400 {
		t.Fatalf("invalid polygon admission: %d", code)
	}
	if code, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/geography/editions", in, nil); code != 403 {
		t.Fatalf("worker admitted navigation: %d", code)
	}
	serviceAdmitter := scopeToken(t, f.h, "geography-service-manager", f.resource, []string{"sources.read", "sources.manage"}, []string{f.source})
	if code, _ := scopeRequest(t, f.h, serviceAdmitter, "POST", "/api/v1/geography/editions", in, nil); code != 403 {
		t.Fatalf("service admitted human-reviewed navigation: %d", code)
	}
	original, e := os.ReadFile("../../testdata/reference/jeddah-makkah/places.json")
	if e != nil {
		t.Fatal(e)
	}
	var referenceInput reference.Input
	if e = json.Unmarshal(original, &referenceInput); e != nil {
		t.Fatal(e)
	}
	referenceInput.SourceID = f.source
	dataset, _, e := svc.Import(scopeContext(t, f.h, f.resource), referenceInput)
	if e != nil {
		t.Fatal(e)
	}
	tile := func(token string) (int, []byte, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", f.h.server.URL+"/api/v1/map/reference/0/0/0.pbf?datasets="+url.QueryEscape(dataset.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		return response.StatusCode, raw, response.Header.Get("Cache-Control")
	}
	code, tileBytes, cache := tile(f.manager)
	if code != 200 || len(tileBytes) == 0 || !strings.Contains(cache, "no-store") {
		t.Fatalf("retained vector tile: status=%d bytes=%d cache=%s", code, len(tileBytes), cache)
	}
	tileReader := scopeToken(t, f.h, "restricted-map-tiles", f.resource, []string{"sources.read"}, []string{"not-this-source"})
	code, tileBytes, _ = tile(tileReader)
	if code != 200 || len(tileBytes) != 0 {
		t.Fatalf("source-hidden tile geometry disclosed: status=%d bytes=%d", code, len(tileBytes))
	}
	code, buffer := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/map/surrounding-area", map[string]any{"longitude": 46.2, "latitude": 24.2, "radiusM": 5000}, nil)
	if code != 200 || buffer["geometry"].(map[string]any)["type"] != "Polygon" {
		t.Fatalf("geodesic context buffer: %d %v", code, buffer)
	}
	for _, body := range []map[string]any{{"radiusM": 5000}, {"longitude": 46., "latitude": 24., "radiusM": 999999}, {"longitude": 179.99, "latitude": 0., "radiusM": 50000}} {
		if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/map/surrounding-area", body, nil); code != 400 {
			t.Fatalf("ambiguous/unbounded context buffer accepted: %d", code)
		}
	}
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	report := func(id, revision string, lng, lat float64, withdrawn bool) publicingest.Record {
		return publicingest.Record{ProviderRecordID: id, ProviderRevision: revision, ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/record", OriginalRecord: json.RawMessage(`{"synthetic":true}`), Observation: &publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Synthetic map protocol report", ObservedAt: at, Position: &geo.Point{Lng: lng, Lat: lat}, GeometryRole: "reported_location", Withdrawn: withdrawn, Properties: map[string]any{}, Quality: map[string]any{}}}
	}
	ingest := func(records []publicingest.Record) {
		t.Helper()
		checkpoint := "done"
		if f.admission.Dataset == publicingest.METARDataset {
			checkpoint = ""
		}
		f.fetcher.set(publicingest.Page{Records: records, Checkpoint: checkpoint}, nil)
		f.ready(t)
		run := f.execute(t)
		if run["status"] != "SUCCEEDED" {
			t.Fatalf("ingest: %v", run)
		}
		if len(records) > 100 {
			for i := 0; i < 5; i++ {
				code, b := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, nil)
				if code != 200 {
					t.Fatalf("publication drain: %d %v", code, b)
				}
				if b["published"] == float64(0) {
					break
				}
			}
		}
	}
	ingest([]publicingest.Record{report("inside", "1", 46.1, 24.1, false), report("bbox-neighbor", "1", 46.9, 24.9, false), report("boundary", "1", 46.5, 24.5, false)})
	query := func(parameters url.Values, want int) map[string]any {
		t.Helper()
		parameters.Set("family", "earthquakes")
		code, b := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/map/features?"+parameters.Encode(), nil, nil)
		if code != 200 || b["total"] != float64(want) {
			t.Fatalf("map query %v: status=%d total=%v want=%d state=%v error=%v", parameters, code, b["total"], want, b["state"], b["error"])
		}
		return b
	}
	query(url.Values{"bbox": {"46,24,47,25"}}, 3)
	query(url.Values{"region_id": {region}}, 2)
	query(url.Values{"geometry": {string(polygon)}}, 2)
	var cutoff time.Time
	if e = f.h.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&cutoff); e != nil {
		t.Fatal(e)
	}
	at = at.Add(time.Second)
	ingest([]publicingest.Record{report("inside", "2", 46.8, 24.8, false), report("boundary", "2", 46.5, 24.5, true)})
	query(url.Values{"region_id": {region}}, 0)
	query(url.Values{"region_id": {region}, "view": {"known_at"}, "as_of": {cutoff.UTC().Format(time.RFC3339Nano)}}, 2)
	for batch := 0; batch < 3; batch++ {
		records := []publicingest.Record{}
		for i := 0; i < 710; i++ {
			records = append(records, report(fmt.Sprintf("bulk-%d-%d", batch, i), "1", 46.05+float64(i%37)/200, 24.05+float64(i/37)/200, false))
		}
		ingest(records)
	}
	aggregated := query(url.Values{"region_id": {region}, "zoom": {"16"}}, 2130)
	if aggregated["state"] != "aggregated" || aggregated["truncated"] != false {
		t.Fatalf("incomplete overview: %v", aggregated)
	}
	sum := 0.
	for _, feature := range aggregated["features"].([]any) {
		sum += feature.(map[string]any)["properties"].(map[string]any)["count"].(float64)
	}
	if sum != 2130 {
		t.Fatalf("aggregate count lost records: %.0f", sum)
	}
	if len(aggregated["features"].([]any)) > 1700 {
		t.Fatal("aggregate geometry exceeds display budget")
	}
	weatherFixture := *f
	code, weatherSource := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources", map[string]any{"name": "Synthetic map station fixture", "type": "external"}, nil)
	if code != 201 {
		t.Fatalf("weather source %d %v", code, weatherSource)
	}
	weatherFixture.source = weatherSource["id"].(string)
	weatherFixture.worker = scopeToken(t, f.h, "map-weather-worker", f.resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{weatherFixture.source})
	weatherFixture.admission.ConnectorPrincipalID = "map-weather-worker"
	weatherFixture.admission.Policy.MaxRecordsPerPage = 200
	setupContext(t, &weatherFixture, false)
	f = &weatherFixture
	for batch := 0; batch < 15; batch++ {
		records := []publicingest.Record{}
		for i := 0; i < 142; i++ {
			index := batch*142 + i
			records = append(records, publicingest.Record{ProviderRecordID: fmt.Sprintf("station-%d", index), ProviderRevision: "1", ReferenceURL: publicingest.METARURL, OriginalRecord: json.RawMessage(`{"synthetic":true}`), Context: &publicingest.ContextRecord{Kind: "WEATHER", Title: "Synthetic station", Position: geo.Point{Lng: 46.1 + float64(i%37)/300, Lat: 24.1 + float64(i/37)/300}, LocationBasis: "Synthetic fixture station", TimeBasis: "Fixture observation time", From: at, To: at, ExtractionMethod: "Fixture", Uncertainty: "Synthetic isolated test record", Weather: &publicingest.StationWeather{StationID: fmt.Sprintf("S%06d", index), RawMETAR: "SYNTHETIC TEST", ReportType: "METAR"}}})
		}
		ingest(records)
	}
	code, weather := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/map/features?family=weather&region_id="+url.QueryEscape(region), nil, nil)
	if code != 200 || weather["total"] != float64(2130) || weather["state"] != "aggregated" {
		t.Fatalf("generic aggregates: status=%d total=%v state=%v error=%v", code, weather["total"], weather["state"], weather["error"])
	}
	total := 0.
	for _, feature := range weather["features"].([]any) {
		total += feature.(map[string]any)["properties"].(map[string]any)["count"].(float64)
	}
	if total != 2130 {
		t.Fatalf("generic aggregate total %.0f", total)
	}

	code, b = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/map/measure", map[string]any{"geometry": json.RawMessage(`{"type":"LineString","coordinates":[[46,24],[46.01,24]]}`)}, nil)
	if code != 200 || b["lengthM"].(float64) < 1000 || b["lengthM"].(float64) > 1050 {
		t.Fatalf("WGS84 distance: %d %v", code, b)
	}
	code, b = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/map/measure", map[string]any{"geometry": polygon}, nil)
	if code != 200 || b["areaM2"].(float64) <= 0 {
		t.Fatalf("geodesic area: %d %v", code, b)
	}
	if code, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/map/measure", map[string]any{"geometry": json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,25],[47,24],[46,25],[46,24]]]}`)}, nil); code != 400 {
		t.Fatalf("invalid study polygon measured: %d", code)
	}
	other := scopeToken(t, f.h, "source-restricted-geography", f.resource, []string{"sources.read", "observations.read"}, []string{"not-this-source"})
	code, b = scopeRequest(t, f.h, other, "GET", "/api/v1/geography/regions", nil, nil)
	if code != 200 || len(b["features"].([]any)) != 0 {
		t.Fatalf("geographic source isolation: %d %v", code, b)
	}
	if code, _ = scopeRequest(t, f.h, other, "GET", "/api/v1/map/features?family=earthquakes&region_id="+url.QueryEscape(region), nil, nil); code != 404 {
		t.Fatalf("source-hidden scope disclosed: %d", code)
	}
	foreign := f.resource
	foreign.WorkspaceID = "foreign-geography"
	var count int
	if e = db.Scoped(f.h.pool).QueryRow(scopeContext(t, f.h, foreign), `SELECT count(*) FROM geographic_regions`).Scan(&count); e != nil || count != 0 {
		t.Fatalf("cross-workspace geography: %d %v", count, e)
	}
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE geographic_editions SET document='{}' WHERE id=$1`, admitted.ID); e == nil {
		t.Fatal("retained edition mutable")
	}
	for _, partition := range []access.Partition{access.ExerciseInput, access.ExerciseObservation, access.ExerciseTruth} {
		r := f.resource
		r.Partition = partition
		if partition != access.ExerciseInput {
			r.RunID = "private-run"
		}
		_, e = svc.MapFeatures(scopeContext(t, f.h, r), "weather", url.Values{})
		if e == nil {
			t.Fatalf("public map accepted %s", partition)
		}
	}
}
