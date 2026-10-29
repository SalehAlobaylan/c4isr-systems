package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func TestMapFirstSiblingFailureUnknownCountsAndFrozenValidity(t *testing.T) {
	f, in := acquisitionFixture(t)
	at := time.Now().UTC().Add(-90 * time.Minute).Truncate(time.Second)
	record := publicingest.Record{ProviderRecordID: "reported-event", ProviderRevision: "1", ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/event", OriginalRecord: json.RawMessage(`{"fixture":"provider-report"}`), Observation: &publicingest.ReportedObservation{Type: "event.earthquake", SemanticKind: "point_event", Title: "Synthetic retained provider report", ObservedAt: at, Position: &geo.Point{Lng: 46.5, Lat: 24.5}, GeometryRole: "reported_location", Properties: map[string]any{}, Quality: map[string]any{"reviewStatus": "automatic"}}}
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{record}}, nil)
	f.execute(t)
	first := f.evidence(t)["items"].([]any)[0].(map[string]any)["id"].(string)
	// Monitoring has an explicit evaluated interval and can query current-only
	// families. Force a real SQL-family error inside its repeatable-read batch.
	monitoring := in.Scope
	period := *in.Scope.Temporal.Interval
	monitoring.Temporal.Interval = &period
	monitoring.Temporal.Mode = "monitoring"
	monitoring.Temporal.Interval.To = time.Now().UTC()
	monitoring.Temporal.Interval.From = at.Add(-time.Minute)
	if _, e := f.h.pool.Exec(context.Background(), `REVOKE SELECT ON public_context_records FROM emad_data`); e != nil {
		t.Fatal(e)
	}
	batch := acquisitionCall(t, f, f.manager, "POST", "/api/v1/analyst-scope/read", analystscope.ReadInput{Input: monitoring, Families: []string{"earthquakes", "weather"}, IncludeEvents: true}, 200)
	layers := batch["layers"].(map[string]any)
	weather := layers["weather"].(map[string]any)
	if layers["earthquakes"].(map[string]any)["total"] != float64(1) || weather["state"] != "unavailable" || weather["total"] != nil || batch["events"].(map[string]any)["total"] != float64(1) {
		t.Fatal("failed family erased siblings or supplied zero", batch)
	}
	if _, e := f.h.pool.Exec(context.Background(), `GRANT SELECT ON public_context_records TO emad_data`); e != nil {
		t.Fatal(e)
	}
	// Save the exact evidence and fixed historical read before a correction.
	store := access.NewStore(f.h.pool)
	if e := store.AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"investigations.read", "investigations.create"}}); e != nil {
		t.Fatal(e)
	}
	historical := in.Scope
	frozen := acquisitionCall(t, f, f.manager, "POST", "/api/v1/analyst-scope/read", analystscope.ReadInput{Input: historical, Families: []string{"earthquakes", "weather"}, IncludeEvents: true}, 200)
	if frozen["layers"].(map[string]any)["weather"].(map[string]any)["state"] != "historical_unsupported" {
		t.Fatal("current-only family claimed history", frozen)
	}
	saved := acquisitionCall(t, f, f.manager, "POST", "/api/v1/investigations", map[string]any{"title": "Saved historical provider report", "question": "What was reported in the selected fixed interval?", "notes": "Provider statement, not independent confirmation.", "state": "DRAFT", "scope": historical, "evidence": []map[string]any{{"kind": "observation", "id": first}}}, 201)
	withdrawn := record
	withdrawn.ProviderRevision = "2"
	changed := time.Now().UTC().Add(-time.Minute)
	withdrawn.ProviderUpdatedAt = &changed
	copyObservation := *record.Observation
	copyObservation.Withdrawn = true
	withdrawn.Observation = &copyObservation
	withdrawn.OriginalRecord = json.RawMessage(`{"fixture":"provider-withdrawal"}`)
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{withdrawn}}, nil)
	f.ready(t)
	f.execute(t)
	oldPage := acquisitionCall(t, f, f.manager, "GET", "/api/v1/analyst-scope/reads/"+frozen["id"].(string)+"?family=earthquakes&offset=0&limit=20", nil, 200)
	if oldPage["total"] != float64(1) {
		t.Fatal("later withdrawal changed frozen result membership", oldPage)
	}
	detail := acquisitionCall(t, f, f.manager, "GET", "/api/v1/public-events/"+first, nil, 200)
	if detail["currentValidity"].(map[string]any)["state"] != "withdrawn_by_provider" || detail["reported"].(map[string]any)["withdrawn"] == true {
		t.Fatal("validity rewrote original provider report", detail)
	}
	reopened := acquisitionCall(t, f, f.manager, "GET", "/api/v1/investigations/"+saved["id"].(string), nil, 200)
	notices := reopened["evidenceValidity"].([]any)
	if len(notices) != 1 || notices[0].(map[string]any)["state"] != "withdrawn_by_provider" {
		t.Fatal("saved evidence lacks current validity notice", reopened)
	}
	current := acquisitionCall(t, f, f.manager, "POST", "/api/v1/analyst-scope/read", analystscope.ReadInput{Input: historical, Families: []string{"earthquakes"}}, 200)
	if current["layers"].(map[string]any)["earthquakes"].(map[string]any)["total"] != float64(0) {
		t.Fatal("explicit refresh resurrected withdrawn report", current)
	}
}
