package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/recordings"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
)

type archiveFaultStore struct {
	artifacts.Store
	puts       int
	failPut    int
	corruptGet bool
	afterGet   func()
}

func (s *archiveFaultStore) Put(c context.Context, key string, raw []byte, media string) error {
	s.puts++
	if s.puts == s.failPut {
		return errors.New("fixture interrupted archive")
	}
	return s.Store.Put(c, key, raw, media)
}
func (s *archiveFaultStore) Get(c context.Context, key string, size int64) ([]byte, error) {
	raw, e := s.Store.Get(c, key, size)
	if e == nil && s.corruptGet && len(raw) > 0 {
		raw[0] ^= 1
	}
	if e == nil && s.afterGet != nil {
		s.afterGet()
	}
	return raw, e
}

func TestRecordingQueueRetriesAreBoundedAndEnvironmentScoped(t *testing.T) {
	h := newHarness(t)
	identity := access.NewStore(h.pool)
	ctx := context.Background()
	r := access.Resource{WorkspaceID: "queue-proof", EnvironmentID: "local", Partition: access.ExerciseInput}
	token := scopeToken(t, h, "queue-worker", r, []string{"operational.read", "sim.recordings.execute"}, nil)
	session, err := identity.Authenticate(ctx, token, httpx.PermissionsForRole)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := identity.BindSession(ctx, session, r)
	if err != nil {
		t.Fatal(err)
	}
	queue := func(id, environment string) string {
		t.Helper()
		op := access.Resource{WorkspaceID: r.WorkspaceID, EnvironmentID: environment, Partition: access.ExerciseObservation, RunID: id}
		if err := identity.EnsureResource(ctx, op); err != nil {
			t.Fatal(err)
		}
		// Deliberately incomplete run: archive validation must fail visibly.
		if _, err := h.pool.Exec(ctx, `INSERT INTO simulation_recordings(scope_id,status,requested_by)VALUES($1,'QUEUED','fixture')`, op.ID()); err != nil {
			t.Fatal(err)
		}
		return op.ID()
	}
	foreign := queue("foreign", "other")
	a := queue("first", "local")
	worker := &recordings.Service{Pool: h.pool}
	fail := func() {
		t.Helper()
		if did, err := worker.WorkOne(bound); !did || err == nil {
			t.Fatal("invalid job did not fail visibly", did, err)
		}
	}
	idle := func() {
		t.Helper()
		if did, err := worker.WorkOne(bound); did || err != nil {
			t.Fatal("ineligible job was claimed", did, err)
		}
	}
	advance := func(scope string) {
		t.Helper()
		if _, err := h.pool.Exec(ctx, `UPDATE simulation_recordings SET lease_until=now()-interval '1 second' WHERE scope_id=$1`, scope); err != nil {
			t.Fatal(err)
		}
	}
	fail()
	idle()
	b := queue("second", "local")
	fail() // The first job's cooldown must not block a later job.
	advance(a)
	fail()
	var retrySeconds float64
	if err := h.pool.QueryRow(ctx, `SELECT extract(epoch FROM lease_until-clock_timestamp())::double precision FROM simulation_recordings WHERE scope_id=$1`, a).Scan(&retrySeconds); err != nil || retrySeconds < 50 || retrySeconds > 60 {
		t.Fatal("second retry cooldown unavailable", retrySeconds, err)
	}
	advance(a)
	fail()
	advance(a) // Even an elapsed cooldown cannot reopen the attempt budget.
	idle()
	for scope, want := range map[string]int{a: 3, b: 1, foreign: 0} {
		var attempts int
		if err := h.pool.QueryRow(ctx, `SELECT attempts FROM simulation_recordings WHERE scope_id=$1`, scope).Scan(&attempts); err != nil || attempts != want {
			t.Fatal("wrong queue attempt count", scope, attempts, err)
		}
	}
	if err := identity.RevokeCredential(access.Bootstrap(ctx), session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	if did, err := worker.WorkOne(bound); did || !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatal("revoked worker credential remained usable", did, err)
	}
}

func TestM6NativeRecordingReplayEvaluationAndPresets(t *testing.T) {
	blobs := &archiveFaultStore{Store: newS3Fixture(t)}
	h := newHarness(t, app.WithArtifactStore(blobs))
	identity := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	r := access.Resource{WorkspaceID: "recording-proof", EnvironmentID: "local", Partition: access.ExerciseInput}
	if e := identity.CreateHuman(admin, "recording-author", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "sim.runtime.control"}}); e != nil {
		t.Fatal(e)
	}
	_, author, e := identity.IssueCredential(admin, "recording-author", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	scene := seedStudioScene(t, h, blobs, r, author, "recording-author")
	input := simscene.PresetInput{Name: "M6 delayed abstract observer", PeriodMs: 1000, DropEvery: 2, DelayMs: 500, NoiseM: 10, Confidence: .7, Limitations: "Abstract repeatable exercise; no physical sensing"}
	p := studioCall(t, h, author, "POST", "/api/v1/simulation-studio/sensor-presets", input, 201, nil)
	again := studioCall(t, h, author, "POST", "/api/v1/simulation-studio/sensor-presets", input, 201, nil)
	if p["id"] != again["id"] {
		t.Fatal("preset create retry not stable")
	}
	in := scene.Frozen.Input
	in.Sensors = []simscene.Sensor{input.Sensor("HIDDEN-PRESET-OBSERVER")}
	in.Sensors[0].PresetID = p["id"].(string)
	in.Controlled = []simscene.Controlled{{ActorID: "waypoint-a", Callsign: "REPLAY-TRAINER", TelemetryMs: 1000}}
	in.Sensors[0].NoiseM = 20
	studioCall(t, h, author, "POST", "/api/v1/simulation-studio/scenes", in, 400, nil)
	in.Sensors[0].NoiseM = 10
	frozen := studioCall(t, h, author, "POST", "/api/v1/simulation-studio/scenes", in, 201, nil)
	bin := filepath.Join(t.TempDir(), "emad-runtime.exe")
	cmd := exec.Command("go", "build", "-o", bin, "./apps/emad-runtime")
	cmd.Dir = "../.."
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	raw, e := os.ReadFile(bin)
	if e != nil {
		t.Fatal(e)
	}
	start := simruntime.Start{RequestKey: "recording-proof", SceneID: frozen["id"].(string), RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: artifacts.Hash(raw), Seed: 99, Epoch: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Scale: 2, DurationMs: 8000}
	created := studioCall(t, h, author, "POST", "/api/v1/simulation-runtime/runs", start, 201, nil)
	id := created["id"].(string)
	op := r
	op.Partition = access.ExerciseObservation
	op.RunID = id
	permissions := append([]string{"operational.read", "sim.recordings.read", "sim.recordings.manage", "commands.issue", "missions.create", "classifications.create"}, recordings.ViewPermissions...)
	if e = identity.CreateHuman(admin, "recording-operator", "operator-01", access.Grant{Resource: op, Permissions: permissions}); e != nil {
		t.Fatal(e)
	}
	key, operator, e := identity.IssueCredential(admin, "recording-operator", op, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, path string, v any, want int) map[string]any {
		return studioCall(t, h, operator, method, path, v, want, nil)
	}
	base := "/api/v1/simulation-recordings/" + id
	call("POST", base+"/archive", nil, 409)
	worker := scopeToken(t, h, "recording-runtime", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	credentials := filepath.Join(t.TempDir(), "credential")
	if e = os.WriteFile(credentials, []byte(worker), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, bin, "-url", h.server.URL, "-token-file", credentials, "-run", id)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	wait := func(fn func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
			if fn() {
				return
			}
			time.Sleep(75 * time.Millisecond)
		}
		t.Fatal("native recording condition timed out")
	}
	wait(func() bool {
		return studioCall(t, h, author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200, nil)["status"] == "RUNNING"
	})
	assets := call("GET", "/api/v1/assets", nil, 200)["items"].([]any)
	asset := assets[0].(map[string]any)["id"].(string)
	mission := call("POST", "/api/v1/missions", map[string]any{"name": "Recorded exercise mission", "objective": "Hold a virtual actor and review the recorded outcome", "priority": "medium", "assets": []string{asset}, "tasks": []any{map[string]any{"type": "exercise-control", "description": "Recorded hold task"}}}, 201)
	taskID := mission["tasks"].([]any)[0].(map[string]any)["id"].(string)
	command := call("POST", "/api/v1/commands", map[string]any{"assetId": asset, "missionId": mission["id"], "payload": map[string]any{"taskId": taskID}, "type": "HOLD", "requestKey": "recorded-hold"}, 201)
	wait(func() bool {
		return call("GET", "/api/v1/commands/"+command["id"].(string), nil, 200)["state"] == "COMPLETED"
	})
	wait(func() bool {
		return call("GET", "/api/v1/missions/"+mission["id"].(string), nil, 200)["tasks"].([]any)[0].(map[string]any)["status"] == "COMPLETED"
	})
	tracks := call("GET", "/api/v1/tracks", nil, 200)["items"].([]any)
	trackID := tracks[0].(map[string]any)["id"].(string)
	call("POST", "/api/v1/classifications", map[string]any{"trackId": trackID, "label": "aircraft", "confidence": .4, "method": "OPERATOR"}, 201)
	if e = cmd.Wait(); e != nil {
		t.Fatal(e)
	}
	run := studioCall(t, h, author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200, nil)
	if run["status"] != "COMPLETED" {
		t.Fatal(run)
	}
	call("POST", "/api/v1/classifications", map[string]any{"trackId": trackID, "label": "vehicle", "confidence": .6, "method": "OPERATOR"}, 201)
	_ = scopeToken(t, h, "recording-disabled-observer", op, []string{"operational.read", "observations.read"}, nil)
	if _, e = h.pool.Exec(context.Background(), `UPDATE principals SET disabled_at=now() WHERE id='recording-disabled-observer'`); e != nil {
		t.Fatal(e)
	}
	queued := call("POST", base+"/archive", nil, 202)
	if queued["status"] != "QUEUED" {
		t.Fatal(queued)
	}
	archiveToken := scopeToken(t, h, "recording-archive-worker", r, []string{"operational.read", "sim.recordings.execute"}, nil)
	session, e := identity.Authenticate(context.Background(), archiveToken, httpx.PermissionsForRole)
	if e != nil {
		t.Fatal(e)
	}
	bound, e := identity.BindSession(context.Background(), session, r)
	if e != nil {
		t.Fatal(e)
	}
	failed := &recordings.Service{Pool: h.pool, Store: &archiveFaultStore{Store: blobs, failPut: 2}}
	if did, e := failed.WorkOne(bound); !did || e == nil {
		t.Fatal("interrupted archive did not fail closed", did, e)
	}
	if v := call("GET", base, nil, 200); v["status"] != "FAILED" || v["manifest"] != nil || v["attempts"] != float64(1) || v["maxAttempts"] != float64(3) || v["retryAt"] == nil {
		t.Fatal("partial archive became readable", v)
	}
	recorder := &recordings.Service{Pool: h.pool, Store: blobs}
	if did, err := recorder.WorkOne(bound); did || err != nil {
		t.Fatal("failed job retried before its scheduled time", did, err)
	}
	// Advance only the disposable queue's retry time; keep real worker logic.
	if _, e = h.pool.Exec(context.Background(), `UPDATE simulation_recordings SET lease_until=now()-interval '1 second' WHERE scope_id=$1`, op.ID()); e != nil {
		t.Fatal(e)
	}
	archiveStarted := time.Now()
	did, e := recorder.WorkOne(bound)
	archiveDuration := time.Since(archiveStarted)
	if e != nil || !did {
		t.Fatal("archive", did, e)
	}
	index := call("GET", base, nil, 200)
	if index["status"] != "READY" || index["attempts"] != float64(2) || index["retryAt"] != nil {
		t.Fatal(index)
	}
	frameStarted := time.Now()
	final := call("GET", base+"/frame?elapsedMs=8000", nil, 200)
	frameDuration := time.Since(frameStarted)
	early := call("GET", base+"/frame?elapsedMs=0", nil, 200)
	if final["counts"].(map[string]any)["observations"].(float64) == 0 {
		t.Fatal("recording lacks observations")
	}
	if final["counts"].(map[string]any)["classifications"] != float64(2) || final["counts"].(map[string]any)["mission_tasks"] != float64(1) {
		t.Fatal("workflow or classification history missing", final["counts"])
	}
	if v, ok := early["counts"].(map[string]any)["observations"]; ok && v.(float64) > 0 {
		t.Fatal("future observations appeared at virtual zero")
	}
	checkpoint := call("GET", base+"/checkpoint", nil, 200)
	if checkpoint["schema"] != recordings.Schema {
		t.Fatal(checkpoint)
	}
	for _, path := range []string{"/frame?elapsedMs=8000", "/events", "/checkpoint"} {
		name := strings.Split(strings.TrimPrefix(path, "/"), "?")[0]
		t.Run("revocation-during-"+name, func(t *testing.T) {
			token := scopeToken(t, h, "read-revocation-"+name, op, permissions, nil)
			session, err := identity.Authenticate(context.Background(), token, httpx.PermissionsForRole)
			if err != nil {
				t.Fatal(err)
			}
			changed := false
			blobs.afterGet = func() {
				if !changed {
					changed = true
					if err := identity.RevokeCredential(admin, session.Principal.CredentialID); err != nil {
						t.Error(err)
					}
				}
			}
			defer func() { blobs.afterGet = nil }()
			studioCall(t, h, token, "GET", base+path, nil, http.StatusUnauthorized, nil)
			if !changed {
				t.Fatal("fixture did not revoke during artifact retrieval")
			}
		})
	}
	t.Run("retention-expires-during-replay", func(t *testing.T) {
		var retention time.Time
		if err := h.pool.QueryRow(context.Background(), `SELECT retain_until FROM simulation_recordings WHERE scope_id=$1`, op.ID()).Scan(&retention); err != nil {
			t.Fatal(err)
		}
		blobs.afterGet = func() {
			if _, err := h.pool.Exec(context.Background(), `UPDATE simulation_recordings SET retain_until=now()-interval '1 second' WHERE scope_id=$1`, op.ID()); err != nil {
				t.Error(err)
			}
		}
		defer func() {
			blobs.afterGet = nil
			if _, err := h.pool.Exec(context.Background(), `UPDATE simulation_recordings SET retain_until=$1 WHERE scope_id=$2`, retention, op.ID()); err != nil {
				t.Error(err)
			}
		}()
		call("GET", base+"/frame?elapsedMs=8000", nil, http.StatusNotFound)
	})
	operatorSession, e := identity.Authenticate(context.Background(), operator, httpx.PermissionsForRole)
	if e != nil {
		t.Fatal(e)
	}
	operatorContext, e := identity.BindSession(context.Background(), operatorSession, op)
	if e != nil {
		t.Fatal(e)
	}
	broken := &recordings.Service{Pool: h.pool, Store: &archiveFaultStore{Store: blobs, corruptGet: true}}
	if _, e := broken.Frame(operatorContext, id, 8000, 0); !apperr.Is(e, apperr.CodeConflict) {
		t.Fatal("corrupt archive was accepted", e)
	}
	sourceLimited := scopeToken(t, h, "source-restricted-replay", op, permissions, []string{"only-one-source"})
	studioCall(t, h, sourceLimited, "GET", base, nil, 403, nil)
	for _, path := range []string{base, base + "/events", base + "/frame?elapsedMs=8000", base + "/checkpoint"} {
		v := call("GET", path, nil, 200)
		b, _ := json.Marshal(v)
		for _, secret := range []string{"waypoint-a", "marker-a", "HIDDEN-PRESET-OBSERVER", "runtimeBuild", "sensor_preset_", "contact_"} {
			if strings.Contains(string(b), secret) {
				t.Fatalf("ordinary recording leaked %s through %s", secret, path)
			}
		}
	}
	call("GET", base+"/evaluation", nil, 403)
	truth := op
	truth.Partition = access.ExerciseTruth
	evaluatorAdmin, _ := access.Bind(context.Background(), access.Principal{ID: "test:truth-admin", Kind: "service", Grants: []access.Grant{{Resource: access.LegacyResource(), Permissions: []string{"admin.manage", "sim.truth.grant"}}}}, access.LegacyResource(), nil)
	if e = identity.CreateHuman(evaluatorAdmin, "recording-evaluator", "operator-01", access.Grant{Resource: truth, Permissions: []string{"operational.read", "sim.truth.read", "sim.recordings.read", "sim.evaluation.read", "sim.evaluation.release"}}); e != nil {
		t.Fatal(e)
	}
	_, evaluator, e := identity.IssueCredential(admin, "recording-evaluator", truth, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	evaluation := studioCall(t, h, evaluator, "GET", base+"/evaluation", nil, 200, nil)
	if evaluation["version"] != recordings.MetricVersion || len(evaluation["metrics"].([]any)) != 13 {
		t.Fatal(evaluation)
	}
	for _, raw := range evaluation["metrics"].([]any) {
		metric := raw.(map[string]any)
		switch metric["name"] {
		case "late_observations":
			if metric["denominator"].(float64) != final["counts"].(map[string]any)["observations"].(float64) {
				t.Fatal("published perception was not matched to sealed samples", metric, final["counts"])
			}
		case "missed_observations":
			if metric["numerator"] != float64(12) || metric["denominator"] != float64(24) {
				t.Fatal("frozen active sample denominator incorrect", metric)
			}
		case "merged_tracks":
			if metric["denominator"] != float64(3) || metric["numerator"] != float64(0) {
				t.Fatal("recorded associations not matched to actors", metric)
			}
		}
	}
	privileged := studioCall(t, h, evaluator, "GET", base+"/events", nil, 200, nil)
	b, _ := json.Marshal(privileged)
	if !strings.Contains(string(b), "waypoint-a") {
		t.Fatal("privileged frozen identity not retained")
	}
	if !strings.Contains(string(b), `"authorization"`) || !strings.Contains(string(b), "principal.disabled_state") || strings.Contains(string(b), "token_hash") {
		t.Fatal("private permission history missing or credential hash retained")
	}
	release := map[string]any{"metrics": []string{"command_completed", "scenario_completed"}, "reason": "Approved numeric exercise summary"}
	studioCall(t, h, evaluator, "POST", base+"/release", release, 201, nil)
	studioCall(t, h, evaluator, "POST", base+"/release", release, 201, nil)
	release["reason"] = "A conflicting replacement summary"
	studioCall(t, h, evaluator, "POST", base+"/release", release, 409, nil)
	report := call("GET", base+"/report", nil, 200)
	if len(report["metrics"].([]any)) != 2 {
		t.Fatal(report)
	}
	// Mutating current COP state cannot rewrite an archived frame.
	if _, e = h.pool.Exec(context.Background(), `UPDATE assets SET name='Changed after recording' WHERE scope_id=$1 AND id=$2`, op.ID(), asset); e != nil {
		t.Fatal(e)
	}
	after := call("GET", base+"/frame?elapsedMs=8000", nil, 200)
	beforeRaw, _ := json.Marshal(final)
	afterRaw, _ := json.Marshal(after)
	if string(beforeRaw) != string(afterRaw) {
		t.Fatal("mutable COP state contaminated replay")
	}
	var observations, commands int
	if e = h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM observations),(SELECT count(*) FROM commands)`).Scan(&observations, &commands); e != nil {
		t.Fatal(e)
	}
	call("GET", base+"/frame?elapsedMs=8000", nil, 200)
	var o2, c2 int
	_ = h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM observations),(SELECT count(*) FROM commands)`).Scan(&o2, &c2)
	if o2 != observations || c2 != commands {
		t.Fatal("replay mutated evidence or reissued commands")
	}
	var retention time.Time
	if e = h.pool.QueryRow(context.Background(), `SELECT retain_until FROM simulation_recordings WHERE scope_id=$1`, op.ID()).Scan(&retention); e != nil {
		t.Fatal(e)
	}
	if _, e = h.pool.Exec(context.Background(), `UPDATE simulation_recordings SET retain_until=now()-interval '1 second' WHERE scope_id=$1`, op.ID()); e != nil {
		t.Fatal(e)
	}
	call("GET", base+"/frame?elapsedMs=8000", nil, 404)
	if _, e = h.pool.Exec(context.Background(), `UPDATE simulation_recordings SET retain_until=$1 WHERE scope_id=$2`, retention, op.ID()); e != nil {
		t.Fatal(e)
	}
	other := scopeToken(t, h, "recording-other-run", access.Resource{WorkspaceID: r.WorkspaceID, EnvironmentID: r.EnvironmentID, Partition: access.ExerciseObservation, RunID: "other"}, permissions, nil)
	studioCall(t, h, other, "GET", base, nil, 403, nil)
	if e = identity.RevokeCredential(admin, key); e != nil {
		t.Fatal(e)
	}
	studioCall(t, h, operator, "GET", base+"/frame?elapsedMs=8000", nil, http.StatusUnauthorized, nil)
	measurement := map[string]any{"actors": 3, "observers": 1, "controlledAssets": 1, "durationVirtualMs": 8000, "scale": 2, "archiveMs": float64(archiveDuration.Microseconds()) / 1000, "finalFrameMs": float64(frameDuration.Microseconds()) / 1000, "operatorManifest": index["manifest"], "observations": final["counts"].(map[string]any)["observations"], "evaluation": evaluation}
	measurementRaw, _ := json.Marshal(measurement)
	t.Logf("M6_MEASUREMENTS %s", measurementRaw)
	// The finalized immutable archive is idempotent and never reprocessed.
	did, e = recorder.WorkOne(bound)
	if e != nil || did {
		t.Fatal("ready archive reprocessed", did, e)
	}
}
