package integration

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOwnershipMigrationPreservesAndQuarantinesLegacyGraph(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `CREATE DATABASE ownership_fixture`); err != nil {
		t.Fatal(err)
	}
	cfg := h.pool.Config().Copy()
	cfg.ConnConfig.Database = "ownership_fixture"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	files, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	apply := func(path string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, extractUpSection(string(raw))); err != nil {
			t.Fatalf("%s: %v", filepath.Base(path), err)
		}
	}
	for _, path := range files {
		if filepath.Base(path) < "00019" {
			apply(path)
		}
	}
	_, err = pool.Exec(ctx, `
        INSERT INTO scenario_runs(id,scenario_name,seed,status) VALUES('old-run','old fixture',1,'COMPLETED'),('other-run','other fixture',2,'COMPLETED');
        INSERT INTO sources(id,name,type,metadata) VALUES
          ('old-run__source__a','a','sensor','{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}'),
          ('other-run__source__b','b','sensor','{"scenarioRunId":"other-run","resourceNamespace":"other-run__"}'),
          ('ambiguous','unverified','synthetic','{}');
        INSERT INTO audit_events(id,occurred_at,actor_type,action,subject_type,subject_id,data)
        SELECT 'audit-'||id,now(),'SCENARIO','source.created','source',id,metadata FROM sources WHERE id<>'ambiguous';
        INSERT INTO observations(id,source_id,observation_type,observed_at,payload) VALUES
          ('old-run__observation__known','old-run__source__a','track',now(),'{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}'),
          ('old-run__observation__mixed','old-run__source__a','track',now(),'{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}'),
          ('other-run__observation__mixed','other-run__source__b','track',now(),'{"scenarioRunId":"other-run","resourceNamespace":"other-run__"}');
        INSERT INTO audit_events(id,occurred_at,actor_type,action,subject_type,subject_id,data)
        SELECT 'audit-'||id,now(),'SCENARIO','observation.received','observation',id,payload FROM observations;
        INSERT INTO tracks(id,external_ref,first_seen_at,last_seen_at,metadata) VALUES
          ('trk_retained','old-run__track__known',now(),now(),'{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}'),
          ('trk_mixed','old-run__track__mixed',now(),now(),'{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}');
        INSERT INTO track_observations(track_id,observation_id,source_id,observed_at)
          SELECT CASE WHEN id='old-run__observation__known' THEN 'trk_retained' ELSE 'trk_mixed' END,id,source_id,observed_at FROM observations;
        INSERT INTO classifications(id,track_id,label,method) VALUES('cls_retained','trk_retained','vehicle','SCENARIO'),('cls_mixed','trk_mixed','unknown','SCENARIO');
        INSERT INTO assessments(id,subject_type,subject_id,type,conclusion,method) VALUES
          ('asm_retained','track','old-run__track__known','fixture','kept','RULE'),
          ('asm_mixed','track','old-run__track__known','fixture','mixed evidence','RULE');
        INSERT INTO assessment_evidence(assessment_id,evidence_type,evidence_id) VALUES
          ('asm_retained','observation','old-run__observation__known'),('asm_mixed','observation','other-run__observation__mixed');
        INSERT INTO audit_events(id,occurred_at,actor_type,action,subject_type,subject_id,data)
          SELECT 'audit-'||id,now(),'SCENARIO','assessment.created','assessment',id,'{"scenarioRunId":"old-run","resourceNamespace":"old-run__"}' FROM assessments;
    `)
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]int{}
	for _, table := range []string{"sources", "observations", "tracks", "track_observations", "classifications", "assessments", "assessment_evidence", "audit_events"} {
		var n int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		before[table] = n
	}
	for _, path := range files {
		if filepath.Base(path) >= "00019" {
			apply(path)
		}
	}
	for _, table := range []string{"sources", "observations", "tracks", "track_observations", "classifications", "assessments", "assessment_evidence", "audit_events"} {
		var n int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != before[table] {
			t.Fatalf("%s rows changed: %d -> %d (%v)", table, before[table], n, err)
		}
	}
	for _, id := range []string{"trk_retained", "trk_mixed"} {
		var scope *string
		if err = pool.QueryRow(ctx, `SELECT scope_id FROM tracks WHERE id=$1`, id).Scan(&scope); err != nil {
			t.Fatal(err)
		}
		if (scope != nil) != (id == "trk_retained") {
			t.Fatalf("track ownership misclassified: %s %v", id, scope)
		}
	}
	for _, id := range []string{"asm_retained", "asm_mixed"} {
		var scope *string
		if err = pool.QueryRow(ctx, `SELECT scope_id FROM assessments WHERE id=$1`, id).Scan(&scope); err != nil {
			t.Fatal(err)
		}
		if (scope != nil) != (id == "asm_retained") {
			t.Fatalf("assessment ownership misclassified: %s %v", id, scope)
		}
	}
	for repeat := 0; repeat < 2; repeat++ {
		if _, err = pool.Exec(ctx, `SELECT emad_backfill_ownership()`); err != nil {
			t.Fatalf("backfill retry: %v", err)
		}
	}
	r := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseObservation, RunID: "old-run"}
	p := access.Principal{ID: "migration-reader", Kind: "human", Grants: []access.Grant{{Resource: r, Permissions: []string{"sources.read"}}}}
	scoped, err := access.Bind(ctx, p, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var visible, ambiguous int
	if err = platformdb.Scoped(pool).QueryRow(scoped, `SELECT count(*),count(*) FILTER(WHERE id='ambiguous') FROM sources`).Scan(&visible, &ambiguous); err != nil || visible != 1 || ambiguous != 0 {
		t.Fatalf("quarantine was visible: %d %d (%v)", visible, ambiguous, err)
	}
	var quarantined int
	if err = pool.QueryRow(ctx, `SELECT sum(quarantined) FROM emad_ownership_report`).Scan(&quarantined); err != nil || quarantined < 4 {
		t.Fatalf("missing quarantine report: %d (%v)", quarantined, err)
	}
	if _, err = platformdb.Scoped(pool).Exec(scoped, `SELECT emad_backfill_ownership()`); err == nil {
		t.Fatal("query role can run privileged backfill")
	}
}
