package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestMapFirstPopulatedProfileMigrationPreservesHistoryAndFencesLeases(t *testing.T) {
	f := newPublicFixture(t)
	ctx := context.Background()
	if _, e := f.h.pool.Exec(ctx, `CREATE DATABASE profile_upgrade_fixture`); e != nil {
		t.Fatal(e)
	}
	cfg := f.h.pool.Config().Copy()
	cfg.ConnConfig.Database = "profile_upgrade_fixture"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	files, e := filepath.Glob("../../db/migrations/*.sql")
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(files)
	var cutover []byte
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if filepath.Base(file) >= "00049_retrieval_profiles.sql" {
			if filepath.Base(file) == "00049_retrieval_profiles.sql" {
				cutover = raw
			}
			continue
		}
		if _, e = pool.Exec(ctx, extractUpSection(string(raw))); e != nil {
			t.Fatal("legacy migration", file, e)
		}
	}
	resource := "upgrade/world/PUBLIC_EVIDENCE/"
	if _, e = pool.Exec(ctx, `INSERT INTO workspaces(id)VALUES('upgrade');INSERT INTO environments(workspace_id,id)VALUES('upgrade','world');INSERT INTO data_scopes(id,workspace_id,environment_id,partition,run_id)VALUES('upgrade/world/PUBLIC_EVIDENCE/','upgrade','world','PUBLIC_EVIDENCE','');INSERT INTO principals(id,kind,name)VALUES('legacy-worker','service','Legacy executor');INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions,source_ids)VALUES('legacy-worker','upgrade','world','PUBLIC_EVIDENCE','',ARRAY['operational.read','sources.read','connectors.execute'],ARRAY['legacy-source'])`); e != nil {
		t.Fatal(e)
	}
	var sourceRaw []byte
	if e = f.h.pool.QueryRow(ctx, `SELECT to_jsonb(s) FROM sources s WHERE id=$1`, f.source).Scan(&sourceRaw); e != nil {
		t.Fatal(e)
	}
	var source map[string]any
	_ = json.Unmarshal(sourceRaw, &source)
	source["id"] = "legacy-source"
	source["scope_id"] = resource
	sourceRaw, _ = json.Marshal(source)
	if _, e = pool.Exec(ctx, `INSERT INTO sources SELECT * FROM jsonb_populate_record(NULL::sources,$1)`, sourceRaw); e != nil {
		t.Fatal(e)
	}
	hashes := map[string]string{}
	for version, id := range []string{"legacy-admission-one", "legacy-admission-two"} {
		in := f.admission
		in.ConnectorPrincipalID = "legacy-worker"
		a := publicingest.Admission{AdmissionInput: in, ID: id, SourceID: "legacy-source", Version: version + 1, ApprovedBy: "legacy-human", ApprovedAt: time.Now().UTC()}
		raw, _ := json.Marshal(a)
		if _, e = pool.Exec(ctx, `INSERT INTO source_admissions(id,source_id,scope_id,version,document)VALUES($1,'legacy-source',$2,$3,$4)`, id, resource, a.Version, raw); e != nil {
			t.Fatal(e)
		}
		var retained []byte
		if e = pool.QueryRow(ctx, `SELECT document FROM source_admissions WHERE id=$1`, id).Scan(&retained); e != nil {
			t.Fatal(e)
		}
		hashes[id] = artifacts.Hash(retained)
	}
	stamp := time.Now().UTC().Truncate(time.Microsecond)
	from := stamp.Add(-time.Hour)
	next := stamp.Add(time.Hour)
	if _, e = pool.Exec(ctx, `INSERT INTO connector_runs(id,source_id,scope_id,admission_id,document)VALUES('legacy-running','legacy-source',$1,'legacy-admission-two','{"status":"RUNNING"}');`, resource); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO source_connector_state(scope_id,source_id,admission_id,checkpoint,next_attempt_at,query_from,query_to,quota_day,requests_today,lease_id,lease_until)VALUES($1,'legacy-source','legacy-admission-two','exact-retained-cursor',$2,$3,$4::timestamptz,$4::timestamptz::date,7,'legacy-running',$2)`, resource, next, from, stamp); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, extractUpSection(string(cutover))); e != nil {
		t.Fatal("populated profile cutover", e)
	}
	for id, before := range hashes {
		var raw []byte
		if e = pool.QueryRow(ctx, `SELECT document FROM source_admissions WHERE id=$1`, id).Scan(&raw); e != nil || artifacts.Hash(raw) != before {
			t.Fatal("immutable admission changed", id, e)
		}
	}
	var checkpoint, head, runState string
	var quota, count int
	var queryFrom, queryTo, nextAt time.Time
	var lease *string
	if e = pool.QueryRow(ctx, `SELECT checkpoint,requests_today,query_from,query_to,next_attempt_at,lease_id FROM source_connector_state WHERE source_id='legacy-source'`).Scan(&checkpoint, &quota, &queryFrom, &queryTo, &nextAt, &lease); e != nil {
		t.Fatal(e)
	}
	if checkpoint != "exact-retained-cursor" || quota != 7 || !queryFrom.Equal(from) || !queryTo.Equal(stamp) || !nextAt.Equal(next) || lease != nil {
		t.Fatal("default state backfill changed cursor/window/quota/backoff or retained lease", checkpoint, quota, queryFrom, queryTo, nextAt, lease)
	}
	if e = pool.QueryRow(ctx, `SELECT head_revision_id FROM retrieval_profiles WHERE source_id='legacy-source' AND profile_key='default'`).Scan(&head); e != nil || head != "rpr_legacy-admission-two" {
		t.Fatal("default head mapping", head, e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM retrieval_profile_revisions WHERE profile_key='default'`).Scan(&count); e != nil || count != 2 {
		t.Fatal("historical admission mapping", count, e)
	}
	if e = pool.QueryRow(ctx, `SELECT document->>'status' FROM connector_runs WHERE id='legacy-running'`).Scan(&runState); e != nil || runState != "INTERRUPTED" {
		t.Fatal("legacy writer was not fenced", runState, e)
	}
}
