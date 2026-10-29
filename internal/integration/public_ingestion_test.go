package integration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

type fixtureFetcher struct {
	mu               sync.Mutex
	page             publicingest.Page
	err              error
	calls            []string
	windows          [][2]time.Time
	delegate         publicingest.Fetcher
	entered, release chan struct{}
}

func (f *fixtureFetcher) Fetch(ctx context.Context, location publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
	f.mu.Lock()
	f.calls = append(f.calls, in.Checkpoint)
	f.windows = append(f.windows, [2]time.Time{in.From, in.To})
	page, err, entered, release := f.page, f.err, f.entered, f.release
	page.CoverageFrom = in.From
	page.CoverageTo = in.To
	f.mu.Unlock()
	if f.delegate != nil {
		return f.delegate.Fetch(ctx, location, in)
	}
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return page, publicingest.Retrieval{}, ctx.Err()
		}
	}
	return page, publicingest.Retrieval{LocationID: location.ID, HTTPStatus: 200, ETag: "fixture-etag"}, err
}
func (f *fixtureFetcher) set(page publicingest.Page, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.page = page
	f.err = err
}
func (f *fixtureFetcher) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.calls) }

type publicFixture struct {
	h                       *harness
	fetcher                 *fixtureFetcher
	registry                publicingest.Registry
	resource                access.Resource
	source, manager, worker string
	admission               publicingest.AdmissionInput
}

func newPublicFixture(t *testing.T, options ...app.Option) *publicFixture {
	t.Helper()
	registry := publicingest.Registry{}
	fetcher := &fixtureFetcher{}
	h := newHarness(t, append([]app.Option{app.WithPublicIngestion(registry, fetcher)}, options...)...)
	r := access.Resource{WorkspaceID: "public-fixture", EnvironmentID: "world", Partition: access.PublicEvidence}
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	if err := store.CreateHuman(admin, "source-reviewer", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sources.read", "sources.manage", "observations.read", "tracks.read", "connectors.execute"}}); err != nil {
		t.Fatal(err)
	}
	_, manager, err := store.IssueCredential(admin, "source-reviewer", r, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	status, source := scopeRequest(t, h, manager, "POST", "/api/v1/sources", map[string]any{"id": "fixture-feed", "name": "Reviewed fixture feed", "type": "external"}, nil)
	if status != 201 {
		t.Fatalf("source: %d %v", status, source)
	}
	id := source["id"].(string)
	worker := scopeToken(t, h, "connector-fixture", r, []string{"operational.read", "sources.read", "connectors.execute"}, []string{id})
	locations, err := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: id, Resource: r, URL: "https://provider.example.invalid/feed", Adapter: "emad-json-page-v1", MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range locations {
		registry[key] = value
	}
	in := publicingest.AdmissionInput{OriginalProducer: "fixture-producer", UpstreamProvider: "fixture-upstream", ConnectorID: "fixture-location", ConnectorPrincipalID: "connector-fixture", Dataset: "fixture-events", ProductVersion: "v1", CoverageDescription: "Test-only bounded region, no live provider admission", RevisionPolicy: "provider_version", Enabled: true,
		Rights: publicingest.Rights{ID: "fixture-rights-v1", LegalBasis: "Synthetic test data owned by the test", TermsURL: "https://provider.example.invalid/terms", Attribution: "EMAD fixture", RetainRaw: true, AllowAnalysis: true},
		Policy: publicingest.Policy{AOI: publicingest.Bounds{West: 46, South: 24, East: 47, North: 25}, LookbackSeconds: 86400, FreshForSeconds: 60, UnavailableAfterSeconds: 120, MinIntervalSeconds: 1, TimeoutSeconds: 2, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 4096, MaxRequestsPerDay: 20}}
	f := &publicFixture{h, fetcher, registry, r, id, manager, worker, in}
	f.record("v1", `{ "event": "fixture", "value": 1 }`, false, "checkpoint-1")
	return f
}
func (f *publicFixture) record(revision, raw string, more bool, checkpoint string) {
	now := time.Now().UTC()
	f.fetcher.set(publicingest.Page{Records: []publicingest.Record{{ProviderRecordID: " record-1 ", ProviderRevision: revision, ReferenceURL: "https://provider.example.invalid/records/record-1", OriginalRecord: json.RawMessage(raw)}}, Checkpoint: checkpoint, HasMore: more, CoverageFrom: now.Add(-time.Hour), CoverageTo: now}, nil)
}
func (f *publicFixture) admit(t *testing.T) {
	t.Helper()
	status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", f.admission, nil)
	if status != 201 {
		t.Fatalf("admit: %d %v", status, body)
	}
}
func (f *publicFixture) ready(t *testing.T) {
	t.Helper()
	// Advance the fixture's account pacing clock alongside its source clock;
	// this never refunds spent provider credits.
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE provider_budget_state SET next_at='epoch';UPDATE provider_cost_reservations SET lease_until='epoch' WHERE state='UNCERTAIN'`); e != nil {
		t.Fatal(e)
	}
	_, err := f.h.pool.Exec(context.Background(), `UPDATE retrieval_profile_state SET next_attempt_at=$2,last_attempt_at=$2 WHERE source_id=$1`, f.source, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
}
func (f *publicFixture) execute(t *testing.T) map[string]any {
	t.Helper()
	status, body := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil)
	if status != 201 {
		t.Fatalf("execute: %d %v", status, body)
	}
	return body
}
func (f *publicFixture) health(t *testing.T) map[string]any {
	t.Helper()
	status, body := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/health", nil, nil)
	if status != 200 {
		t.Fatalf("health: %d %v", status, body)
	}
	return body
}
func (f *publicFixture) evidence(t *testing.T) map[string]any {
	t.Helper()
	status, body := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/evidence", nil, nil)
	if status != 200 {
		t.Fatalf("evidence: %d %v", status, body)
	}
	return body
}

func TestPublicAdmissionRetrievalRevisionsAndCoverage(t *testing.T) {
	f := newPublicFixture(t)
	bad := f.admission
	bad.Policy.AOI.East = bad.Policy.AOI.West
	if status, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", bad, nil); status != 400 {
		t.Fatalf("invalid AOI admitted: %d", status)
	}
	f.admit(t)
	if h := f.health(t); h["state"] != "unknown" {
		t.Fatalf("unqueried world claimed current: %v", h)
	}
	if status, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil); status != 403 {
		t.Fatalf("human executed connector: %d", status)
	}
	first := f.execute(t)
	if first["status"] != "SUCCEEDED" || first["newEvidence"] != float64(1) {
		t.Fatalf("first run: %v", first)
	}
	items := f.evidence(t)
	if items["total"] != float64(1) {
		t.Fatalf("catalog: %v", items)
	}
	e := items["items"].([]any)[0].(map[string]any)
	if e["originalProducer"] != "fixture-producer" || e["upstreamProvider"] != "fixture-upstream" || e["providerRecordId"] != "record-1" || e["connectorRunId"] != first["id"] || e["hashVersion"] != "emad-json-v1" || e["originalRecord"] != nil {
		t.Fatalf("provenance/list retention: %v", e)
	}
	var raw []byte
	if err := f.h.pool.QueryRow(context.Background(), `SELECT original_record FROM public_evidence WHERE id=$1`, e["id"]).Scan(&raw); err != nil || string(raw) != `{ "event": "fixture", "value": 1 }` {
		t.Fatalf("original bytes changed: %q %v", raw, err)
	}
	request, err := http.NewRequest("GET", f.h.server.URL+"/api/v1/sources/"+f.source+"/evidence/"+e["id"].(string)+"/original", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.manager)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	original, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != 200 || string(original) != string(raw) || fmt.Sprintf("%x", sha256.Sum256(original)) != e["rawHash"] {
		t.Fatalf("original download/hash mismatch: %d %v", response.StatusCode, readErr)
	}
	f.ready(t)
	f.record("v1", `{"value":1,"event":"fixture"}`, false, "checkpoint-1")
	if run := f.execute(t); run["duplicates"] != float64(1) || run["newEvidence"] != float64(0) {
		t.Fatalf("retry duplicated revision: %v", run)
	}
	f.ready(t)
	f.record("v2", `{"event":"fixture","value":2}`, false, "checkpoint-2")
	if run := f.execute(t); run["newEvidence"] != float64(1) {
		t.Fatalf("new revision lost: %v", run)
	}
	f.ready(t)
	f.record("v2", `{"event":"fixture","value":999}`, false, "must-not-commit")
	if run := f.execute(t); run["status"] != "FAILED" || run["failureCode"] != "revision_conflict" {
		t.Fatalf("revision mutation accepted: %v", run)
	}
	var checkpoint string
	if err := f.h.pool.QueryRow(context.Background(), `SELECT checkpoint FROM source_connector_state WHERE source_id=$1`, f.source).Scan(&checkpoint); err != nil || checkpoint != "checkpoint-2" {
		t.Fatalf("checkpoint advanced on failed commit: %q %v", checkpoint, err)
	}
	if got := f.evidence(t)["total"]; got != float64(2) {
		t.Fatalf("immutable revision count: %v", got)
	}
	f.ready(t)
	f.fetcher.set(publicingest.Page{}, &publicingest.RetrievalError{Code: "provider_unavailable", RetryAt: time.Now().Add(time.Hour)})
	if run := f.execute(t); run["status"] != "FAILED" {
		t.Fatalf("outage hidden: %v", run)
	}
	if health := f.health(t); health["state"] != "stale" || health["lastSuccessAt"] == nil {
		t.Fatalf("outage erased known coverage: %v", health)
	}
	before := f.fetcher.count()
	if status, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil); status != 409 || f.fetcher.count() != before {
		t.Fatalf("backoff bypass: %d", status)
	}
	for _, path := range []string{"observations", "tracks"} {
		status, body := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/"+path, nil, nil)
		if status != 200 || body["total"] != float64(0) {
			t.Fatalf("provider evidence became operational %s: %d %v", path, status, body)
		}
	}
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE public_evidence SET content_hash='changed' WHERE source_id=$1`, f.source); err == nil {
		t.Fatal("evidence storage was mutable")
	}
}

func TestPublicPublicationRecoveryAndCheckpointResume(t *testing.T) {
	f := newPublicFixture(t)
	f.admit(t)
	if _, err := f.h.pool.Exec(context.Background(), `CREATE FUNCTION fixture_fail_catalog() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture projection failure'; END $$; CREATE TRIGGER fixture_fail_catalog BEFORE INSERT ON public_evidence_catalog FOR EACH ROW EXECUTE FUNCTION fixture_fail_catalog()`); err != nil {
		t.Fatal(err)
	}
	f.record("v1", `{"event":"one"}`, true, "next-page")
	if run := f.execute(t); run["status"] != "PARTIAL" || run["newEvidence"] != float64(1) {
		t.Fatalf("partial retrieval: %v", run)
	}
	if health := f.health(t); health["pendingPublications"] != float64(1) || health["state"] != "unknown" {
		t.Fatalf("pending failure/partial coverage hidden: %v", health)
	}
	if got := f.evidence(t)["total"]; got != float64(0) {
		t.Fatalf("failed projection appeared: %v", got)
	}
	if _, err := f.h.pool.Exec(context.Background(), `DROP TRIGGER fixture_fail_catalog ON public_evidence_catalog`); err != nil {
		t.Fatal(err)
	}
	before := f.fetcher.count()
	status, reply := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, nil)
	if status != 200 || reply["published"] != float64(1) || f.fetcher.count() != before {
		t.Fatalf("recovery refetched evidence: %d %v", status, reply)
	}
	_, reply = scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/publications/retry", nil, nil)
	if reply["published"] != float64(0) {
		t.Fatalf("duplicate catalog publication: %v", reply)
	}
	f.ready(t)
	f.record("v2", `{"event":"two"}`, false, "watermark")
	if run := f.execute(t); run["status"] != "SUCCEEDED" {
		t.Fatalf("resume failed: %v", run)
	}
	f.fetcher.mu.Lock()
	cursor := f.fetcher.calls[len(f.fetcher.calls)-1]
	firstWindow, resumedWindow := f.fetcher.windows[0], f.fetcher.windows[len(f.fetcher.windows)-1]
	f.fetcher.mu.Unlock()
	if cursor != "next-page" {
		t.Fatalf("checkpoint was not resumed: %q", cursor)
	}
	if !firstWindow[0].Equal(resumedWindow[0]) || !firstWindow[1].Equal(resumedWindow[1]) {
		t.Fatal("resuming a cursor changed its frozen query window")
	}
	if got := f.evidence(t)["total"]; got != float64(2) {
		t.Fatalf("evidence recovery count: %v", got)
	}
}

func TestPublicConnectorLeasesRevocationAndIsolation(t *testing.T) {
	f := newPublicFixture(t)
	f.admit(t)
	f.fetcher.entered = make(chan struct{}, 1)
	f.fetcher.release = make(chan struct{})
	finished := make(chan int, 1)
	go func() {
		status, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil)
		finished <- status
	}()
	select {
	case <-f.fetcher.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("fetch not started")
	}
	if status, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil); status != 409 {
		t.Fatalf("concurrent lease allowed: %d", status)
	}
	store := access.NewStore(f.h.pool)
	session, err := store.Authenticate(context.Background(), f.worker, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeCredential(access.Bootstrap(context.Background()), session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	close(f.fetcher.release)
	select {
	case status := <-finished:
		if status != 401 {
			t.Fatalf("revoked fetch committed: %d", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("revoked request did not stop")
	}
	var total int
	if err = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM public_evidence WHERE source_id=$1`, f.source).Scan(&total); err != nil || total != 0 {
		t.Fatalf("revoked evidence retained: %d %v", total, err)
	}
	if _, err = f.h.pool.Exec(context.Background(), `UPDATE source_connector_state SET lease_until=$2,next_attempt_at=$2 WHERE source_id=$1`, f.source, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	f.ready(t)
	_, f.worker, err = store.IssueCredential(access.Bootstrap(context.Background()), "connector-fixture", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if run := f.execute(t); run["status"] != "SUCCEEDED" {
		t.Fatalf("lease recovery failed: %v", run)
	}
	var interrupted int
	if err = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM connector_runs WHERE source_id=$1 AND document->>'status'='INTERRUPTED'`, f.source).Scan(&interrupted); err != nil || interrupted != 1 {
		t.Fatalf("interruption history lost: %d %v", interrupted, err)
	}
	for _, tc := range []struct {
		name    string
		headers map[string]string
	}{
		{"workspace", map[string]string{"X-EMAD-Workspace-ID": "foreign"}},
		{"environment", map[string]string{"X-EMAD-Environment-ID": "foreign"}},
		{"exercise", map[string]string{"X-EMAD-Partition": "EXERCISE_OBSERVATION", "X-EMAD-Run-ID": "legacy"}},
		{"truth", map[string]string{"X-EMAD-Partition": "EXERCISE_TRUTH", "X-EMAD-Run-ID": "private"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if status, _ := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/evidence", nil, tc.headers); status != 403 {
				t.Fatalf("cross-scope evidence: %d", status)
			}
		})
	}
	other := scopeToken(t, f.h, "other-source-worker", f.resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{"foreign-source"})
	if status, _ := scopeRequest(t, f.h, other, "GET", "/api/v1/sources/"+f.source+"/evidence", nil, nil); status != 403 {
		t.Fatalf("source-restricted evidence disclosed: %d", status)
	}
	foreign := f.resource
	foreign.WorkspaceID = "foreign"
	for _, table := range []string{"source_admissions", "source_connector_state", "connector_runs", "public_evidence", "public_evidence_outbox", "public_evidence_catalog"} {
		var count int
		if err = platformdb.Scoped(f.h.pool).QueryRow(scopeContext(t, f.h, foreign), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("database scope isolation for %s: %d %v", table, count, err)
		}
	}
}

func TestPublicInitialOutageAndDurableQuota(t *testing.T) {
	f := newPublicFixture(t)
	f.admission.Policy.MaxRequestsPerDay = 1
	f.admit(t)
	f.fetcher.set(publicingest.Page{}, &publicingest.RetrievalError{Code: "provider_unavailable"})
	if run := f.execute(t); run["status"] != "FAILED" || run["requests"] != float64(1) {
		t.Fatalf("failed request not counted: %v", run)
	}
	if h := f.health(t); h["state"] != "unavailable" || h["lastSuccessAt"] != nil {
		t.Fatalf("initial outage claimed coverage: %v", h)
	}
	f.ready(t)
	// Re-admission and a new process invocation cannot reset a used budget.
	f.admit(t)
	before := f.fetcher.count()
	if run := f.execute(t); run["status"] != "PARTIAL" || run["failureCode"] != "daily_quota" || run["requests"] != float64(0) || f.fetcher.count() != before {
		t.Fatalf("daily quota bypassed: %v", run)
	}
	if status, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", nil, nil); status != 409 {
		t.Fatalf("quota retry did not defer until next day: %d", status)
	}
}

func TestPublicRightsHistoryAndReferenceOnlyRetention(t *testing.T) {
	f := newPublicFixture(t)
	bad := f.admission
	bad.ConnectorPrincipalID = "source-reviewer"
	if status, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", bad, nil); status != 400 {
		t.Fatalf("human admitted as a connector: %d", status)
	}
	f.admit(t)
	f.execute(t)
	first := f.evidence(t)["items"].([]any)[0].(map[string]any)
	bad = f.admission
	bad.Rights.RetainRaw = false
	if status, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", bad, nil); status != 409 {
		t.Fatalf("rights ID reused for different terms: %d", status)
	}
	f.admission.Rights.ID = "fixture-rights-v2"
	f.admission.Rights.RetainRaw = false
	f.admit(t)
	f.ready(t)
	f.record("v2", `{"event":"reference-only"}`, false, "watermark")
	if run := f.execute(t); run["status"] != "SUCCEEDED" || run["newEvidence"] != float64(1) {
		t.Fatalf("reference-only revision failed: %v", run)
	}
	status, old := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/admissions/"+first["admissionId"].(string), nil, nil)
	if status != 200 || old["rights"].(map[string]any)["retainRaw"] != true {
		t.Fatalf("historical rights changed: %d %v", status, old)
	}
	for _, item := range f.evidence(t)["items"].([]any) {
		e := item.(map[string]any)
		if e["providerRevision"] != "v2" {
			continue
		}
		status, detail := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/evidence/"+e["id"].(string), nil, nil)
		if status != 200 || detail["originalRecord"] != nil || detail["referenceUrl"] == "" || detail["rightsId"] != "fixture-rights-v2" {
			t.Fatalf("reference-only terms ignored: %d %v", status, detail)
		}
		if status, _ = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/sources/"+f.source+"/evidence/"+e["id"].(string)+"/original", nil, nil); status != 404 {
			t.Fatalf("reference-only original retained: %d", status)
		}
	}
	// Returning to an old rights ID must still preserve its original terms.
	f.admission.Rights.ID = "fixture-rights-v1"
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", f.admission, nil); status != 409 {
		t.Fatalf("historical rights ID overwritten: %d", status)
	}
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE source_connector_state SET coverage_to=$2 WHERE source_id=$1`, f.source, time.Now().Add(-3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if h := f.health(t); h["state"] != "unavailable" {
		t.Fatalf("old resumed coverage appeared current: %v", h)
	}
}
