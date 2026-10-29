package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"sync"
	"testing"
	"time"
)

type gatedProfileStore struct {
	artifacts.Store
	mu               sync.Mutex
	entered, release chan struct{}
}

func (s *gatedProfileStore) Put(ctx context.Context, key string, raw []byte, media string) error {
	s.mu.Lock()
	entered, release := s.entered, s.release
	s.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.Put(ctx, key, raw, media)
}
func TestMapFirstRetrievalProfilesOriginalRightsExecutionAndSuspension(t *testing.T) {
	storage := &gatedProfileStore{Store: newS3Fixture(t)}
	f := newPublicFixture(t, app.WithArtifactStore(storage))
	f.admit(t)
	f.execute(t)
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		code, v := scopeRequest(t, f.h, token, method, path, in, nil)
		if code != want {
			t.Fatalf("profile %s %s: %d want %d %v", method, path, code, want, v)
		}
		return v
	}
	profilesPath := "/api/v1/sources/" + f.source + "/profiles"
	makeProfile := func(key string, export bool) publicingest.AdmissionInput {
		v := f.admission
		v.AnalystQuestion = "Evaluate retained observations within this reviewed named area"
		v.Rights.ID = "profile-" + key + "-terms"
		v.Rights.AllowSimulationExport = export
		return v
	}
	aInput := makeProfile("a", false)
	a := call(f.manager, "POST", profilesPath+"/a", publicingest.ProfileInput{Admission: aInput}, 201)
	aID := a["id"].(string)
	bInput := makeProfile("b", true)
	b := call(f.manager, "POST", profilesPath+"/b", publicingest.ProfileInput{Admission: bInput}, 201)
	bID := b["id"].(string)
	if a["version"] != float64(2) || b["version"] != float64(3) {
		t.Fatal("admission versions are not source-monotonic", a["version"], b["version"])
	}
	calls := f.fetcher.count()
	call(f.worker, "POST", profilesPath+"/a/connector-runs", nil, 403)
	if f.fetcher.count() != calls {
		t.Fatal("ungranted profile performed provider I/O")
	}
	call(f.manager, "POST", profilesPath+"/a/execution", publicingest.ExecutionGrant{ProfileRevisionID: "rpr_" + aID, PrincipalID: "connector-fixture", Enabled: true}, 200)
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE retrieval_profile_state SET next_attempt_at='epoch' WHERE source_id=$1`, f.source); e != nil {
		t.Fatal(e)
	}
	f.ready(t)
	run := call(f.worker, "POST", profilesPath+"/a/connector-runs", nil, 201)
	if run["duplicates"] != float64(1) || run["profileKey"] != "a" {
		t.Fatal("profile overlap did not preserve source evidence identity", run)
	}
	var evidence, occurrences int
	if e := f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public_evidence),(SELECT count(*) FROM retrieval_occurrences)`).Scan(&evidence, &occurrences); e != nil || evidence != 1 || occurrences != 2 {
		t.Fatal("retrieval occurrence/deduplication", evidence, occurrences, e)
	}
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE retrieval_profile_state SET next_attempt_at='epoch',quota_day=CURRENT_DATE,requests_today=CASE WHEN profile_key='default' THEN 19 ELSE requests_today END WHERE source_id=$1`, f.source); e != nil {
		t.Fatal(e)
	}
	calls = f.fetcher.count()
	limited := call(f.worker, "POST", profilesPath+"/a/connector-runs", nil, 201)
	if limited["failureCode"] != "daily_quota" || f.fetcher.count() != calls {
		t.Fatal("profiles multiplied the trusted source allowance", limited)
	}
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE retrieval_profile_state SET next_attempt_at='epoch',requests_today=CASE WHEN profile_key='default' THEN 1 ELSE requests_today END WHERE source_id=$1`, f.source); e != nil {
		t.Fatal(e)
	}
	store := access.NewStore(f.h.pool)
	session, e := store.Authenticate(context.Background(), f.manager, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx, e := store.BindSession(context.Background(), session, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	aPolicy, e := publicingest.ResolvePolicy(ctx, f.h.pool, f.source, aID)
	if e != nil || aPolicy.Rights.AllowSimulationExport || aPolicy.ProfileKey != "a" {
		t.Fatal("profile A borrowed B rights", aPolicy, e)
	}
	bPolicy, e := publicingest.ResolvePolicy(ctx, f.h.pool, f.source, bID)
	if e != nil || !bPolicy.Rights.AllowSimulationExport || bPolicy.ProfileKey != "b" {
		t.Fatal("profile B original policy", e)
	}
	if current := call(f.manager, "GET", "/api/v1/sources/"+f.source+"/admission", nil, 200); current["version"] != float64(1) {
		t.Fatal("legacy source API changed default profile", current)
	}
	blobs := artifacts.NewService(f.h.pool, storage)
	raw := []byte(`{"synthetic":"profile-owned artifact"}`)
	artifact, e := blobs.Reserve(ctx, artifacts.Input{RequestedAdmissionID: aID, ProfileKey: "a", RequestKey: "profile-a-artifact", SourceID: f.source, SHA256: artifacts.Hash(raw), Bytes: int64(len(raw)), MediaType: "application/json", Kind: "imagery_subset", RetainUntil: time.Now().Add(time.Hour), Provenance: "Isolated profile policy test"})
	if e != nil || artifact.AdmissionID != aID || artifact.Rights.AllowSimulationExport {
		t.Fatal("artifact original profile", artifact, e)
	}
	if e = db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT set_config('emad.profile_policy','',true)`); e != nil {
			return e
		}
		var visible int
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM artifacts WHERE id=$1`, artifact.ID).Scan(&visible); e != nil {
			return e
		}
		if visible != 0 {
			t.Fatal("policy-unaware reader saw a named-profile raster")
		}
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM public_evidence`).Scan(&visible); e != nil {
			return e
		}
		if visible != 1 {
			t.Fatal("legacy default reads were not preserved")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if _, e = blobs.Reserve(ctx, artifacts.Input{RequestedAdmissionID: aID, ProfileKey: "b", RequestKey: "mixed-profile", SourceID: f.source, SHA256: artifacts.Hash(raw), Bytes: int64(len(raw)), MediaType: "application/json", Kind: "imagery_subset", RetainUntil: time.Now().Add(time.Hour), Provenance: "Isolated conflicting profile"}); e == nil {
		t.Fatal("conflicting original admission/profile accepted")
	}
	rasterToken := scopeToken(t, f.h, "profile-raster-worker", f.resource, []string{"operational.read", "sources.read", "imagery.jobs.execute"}, []string{f.source})
	rasterSession, e := store.Authenticate(context.Background(), rasterToken, nil)
	if e != nil {
		t.Fatal(e)
	}
	rasterCtx, e := store.BindSession(context.Background(), rasterSession, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = blobs.Policy(rasterCtx, f.source, aID); e == nil {
		t.Fatal("source-level worker grant inherited named profile execution")
	}
	// Change the approved profile while bytes are being written. The after-I/O
	// check and locked completion must reject READY under the disabled profile.
	storage.entered = make(chan struct{})
	storage.release = make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(storage.release) })
	done := make(chan error, 1)
	go func() { _, e := blobs.Upload(ctx, artifact.ID, raw); done <- e }()
	select {
	case <-storage.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("upload did not reach private S3 boundary")
	}
	aInput.Enabled = false
	aInput.Rights.ID = "profile-a-disabled-terms"
	call(f.manager, "POST", profilesPath+"/a", publicingest.ProfileInput{ExpectedRevisionID: "rpr_" + aID, Admission: aInput}, 201)
	releaseOnce.Do(func() { close(storage.release) })
	if e = <-done; e == nil {
		t.Fatal("disabled profile completed artifact upload")
	}
	actual, e := blobs.Get(ctx, artifact.ID)
	if e != nil || actual.State != "PENDING" {
		t.Fatal("rejected completion changed artifact state", actual.State, e)
	}
	call(f.manager, "GET", "/api/v1/sources/"+f.source+"/admissions/"+aID, nil, 200)
	bPolicy, e = publicingest.ResolvePolicy(ctx, f.h.pool, f.source, bID)
	if e != nil || !bPolicy.Rights.AllowSimulationExport {
		t.Fatal("disabled A contaminated unrelated B", e)
	}
	call(f.manager, "POST", "/api/v1/sources/"+f.source+"/status", map[string]string{"status": "offline"}, 200)
	if _, e = publicingest.ResolvePolicy(ctx, f.h.pool, f.source, bID); e == nil {
		t.Fatal("suspended source allowed new profile processing")
	}
	call(f.manager, "GET", "/api/v1/sources/"+f.source+"/admissions/"+bID, nil, 200)
	var original []byte
	if e = f.h.pool.QueryRow(context.Background(), `SELECT document FROM source_admissions WHERE id=$1`, aID).Scan(&original); e != nil {
		t.Fatal(e)
	}
	var retained publicingest.Admission
	_ = json.Unmarshal(original, &retained)
	if !retained.Enabled || retained.Rights.ID != "profile-a-terms" {
		t.Fatal("original admission bytes changed")
	}
}
