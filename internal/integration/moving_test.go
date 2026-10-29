package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func TestP3NativeMovingEvidenceSegmentsAndSourceIsolation(t *testing.T) {
	f := newPublicFixture(t)
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.OpenSkyURL, Adapter: publicingest.OpenSkyAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	f.admission.Dataset = publicingest.OpenSkyDataset
	f.admission.ProductVersion = "v1"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}
	f.admission.Policy.LookbackSeconds = 300
	f.admission.Policy.MinIntervalSeconds = 60
	f.admission.Policy.MaxRecordsPerPage = 10
	f.admission.Policy.MaxResponseBytes = 1048576
	f.admission.Rights.AllowAnalysis = true
	f.admit(t)
	at := time.Now().UTC().Add(-45 * time.Second).Truncate(time.Second)
	counter := 0
	reports := []struct {
		at  time.Time
		lng float64
	}{{at, 39.2}, {at.Add(20 * time.Second), 39.21}, {at.Add(5 * time.Second), 39.201}, {at.Add(21 * time.Second), 39.99}}
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		index := min(counter, len(reports)-1)
		counter++
		r := reports[index]
		raw, _ := json.Marshal(map[string]any{"time": in.Now.Unix(), "states": []any{[]any{"abcdef", "SYNTHETIC123", "Provider inference", r.at.Unix(), r.at.Add(time.Second).Unix(), r.lng, 21.4, 8000, false, 200, 90, nil, nil, nil, nil, false, 0}}})
		page, err := publicingest.DecodeOpenSky(raw, in)
		return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, err
	})
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		code, b := scopeRequest(t, f.h, token, method, path, in, nil)
		if code != want {
			t.Fatalf("%s %s %d %v", method, path, code, b)
		}
		return b
	}
	for i := 0; i < 4; i++ {
		f.ready(t)
		run := f.execute(t)
		if run["newEvidence"] != float64(1) {
			t.Fatal(run)
		}
	}
	list := call(f.manager, "GET", "/api/v1/external-tracks", nil, 200)
	if list["total"] != float64(1) {
		t.Fatal(list)
	}
	track := list["items"].([]any)[0].(map[string]any)
	id := track["id"].(string)
	if track["controllable"] != false || track["identityClaim"] != "abcdef" || track["reported"].(map[string]any)["position"].(map[string]any)["lng"] != 39.21 {
		t.Fatal("late/quarantined report moved current state", track)
	}
	history := call(f.manager, "GET", "/api/v1/external-tracks/"+id+"/associations", nil, 200)
	decisions := map[string]int{}
	for _, v := range history["items"].([]any) {
		decisions[v.(map[string]any)["decision"].(string)]++
	}
	if decisions["APPLIED"] != 2 || decisions["LATE"] != 1 || decisions["QUARANTINED"] != 1 {
		t.Fatal(decisions)
	}
	f.ready(t)
	duplicate := f.execute(t)
	if duplicate["newEvidence"] != float64(0) {
		t.Fatal("duplicate vector inflated evidence", duplicate)
	}
	restricted := scopeToken(t, f.h, "external-other-source", f.resource, []string{"operational.read", "tracks.read", "observations.read", "sources.read"}, []string{"unrelated"})
	for _, path := range []string{"/api/v1/external-tracks/" + id, "/api/v1/tracks/" + id} {
		call(restricted, "GET", path, nil, 404)
	}
	if b := call(restricted, "GET", "/api/v1/tracks", nil, 200); b["total"] != float64(0) {
		t.Fatal("generic track list bypassed source visibility", b)
	}
	if b := call(restricted, "GET", "/api/v1/tracks/"+id+"/history", nil, 200); b["total"] != float64(0) {
		t.Fatal("track history bypassed source visibility", b)
	}
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	if err := store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"assets.manage", "commands.issue"}}); err != nil {
		t.Fatal(err)
	}
	call(f.manager, "POST", "/api/v1/assets", map[string]any{"id": id, "name": "forbidden public controlled actor", "type": "aircraft", "sourceId": f.source}, 403)
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE external_track_associations SET document='{}' WHERE track_id=$1`, id); err == nil {
		t.Fatal("association history was mutable")
	}
	// Move only the fixture projection clock backwards to exercise a later capture
	// after a reception gap. Exact provider evidence is left immutable.
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE external_track_segments SET first_seen_at=now()-interval '2 hours',last_seen_at=now()-interval '1 hour' WHERE track_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	f.ready(t)
	reports[3].at = time.Now().Add(-time.Second)
	reports[3].lng = 39.22
	f.execute(t)
	if b := call(f.manager, "GET", "/api/v1/external-tracks", nil, 200); b["total"] != float64(2) {
		t.Fatal("long gap did not segment identity", b)
	}

	// A full admitted moving page must not create a permanent publication deficit.
	// Projection replay remains available without refetching provider data.
	f.admission.Policy.MaxRecordsPerPage = 200
	f.admit(t)
	f.ready(t)
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		rows := []any{}
		for i := 1; i <= 200; i++ {
			rows = append(rows, []any{fmt.Sprintf("%06x", i), "SYNTHETIC BATCH", "Provider inference", in.Now.Add(-time.Second).Unix(), in.Now.Add(-time.Second).Unix(), 39.2, 21.4, 8000, false, 180, 90, nil, nil, nil, nil, false, 0})
		}
		raw, _ := json.Marshal(map[string]any{"time": in.Now.Unix(), "states": rows})
		page, err := publicingest.DecodeOpenSky(raw, in)
		return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, err
	})
	if run := f.execute(t); run["newEvidence"] != float64(200) {
		t.Fatal("full native page not retained", run)
	}
	var pending int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM public_evidence_outbox WHERE source_id=$1 AND delivered_at IS NULL`, f.source).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending > 0 {
		call(f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, 200)
	}
	var associations int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM external_track_associations WHERE source_id=$1`, f.source).Scan(&associations); err != nil || associations != 205 {
		t.Fatal("full native batch left unpublished", associations, err)
	}
	for _, table := range []string{"assets", "commands", "classifications"} {
		var n int
		if err := f.h.pool.QueryRow(context.Background(), fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&n); err != nil || n != 0 {
			t.Fatal("public projection generated authoritative/control state", table, n, err)
		}
	}
}

func TestP3AISNativePartialCaptureAndRecovery(t *testing.T) {
	f := newPublicFixture(t)
	t.Setenv("EMAD_AIS_FIXTURE_KEY", "server-fixture-key")
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.AISURL, Adapter: publicingest.AISAdapter, AuthTokenEnv: "EMAD_AIS_FIXTURE_KEY", MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	f.admission.Dataset = publicingest.AISDataset
	f.admission.ProductVersion = "v1"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}
	f.admission.Policy.LookbackSeconds = 300
	f.admission.Policy.MinIntervalSeconds = 10
	f.admission.Policy.TimeoutSeconds = 6
	f.admission.Policy.MaxResponseBytes = 1048576
	f.admit(t)
	at := time.Now().UTC().Add(-10 * time.Second)
	fail := false
	empty := false
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		if fail {
			return publicingest.Page{}, publicingest.Retrieval{}, &publicingest.RetrievalError{Code: "ais_connection_unavailable"}
		}
		raw, _ := json.Marshal(map[string]any{"MessageType": "PositionReport", "MetaData": map[string]any{"MMSI": 123456789, "ShipName": "SYNTHETIC FIXTURE", "time_utc": at.Format(time.RFC3339Nano)}, "Message": map[string]any{"PositionReport": map[string]any{"UserID": 123456789, "Valid": true, "Latitude": 21.4, "Longitude": 39.2, "Sog": 10, "Cog": 360, "TrueHeading": 511, "PositionAccuracy": true}}})
		record, e := publicingest.DecodeAIS(raw, in)
		if e != nil {
			return publicingest.Page{}, publicingest.Retrieval{}, e
		}
		records := []publicingest.Record{}
		if !empty && record != nil {
			records = append(records, *record)
		}
		return publicingest.Page{Records: records, CoverageFrom: in.Now, CoverageTo: in.Now.Add(time.Millisecond), PartialReason: func() string {
			if empty {
				return ""
			}
			return "ais_disconnected"
		}()}, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 101, Discontinuous: true, CompressionEnabled: true, CoverageMeaning: "Fixture bounded stream with gaps"}, nil
	})
	first := f.execute(t)
	if first["status"] != "PARTIAL" || first["newEvidence"] != float64(1) || first["retrieval"].(map[string]any)["discontinuous"] != true {
		t.Fatal(first)
	}
	code, list := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/external-tracks", nil, nil)
	if code != 200 || list["total"] != float64(1) {
		t.Fatal(code, list)
	}
	track := list["items"].([]any)[0].(map[string]any)
	reported := track["reported"].(map[string]any)
	if track["kind"] != "vessel" || track["identityClaim"] != "123456789" || reported["course"] != nil || reported["speedMps"].(float64) < 5.14 || track["controllable"] != false {
		t.Fatal(track)
	}
	fail = true
	f.ready(t)
	if run := f.execute(t); run["status"] != "FAILED" || run["failureCode"] != "ais_connection_unavailable" {
		t.Fatal(run)
	}
	_, list = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/external-tracks", nil, nil)
	if list["items"].([]any)[0].(map[string]any)["coverageState"] != "partial_capture_or_source_failure" {
		t.Fatal(list)
	}
	fail = false
	empty = true
	f.ready(t)
	if run := f.execute(t); run["status"] != "SUCCEEDED" || run["newEvidence"] != float64(0) || run["captureTo"] == nil {
		t.Fatal(run)
	}
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE external_track_segments SET last_seen_at=now()-interval '5 minutes' WHERE track_id=$1`, track["id"]); err != nil {
		t.Fatal(err)
	}
	_, stale := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/external-tracks/"+track["id"].(string), nil, nil)
	if stale["status"] != "lost" {
		t.Fatal("stale track shown live", stale)
	}
	f.admission.Enabled = false
	f.admit(t)
	_, disabled := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/external-tracks/"+track["id"].(string), nil, nil)
	if disabled["coverageState"] != "source_disabled_or_rights_expired" {
		t.Fatal(disabled)
	}

	// A source stops at a finite retained-byte budget. The rejected page
	// cannot partially advance evidence or association state.
	f.admission.Enabled = true
	f.admit(t)
	f.ready(t)
	if _, err := f.h.pool.Exec(context.Background(), `INSERT INTO public_evidence(id,scope_id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document) SELECT 'quota_'||n::text,e.scope_id,e.source_id,e.admission_id,e.connector_run_id,'quota:'||n::text,e.provider_revision,e.content_hash,'{}'::jsonb FROM public_evidence e CROSS JOIN generate_series(1,9999)n WHERE e.id=$1`, track["latestEvidenceId"]); err != nil {
		t.Fatal(err)
	}
	empty = false
	at = time.Now().UTC().Add(-time.Second)
	// Historical row count no longer blocks a bounded new capture. Byte and
	// active-work limits are independent of the immutable lifetime history.
	if run := f.execute(t); run["status"] != "PARTIAL" || run["newEvidence"] != float64(1) {
		t.Fatal("legacy row ceiling still blocked moving capture", run)
	}
	// Synthetic byte-budget records have no publication catalog entries and
	// do not fabricate provider observations. Each retained raw object respects
	// the per-record SQL bound; together they exhaust the source byte budget.
	if _, err := f.h.pool.Exec(context.Background(), `INSERT INTO public_evidence(id,scope_id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document,original_record) SELECT 'byte_quota_'||n,e.scope_id,e.source_id,e.admission_id,e.connector_run_id,'byte-quota:'||n,e.provider_revision,e.content_hash,'{"synthetic":true,"purpose":"storage-budget-fixture"}'::jsonb,convert_to(repeat('x',262144),'UTF8') FROM public_evidence e CROSS JOIN generate_series(1,256)n WHERE e.id=$1`, track["latestEvidenceId"]); err != nil {
		t.Fatal(err)
	}
	f.ready(t)
	at = time.Now().UTC().Add(-time.Second)
	if run := f.execute(t); run["status"] != "FAILED" || run["failureCode"] != "source_storage_budget" || run["newEvidence"] != float64(0) {
		t.Fatal("storage budget was not atomic", run)
	}
	var n int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM public_evidence WHERE source_id=$1`, f.source).Scan(&n); err != nil || n != 10257 {
		t.Fatal(n, err)
	}
}
