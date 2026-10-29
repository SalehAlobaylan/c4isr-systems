package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/acquisition"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/coverage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

type accountFetcherFunc func(context.Context, publicingest.Location, publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error)

func (f accountFetcherFunc) Fetch(c context.Context, l publicingest.Location, r publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
	return f(c, l, r)
}
func acquisitionFixture(t *testing.T) (*publicFixture, acquisition.Input) {
	t.Helper()
	f := newPublicFixture(t)
	if e := access.NewStore(f.h.pool).AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"acquisitions.read", "acquisitions.request"}}); e != nil {
		t.Fatal(e)
	}
	f.admit(t)
	var admission string
	if e := f.h.pool.QueryRow(context.Background(), `SELECT admission_id FROM retrieval_profile_state WHERE source_id=$1`, f.source).Scan(&admission); e != nil {
		t.Fatal(e)
	}
	end := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	in := acquisition.Input{RequestKey: "durable-acquisition-intent-1", SourceID: f.source, ProfileKey: "default", ProfileRevisionID: "rpr_" + admission, Purpose: "discovery", Scope: analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Selected fixture area", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46.2,24.2],[46.8,24.2],[46.8,24.8],[46.2,24.8],[46.2,24.2]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: end.Add(-time.Hour), To: end}, RevisionView: "latest", Timezone: "UTC"}}}
	return f, in
}
func acquisitionCall(t *testing.T, f *publicFixture, token, method, path string, in any, want int) map[string]any {
	t.Helper()
	code, v := scopeRequest(t, f.h, token, method, path, in, nil)
	if code != want {
		t.Fatalf("%s %s: %d want %d %v", method, path, code, want, v)
	}
	return v
}
func TestMapFirstDurableAcquisitionLostResponseCoverageAndCancellation(t *testing.T) {
	f, in := acquisitionFixture(t)
	create := func(in acquisition.Input) acquisition.Request {
		raw, _ := json.Marshal(acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", in, 201))
		var v acquisition.Request
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := create(in)
	if first.Kind != "ACQUISITION" || len(first.Parts) != 1 {
		t.Fatal("not queued", first)
	}
	retry := create(in)
	if retry.ID != first.ID || retry.Scope.ID != first.Scope.ID {
		t.Fatal("lost-response retry changed frozen intent", first, retry)
	}
	changed := in
	changed.Purpose = "analysis"
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", changed, 409)
	acquisitionCall(t, f, f.worker, "POST", "/api/v1/acquisitions", in, 403)
	query := func(scope analystscope.Input) coverage.Result {
		raw, _ := json.Marshal(acquisitionCall(t, f, f.manager, "POST", "/api/v1/coverage/query", coverage.Input{Scope: scope, SourceID: f.source}, 200))
		var v coverage.Result
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if before := query(in.Scope); len(before.Sources) != 1 || before.Sources[0].Retrieval != "no_retrieval" {
		t.Fatal("unqueried scope misrepresented", before)
	}
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{}, Checkpoint: ""}, nil)
	run := acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, 201)
	if run["status"] != "SUCCEEDED" || run["acquisitionRequestId"] != first.ID {
		t.Fatal("request not executed", run)
	}
	completed := acquisitionCall(t, f, f.manager, "GET", "/api/v1/acquisitions/"+first.ID, nil, 200)
	if completed["status"] != "SUCCEEDED" {
		t.Fatal("request not completed", completed)
	}
	after := query(in.Scope)
	if got := after.Sources[0]; got.Retrieval != "complete_query_footprint" || got.DataResult != "successful_empty_retrieval" || got.ObservationCompleteness != "unknown" {
		t.Fatal("empty retrieval meaning", got)
	}
	// A smaller historic query is covered by its original receipt even after
	// today's approved footprint moves elsewhere.
	moved := f.admission
	moved.Policy.AOI = publicingest.Bounds{West: 50, South: 24, East: 51, North: 25}
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", moved, 201)
	historic := query(in.Scope).Sources[0]
	if historic.SpatialRelation != "none" || historic.Retrieval != "complete_query_footprint" {
		t.Fatal("current footprint replaced historical receipt", historic)
	}
	in.RequestKey = "outside-approved-envelope"
	in.ProfileRevisionID = "rpr_" + acquisitionCall(t, f, f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200)["id"].(string)
	review := create(in)
	if review.Kind != "COVERAGE_REVIEW" || len(review.Parts) != 0 {
		t.Fatal("outside area auto-authorized", review)
	}
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions/"+review.ID+"/cancel", nil, 200)
	// Restore the approved envelope, then cancel during actual provider I/O.
	f.admit(t)
	in.RequestKey = "cancel-inflight-intent"
	in.ProfileRevisionID = "rpr_" + acquisitionCall(t, f, f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200)["id"].(string)
	pending := create(in)
	f.ready(t)
	f.fetcher.entered = make(chan struct{}, 1)
	release := make(chan struct{})
	f.fetcher.release = release
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	done := make(chan struct{})
	go func() {
		scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, nil)
		close(done)
	}()
	select {
	case <-f.fetcher.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider I/O not reached")
	}
	var owner string
	if e := f.h.pool.QueryRow(context.Background(), `SELECT query_request_id FROM retrieval_profile_state WHERE source_id=$1 AND profile_key='default'`, f.source).Scan(&owner); e != nil || owner != pending.ID {
		t.Fatal("fixture did not reach the claimed acquisition's provider I/O", owner, e)
	}
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions/"+pending.ID+"/cancel", nil, 200)
	close(release)
	select {
	case <-done:
	case <-time.After(7 * time.Second):
		t.Fatal("cancelled worker did not finish")
	}
	var count int
	if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM retrieval_receipts WHERE request_id=$1`, pending.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("stale completion receipt", count, e)
	}
	if got := acquisitionCall(t, f, f.manager, "GET", "/api/v1/acquisitions/"+pending.ID, nil, 200); got["status"] != "CANCELLED" {
		t.Fatal("cancelled intent changed", got)
	}
}

func TestMapFirstSharedAccountQuotaUncertainChargeAndStorageGuards(t *testing.T) {
	f, in := acquisitionFixture(t)
	loc := f.registry["fixture-location"]
	loc.AccountBudget = &publicingest.AccountBudget{Group: "shared-fixture-account", DailyCost: 2, RequestCost: 1, Concurrency: 1, MinIntervalSeconds: 1}
	f.registry[loc.ID] = loc
	f.execute(t)
	f.ready(t)
	// A second source sharing the account cannot multiply its allowance.
	source := acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources", map[string]any{"id": "second-fixture-feed", "name": "Second reviewed feed", "type": "external"}, 201)["id"].(string)
	worker := scopeToken(t, f.h, "second-account-worker", f.resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{source})
	second := loc
	second.ID = "second-account-location"
	second.SourceID = source
	f.registry[second.ID] = second
	admission := f.admission
	admission.ConnectorID = second.ID
	admission.ConnectorPrincipalID = "second-account-worker"
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+source+"/admission", admission, 201)
	f.fetcher.set(publicingest.Page{}, &publicingest.RetrievalError{Code: "provider_unavailable"})
	f.fetcher.delegate = accountFetcherFunc(func(context.Context, publicingest.Location, publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		return publicingest.Page{}, publicingest.Retrieval{}, &publicingest.RetrievalError{Code: "provider_unavailable"}
	})
	failed := acquisitionCall(t, f, worker, "POST", "/api/v1/sources/"+source+"/connector-runs", nil, 201)
	if failed["status"] != "FAILED" {
		t.Fatal(failed)
	}
	f.ready(t)
	before := f.fetcher.count()
	limited := f.execute(t)
	if limited["failureCode"] != "account_quota" || f.fetcher.count() != before {
		t.Fatal("account allowance multiplied", limited)
	}
	var uncertain int
	var spent int64
	if e := f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM provider_cost_reservations WHERE state='UNCERTAIN'),spent FROM provider_budget_state WHERE group_key='shared-fixture-account'`).Scan(&uncertain, &spent); e != nil || uncertain != 1 || spent != 2 {
		t.Fatal("lost response refunded", uncertain, spent, e)
	}
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE provider_cost_reservations SET lease_until='epoch' WHERE state='UNCERTAIN'`); e != nil {
		t.Fatal(e)
	}
	f.ready(t)
	if got := f.execute(t); got["failureCode"] != "account_quota" {
		t.Fatal("expired concurrency lease refunded uncertain charge", got)
	}
	// A source-restricted caller gets no account group or other-source totals.
	metrics := acquisitionCall(t, f, f.worker, "GET", "/api/v1/sources/"+f.source+"/acquisition-metrics", nil, 200)
	if _, ok := metrics["group"]; ok || metrics["attemptsToday"] != float64(1) {
		t.Fatal("account usage leaked", metrics)
	}
	// Active estimated bytes are reserved before provider work. Exhaustion
	// blocks another acquisition without deleting retained evidence/history.
	ctx := scopeContext(t, f.h, f.resource)
	if e := capacity.Reserve(ctx, f.h.pool, "fixture-full-storage", f.source, "fixture", capacity.StorageBytes, time.Now().Add(time.Minute)); e == nil {
		t.Fatal("reservation ignored already retained bytes")
	}
	if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO resource_capacity_reservations(id,scope_id,source_id,kind,estimated_bytes,lease_until)VALUES('fixture-full-storage',$1,$2,'fixture',$3,clock_timestamp()+interval '1 minute')`, f.resource.ID(), f.source, capacity.StorageBytes); e != nil {
		t.Fatal(e)
	}
	in.RequestKey = "storage-coverage-query"
	body := acquisitionCall(t, f, f.manager, "POST", "/api/v1/coverage/query", coverage.Input{Scope: in.Scope, SourceID: f.source}, 200)
	if body["storage"] != "blocked" {
		t.Fatal("storage gate absent", body)
	}
}

func TestMapFirstCoverageUnionCappedReceiptsAndAccessIsolation(t *testing.T) {
	f, in := acquisitionFixture(t)
	profilePath := "/api/v1/sources/" + f.source + "/profiles"
	requests := []acquisition.Input{}
	for i, key := range []string{"a", "b"} {
		policy := f.admission
		policy.AnalystQuestion = "Inspect an approved adjacent part of the selected study area"
		west, east := 46.2, 46.5
		if i == 1 {
			west, east = 46.5, 46.8
		}
		policy.Policy.AOI = publicingest.Bounds{West: west, South: 24.2, East: east, North: 24.8}
		approved := acquisitionCall(t, f, f.manager, "POST", profilePath+"/"+key, publicingest.ProfileInput{Admission: policy}, 201)
		revision := "rpr_" + approved["id"].(string)
		acquisitionCall(t, f, f.manager, "POST", profilePath+"/"+key+"/execution", publicingest.ExecutionGrant{ProfileRevisionID: revision, PrincipalID: "connector-fixture", Enabled: true}, 200)
		request := in
		request.RequestKey = "adjacent-profile-" + key
		request.ProfileKey = key
		request.ProfileRevisionID = revision
		geometry, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][]float64{{{west, 24.2}, {east, 24.2}, {east, 24.8}, {west, 24.8}, {west, 24.2}}}})
		request.Scope.Area = areas.Selection{Kind: "drawn", Name: "Adjacent part " + key, Geometry: geometry}
		acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", request, 201)
		requests = append(requests, request)
	}
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{}}, nil)
	for range requests {
		f.ready(t)
		run := acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, 201)
		if run["status"] != "SUCCEEDED" || run["acquisitionRequestId"] == "" {
			t.Fatal("adjacent profile did not execute", run)
		}
	}
	getCoverage := func(scope analystscope.Input) coverage.Result {
		raw, _ := json.Marshal(acquisitionCall(t, f, f.manager, "POST", "/api/v1/coverage/query", coverage.Input{Scope: scope, SourceID: f.source}, 200))
		var v coverage.Result
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	union := getCoverage(in.Scope).Sources[0]
	if union.Retrieval != "complete_query_footprint" || len(union.Receipts) != 2 || len(union.MissingWindows) != 0 {
		t.Fatal("adjacent receipt footprints were not unioned", union)
	}
	// A capped/rejected response for a different interval cannot become a
	// complete or successfully empty retrieval merely because no row appeared.
	partial := in
	partial.RequestKey = "capped-provider-intent"
	period := *in.Scope.Temporal.Interval
	period.From = period.From.Add(-4 * time.Hour)
	period.To = period.To.Add(-4 * time.Hour)
	partial.Scope.Temporal.Interval = &period
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", partial, 201)
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{}, PartialReason: "provider_cap", ExcludedRecords: 2}, nil)
	f.ready(t)
	run := acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, 201)
	if run["status"] != "PARTIAL" {
		t.Fatal("cap ignored", run)
	}
	capped := getCoverage(partial.Scope).Sources[0]
	if capped.Retrieval != "partial_retrieval" || capped.DataResult == "successful_empty_retrieval" || len(capped.CompletedWindows) > 0 || capped.Receipts[0].RejectedRecords != 2 {
		t.Fatal("partial response claimed completeness", capped)
	}
	// A live provider prerequisite is independent from retained coverage.
	loc := f.registry["fixture-location"]
	loc.GateState = "NEEDS_CREDENTIALS"
	loc.GateNextAction = "Supply reviewed server credentials"
	f.registry[loc.ID] = loc
	gated := getCoverage(in.Scope).Sources[0]
	if gated.Profiles[0].Gate != "NEEDS_CREDENTIALS" || gated.Retrieval != "complete_query_footprint" {
		t.Fatal("prerequisite erased retained receipts", gated)
	}
	for _, action := range gated.Actions {
		if action.Kind == "request_acquisition" {
			t.Fatal("gated source offered execution", action)
		}
	}
	hidden := scopeToken(t, f.h, "coverage-other-source", f.resource, []string{"sources.read"}, []string{"foreign-source"})
	acquisitionCall(t, f, hidden, "POST", "/api/v1/coverage/query", coverage.Input{Scope: in.Scope, SourceID: f.source}, 403)
	private := acquisitionCall(t, f, hidden, "POST", "/api/v1/coverage/query", coverage.Input{Scope: in.Scope}, 200)
	if len(private["sources"].([]any)) != 0 {
		t.Fatal("hidden source leaked through coverage", private)
	}
}

func TestMapFirstCoveragePublicationRecoveryFreshnessAndUnavailableQuery(t *testing.T) {
	f, in := acquisitionFixture(t)
	if _, e := f.h.pool.Exec(context.Background(), `CREATE FUNCTION fixture_publication_gap() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic publication gap'; END $$;CREATE TRIGGER fixture_publication_gap BEFORE INSERT ON public_evidence_catalog FOR EACH ROW EXECUTE FUNCTION fixture_publication_gap()`); e != nil {
		t.Fatal(e)
	}
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", in, 201)
	run := acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, 201)
	if run["status"] != "SUCCEEDED" {
		t.Fatal(run)
	}
	query := func(scope analystscope.Input) coverage.Source {
		raw, _ := json.Marshal(acquisitionCall(t, f, f.manager, "POST", "/api/v1/coverage/query", coverage.Input{Scope: scope, SourceID: f.source}, 200))
		var v coverage.Result
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v.Sources[0]
	}
	before := query(in.Scope)
	if before.Retrieval != "partial_retrieval" || before.Receipts[0].UnpublishedRecords != 1 || before.Receipts[0].QueryComplete {
		t.Fatal("unpublished response claimed coverage", before)
	}
	if _, e := f.h.pool.Exec(context.Background(), `DROP TRIGGER fixture_publication_gap ON public_evidence_catalog;UPDATE public_evidence_outbox SET next_attempt_at='epoch'`); e != nil {
		t.Fatal(e)
	}
	calls := f.fetcher.count()
	acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, 200)
	after := query(in.Scope)
	if after.Retrieval != "complete_query_footprint" || after.Receipts[0].RecordedUnpublishedRecords != 1 || after.Receipts[0].UnpublishedRecords != 0 || !after.Receipts[0].PublicationReconciled || f.fetcher.count() != calls {
		t.Fatal("publication recovery refetched or lost original gap", after)
	}
	if after.DataResult == "successful_empty_retrieval" {
		t.Fatal("nonempty provider response with unusable geometry became an empty retrieval", after)
	}
	var originalGaps int
	if e := f.h.pool.QueryRow(context.Background(), `SELECT (document->>'unpublishedRecords')::int FROM retrieval_receipts WHERE run_id=$1`, run["id"]).Scan(&originalGaps); e != nil || originalGaps != 1 {
		t.Fatal("original receipt was rewritten", originalGaps, e)
	}
	// Freshness measures the last successful retrieval, independently of input
	// usability and of a later failed provider query.
	f.admission.Policy.FreshForSeconds = 1
	f.admit(t)
	time.Sleep(1100 * time.Millisecond)
	if stale := query(in.Scope); stale.Freshness != "stale" {
		t.Fatal("historical receipt age replaced by current admission", stale)
	}
	next := in
	next.RequestKey = "failed-query-separate-window"
	period := *in.Scope.Temporal.Interval
	period.From = period.From.Add(-4 * time.Hour)
	period.To = period.To.Add(-4 * time.Hour)
	next.Scope.Temporal.Interval = &period
	next.ProfileRevisionID = "rpr_" + acquisitionCall(t, f, f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200)["id"].(string)
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/acquisitions", next, 201)
	f.ready(t)
	f.fetcher.set(publicingest.Page{}, &publicingest.RetrievalError{Code: "provider_unavailable"})
	acquisitionCall(t, f, f.worker, "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, 201)
	if failed := query(next.Scope); failed.Retrieval != "unavailable_query" || failed.Freshness != "unknown" || failed.LatestRetrieval != nil {
		t.Fatal("failed provider attempt became successful freshness", failed)
	}
}
