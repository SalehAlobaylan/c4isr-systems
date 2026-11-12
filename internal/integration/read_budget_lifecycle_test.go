package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
)

// A retained map read holds one of a bounded number of budget slots. The
// lifecycle under test: refreshing one logical view supersedes the manifest it
// replaces and reuses its slot, a closed view releases its manifest, a
// genuinely different view keeps its own slot, and no principal can reach
// another principal's retained read.
func TestMapFirstReadBudgetLifecycleReplacementAndRelease(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopedFixtureContext(t, f)
	svc := analystscope.Service{Pool: f.h.pool}
	in := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Read budget lifecycle area", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: fixedPeriod()}
	read := func(families []string, filters analystscope.QueryFilters, zoom int, events bool) analystscope.ReadSnapshot {
		t.Helper()
		v, e := svc.Read(ctx, analystscope.ReadInput{Input: in, Families: families, Filters: filters, Zoom: zoom, IncludeEvents: events})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	slot := func(id string) int {
		t.Helper()
		var value int
		if e := f.h.pool.QueryRow(context.Background(), `SELECT budget_slot FROM analyst_read_snapshots WHERE id=$1`, id).Scan(&value); e != nil {
			t.Fatal(e)
		}
		return value
	}
	retained := func() int {
		t.Helper()
		var count int
		if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM analyst_read_snapshots WHERE principal_id='scope-fixture' AND expires_at>clock_timestamp()`).Scan(&count); e != nil {
			t.Fatal(e)
		}
		return count
	}
	exists := func(id string) bool {
		t.Helper()
		var count int
		if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM analyst_read_snapshots WHERE id=$1`, id).Scan(&count); e != nil {
			t.Fatal(e)
		}
		return count > 0
	}

	// One logical view: the same area, period, families and filters.
	view := read([]string{"weather"}, analystscope.QueryFilters{}, 0, false)
	if view.ViewKey == "" {
		t.Fatal("read did not name the logical view it belongs to")
	}
	if slot(view.ID) != 1 {
		t.Fatal("first retained read did not take the first budget slot", slot(view.ID))
	}
	// A different view keeps its own slot.
	other := read([]string{"earthquakes"}, analystscope.QueryFilters{}, 0, false)
	if other.ViewKey == view.ViewKey {
		t.Fatal("different families did not produce a different view")
	}
	if slot(other.ID) != 2 {
		t.Fatal("a different view did not keep its own budget slot", slot(other.ID))
	}

	// Repeatedly refreshing the same view, including the zoom churn that
	// exhausted the budget, must not accumulate retained reads.
	for i := 0; i < 40; i++ {
		refreshed := read([]string{"weather"}, analystscope.QueryFilters{}, 1+i%9, false)
		if refreshed.ViewKey != view.ViewKey {
			t.Fatal("a zoom change changed the identity of the logical view")
		}
		if refreshed.QueryFingerprint == view.QueryFingerprint {
			t.Fatal("a zoom change stopped changing the query fingerprint")
		}
		if refreshed.ID == view.ID {
			t.Fatal("a refresh reused the superseded manifest identity")
		}
	}
	if got := retained(); got != 2 {
		t.Fatal("refreshing one view accumulated retained reads", got)
	}
	if exists(view.ID) {
		t.Fatal("the superseded manifest was retained")
	}
	// The refreshed view reused the budget slot the superseded one held.
	refreshed := read([]string{"weather"}, analystscope.QueryFilters{}, 11, false)
	if got := slot(refreshed.ID); got != 1 {
		t.Fatal("the superseded manifest did not release its budget slot", got)
	}

	// A different filter, and events instead of layers, are different views.
	filtered := read([]string{"weather"}, analystscope.QueryFilters{Quality: "nominal"}, 0, false)
	if filtered.ViewKey == refreshed.ViewKey {
		t.Fatal("a different filter did not produce a different view")
	}
	withEvents := read([]string{"weather"}, analystscope.QueryFilters{}, 0, true)
	if withEvents.ViewKey == refreshed.ViewKey {
		t.Fatal("an event read and a layer read shared one view")
	}

	// Releasing a manifest frees its slot immediately.
	released, e := svc.Release(ctx, refreshed.ID)
	if e != nil || !released {
		t.Fatal("release did not release the retained read", released, e)
	}
	if exists(refreshed.ID) {
		t.Fatal("released manifest was retained")
	}
	after := read([]string{"bulletins"}, analystscope.QueryFilters{}, 0, false)
	if got := slot(after.ID); got != 1 {
		t.Fatal("the released budget slot was not reusable", got)
	}

	// Another principal can neither release nor read this principal's manifest.
	foreign := access.Principal{ID: "other-read-principal", Kind: "human", Grants: []access.Grant{{Resource: f.resource, Permissions: access.Permissions}}}
	foreignCtx, e := access.Bind(context.Background(), foreign, f.resource, nil)
	if e != nil {
		t.Fatal(e)
	}
	if outcome, e := svc.Release(foreignCtx, after.ID); e != nil || outcome {
		t.Fatal("another principal released a foreign retained read", outcome, e)
	}
	if !exists(after.ID) {
		t.Fatal("a foreign release removed the owner's retained read")
	}
	if _, e = svc.ReadPage(foreignCtx, after.ID, "earthquakes", 0, 10); e == nil {
		t.Fatal("another principal read a foreign retained read")
	}
	if outcome, e := svc.Release(ctx, after.ID); e != nil || !outcome {
		t.Fatal("the owner could not release its own retained read", outcome, e)
	}

	// Expiry still invalidates a retained read.
	expiring := read([]string{"earthquakes"}, analystscope.QueryFilters{}, 0, false)
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE analyst_read_snapshots SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, expiring.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.ReadPage(ctx, expiring.ID, "earthquakes", 0, 10); e == nil {
		t.Fatal("an expired retained read stayed readable")
	}
	// A changed authorization revision still invalidates a retained read.
	principal, _, _ := access.From(ctx)
	changed := principal
	changed.Grants = []access.Grant{{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "observations.read"}, SourceIDs: []string{"another-source"}}}
	restricted, e := access.Bind(context.Background(), changed, f.resource, nil)
	if e != nil {
		t.Fatal(e)
	}
	live := read([]string{"earthquakes"}, analystscope.QueryFilters{}, 0, false)
	if _, e = svc.ReadPage(restricted, live.ID, "earthquakes", 0, 10); e == nil {
		t.Fatal("an authorization change did not invalidate the retained read")
	}
	if _, e = svc.ReadPage(ctx, live.ID, "earthquakes", 0, 10); e != nil {
		t.Fatal("the owner's own retained read was invalidated", e)
	}
}

// The read budget stays bounded for genuinely different views, and the
// failure stays honest: the capacity conflict names the remedy instead of
// the symptom disappearing behind a larger quota.
func TestMapFirstReadBudgetStillBoundedForDifferentViews(t *testing.T) {
	f := newPublicFixture(t)
	ctx := scopedFixtureContext(t, f)
	svc := analystscope.Service{Pool: f.h.pool}
	in := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Bounded budget area", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: fixedPeriod()}
	bounded := false
	for i := 0; i < 24; i++ {
		variant := in
		variant.Temporal.Interval = &queryscope.Interval{From: in.Temporal.Interval.From.Add(time.Duration(i) * time.Second), To: in.Temporal.Interval.To}
		_, e := svc.Read(ctx, analystscope.ReadInput{Input: variant, Families: []string{"weather"}})
		if apperr.Is(e, apperr.CodeConflict) {
			bounded = true
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if !bounded {
		t.Fatal("the retained read budget was not bounded")
	}
	var count int
	if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM analyst_read_snapshots WHERE principal_id='scope-fixture' AND expires_at>clock_timestamp()`).Scan(&count); e != nil || count != 16 {
		t.Fatal("read slot budget exceeded", count, e)
	}
}

// The release is an ordinary principal-scoped transport operation.
func TestMapFirstReadReleaseOverHTTP(t *testing.T) {
	f := newPublicFixture(t)
	body := analystscope.ReadInput{Input: analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Release transport area", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: fixedPeriod()}, Families: []string{"earthquakes"}}
	code, read := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/analyst-scope/read", body, nil)
	if code != 200 {
		t.Fatal("read", code, read)
	}
	id, _ := read["id"].(string)
	if id == "" {
		t.Fatal("read did not return a retained manifest identity")
	}
	if code, released := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/analyst-scope/reads/"+id+"/release", nil, nil); code != 200 || released["released"] != true {
		t.Fatal("release", code, released)
	}
	if code, _ = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/analyst-scope/reads/"+id+"?family=earthquakes", nil, nil); code != 409 {
		t.Fatal("a released manifest stayed readable", code)
	}
	if code, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/analyst-scope/reads/"+id+"/release", nil, nil); code != 404 {
		t.Fatal("releasing a manifest twice was accepted", code)
	}
	// Another principal cannot release this principal's manifest.
	other := scopeToken(t, f.h, "other-read-token", f.resource, []string{"sources.read", "observations.read"}, nil)
	code, read = scopeRequest(t, f.h, other, "POST", "/api/v1/analyst-scope/read", body, nil)
	if code != 200 {
		t.Fatal("foreign read", code, read)
	}
	id, _ = read["id"].(string)
	if code, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/analyst-scope/reads/"+id+"/release", nil, nil); code != 404 {
		t.Fatal("another principal released a foreign manifest", code)
	}
	if code, _ = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/analyst-scope/reads/"+id+"?family=earthquakes", nil, nil); code != 409 {
		t.Fatal("another principal read a foreign manifest", code)
	}
}
