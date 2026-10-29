package integration

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/jackc/pgx/v5"
)

func TestMapFirstVersionedAreasReleasesAndComponents(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	refs := reference.NewService(f.h.pool)
	svc := areas.Service{Pool: f.h.pool}
	edition := func(version, geometry string, parent bool) reference.GeographyEdition {
		t.Helper()
		features := []reference.Feature{{Type: "Feature", Geometry: json.RawMessage(geometry), Properties: map[string]any{"key": "country:TEST", "kind": "country", "name": "Test edition " + version}}}
		if parent {
			features = append(features, reference.Feature{Type: "Feature", Geometry: json.RawMessage(geometry), Properties: map[string]any{"key": "region:TEST-1", "parentKey": "country:TEST", "kind": "region", "name": "Child " + version}})
		}
		raw, _ := json.Marshal(reference.Collection{Type: "FeatureCollection", Features: features})
		v, _, e := refs.AdmitGeography(ctx, reference.GeographyInput{SourceID: f.source, Label: "Synthetic isolated geography", Version: version, Rights: f.admission.Rights, Receipt: json.RawMessage(`{"fixture":true}`), Artifact: raw, SHA256: reference.Hash(raw)})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	a := edition("a", `{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`, true)
	release, e := refs.Release(ctx, reference.ReleaseInput{SourceID: f.source, Key: "navigation", Version: "a", EditionIDs: []string{a.ID}, Activate: true})
	if e != nil {
		t.Fatal(e)
	}
	in := areas.Selection{Kind: "administrative", RegionID: a.ID + ":country:TEST", ReleaseID: release.ID}
	original, e := svc.Resolve(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	var before int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM area_revisions`).Scan(&before); e != nil || before != 0 {
		t.Fatal("transient navigation persisted", before, e)
	}
	e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error { var e error; original, e = svc.Persist(ctx, tx, in); return e })
	if e != nil {
		t.Fatal(e)
	}
	b := edition("b", `{"type":"Polygon","coordinates":[[[46,24],[48,24],[48,25],[46,25],[46,24]]]}`, true)
	latest, e := refs.Geography(ctx, "", "", "country", "", 0)
	if e != nil || latest.Features[0].ID != a.ID+":country:TEST" {
		t.Fatal("staged edition leaked into active release", latest, e)
	}
	rb, e := refs.Release(ctx, reference.ReleaseInput{SourceID: f.source, Key: "navigation", Version: "b", EditionIDs: []string{b.ID}, ExpectedHead: release.ID, Activate: true})
	if e != nil {
		t.Fatal(e)
	}
	_ = rb
	exact, e := refs.Geography(ctx, "", "", "", a.ID+":country:TEST", 0)
	if e != nil || len(exact.Features) != 1 {
		t.Fatal("old exact ID went through latest discovery", exact, e)
	}
	retained, e := svc.Get(ctx, original.RevisionID)
	if e != nil || retained.GeometrySHA256 != original.GeometrySHA256 || string(retained.Geometry) != string(original.Geometry) {
		t.Fatal("retained area changed across release", retained, e)
	}
	latest, e = refs.Geography(ctx, "", "", "country", "", 0)
	if e != nil || latest.Features[0].ID != b.ID+":country:TEST" {
		t.Fatal("active release not atomic", latest, e)
	}
	child, e := svc.Resolve(ctx, areas.Selection{RegionID: a.ID + ":region:TEST-1", ReleaseID: release.ID})
	if e != nil || child.Geography.ParentRegionID != a.ID+":country:TEST" {
		t.Fatal("old parent identity drifted", child, e)
	}
	dateline := json.RawMessage(`{"type":"Polygon","coordinates":[[[179,10],[-179,10],[-179,14],[179,14],[179,10]],[[179.4,11],[-179.4,11],[-179.4,12],[179.4,12],[179.4,11]]]}`)
	split, e := svc.Resolve(ctx, areas.Selection{Kind: "drawn", Geometry: dateline})
	if e != nil || len(split.Components) != 2 || !split.Normalized {
		t.Fatalf("date-line holes: %+v %v", split, e)
	}
	for _, part := range split.Components {
		if part.Bounds[2]-part.Bounds[0] > 2 {
			t.Fatal("date-line became a world box", part)
		}
	}
	var areaCovered bool
	e = db.Scoped(f.h.pool).QueryRow(ctx, `SELECT ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON($1),4326),ST_SetSRID(ST_MakePoint(179.7,11.5),4326))`, string(split.Geometry)).Scan(&areaCovered)
	if e != nil || areaCovered {
		t.Fatal("seam normalization filled a hole", e)
	}
	polar, e := svc.Resolve(ctx, areas.Selection{Kind: "drawn", Geometry: json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[10,86],[12,86],[12,88],[10,88],[10,86]]],[[[20,82],[21,82],[21,83],[20,83],[20,82]]]]}`)})
	if e != nil || polar.Display.State != "partial" || len(polar.Components) != 2 || !strings.Contains(string(polar.Geometry), "88") {
		t.Fatal("high latitude geometry discarded", polar, e)
	}
	e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		_, e := svc.Persist(ctx, tx, areas.Selection{Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46.1,24.1],[46.2,24.1],[46.2,24.2],[46.1,24.2],[46.1,24.1]]]}`), ParentRevisionID: original.RevisionID})
		return e
	})
	if e != nil {
		t.Fatal("deliberate subset", e)
	}
	for _, bad := range []areas.Selection{{Kind: "world", Geometry: dateline}, {RevisionID: original.RevisionID, RegionID: in.RegionID}, {Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,25],[47,24],[46,25],[46,24]]]}`)}, {Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[1,1],[2,1],[2,2],[1,2],[1,1]]]}`), ParentRevisionID: original.RevisionID}} {
		if _, e := svc.Resolve(ctx, bad); e == nil {
			t.Fatal("malformed/conflicting/outside scope accepted", bad)
		}
	}
	restricted := scopeToken(t, f.h, "areas-hidden-source", f.resource, []string{"sources.read"}, []string{"another-source"})
	if code, _ := scopeRequest(t, f.h, restricted, "GET", "/api/v1/areas/revisions/"+original.RevisionID, nil, nil); code != 404 {
		t.Fatal("area source restriction bypassed", code)
	}
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/profiles", reference.ProfileInput{Key: "working-region", Version: "1", Name: "Test working overview", Definition: "Synthetic source-backed working overview, not a jurisdictional definition", Members: []areas.Selection{{RegionID: a.ID + ":country:TEST", ReleaseID: release.ID}}}, nil); code != 201 {
		t.Fatal("regional profile", code)
	}
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE area_revisions SET area_id='changed' WHERE id=$1`, original.RevisionID); e == nil {
		t.Fatal("area revision could be rewritten")
	}
	if _, e := refs.Release(ctx, reference.ReleaseInput{SourceID: f.source, Key: "invalid-mixed", Version: "1", EditionIDs: []string{a.ID, b.ID}, Activate: true, ExpectedHead: rb.ID}); e == nil {
		t.Fatal("duplicate identity release activated")
	}
	// Retained observations in separate date-line components are counted once;
	// a point in a split hole remains excluded after normalization.
	f.admission.Policy.AOI = publicingest.Bounds{West: 179, South: 10, East: 180, North: 14}
	f.admit(t)
	at := time.Now().UTC().Add(-time.Minute)
	records := []publicingest.Record{}
	for i, p := range []geo.Point{{Lng: 179.2, Lat: 10.5}, {Lng: 179.7, Lat: 11.5}} {
		id := []string{"component-event", "hole-event"}[i]
		records = append(records, publicingest.Record{ProviderRecordID: id, ProviderRevision: "1", ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/synthetic", OriginalRecord: json.RawMessage(`{"synthetic":true}`), Observation: &publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Date-line fixture", ObservedAt: at, Position: &p, GeometryRole: "reported_location"}})
	}
	f.fetcher.set(publicingest.Page{Records: records, Checkpoint: "done"}, nil)
	if run := f.execute(t); run["status"] != "SUCCEEDED" {
		t.Fatal(run)
	}
	read, e := (analystscope.Service{Pool: f.h.pool}).Read(ctx, analystscope.ReadInput{Input: analystscope.Input{Area: areas.Selection{Kind: "composite", Geometry: dateline}, Temporal: fixedPeriod()}, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || read.Layers["earthquakes"].Total != 1 || read.Events.Total != 1 {
		t.Fatal("date-line count or hole meaning changed", read.Layers, e)
	}
}
func scopedCatalogURL(path string, s analystscope.Snapshot) string {
	raw, _ := json.Marshal(analystscope.FromSnapshot(s))
	return path + "?" + url.Values{"scope": {string(raw)}, "scope_hash": {s.ScopeHash}}.Encode()
}
func fixedPeriod() queryscope.Temporal {
	now := time.Now().UTC()
	return queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: now.Add(-24 * time.Hour), To: now}, RevisionView: "latest", Timezone: "UTC"}
}
func scopedFixtureContext(t *testing.T, f *publicFixture) context.Context {
	return scopeContext(t, f.h, f.resource)
}

func TestMapFirstTemporalReadSnapshotsPublicationRacesAndAccess(t *testing.T) {
	f := newPublicFixture(t)
	f.admit(t)
	ctx := scopedFixtureContext(t, f)
	svc := analystscope.Service{Pool: f.h.pool}
	end := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := end.Add(-2 * time.Hour)
	record := func(id, rev string, at time.Time, lng float64, withdrawn bool) publicingest.Record {
		return publicingest.Record{ProviderRecordID: id, ProviderRevision: rev, ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/synthetic", OriginalRecord: json.RawMessage(`{"fixture":true}`), Observation: &publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Synthetic scoped report", ObservedAt: at, Position: &geo.Point{Lng: lng, Lat: 24.5}, Withdrawn: withdrawn, GeometryRole: "reported_location", Properties: map[string]any{}, Quality: map[string]any{}}}
	}
	ingest := func(records ...publicingest.Record) {
		t.Helper()
		f.fetcher.set(publicingest.Page{Records: records, Checkpoint: "done"}, nil)
		f.ready(t)
		if run := f.execute(t); run["status"] != "SUCCEEDED" {
			t.Fatal(run)
		}
	}
	ingest(record("at-start", "1", start, 46.5, false), record("inside", "1", start.Add(time.Hour), 46.5, false), record("at-end", "1", end, 46.5, false))
	in := analystscope.Input{Area: areas.Selection{Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: start, To: end}, RevisionView: "latest", Timezone: "Asia/Riyadh"}}
	read, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: []string{"earthquakes", "weather"}, IncludeEvents: true})
	if e != nil {
		t.Fatal(e)
	}
	if read.Events.Total != 2 || read.Layers["earthquakes"].Total != 2 || read.Layers["weather"].State != "historical_unsupported" {
		t.Fatal("exclusive end/family support", read)
	}
	legacy := url.Values{"observed_from": {start.Format(time.RFC3339Nano)}, "observed_to": {end.Format(time.RFC3339Nano)}}
	if code, b := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events?"+legacy.Encode(), nil, nil); code != 200 || b["total"] != float64(3) {
		t.Fatal("legacy inclusive interval changed", code, b)
	}
	imported := in
	imported.Temporal.LegacyEndInclusive = true
	importedRead, e := svc.Read(ctx, analystscope.ReadInput{Input: imported, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || importedRead.Events.Total != 3 {
		t.Fatalf("legacy bookmark lost its inclusive end at database precision: %v %v", importedRead.Events, e)
	}
	scopedPath := scopedCatalogURL("/api/v1/public-events", read.Scope)
	if code, b := scopeRequest(t, f.h, f.manager, "GET", scopedPath, nil, nil); code != 200 || b["total"] != float64(2) {
		t.Fatal("scope catalog adapter", code, b)
	}
	if code, _ := scopeRequest(t, f.h, f.manager, "GET", scopedPath+"&bbox=46,24,47,25", nil, nil); code != 400 {
		t.Fatal("conflicting catalog scope accepted", code)
	}
	// Retain an unpublished report through the actual outbox failure boundary,
	// then start its publication transaction before taking a read snapshot.
	if _, e = f.h.pool.Exec(context.Background(), `CREATE FUNCTION fixture_scope_hold() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic publication hold'; END $$; CREATE TRIGGER fixture_scope_hold BEFORE INSERT ON public_evidence_catalog FOR EACH ROW EXECUTE FUNCTION fixture_scope_hold()`); e != nil {
		t.Fatal(e)
	}
	ingest(record("late-commit", "1", start.Add(30*time.Minute), 46.5, false))
	if _, e = f.h.pool.Exec(context.Background(), `DROP TRIGGER fixture_scope_hold ON public_evidence_catalog`); e != nil {
		t.Fatal(e)
	}
	tx, e := f.h.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if e = db.BindTx(ctx, tx); e != nil {
		t.Fatal(e)
	}
	var evidence publicingest.Evidence
	var raw []byte
	if e = tx.QueryRow(ctx, `SELECT document FROM public_evidence WHERE provider_record_id='late-commit'`).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &evidence); e != nil {
		t.Fatal(e)
	}
	if e = (publicevents.Projector{}).Project(ctx, tx, evidence); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public_evidence_catalog(evidence_id,source_id)VALUES($1,$2)`, evidence.ID, evidence.SourceID); e != nil {
		t.Fatal(e)
	}
	beforeCommit, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || beforeCommit.Events.Total != 2 {
		t.Fatal("uncommitted publication visible", e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	page, e := svc.ReadPage(ctx, beforeCommit.ID, "event_records", 0, 1)
	if e != nil || page["total"] != 2 || page["materialized"] != 2 {
		t.Fatal("late commit changed old membership", page, e)
	}
	afterCommit, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || afterCommit.Events.Total != 3 {
		t.Fatal("new read missed committed publication", afterCommit.Events, e)
	}
	// A correction moved outside the selected geometry must not resurrect its
	// earlier matching revision. A withdrawal is retained in tabular evidence.
	ingest(record("inside", "2", start.Add(70*time.Minute), 46.95, false))
	narrow := in
	narrow.Area.Geometry = json.RawMessage(`{"type":"Polygon","coordinates":[[[46.1,24.1],[46.8,24.1],[46.8,24.8],[46.1,24.8],[46.1,24.1]]]}`)
	corrected, e := svc.Read(ctx, analystscope.ReadInput{Input: narrow, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || corrected.Events.Total != 2 {
		t.Fatal("old revision resurrected after correction", e)
	}
	ingest(record("at-start", "2", start.Add(80*time.Minute), 46.5, true))
	withdrawn, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: []string{"earthquakes"}, IncludeEvents: true})
	if e != nil || withdrawn.Layers["earthquakes"].Total != 2 {
		t.Fatal("withdrawal map semantics", withdrawn.Layers, e)
	}
	// Promotion copies the exact bounded result/receipts into durable scope
	// evidence rather than retaining a database transaction or expiring cursor.
	cite := analystscope.FromSnapshot(beforeCommit.Scope)
	cite.ReadSnapshotID = beforeCommit.ID
	var frozen analystscope.Snapshot
	e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error { var e error; frozen, e = svc.Freeze(ctx, tx, cite); return e })
	if e != nil || len(frozen.ReadManifest) == 0 {
		t.Fatal("citation promotion", e)
	}
	changed := access.Principal{ID: "test:integration", Kind: "human", Grants: []access.Grant{{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "observations.read"}, SourceIDs: []string{"unavailable-source"}}}}
	// Use the original principal with a changed grant revision, as an auth
	// refresh would; a different owner cannot read the snapshot either.
	principal, _, _ := access.From(ctx)
	changed.ID = principal.ID
	restricted, e := access.Bind(context.Background(), changed, f.resource, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.ReadPage(restricted, beforeCommit.ID, "event_records", 0, 100); e == nil {
		t.Fatal("access revision did not invalidate cursor")
	}
	if _, e = svc.Get(restricted, frozen.ID); e == nil {
		t.Fatal("durable citation leaked a required source")
	}
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE analyst_read_snapshots SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, beforeCommit.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.ReadPage(ctx, beforeCommit.ID, "event_records", 0, 100); e == nil {
		t.Fatal("expired cursor remained readable")
	}
	if e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error { _, e := svc.Freeze(ctx, tx, cite); return e }); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("expired cursor could be promoted", e)
	}
	if _, e = svc.Get(ctx, frozen.ID); e != nil {
		t.Fatal("durable citation expired with its cursor", e)
	}
	bounded := false
	for i := 0; i < 17; i++ {
		_, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: []string{"weather"}})
		if apperr.Is(e, apperr.CodeConflict) {
			bounded = true
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if !bounded {
		t.Fatal("active read manifests were not bounded")
	}
	var count int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM analyst_read_snapshots WHERE principal_id='scope-fixture' AND expires_at>clock_timestamp()`).Scan(&count); e != nil || count != 16 {
		t.Fatal("read slot budget exceeded", count, e)
	}
}

func TestMapFirstCaptureScopeContainmentTimeAndAtomicity(t *testing.T) {
	f := newPublicFixture(t)
	admin := access.Bootstrap(context.Background())
	store := access.NewStore(f.h.pool)
	if e := store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"imagery.jobs.submit"}}); e != nil {
		t.Fatal(e)
	}
	registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.EarthSearchURL, Adapter: publicingest.EarthSearchAdapter, MaxRequestsPerDay: 20}})
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	assets := map[string]any{}
	for _, key := range []string{"red", "green", "blue", "nir", "scl", "product_metadata"} {
		assets[key] = map[string]any{"href": "https://e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com/sentinel-2-c1-l2a/synthetic-never-fetched/" + key + ".tif", "type": "image/tiff", "raster:bands": []any{map[string]any{"nodata": 0, "scale": .0001, "offset": 0, "spatial_resolution": 10}}}
	}
	feature := map[string]any{"type": "Feature", "id": "S2A_SYNTHETIC_SCOPE_TEST", "collection": publicingest.EarthSearchDataset, "stac_version": "1.1.0", "bbox": []float64{46, 24, 47, 25}, "geometry": json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`), "properties": map[string]any{"datetime": at, "platform": "sentinel-2a", "grid:code": "SYNTHETIC", "eo:cloud_cover": 0, "s2:processing_baseline": "05.11", "s2:product_type": "S2MSI2A", "s2:product_uri": "S2A_SYNTHETIC_R005_METADATA"}, "assets": assets}
	raw, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": []any{feature}, "links": []any{}})
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		p, e := publicingest.DecodeEarthSearch(raw, in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
	})
	f.admission.Dataset = publicingest.EarthSearchDataset
	f.admission.ProductVersion = "sentinel2-c1-l2a"
	f.admission.Policy.MinIntervalSeconds = 5
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.MaxResponseBytes = 32768
	f.admit(t)
	if run := f.execute(t); run["status"] != "SUCCEEDED" {
		t.Fatal("synthetic C1 metadata retention", run)
	}
	code, catalog := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/imagery/scenes", nil, nil)
	if code != 200 || len(catalog["items"].([]any)) != 1 {
		t.Fatal("C1 metadata projection", code, catalog)
	}
	scene := catalog["items"].([]any)[0].(map[string]any)
	in := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Holed synthetic capture scope", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[46.5,24],[46.5,24.5],[46,24.5],[46,24]],[[46.10,24.10],[46.13,24.10],[46.13,24.13],[46.10,24.13],[46.10,24.10]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: at.Add(-time.Hour), To: at.Add(time.Hour)}, RevisionView: "latest", Timezone: "UTC"}}
	capture := imagery.CaptureInput{RequestKey: "frozen-scope-capture", SceneID: scene["id"].(string), AOI: publicingest.Bounds{West: 46.2, South: 24.2, East: 46.22, North: 24.22}, Scope: &in}
	code, queued := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", capture, nil)
	if code != 201 {
		t.Fatal("scoped capture queue", code, queued)
	}
	frozen := queued["input"].(map[string]any)["scope"].(map[string]any)
	if frozen["area"].(map[string]any)["persistent"] != true || frozen["scopeHash"] == "" {
		t.Fatal("capture omitted durable scope", frozen)
	}
	if code, again := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", capture, nil); code != 201 || again["id"] != queued["id"] {
		t.Fatal("scoped capture retry lost identity", code, again)
	}
	var before int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM analyst_scope_snapshots`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	for _, b := range []publicingest.Bounds{{West: 46.10, South: 24.10, East: 46.12, North: 24.12}, {West: 46.8, South: 24.2, East: 46.82, North: 24.22}} {
		bad := capture
		bad.RequestKey = "wrong-area"
		bad.AOI = b
		if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", bad, nil); code != 400 {
			t.Fatal("hole/outside-parent capture accepted", code)
		}
	}
	badScope := in
	badScope.Temporal = in.Temporal
	badScope.Temporal.Interval = &queryscope.Interval{From: at.Add(-time.Hour), To: at}
	bad := capture
	bad.RequestKey = "outside-period"
	bad.Scope = &badScope
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", bad, nil); code != 400 {
		t.Fatal("exclusive end acquisition accepted", code)
	}
	var after, jobs int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM analyst_scope_snapshots),(SELECT count(*) FROM imagery_capture_jobs)`).Scan(&after, &jobs); e != nil || after != before || jobs != 1 {
		t.Fatal("rejected capture left partial scope/job writes", before, after, jobs, e)
	}
	resolved, e := (analystscope.Service{Pool: f.h.pool}).Resolve(scopedFixtureContext(t, f), badScope)
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/v1/imagery/scenes", "/api/v1/imagery/captures"} {
		if code, b := scopeRequest(t, f.h, f.manager, "GET", scopedCatalogURL(path, resolved), nil, nil); code != 200 || len(b["items"].([]any)) != 0 {
			t.Fatal("acquisition scope ignored", path, code, b)
		}
	}
	// Geography and imagery can have different sources. The frozen nested
	// scope follows both grants even though the capture's primary source is A.
	code, sourceB := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources", map[string]any{"id": "scope-geography", "name": "Synthetic separate geography source", "type": "external"}, nil)
	if code != 201 {
		t.Fatal(code, sourceB)
	}
	geoSource := sourceB["id"].(string)
	geoRaw := json.RawMessage(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]},"properties":{"key":"country:CAPTURE","kind":"country","name":"Source-restricted synthetic boundary"}}]}`)
	edition, _, e := reference.NewService(f.h.pool).AdmitGeography(scopedFixtureContext(t, f), reference.GeographyInput{SourceID: geoSource, Label: "Synthetic source boundary", Version: "1", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"synthetic":true}`), Artifact: geoRaw, SHA256: reference.Hash(geoRaw)})
	if e != nil {
		t.Fatal(e)
	}
	withBoundary := in
	withBoundary.Area = areas.Selection{RegionID: edition.ID + ":country:CAPTURE"}
	capture.Scope = &withBoundary
	capture.RequestKey = "capture-with-separate-geography"
	code, adminCapture := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", capture, nil)
	if code != 201 {
		t.Fatal("administrative capture scope", code, adminCapture)
	}
	reader := scopeToken(t, f.h, "capture-no-boundary-grant", f.resource, []string{"sources.read"}, []string{f.source})
	if code, _ := scopeRequest(t, f.h, reader, "GET", "/api/v1/imagery/captures/"+adminCapture["id"].(string), nil, nil); code != 404 {
		t.Fatal("nested boundary scope source was disclosed", code)
	}
	if code, _ := scopeRequest(t, f.h, reader, "GET", "/api/v1/imagery/captures/"+queued["id"].(string), nil, nil); code != 200 {
		t.Fatal("independent drawn capture was hidden", code)
	}
	for _, key := range []string{"private-boundary-second", "private-boundary-third"} {
		capture.RequestKey = key
		if code, b := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/imagery/captures", capture, nil); code != 201 {
			t.Fatal(code, b)
		}
	}
	limited, _ := store.Authenticate(context.Background(), reader, nil)
	limitedCtx, e := store.BindSession(context.Background(), limited, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	var available bool
	if e = db.Scoped(f.h.pool).QueryRow(limitedCtx, `SELECT emad_imagery_queue_available('capture',$1)`, f.source).Scan(&available); e != nil || available {
		t.Fatal("hidden provenance bypassed the shared existing queue bound", available, e)
	}
}
