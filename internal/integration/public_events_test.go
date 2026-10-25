package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func TestPublicEventSemanticsCorrectionsAndSpatialRules(t *testing.T) {
	f := newPublicFixture(t)
	store := access.NewStore(f.h.pool)
	if err := store.AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"geofences.manage", "alerts.read", "observations.ingest"}}); err != nil {
		t.Fatal(err)
	}
	status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geofences", map[string]any{"id": "event-zone", "name": "Reviewed zone", "type": "restricted", "severity": "high", "polygon": []geo.Point{{Lat: 24, Lng: 46}, {Lat: 25, Lng: 46}, {Lat: 25, Lng: 47}, {Lat: 24, Lng: 47}}}, nil)
	if status != 201 {
		t.Fatalf("zone: %d %v", status, body)
	}
	f.admit(t)
	base := time.Now().UTC().Add(-time.Hour)
	projectionFault := true
	if _, err := f.h.pool.Exec(context.Background(), `CREATE FUNCTION fixture_fail_event_catalog() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture projection failure'; END $$; CREATE TRIGGER fixture_fail_event_catalog BEFORE INSERT ON public_evidence_catalog FOR EACH ROW EXECUTE FUNCTION fixture_fail_event_catalog()`); err != nil {
		t.Fatal(err)
	}
	publish := func(id, revision, typ string, at time.Time, position *geo.Point, area *publicingest.Footprint, withdrawn bool) {
		t.Helper()
		f.ready(t)
		role := "reported_location"
		semantic := "point_event"
		if area != nil {
			role = "measurement_footprint"
			semantic = "area"
		}
		raw, _ := json.Marshal(map[string]any{"id": id, "revision": revision, "position": position, "withdrawn": withdrawn})
		f.fetcher.set(publicingest.Page{Records: []publicingest.Record{{ProviderRecordID: id, ProviderRevision: revision, ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/record", OriginalRecord: raw, Observation: &publicingest.ReportedObservation{SemanticKind: semantic, Type: typ, Title: "Provider report", ObservedAt: base, Position: position, Area: area, GeometryRole: role, Withdrawn: withdrawn}}}}, nil)
		if run := f.execute(t); run["status"] != "SUCCEEDED" {
			t.Fatalf("publish: %v", run)
		}
		if h := f.health(t); !projectionFault && h["pendingPublications"] != float64(0) {
			t.Fatalf("projection pending: %v", h)
		}
	}
	p := &geo.Point{Lat: 24.5, Lng: 46.5}
	publish("quake", "1", "event.earthquake", base, p, nil, false)
	var partial int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM observations)+(SELECT count(*) FROM public_event_current)+(SELECT count(*) FROM alerts)`).Scan(&partial); err != nil || partial != 0 {
		t.Fatalf("failed projection leaked partial event state: %d %v", partial, err)
	}
	if _, err := f.h.pool.Exec(context.Background(), `DROP TRIGGER fixture_fail_event_catalog ON public_evidence_catalog`); err != nil {
		t.Fatal(err)
	}
	if status, body := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, nil); status != 200 || body["published"] != float64(1) {
		t.Fatalf("event publication recovery: %d %v", status, body)
	}
	projectionFault = false
	publish("quake", "1", "event.earthquake", base, p, nil, false)
	_, list := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events", nil, nil)
	if list["total"] != float64(1) {
		t.Fatalf("duplicate event: %v", list)
	}
	first := list["items"].([]any)[0].(map[string]any)
	_, alerts := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/alerts?state=ACTIVE", nil, nil)
	if alerts["total"] != float64(1) {
		t.Fatalf("point PostGIS alert absent: %v", alerts)
	}
	a := alerts["items"].([]any)[0].(map[string]any)
	ref := a["sourceReference"].(map[string]any)
	if ref["ruleVersion"] == "" || ref["evidenceId"] != first["evidenceId"] {
		t.Fatalf("rule provenance missing: %v", a)
	}
	publish("quake", "2", "event.earthquake", base.Add(time.Minute), &geo.Point{Lat: 26, Lng: 48}, nil, false)
	_, alerts = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/alerts?state=ACTIVE", nil, nil)
	if alerts["total"] != float64(0) {
		t.Fatalf("corrected location retained old alert: %v", alerts)
	}
	publish("quake", "old", "event.earthquake", base.Add(-time.Minute), p, nil, false)
	_, list = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events", nil, nil)
	if list["items"].([]any)[0].(map[string]any)["providerRevision"] != "2" {
		t.Fatal("late revision rolled current event backwards")
	}
	publish("quake", "3", "event.earthquake", base.Add(2*time.Minute), nil, nil, true)
	_, history := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-events/"+first["evidenceId"].(string)+"/revisions", nil, nil)
	if history["total"] != float64(4) {
		t.Fatalf("revision history lost: %v", history)
	}
	publish("thermal", "1", "detection.thermal", base, p, nil, false)
	publish("footprint", "1", "detection.thermal", base, nil, &publicingest.Footprint{Kind: "hazard", Polygon: []geo.Point{{Lat: 24.9, Lng: 46.9}, {Lat: 25.1, Lng: 46.9}, {Lat: 25.1, Lng: 47.1}, {Lat: 24.9, Lng: 47.1}}}, false)
	var tracks, observations int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM tracks),(SELECT count(*) FROM observations)`).Scan(&tracks, &observations); err != nil || tracks != 0 || observations != 5 {
		t.Fatalf("semantic isolation: tracks=%d observations=%d err=%v", tracks, observations, err)
	}
	// Direct point-event ingestion takes the same semantic route.
	status, body = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/observations", map[string]any{"id": "direct-earthquake", "sourceId": f.source, "type": "event.earthquake", "observedAt": base, "position": p}, nil)
	if status != 201 || body["semanticKind"] != "point_event" {
		t.Fatalf("direct event: %d %v", status, body)
	}
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/observations", map[string]any{"sourceId": f.source, "type": "event.earthquake", "semanticKind": "moving", "observedAt": base, "position": p}, nil); status != 400 {
		t.Fatalf("explicit moving earthquake accepted: %d", status)
	}
	other := scopeToken(t, f.h, "other-event-reader", f.resource, []string{"operational.read", "observations.read"}, []string{"foreign-source"})
	_, hidden := scopeRequest(t, f.h, other, "GET", "/api/v1/public-events", nil, nil)
	if hidden["total"] != float64(0) {
		t.Fatal("source-restricted event list disclosed evidence")
	}
	if status, _ = scopeRequest(t, f.h, other, "GET", "/api/v1/public-events/"+first["evidenceId"].(string), nil, nil); status != 404 {
		t.Fatalf("source-restricted event detail: %d", status)
	}
	foreign := f.resource
	foreign.WorkspaceID = "foreign-events"
	for _, table := range []string{"public_event_current", "public_event_revisions"} {
		var n int
		if err := platformdb.Scoped(f.h.pool).QueryRow(scopeContext(t, f.h, foreign), "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("event database scope leak: %d %v", n, err)
		}
	}
}
