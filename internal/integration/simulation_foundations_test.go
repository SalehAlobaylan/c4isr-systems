package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/scenarios"
)

func TestFrozenRunRestartClockAndReproduction(t *testing.T) {
	h := newHarness(t)
	var first map[string]any
	if status := h.do("POST", "/api/v1/scenarios/normal-patrol/start", map[string]any{"speed": 200}, &first); status != 201 {
		t.Fatalf("start %d %v", status, first)
	}
	id := first["id"].(string)
	var paused, after map[string]any
	if status := h.doRun(id, "POST", "/api/v1/scenarios/runs/"+id+"/pause", nil, &paused); status != 200 {
		t.Fatalf("pause %d %v", status, paused)
	}
	time.Sleep(90 * time.Millisecond)
	h.doRun(id, "GET", "/api/v1/scenarios/runs/"+id, nil, &after)
	if paused["virtualTimeMs"] != after["virtualTimeMs"] {
		t.Fatalf("paused clock advanced: %v %v", paused, after)
	}
	var version map[string]any
	if status := h.doRun(id, "GET", "/api/v1/scenarios/runs/"+id+"/version", nil, &version); status != 200 {
		t.Fatalf("frozen version %d %v", status, version)
	}
	if version["runtimeBuild"] == "" || version["definition"].(map[string]any)["assets"].([]any)[0].(map[string]any)["fidelity"] == nil {
		t.Fatalf("missing frozen build/fidelity %v", version)
	}
	h.doRun(id, "POST", "/api/v1/scenarios/runs/"+id+"/speed", map[string]any{"speed": 1000}, nil)
	h.doRun(id, "POST", "/api/v1/scenarios/runs/"+id+"/resume", nil, nil)
	waitForRunCompletion(t, h, id)
	var second map[string]any
	if status := h.doRun(id, "POST", "/api/v1/scenarios/runs/"+id+"/restart", nil, &second); status != 201 {
		t.Fatalf("restart %d %v", status, second)
	}
	other := second["id"].(string)
	waitForRunCompletion(t, h, other)
	frozen1 := first["frozen"].(map[string]any)
	frozen2 := second["frozen"].(map[string]any)
	if frozen1["versionId"] != frozen2["versionId"] || !reflect.DeepEqual(frozen1["clock"], frozen2["clock"]) || frozen2["restartOf"] != id {
		t.Fatal("restart did not preserve frozen inputs")
	}
	// Compare stored declared outputs, excluding database IDs and receipt/audit wall time.
	outputs := func(run string) []string {
		rows, err := h.pool.Query(context.Background(), `SELECT observed_at::text,COALESCE(ST_AsText(position::geometry),''),observation_type FROM observations WHERE scope_id=$1 ORDER BY observed_at,id`, "default/local/EXERCISE_OBSERVATION/"+run)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var at, pos, kind string
			if err := rows.Scan(&at, &pos, &kind); err != nil {
				t.Fatal(err)
			}
			out = append(out, at+"|"+pos+"|"+kind)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	a, b := outputs(id), outputs(other)
	if len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatalf("re-simulation outputs differ: %v\n%v", a, b)
	}
	// Ordinary operators can inspect the clock but cannot retrieve authored actors.
	r := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseObservation, RunID: id}
	reader := scopeToken(t, h, "s0-operator", r, []string{"operational.read", "scenarios.read", "tracks.read"}, nil)
	if status, _ := scopeRequest(t, h, reader, "GET", "/api/v1/scenarios/runs/"+id+"/version", nil, nil); status != 403 {
		t.Fatalf("operator read hidden version: %d", status)
	}
	if status, body := scopeRequest(t, h, reader, "GET", "/api/v1/scenarios/runs/"+id+"/manifest", nil, nil); status != 200 || body["seed"] != nil || body["definition"] != nil {
		t.Fatalf("manifest disclosure %d %v", status, body)
	}
	// A terminal run's freshness uses its sealed virtual clock, not today's wall clock.
	if _, err := h.pool.Exec(context.Background(), `UPDATE scenario_runs SET frozen_run=frozen_run||'{"seed":999}'::jsonb WHERE id=$1`, id); err == nil {
		t.Fatal("run configuration was mutable")
	}
	input := r
	input.Partition = access.ExerciseInput
	input.RunID = ""
	var count int
	if err := platformdb.Scoped(h.pool).QueryRow(scopeContext(t, h, input), `SELECT count(*) FROM simulation_versions`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("version persistence %d %v", count, err)
	}
	// Exact canonical bytes preserve large integers and exponent spellings in authored payloads.
	versionRaw, _ := json.Marshal(version)
	var numeric scenarios.Version
	if err := json.Unmarshal(versionRaw, &numeric); err != nil {
		t.Fatal(err)
	}
	numeric.Definition.Events[0].Payload = map[string]any{"large": json.Number("9007199254740993"), "small": json.Number("1e-20")}
	repo := scenarios.NewPostgresRepository(h.pool)
	numericID, err := repo.SaveVersion(scopeContext(t, h, r), numeric)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.LoadVersion(scopeContext(t, h, r), numericID)
	if err != nil || loaded.Definition.Events[0].Payload["large"] != json.Number("9007199254740993") || loaded.Definition.Events[0].Payload["small"] != json.Number("1e-20") {
		t.Fatalf("frozen numeric precision changed: %+v %v", loaded, err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE simulation_versions SET document='{}' WHERE id=$1`, numericID); err == nil {
		t.Fatal("frozen version bytes mutable")
	}
	foreign := input
	foreign.WorkspaceID = "other"
	if err := platformdb.Scoped(h.pool).QueryRow(scopeContext(t, h, foreign), `SELECT count(*) FROM simulation_versions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("version isolation %d %v", count, err)
	}
}

func TestApprovedReferenceTerrainPackageOfflineAndIsolation(t *testing.T) {
	f := newPublicFixture(t)
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	if err := store.CreateHuman(admin, "context-reviewer", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "context.export"}}); err != nil {
		t.Fatal(err)
	}
	_, token, err := store.IssueCredential(admin, "context-reviewer", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	var old reference.Input
	for _, name := range []string{"places", "elevation"} {
		raw, err := os.ReadFile("../../testdata/reference/jeddah-makkah/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var in reference.Input
		if err = json.Unmarshal(raw, &in); err != nil {
			t.Fatal(err)
		}
		in.SourceID = f.source
		status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", in, nil)
		if status != 201 {
			t.Fatalf("import %d %v", status, body)
		}
		ids = append(ids, body["id"].(string))
		old = in
	}
	now := time.Now().UTC()
	in := contextpackage.BuildInput{DatasetIDs: ids, AOI: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, TimeWindow: contextpackage.Window{From: now.Add(-24 * time.Hour), To: now}, AcceptedQuality: "Regional context only; coarse EGM2008 terrain remains unresolved for ellipsoidal scene height", Approval: "Approve retained public-domain geography for an exercise"}
	status, created := scopeRequest(t, f.h, token, "POST", "/api/v1/context-packages", in, nil)
	if status != 201 {
		t.Fatalf("build %d %v", status, created)
	}
	id := created["id"].(string)
	status, download := scopeRequest(t, f.h, token, "GET", "/api/v1/context-packages/"+id+"/bundle", nil, nil)
	if status != 200 {
		t.Fatalf("download %d %v", status, download)
	}
	raw, _ := json.Marshal(download)
	var bundle contextpackage.Bundle
	if err = json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	if err = bundle.Validate(time.Now(), true); err != nil {
		t.Fatalf("offline retained real region %v", err)
	}
	if len(bundle.Manifest.Assets) != 2 || len(bundle.Assets) != 2 {
		t.Fatal("missing package inputs")
	}
	// Freeze the approved retained package into a separate exercise input version.
	exercise := f.resource
	exercise.Partition = access.ExerciseObservation
	exercise.RunID = "authoring"
	if err = store.EnsureResource(admin, exercise); err != nil {
		t.Fatal(err)
	}
	wildcard := exercise
	wildcard.RunID = "*"
	if err = store.AddGrant(admin, "context-reviewer", access.Grant{Resource: wildcard, Permissions: []string{"operational.read", "scenarios.read", "scenarios.control", "sim.scenarios.author"}}); err != nil {
		t.Fatal(err)
	}
	_, runToken, err := store.IssueCredential(admin, "context-reviewer", exercise, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.Authenticate(context.Background(), runToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	authorCtx, err := store.BindSession(context.Background(), session, exercise)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	definitionPath := filepath.Join(dir, "packaged.yaml")
	if err = os.WriteFile(definitionPath, []byte("scenario: packaged\nseed: 41\ncontext_packages:\n  - "+id+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runner := scenarios.NewService(scenarios.NewPostgresRepository(f.h.pool), scenarios.NewLoader(dir), scenarios.EngineDeps{ContextPackages: contextpackage.NewService(f.h.pool)}, nil)
	defer runner.Close(admin)
	run, err := runner.Start(authorCtx, "packaged", scenarios.StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	selected := exercise
	selected.RunID = run.ID
	runCtx, err := access.Rebind(authorCtx, selected)
	if err != nil {
		t.Fatal(err)
	}
	version, err := runner.Version(runCtx, run.ID)
	if err != nil || len(version.Context) != 1 || version.Context[0].ID != id || version.Context[0].Validate(time.Now(), true) != nil {
		t.Fatalf("retained input freeze %+v %v", version, err)
	}
	if err = os.Remove(definitionPath); err != nil {
		t.Fatal(err)
	}
	if _, err = runner.Restart(runCtx, run.ID); err != nil {
		t.Fatalf("retained input restart fetched authoring source: %v", err)
	}
	for _, a := range bundle.Manifest.Assets {
		if a.Rights.Attribution == "" || a.Processing == "" {
			t.Fatal("lost source provenance")
		}
	}
	// Publishing a different reference version cannot refresh an existing bundle.
	old.Version += "-next"
	old.Label += " reviewed update"
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", old, nil); status != 201 {
		t.Fatalf("new source version %d", status)
	}
	_, again := scopeRequest(t, f.h, token, "GET", "/api/v1/context-packages/"+id+"/bundle", nil, nil)
	if !reflect.DeepEqual(download, again) {
		t.Fatal("source update mutated frozen package")
	}
	for hash, data := range bundle.Assets {
		data[0] ^= 1
		bundle.Assets[hash] = data
		break
	}
	if bundle.Validate(time.Now(), true) == nil {
		t.Fatal("corrupt retained bytes accepted")
	}
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/context-packages", in, nil); status != 403 {
		t.Fatalf("export permission implied from reviewer role %d", status)
	}
	limited := scopeToken(t, f.h, "context-limited-source", f.resource, []string{"operational.read", "context.export", "sources.read"}, []string{"other-source"})
	if status, _ = scopeRequest(t, f.h, limited, "GET", "/api/v1/context-packages/"+id, nil, nil); status != 404 {
		t.Fatalf("source boundary %d", status)
	}
	foreign := f.resource
	foreign.WorkspaceID = "other"
	foreignToken := scopeToken(t, f.h, "context-foreign", foreign, []string{"operational.read", "context.export"}, nil)
	if status, _ = scopeRequest(t, f.h, foreignToken, "GET", "/api/v1/context-packages/"+id, nil, nil); status != 404 {
		t.Fatalf("workspace boundary %d", status)
	}
	if _, err = f.h.pool.Exec(context.Background(), `UPDATE context_packages SET document='{}' WHERE id=$1`, id); err == nil {
		t.Fatal("package database row mutable")
	}

	in.ReferenceOnly = true
	status, weak := scopeRequest(t, f.h, token, "POST", "/api/v1/context-packages", in, nil)
	if status != 201 || weak["manifest"].(map[string]any)["reproduction"] != "external_dependencies" {
		t.Fatalf("reference-only reproduction %d %v", status, weak)
	}
	// Fixture connector is off throughout: package build/load never contacts feeds.
	if f.fetcher.count() != 0 {
		t.Fatal("package operation fetched a live feed")
	}
}
