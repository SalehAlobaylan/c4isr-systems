package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publiccontext"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/jackc/pgx/v5"
)

// Full declared read volumes coexist in ONE disposable database. Native raster
// concurrency, real provider data, browser timing and 24h remain separate gates.
func TestMapFirstFullVolumeConcurrentQualification(t *testing.T) {
	if os.Getenv("C4ISR_QUALIFICATION_FULL") != "1" {
		t.Skip("explicit full-volume qualification only")
	}
	duration, e := time.ParseDuration(os.Getenv("C4ISR_QUALIFICATION_DURATION"))
	if e != nil || duration < time.Second || duration > 25*time.Hour {
		t.Fatal("bounded measured duration required")
	}
	dir := os.Getenv("C4ISR_TEST_EVIDENCE_DIR")
	if dir == "" {
		t.Fatal("evidence directory required")
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	file, e := os.OpenFile(filepath.Join(dir, "workload.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	var mu sync.Mutex
	encoder := json.NewEncoder(file)
	emit := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		if e := encoder.Encode(v); e != nil {
			t.Error(e)
		}
	}
	f := newPublicFixture(t)
	f.admission.Policy.MaxRequestsPerDay = 10000
	f.admission.Policy.MinIntervalSeconds = 1
	location := f.registry["fixture-location"]
	location.MaxRequestsPerDay = 10000
	f.registry["fixture-location"] = location
	f.admit(t)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	seeds := []publicingest.Record{
		{ProviderRecordID: "full-event-seed", ProviderRevision: "1", ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/acceptance", OriginalRecord: json.RawMessage(`{"synthetic":true,"purpose":"acceptance","prototype":"event"}`), Observation: &publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Acceptance synthetic report", ObservedAt: at, Position: &geo.Point{Lng: 46.01, Lat: 24.01}, GeometryRole: "reported_location", Properties: map[string]any{"synthetic": true, "purpose": "acceptance"}, Quality: map[string]any{}}},
		{ProviderRecordID: "full-weather-seed", ProviderRevision: "1", ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/acceptance", OriginalRecord: json.RawMessage(`{"synthetic":true,"purpose":"acceptance","prototype":"weather"}`), Context: &publicingest.ContextRecord{Kind: "WEATHER", Title: "Acceptance synthetic station", Position: geo.Point{Lng: 46.01, Lat: 24.01}, LocationBasis: "synthetic station", TimeBasis: "synthetic observation", From: at, To: at, ExtractionMethod: "acceptance fixture", Weather: &publicingest.StationWeather{StationID: "Q00000", RawMETAR: "SYNTHETIC ACCEPTANCE", ReportType: "METAR"}}},
	}
	// Use the trusted projector boundary for acceptance seeds. Generic raw JSON
	// intentionally does not turn arbitrary input into normalized public facts.
	worker := deliveryAuth(t, f, f.worker)
	repository := publicingest.NewRepository(f.h.pool)
	admissionID := acquisitionCall(t, f, f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200)["id"].(string)
	run, _, e := repository.Begin(worker, f.source, "connector-fixture", admissionID, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	ctx := scopeContext(t, f.h, f.resource)
	e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		for i, seed := range seeds {
			id := []string{"full_event_0", "full_weather_0"}[i]
			hash := reference.Hash(seed.OriginalRecord)
			evidence := publicingest.Evidence{ID: id, SourceID: f.source, AdmissionID: admissionID, ConnectorRunID: run.ID, ProviderRecordID: seed.ProviderRecordID, ProviderRevision: "1", ContentHash: hash, RawHash: hash, OriginalProducer: "Owned synthetic acceptance fixture", RetrievedAt: at, ProviderUpdatedAt: &at, OriginalRecord: seed.OriginalRecord, Observation: seed.Observation, Context: seed.Context}
			document, _ := json.Marshal(evidence)
			if _, e := tx.Exec(ctx, `INSERT INTO public_evidence(id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document)VALUES($1,$2,$3,$4,$5,'1',$6,$7)`, id, f.source, admissionID, run.ID, seed.ProviderRecordID, hash, document); e != nil {
				return e
			}
			if e := (publiccontext.Projector{Next: publicevents.Projector{}}).Project(ctx, tx, evidence); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `INSERT INTO public_evidence_catalog(evidence_id,source_id)VALUES($1,$2)`, id, f.source); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = repository.Finish(worker, run, "SUCCEEDED", "", time.Now().Add(time.Second), nil, time.Now()); e != nil {
		t.Fatal(e)
	}
	features := []reference.Feature{}
	scopes := []analystscope.Input{}
	for i := 0; i < 10; i++ {
		west, east := 46+float64(i)*.1, 46+float64(i+1)*.1
		geometry := json.RawMessage(fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%f,24],[%f,24],[%f,25],[%f,25],[%f,24]]]}`, west, east, east, west, west))
		features = append(features, reference.Feature{Type: "Feature", Geometry: geometry, Properties: map[string]any{"key": fmt.Sprintf("study:Q%d", i), "kind": "study", "name": fmt.Sprintf("Acceptance area %d", i)}})
		scopes = append(scopes, analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: fmt.Sprintf("Acceptance area %d", i), Geometry: geometry}, Temporal: fixedPeriod()})
	}
	raw, _ := json.Marshal(reference.Collection{Type: "FeatureCollection", Features: features})
	refs := reference.NewService(f.h.pool)
	edition, _, e := refs.AdmitGeography(ctx, reference.GeographyInput{SourceID: f.source, Label: "Isolated acceptance geometry", Version: "full-volume-v1", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"synthetic":true,"purpose":"acceptance"}`), SHA256: reference.Hash(raw), Artifact: raw})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = refs.Release(ctx, reference.ReleaseInput{SourceID: f.source, Key: "full-acceptance", Version: "1", EditionIDs: []string{edition.ID}, Activate: true}); e != nil {
		t.Fatal(e)
	}
	statements := []string{
		`INSERT INTO public_evidence(id,scope_id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document) SELECT 'full_event_'||i,e.scope_id,e.source_id,e.admission_id,e.connector_run_id,'full-event-'||i,'1',encode(sha256(i::text::bytea),'hex'),jsonb_set(e.document||jsonb_build_object('id','full_event_'||i,'providerRecordId','full-event-'||i),'{observation,position}',jsonb_build_object('lng',46.0005+(i%1000)*.000999,'lat',24.005+(i/1000)*.0099)) FROM public_evidence e CROSS JOIN generate_series(1,99999)i WHERE provider_record_id='full-event-seed'`,
		`INSERT INTO observations(id,scope_id,source_id,observation_type,observed_at,received_at,processed_at,position,payload,quality,semantic_kind,geometry_role) SELECT 'event_obs_full_event_'||i,o.scope_id,o.source_id,o.observation_type,o.observed_at,o.received_at,o.processed_at,ST_SetSRID(ST_MakePoint(46.0005+(i%1000)*.000999,24.005+(i/1000)*.0099),4326)::geography,o.payload||jsonb_build_object('evidenceId','full_event_'||i,'synthetic',true),o.quality,o.semantic_kind,o.geometry_role FROM observations o CROSS JOIN generate_series(1,99999)i WHERE o.id=(SELECT observation_id FROM public_event_revisions WHERE provider_record_id='full-event-seed')`,
		`INSERT INTO public_event_revisions(evidence_id,scope_id,source_id,provider_record_id,revision_at,observation_id,document) SELECT 'full_event_'||i,r.scope_id,r.source_id,'full-event-'||i,r.revision_at,'event_obs_full_event_'||i,jsonb_set(r.document||jsonb_build_object('evidenceId','full_event_'||i,'providerRecordId','full-event-'||i,'observationId','event_obs_full_event_'||i),'{reported,position}',jsonb_build_object('lng',46.0005+(i%1000)*.000999,'lat',24.005+(i/1000)*.0099)) FROM public_event_revisions r CROSS JOIN generate_series(1,99999)i WHERE provider_record_id='full-event-seed'`,
		`INSERT INTO public_event_current(scope_id,source_id,provider_record_id,evidence_id) SELECT scope_id,source_id,provider_record_id,evidence_id FROM public_event_revisions WHERE evidence_id LIKE 'full_event_%' AND evidence_id<>'full_event_0'`,
		`INSERT INTO public_evidence(id,scope_id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document) SELECT 'full_weather_'||i,e.scope_id,e.source_id,e.admission_id,e.connector_run_id,'full-weather-'||i,'1',encode(sha256(i::text::bytea),'hex'),jsonb_set(jsonb_set(e.document||jsonb_build_object('id','full_weather_'||i,'providerRecordId','full-weather-'||i),'{context,position}',jsonb_build_object('lng',46.0005+(i%1000)*.000999,'lat',24.05+(i/1000)*.099)),'{context,weather,stationId}',to_jsonb('Q'||lpad(i::text,5,'0'))) FROM public_evidence e CROSS JOIN generate_series(1,9999)i WHERE provider_record_id='full-weather-seed'`,
		`INSERT INTO public_context_records(evidence_id,scope_id,source_id,kind,record_key,valid_from,valid_to,revision_at,known_at,position) SELECT 'full_weather_'||i,r.scope_id,r.source_id,r.kind,'Q'||lpad(i::text,5,'0'),r.valid_from,r.valid_to,r.revision_at,r.known_at,ST_SetSRID(ST_MakePoint(46.0005+(i%1000)*.000999,24.05+(i/1000)*.099),4326) FROM public_context_records r CROSS JOIN generate_series(1,9999)i WHERE evidence_id=(SELECT id FROM public_evidence WHERE provider_record_id='full-weather-seed')`,
		`INSERT INTO public_evidence_catalog(evidence_id,scope_id,source_id) SELECT id,scope_id,source_id FROM public_evidence WHERE (id LIKE 'full_event_%' OR id LIKE 'full_weather_%') AND id NOT IN('full_event_0','full_weather_0')`,
		`INSERT INTO geographic_regions(id,edition_id,scope_id,source_id,region_key,parent_key,kind,name,aliases,geometry,properties) SELECT 'full-geo-'||i,g.edition_id,g.scope_id,g.source_id,'full-region-'||i,'study:Q'||((i%1000)/100),'region','Detailed acceptance feature '||i,'acceptance',ST_Buffer(ST_SetSRID(ST_MakePoint(46.0005+(i%1000)*.000999,24.01+(i/1000)*.019),4326),.0003,16),g.properties||jsonb_build_object('synthetic',true,'purpose','acceptance') FROM geographic_regions g CROSS JOIN generate_series(1,50000)i WHERE g.region_key='study:Q0'`,
		`INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,status,created_by) SELECT 'full-history-'||i,s.scope_id,s.id,jsonb_build_object('method','isolated acceptance history','area',i%10),'CANCELLED','source-reviewer' FROM sources s CROSS JOIN generate_series(1,1000)i WHERE s.name='Reviewed fixture feed'`,
		`INSERT INTO content_purpose_revisions(id,scope_id,entity_kind,entity_id,source_id,version,purpose,reason,receipt_reference,actor_id) SELECT 'full-purpose-'||i,s.scope_id,'capture','full-history-'||i,s.id,1,'acceptance','Isolated full-volume workload','full-volume-v1','source-reviewer' FROM sources s CROSS JOIN generate_series(1,1000)i WHERE s.name='Reviewed fixture feed'`,
	}
	for index, statement := range statements {
		if _, e = f.h.pool.Exec(context.Background(), statement); e != nil {
			t.Fatalf("full-volume seed %d: %v", index, e)
		}
	}
	for _, table := range []string{"public_evidence", "observations", "public_event_revisions", "public_event_current", "public_context_records", "public_evidence_catalog", "geographic_regions", "imagery_capture_jobs"} {
		if _, e = f.h.pool.Exec(context.Background(), "ANALYZE "+table); e != nil {
			t.Fatal(e)
		}
	}
	var reports, weather, geography, history int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public_event_revisions),(SELECT count(*) FROM public_context_records),(SELECT count(*) FROM geographic_regions WHERE id LIKE 'full-geo-%'),(SELECT count(*) FROM imagery_capture_jobs)`).Scan(&reports, &weather, &geography, &history); e != nil || reports != 100000 || weather != 10000 || geography != 50000 || history != 1000 {
		t.Fatal("actual volume mismatch", reports, weather, geography, history, e)
	}
	store := access.NewStore(f.h.pool)
	authority := access.Bootstrap(context.Background())
	_, f.manager, e = store.IssueCredential(authority, "source-reviewer", f.resource, time.Now().Add(48*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	_, f.worker, e = store.IssueCredential(authority, "connector-fixture", f.resource, time.Now().Add(48*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	tokens := []string{}
	for i := 0; i < 5; i++ {
		human := fmt.Sprintf("volume-human-%d", i)
		principal := fmt.Sprintf("volume-analyst-%d", i)
		if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO operators(id,name,role)VALUES($1,'Acceptance load identity','operator')`, human); e != nil {
			t.Fatal(e)
		}
		if e = store.CreateHuman(authority, principal, human, access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "sources.manage", "observations.read"}}); e != nil {
			t.Fatal(e)
		}
		_, token, e := store.IssueCredential(authority, principal, f.resource, time.Now().Add(48*time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("volume-%d", i)
		admission := f.admission
		admission.AnalystQuestion = "Isolated full-volume schedule"
		admission.Rights.ID = "volume-rights-" + key
		p := acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+f.source+"/profiles/"+key, publicingest.ProfileInput{Admission: admission}, 201)
		acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+f.source+"/profiles/"+key+"/execution", publicingest.ExecutionGrant{ProfileRevisionID: "rpr_" + p["id"].(string), PrincipalID: "connector-fixture", Enabled: true}, 200)
	}
	emit(map[string]any{"kind": "profile", "schema": "emad.workload-sample.v1", "fullDataset": true, "durationSeconds": duration.Seconds(), "startedAt": time.Now().UTC(), "analysts": 5, "scheduledProfiles": 10, "reportRevisions": reports, "weatherMovingRecords": weather, "detailedReferenceFeatures": geography, "historyRows": history, "areas": 10, "synthetic": true, "goVersion": runtime.Version(), "scope": "Full declared read volumes coexist; scoped cold POST and warm pinned GET maps. Does not qualify raster concurrency, browser, real-provider or 24-hour gates."})
	client := &http.Client{Timeout: 5 * time.Second}
	failures := 0
	call := func(token, operation, method, path string, input any, cold bool) map[string]any {
		raw, _ := json.Marshal(input)
		request, _ := http.NewRequest(method, f.h.server.URL+path, bytes.NewReader(raw))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		start := time.Now()
		response, e := client.Do(request)
		status, size := 0, 0
		value := map[string]any{}
		correlation := ""
		if e == nil {
			status = response.StatusCode
			correlation = response.Header.Get("X-Request-ID")
			body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
			response.Body.Close()
			size = len(body)
			e = err
			if e == nil {
				e = json.Unmarshal(body, &value)
			}
		}
		ok := e == nil && status >= 200 && status < 300
		if operation == "catalog_search" && value["total"] != float64(1000) {
			ok = false
		}
		if operation == "scope_resolve" && value["scopeHash"] == nil {
			ok = false
		}
		if operation == "map_family" {
			if method == "POST" {
				if errs, yes := value["errors"].(map[string]any); !yes || len(errs) != 0 {
					ok = false
				}
			} else if value["scopeHash"] == nil || value["state"] == "unavailable" {
				ok = false
			}
			if size > 2<<20 {
				ok = false
			}
		}
		if code, yes := value["failureCode"]; yes && code != nil && code != "" {
			ok = false
		}
		timeout := false
		if err, yes := e.(interface{ Timeout() bool }); yes {
			timeout = err.Timeout()
		}
		mu.Lock()
		if !ok {
			failures++
		}
		mu.Unlock()
		emit(map[string]any{"kind": "request", "operation": operation, "cold": cold, "elapsedMs": float64(time.Since(start).Microseconds()) / 1000, "success": ok, "timeout": timeout, "status": status, "bytes": size, "correlationId": correlation})
		return value
	}
	for i := 0; i < 10; i++ {
		call(f.worker, "scheduled_retrieval", "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, true)
		time.Sleep(1100 * time.Millisecond)
	}
	var attempted int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM retrieval_profile_state WHERE last_attempt_at IS NOT NULL`).Scan(&attempted); e != nil || attempted != 10 {
		t.Fatal("ten distinct schedules", attempted, e)
	}
	start := time.Now()
	end := start.Add(duration)
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Add(1)
		go func(i int, token string) {
			defer wg.Done()
			catalogID := ""
			maps := [2]string{}
			geoIDs := [2]string{}
			iteration := 0
			for time.Now().Before(end) {
				which := iteration % 2
				areaIndex := i + which*5
				scope := scopes[areaIndex]
				cold := iteration == 0
				page := call(token, "catalog_search", "POST", "/api/v1/catalog/reads?content_view=all", map[string]any{"kind": "captures", "limit": 100, "replaceSnapshotId": catalogID}, cold)
				catalogID, _ = page["snapshotId"].(string)
				call(token, "scope_resolve", "POST", "/api/v1/analyst-scope/resolve", scope, cold)
				if maps[which] == "" {
					v := call(token, "map_family", "POST", "/api/v1/analyst-scope/read", analystscope.ReadInput{Input: scope, Families: []string{"earthquakes", "weather"}, Zoom: 6}, true)
					maps[which], _ = v["id"].(string)
				} else {
					call(token, "map_family", "GET", "/api/v1/analyst-scope/reads/"+maps[which]+"?family=earthquakes", nil, false)
				}
				g := call(token, "geography_search", "POST", "/api/v1/catalog/reads", map[string]any{"kind": "geography", "scope": scope, "filters": map[string]string{"query": "Detailed acceptance feature"}, "limit": 100, "replaceSnapshotId": geoIDs[which]}, iteration < 2)
				geoIDs[which], _ = g["snapshotId"].(string)
				west, east := 46+float64(areaIndex)*.1, 46+float64(areaIndex+1)*.1
				call(token, "weather_query", "GET", fmt.Sprintf("/api/v1/public-context/records?kind=WEATHER&bbox=%f,24,%f,25", west, east), nil, cold)
				iteration++
				time.Sleep(time.Second)
			}
		}(i, token)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		next := start.Add(12 * time.Second)
		for time.Now().Before(end) {
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			var dbBytes int64
			e := f.h.pool.QueryRow(context.Background(), `SELECT pg_database_size(current_database())`).Scan(&dbBytes)
			emit(map[string]any{"kind": "telemetry", "elapsedSeconds": time.Since(start).Seconds(), "heapBytes": memory.HeapAlloc, "dbBytes": dbBytes, "goroutines": runtime.NumGoroutine(), "success": e == nil})
			if time.Now().After(next) {
				call(f.worker, "scheduled_retrieval", "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, false)
				next = next.Add(12 * time.Second)
			}
			time.Sleep(time.Second)
		}
	}()
	wg.Wait()
	emit(map[string]any{"kind": "end", "completed": true, "elapsedSeconds": time.Since(start).Seconds(), "failures": failures, "workerRestart": "NOT_RUN", "rasterConcurrency": "NOT_RUN", "observedHumans": "NOT_RUN"})
	if failures != 0 {
		t.Fatalf("full-volume retained %d failures", failures)
	}
}
