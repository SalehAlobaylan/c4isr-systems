package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/catalog"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/reads"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMapFirstCatalogSnapshotsHistoryCapacityAndCursorIsolation(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	svc := catalog.Service{Pool: f.h.pool}
	// More than the previous lifetime/history/page ceilings. These are explicitly
	// synthetic job history, not fabricated completed imagery or geographic proof.
	input := map[string]any{"aoi": map[string]any{"west": 46.2, "south": 24.2, "east": 46.21, "north": 24.21}, "scene": map[string]any{"id": "history-scene", "acquiredAt": "2026-01-01T10:00:00Z"}, "method": "synthetic-history-fixture"}
	raw, _ := json.Marshal(input)
	if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,status,created_by,created_at) SELECT 'history-'||i,$1,$2,$3,'CANCELLED','source-reviewer','2026-01-01'::timestamptz+i*interval '1 second' FROM generate_series(1,1105)i`, f.resource.ID(), f.source, raw); e != nil {
		t.Fatal(e)
	}
	in := catalog.Input{Kind: "captures", Limit: 100}
	first, e := svc.Read(ctx, in)
	if e != nil || *first.Total != 1105 || first.Returned != 100 || !first.HasMore {
		t.Fatal("first history page", first, e)
	}
	code, legacy := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/imagery/captures", nil, nil)
	if code != 200 || legacy["total"] != nil || legacy["returned"] != float64(100) || legacy["hasMore"] != true {
		t.Fatal("legacy history hid its ceiling", code, legacy["total"], legacy["hasMore"])
	}
	firstID := func(raw json.RawMessage) string {
		var v struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &v)
		return v.ID
	}
	if firstID(first.Items[0]) != "history-1105" {
		t.Fatal("unstable initial ordering", firstID(first.Items[0]))
	}
	// A commit that begins before the read but becomes visible afterward cannot
	// join this snapshot. No transaction is retained between HTTP pages.
	if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,status,created_by)VALUES('late-history',$1,$2,$3,'CANCELLED','source-reviewer')`, f.resource.ID(), f.source, raw); e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, v := range first.Items {
		seen[firstID(v)] = true
	}
	next := in
	next.SnapshotID = first.SnapshotID
	next.Cursor = first.NextCursor
	for next.Cursor != "" {
		page, e := svc.Read(ctx, next)
		if e != nil || *page.Total != 1105 {
			t.Fatal("frozen page", page, e)
		}
		for _, raw := range page.Items {
			id := firstID(raw)
			if seen[id] || id == "late-history" {
				t.Fatal("duplicate/late member", id)
			}
			seen[id] = true
		}
		next.Cursor = page.NextCursor
	}
	if len(seen) != 1105 {
		t.Fatal("incomplete paged history", len(seen))
	}
	wrong := in
	wrong.Cursor = first.NextCursor
	wrong.SnapshotID = first.SnapshotID
	wrong.Filters.Status = "FAILED"
	if _, e = svc.Read(ctx, wrong); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("cursor accepted another query", e)
	}
	wrong.Filters = catalog.Filters{}
	wrong.Cursor += "tampered"
	if _, e = svc.Read(ctx, wrong); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("tampered cursor accepted", e)
	}
	// History no longer consumes active queue slots. Both active and storage
	// limits still apply, including jobs hidden from a restricted source reader.
	if e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		var ok bool
		if e := tx.QueryRow(ctx, `SELECT emad_imagery_queue_available('capture',$1)`, f.source).Scan(&ok); e != nil {
			return e
		}
		if !ok {
			return fmt.Errorf("lifetime history still blocks acquisition")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,created_by) SELECT 'active-'||i,$1,$2,$3,'source-reviewer' FROM generate_series(1,4)i`, f.resource.ID(), f.source, raw); e != nil {
		t.Fatal(e)
	}
	if e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		var ok bool
		if e := tx.QueryRow(ctx, `SELECT emad_imagery_queue_available('capture',$1)`, f.source).Scan(&ok); e != nil {
			return e
		}
		if ok {
			return fmt.Errorf("active guard removed with lifetime cap")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	// Account/access change cannot reuse another read, even in the same resource.
	other := access.Principal{ID: "other-catalog-reader", Kind: "human", Grants: []access.Grant{{Resource: f.resource, Permissions: access.Permissions}}}
	otherCtx, e := access.Bind(context.Background(), other, f.resource, nil)
	if e != nil {
		t.Fatal(e)
	}
	wrong = in
	wrong.Cursor = first.NextCursor
	wrong.SnapshotID = first.SnapshotID
	if _, e = svc.Read(otherCtx, wrong); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("other owner read cursor", e)
	}
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE analyst_read_snapshots SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, first.SnapshotID); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Read(ctx, wrong); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("expired cursor accepted", e)
	}
	current, e := svc.Read(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 20; i++ {
		next := in
		next.ReplaceSnapshotID = current.SnapshotID
		current, e = svc.Read(ctx, next)
		if e != nil {
			t.Fatal("explicit refresh exhausted read slots", i, e)
		}
	}
	var retained int
	if e = db.Scoped(f.h.pool).QueryRow(ctx, `SELECT count(*) FROM analyst_read_snapshots WHERE snapshot_kind='catalog'`).Scan(&retained); e != nil || retained != 1 {
		t.Fatal("refresh retained obsolete cache slots", retained, e)
	}

}

func TestMapFirstGeographicSnapshotBeyond300AndExactRetainedGeometry(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	features := []reference.Feature{}
	for i := 0; i < 405; i++ {
		features = append(features, reference.Feature{Type: "Feature", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`), Properties: map[string]any{"key": fmt.Sprintf("region:R%03d", i), "kind": "region", "name": fmt.Sprintf("منطقة Region %03d", i)}})
	}
	raw, _ := json.Marshal(reference.Collection{Type: "FeatureCollection", Features: features})
	if _, _, e := reference.NewService(f.h.pool).AdmitGeography(ctx, reference.GeographyInput{SourceID: f.source, Label: "Synthetic paged geography", Version: "paging-1", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"fixture":true}`), Artifact: raw, SHA256: reference.Hash(raw)}); e != nil {
		t.Fatal(e)
	}
	svc := catalog.Service{Pool: f.h.pool}
	if os.Getenv("C4ISR_TEST_SCALE") == "1" {
		_, e := f.h.pool.Exec(context.Background(), `INSERT INTO geographic_regions(id,edition_id,scope_id,source_id,region_key,parent_key,kind,name,aliases,geometry,properties) SELECT 'scale-geo-'||i,g.edition_id,g.scope_id,g.source_id,'scale-region-'||i,g.parent_key,'region','Synthetic workload '||i,'synthetic',ST_Buffer(ST_SetSRID(ST_MakePoint(46.1+(i%200)*.003,24.1+(i/200)*.002),4326),.001,16),g.properties FROM geographic_regions g CROSS JOIN generate_series(1,50000)i WHERE g.id=(SELECT min(id) FROM geographic_regions)`)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.h.pool.Exec(context.Background(), `ANALYZE geographic_regions`); e != nil {
			t.Fatal(e)
		}
		recordCatalogPlan(t, f, ctx, "geography-spatial-50000", `SELECT id FROM geographic_regions WHERE ST_Intersects(geometry,ST_MakeEnvelope(46.1,24.1,46.2,24.2,4326))`)
	}
	in := catalog.Input{Kind: "geography", Limit: 300, Zoom: 2, Filters: catalog.Filters{Query: "Region"}}
	first, e := svc.Read(ctx, in)
	if e != nil || *first.Total != 405 || first.Returned != 300 {
		t.Fatal(first, e)
	}
	in.SnapshotID = first.SnapshotID
	in.Cursor = first.NextCursor
	last, e := svc.Read(ctx, in)
	if e != nil || last.Returned != 105 || last.HasMore {
		t.Fatal(last, e)
	}
	var selected reference.Feature
	if e = json.Unmarshal(last.Items[0], &selected); e != nil {
		t.Fatal(e)
	}
	if selected.Properties["displayOnly"] != true {
		t.Fatal("simplified display lacks a boundary warning", selected)
	}
	exact, e := (areas.Service{Pool: f.h.pool}).Resolve(ctx, areas.Selection{Kind: "administrative", RegionID: selected.ID})
	if e != nil || exact.GeometrySHA256 == "" {
		t.Fatal("selection used display geometry instead of retained identity", exact, e)
	}
}

func TestMapFirstAggregateDrillDownKeepsOriginalRevisionsAndCounts(t *testing.T) {
	f := newPublicFixture(t)
	f.admit(t)
	ctx := scopeContext(t, f.h, f.resource)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	// Retain/project published records through the real SQL/projector boundary;
	// every row is labeled a synthetic provider report, with source identity.
	repo := publicingest.NewRepository(f.h.pool)
	workerStore := access.NewStore(f.h.pool)
	session, e := workerStore.Authenticate(context.Background(), f.worker, nil)
	if e != nil {
		t.Fatal(e)
	}
	worker, e := workerStore.BindSession(context.Background(), session, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	var admission publicingest.Admission
	raw := acquisitionCall(t, f, f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200)
	data, _ := json.Marshal(raw)
	_ = json.Unmarshal(data, &admission)
	run, _, e := repo.Begin(worker, f.source, "connector-fixture", admission.ID, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		for i := 0; i < 2110; i++ {
			id := fmt.Sprintf("synthetic-map-%04d", i)
			observed := at
			observation := publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Synthetic map report", ObservedAt: observed, Position: &geo.Point{Lng: 46.25 + float64(i%20)/1000, Lat: 24.25 + float64(i/20)/10000}, GeometryRole: "reported_location", Properties: map[string]any{}, Quality: map[string]any{}}
			evidence := publicingest.Evidence{ID: id, SourceID: f.source, AdmissionID: admission.ID, ConnectorRunID: run.ID, ProviderRecordID: id, ProviderRevision: "1", ContentHash: id, RawHash: id, OriginalProducer: "Synthetic fixture", RetrievedAt: observed, ProviderUpdatedAt: &observed, Observation: &observation}
			document, _ := json.Marshal(evidence)
			if _, e := tx.Exec(ctx, `INSERT INTO public_evidence(id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document)VALUES($1,$2,$3,$4,$1,'1',$1,$5)`, id, f.source, admission.ID, run.ID, document); e != nil {
				return e
			}
			if e := (publicevents.Projector{}).Project(ctx, tx, evidence); e != nil {
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
	total := 2110
	if os.Getenv("C4ISR_TEST_SCALE") == "1" {
		total = 100000
		statements := []string{
			`INSERT INTO public_evidence(id,scope_id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document) SELECT 'scale_'||encode(sha256(i::text::bytea),'hex'),e.scope_id,e.source_id,e.admission_id,e.connector_run_id,'scale_'||i,'1',i::text,e.document||jsonb_build_object('id','scale_'||encode(sha256(i::text::bytea),'hex'),'providerRecordId','scale_'||i) FROM public_evidence e CROSS JOIN generate_series(2110,99999)i WHERE e.id='synthetic-map-0000'`,
			`INSERT INTO observations(id,scope_id,source_id,observation_type,observed_at,received_at,processed_at,position,payload,quality,semantic_kind,geometry_role) SELECT 'event_obs_scale_'||encode(sha256(i::text::bytea),'hex'),o.scope_id,o.source_id,o.observation_type,o.observed_at,o.received_at,o.processed_at,ST_SetSRID(ST_MakePoint(46.1+(i%200)*.003,24.1+(i/200)*.001),4326)::geography,o.payload||jsonb_build_object('synthetic',true),o.quality,o.semantic_kind,o.geometry_role FROM observations o CROSS JOIN generate_series(2110,99999)i WHERE o.id='event_obs_synthetic-map-0000'`,
			`INSERT INTO public_event_revisions(evidence_id,scope_id,source_id,provider_record_id,revision_at,observation_id,document) SELECT 'scale_'||encode(sha256(i::text::bytea),'hex'),r.scope_id,r.source_id,'scale_'||i,r.revision_at,'event_obs_scale_'||encode(sha256(i::text::bytea),'hex'),jsonb_set(r.document||jsonb_build_object('evidenceId','scale_'||encode(sha256(i::text::bytea),'hex'),'providerRecordId','scale_'||i,'observationId','event_obs_scale_'||encode(sha256(i::text::bytea),'hex')),'{reported,position}',jsonb_build_object('lng',46.1+(i%200)*.003,'lat',24.1+(i/200)*.001)) FROM public_event_revisions r CROSS JOIN generate_series(2110,99999)i WHERE r.evidence_id='synthetic-map-0000'`,
			`INSERT INTO public_evidence_catalog(evidence_id,scope_id,source_id) SELECT id,scope_id,source_id FROM public_evidence WHERE id LIKE 'scale_%'`,
			`INSERT INTO public_event_current(scope_id,source_id,provider_record_id,evidence_id) SELECT scope_id,source_id,provider_record_id,evidence_id FROM public_event_revisions WHERE evidence_id LIKE 'scale_%'`,
		}
		for _, sql := range statements {
			if _, e = f.h.pool.Exec(context.Background(), sql); e != nil {
				t.Fatal("synthetic workload setup", e)
			}
		}
		for _, table := range []string{"observations", "public_event_revisions", "public_event_current", "public_evidence", "public_evidence_catalog"} {
			if _, e = f.h.pool.Exec(context.Background(), "ANALYZE "+table); e != nil {
				t.Fatal(e)
			}
		}
		recordCatalogPlan(t, f, ctx, "report-spatial-100000", `SELECT id FROM observations WHERE ST_Intersects(position::geometry,ST_MakeEnvelope(46.1,24.1,46.2,24.2,4326))`)
	}
	svc := analystscope.Service{Pool: f.h.pool}
	scope := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Dense synthetic reports", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: at.Add(-time.Minute), To: at.Add(time.Minute)}, RevisionView: "latest", Timezone: "UTC"}}
	started := time.Now()
	read, e := svc.Read(ctx, analystscope.ReadInput{Input: scope, Families: []string{"earthquakes"}, Zoom: 6})
	t.Logf("aggregate read load=%d elapsed=%v", total, time.Since(started))
	if e != nil || read.Layers["earthquakes"].State != "aggregated" {
		t.Fatal("aggregate read", e)
	}
	started = time.Now()
	first, e := svc.RecordPage(ctx, read.ID, "earthquakes", "", "", 100)
	t.Logf("first drill-down page load=%d elapsed=%v", total, time.Since(started))
	if e != nil || first["total"] != total {
		t.Fatal(first, e)
	}
	token := first["nextCursor"].(string)
	// Correct a current head after the read; later pages retain old revisions.
	if _, e = f.h.pool.Exec(context.Background(), `DELETE FROM public_event_current WHERE provider_record_id='synthetic-map-0200'`); e != nil {
		t.Fatal(e)
	}
	second, e := svc.RecordPage(ctx, read.ID, "earthquakes", "", token, 100)
	if e != nil || second["total"] != total {
		t.Fatalf("late change altered read: total=%v want=%d error=%v", second["total"], total, e)
	}
	cell := read.Layers["earthquakes"].Features[0]
	subset, e := svc.RecordPage(ctx, read.ID, "earthquakes", cell.Properties["aggregateId"].(string), "", 100)
	if e != nil {
		t.Fatal(e)
	}
	expected := int(cell.Properties["count"].(float64))
	if subset["total"] != expected {
		t.Fatal("cell drill-down disagrees with aggregate", subset["total"], expected)
	}
	if _, e = svc.RecordPage(ctx, read.ID, "thermal", "", token, 100); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("cross-family cursor accepted", e)
	}
}

func recordCatalogPlan(t *testing.T, f *publicFixture, ctx context.Context, name, query string) {
	t.Helper()
	var plan []byte
	e := db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "EXPLAIN(ANALYZE,BUFFERS,FORMAT JSON) "+query).Scan(&plan)
	})
	if e != nil {
		t.Fatal(e)
	}
	if dir := os.Getenv("C4ISR_TEST_EVIDENCE_DIR"); dir != "" {
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name+"-explain.json"), plan, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("%s EXPLAIN captured (%d bytes); restricted application role", name, len(plan))
}

func TestMapFirstGeographicCatalogHonorsExactStudyScope(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	features := []reference.Feature{}
	for i, point := range [][2]float64{{46.1, 24.1}, {46.5, 24.5}, {48, 26}} {
		geometry, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": point})
		features = append(features, reference.Feature{Type: "Feature", Geometry: geometry, Properties: map[string]any{"key": fmt.Sprintf("place:scope-%d", i), "kind": "place", "name": fmt.Sprintf("Scoped sample %d", i)}})
	}
	raw, _ := json.Marshal(reference.Collection{Type: "FeatureCollection", Features: features})
	if _, _, e := reference.NewService(f.h.pool).AdmitGeography(ctx, reference.GeographyInput{SourceID: f.source, Label: "Synthetic exact scope regression", Version: "scope-1", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"synthetic":true}`), SHA256: reference.Hash(raw), Artifact: raw}); e != nil {
		t.Fatal(e)
	}
	svc := catalog.Service{Pool: f.h.pool}
	in := catalog.Input{Kind: "geography", Limit: 100, Filters: catalog.Filters{Query: "Scoped sample"}}
	all, e := svc.Read(ctx, in)
	if e != nil || all.Total == nil || *all.Total != 3 {
		t.Fatal("unscoped discovery", all.Total, e)
	}
	scope := analystscope.Input{Area: areas.Selection{Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]],[[46.4,24.4],[46.4,24.6],[46.6,24.6],[46.6,24.4],[46.4,24.4]]]}`)}, Temporal: fixedPeriod()}
	in.Scope = &scope
	selected, e := svc.Read(ctx, in)
	if e != nil || selected.Total == nil || *selected.Total != 1 {
		t.Fatal("exact selected geometry including its hole was ignored", selected.Total, e)
	}
	var feature reference.Feature
	if e = json.Unmarshal(selected.Items[0], &feature); e != nil || feature.Properties["name"] != "Scoped sample 0" {
		t.Fatal("out-of-scope member", feature, e)
	}
}

func TestMapFirstReadBudgetUsesStoredJSONBytes(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	raw := []byte(`{"values":[` + strings.TrimSuffix(strings.Repeat("0,", 2900000), ",") + `]}`)
	if len(raw) >= reads.MaxBytes {
		t.Fatal("fixture does not isolate JSONB expansion")
	}
	e := db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		return reads.Save(ctx, tx, "expanded-json-read", "catalog", raw, time.Now().Add(time.Minute))
	})
	if !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("stored read budget was not explained as a bounded conflict", e)
	}
	var count int
	if e = db.Scoped(f.h.pool).QueryRow(ctx, `SELECT count(*) FROM analyst_read_snapshots`).Scan(&count); e != nil || count != 0 {
		t.Fatal("oversized read left a cache slot", count, e)
	}
}
