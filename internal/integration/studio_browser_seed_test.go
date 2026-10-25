package integration

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/recordings"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func buildStudioBrowserRuntime(t *testing.T) (string, []byte) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "emad-runtime.exe")
	command := exec.Command("go", "build", "-o", bin, "./apps/emad-runtime")
	command.Dir = "../.."
	if out, e := command.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	raw, e := os.ReadFile(bin)
	if e != nil {
		t.Fatal(e)
	}
	return bin, raw
}

func seedStudioBrowser(t *testing.T, h *harness, blobs artifacts.Store, bin string, raw []byte) map[string]any {
	t.Helper()
	admin := access.Bootstrap(context.Background())
	identity := access.NewStore(h.pool)
	r := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseInput}
	const principal = "studio-browser-author"
	if e := identity.CreateHuman(admin, principal, "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "sim.runtime.control"}}); e != nil {
		t.Fatal(e)
	}
	if e := identity.AddGrant(admin, principal, access.Grant{Resource: access.LegacyResource(), Permissions: httpx.PermissionsForRole("supervisor")}); e != nil {
		t.Fatal(e)
	}
	_, seed, e := identity.IssueCredential(admin, principal, r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	scene := seedStudioScene(t, h, blobs, r, seed, principal)
	_, token, e := identity.IssueCredential(admin, principal, access.LegacyResource(), time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	wildcard := access.LegacyResource()
	wildcard.RunID = "*"
	if e = identity.AddGrant(admin, principal, access.Grant{Resource: wildcard, Permissions: append(httpx.PermissionsForRole("supervisor"), "sim.recordings.read", "sim.recordings.manage")}); e != nil {
		t.Fatal(e)
	}
	in := scene.Frozen.Input
	in.Name = "Browser synthetic perception scene"
	in.Sensors = []simscene.Sensor{{ID: "private-observer", PeriodMs: 1000, DropEvery: 2, DelayMs: 1000, NoiseM: 25, Confidence: .6}}
	in.Controlled = []simscene.Controlled{{ActorID: "waypoint-a", Callsign: "TRAINER-01", TelemetryMs: 1000}}
	perception := studioCall(t, h, seed, "POST", "/api/v1/simulation-studio/scenes", in, 201, nil)
	worker := scopeToken(t, h, "studio-browser-runtime", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	dir := t.TempDir()
	credentials := filepath.Join(dir, "credential")
	if e = os.WriteFile(credentials, []byte(worker), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() {
		defer close(done)
		cmd := exec.CommandContext(ctx, bin, "-url", h.server.URL, "-token-file", credentials, "-worker")
		if out, e := cmd.CombinedOutput(); e != nil && ctx.Err() == nil {
			t.Errorf("browser native runtime: %v %s", e, string(out))
		}
	}()
	archiveToken := scopeToken(t, h, "browser-recording-worker", r, []string{"operational.read", "sim.recordings.execute"}, nil)
	session, e := identity.Authenticate(context.Background(), archiveToken, httpx.PermissionsForRole)
	if e != nil {
		t.Fatal(e)
	}
	bound, e := identity.BindSession(context.Background(), session, r)
	if e != nil {
		t.Fatal(e)
	}
	archiveDone := make(chan struct{})
	t.Cleanup(func() { cancel(); <-archiveDone })
	go func() {
		defer close(archiveDone)
		service := &recordings.Service{Pool: h.pool, Store: blobs}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
			if _, e := service.WorkOne(bound); e != nil && ctx.Err() == nil {
				t.Errorf("browser archive worker: %v", e)
				return
			}
		}
	}()
	if e := identity.CreateHuman(admin, "studio-input-only", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author"}}); e != nil {
		t.Fatal(e)
	}
	_, inputToken, e := identity.IssueCredential(admin, "studio-input-only", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	recorded := studioCall(t, h, seed, "POST", "/api/v1/simulation-runtime/runs", simruntime.Start{RequestKey: "browser-recorded-run", SceneID: perception["id"].(string), RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: artifacts.Hash(raw), Seed: 51, Epoch: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Scale: 4, DurationMs: 6000}, 201, nil)
	recordingID := recorded["id"].(string)
	waitFor := func(fn func() bool) {
		t.Helper()
		for deadline := time.Now().Add(25 * time.Second); time.Now().Before(deadline); {
			if fn() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("browser recording seed timed out")
	}
	waitFor(func() bool {
		return studioCall(t, h, seed, "GET", "/api/v1/simulation-runtime/runs/"+recordingID, nil, 200, nil)["status"] == "COMPLETED"
	})
	headers := map[string]string{"X-EMAD-Workspace-ID": r.WorkspaceID, "X-EMAD-Environment-ID": r.EnvironmentID, "X-EMAD-Partition": "EXERCISE_OBSERVATION", "X-EMAD-Run-ID": recordingID}
	studioCall(t, h, token, "POST", "/api/v1/simulation-recordings/"+recordingID+"/archive", nil, 202, headers)
	waitFor(func() bool {
		return studioCall(t, h, token, "GET", "/api/v1/simulation-recordings/"+recordingID, nil, 200, headers)["status"] == "READY"
	})
	truthAdmin, _ := access.Bind(context.Background(), access.Principal{ID: "test:browser-truth-admin", Kind: "service", Grants: []access.Grant{{Resource: access.LegacyResource(), Permissions: []string{"admin.manage", "sim.truth.grant"}}}}, access.LegacyResource(), nil)
	if e = identity.CreateHuman(admin, "browser-recording-evaluator", "operator-01", access.Grant{Resource: access.LegacyResource(), Permissions: append(httpx.PermissionsForRole("supervisor"), "sim.recordings.read")}); e != nil {
		t.Fatal(e)
	}
	if e = identity.AddGrant(admin, "browser-recording-evaluator", access.Grant{Resource: wildcard, Permissions: append(httpx.PermissionsForRole("supervisor"), "sim.recordings.read")}); e != nil {
		t.Fatal(e)
	}
	tr := wildcard
	tr.Partition = access.ExerciseTruth
	if e = identity.AddGrant(truthAdmin, "browser-recording-evaluator", access.Grant{Resource: tr, Permissions: []string{"operational.read", "sim.truth.read", "sim.recordings.read", "sim.evaluation.read", "sim.evaluation.release"}}); e != nil {
		t.Fatal(e)
	}
	_, evaluatorToken, e := identity.IssueCredential(admin, "browser-recording-evaluator", access.LegacyResource(), time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	return map[string]any{"inputToken": inputToken, "authorRecordingToken": seed, "token": token, "sceneId": scene.ID, "perceptionSceneId": perception["id"], "runtimeBuild": artifacts.Hash(raw), "recordingId": recordingID, "evaluatorToken": evaluatorToken}
}
