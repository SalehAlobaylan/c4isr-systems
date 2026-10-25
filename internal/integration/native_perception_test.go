package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestM5NativePerceptionAndExerciseTasking(t *testing.T) {
	blobs := newS3Fixture(t)
	h := newHarness(t, app.WithArtifactStore(blobs))
	identity := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	r := access.Resource{WorkspaceID: "native-proof", EnvironmentID: "local", Partition: access.ExerciseInput}
	if e := identity.CreateHuman(admin, "native-author", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "sim.runtime.control"}}); e != nil {
		t.Fatal(e)
	}
	_, author, e := identity.IssueCredential(admin, "native-author", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	scene := seedStudioScene(t, h, blobs, r, author, "native-author")
	in := scene.Frozen.Input
	in.Sensors = []simscene.Sensor{{ID: "private-observer", PeriodMs: 1000, DropEvery: 2, DelayMs: 1000, NoiseM: 25, Confidence: .6}}
	in.Controlled = []simscene.Controlled{{ActorID: "waypoint-a", Callsign: "TRAINER-01", TelemetryMs: 1000}}
	frozen := studioCall(t, h, author, "POST", "/api/v1/simulation-studio/scenes", in, 201, nil)
	sceneID := frozen["id"].(string)
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
	build := artifacts.Hash(raw)
	start := simruntime.Start{RequestKey: "perception-proof", SceneID: sceneID, RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build, Seed: 51, Epoch: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Scale: 1, DurationMs: 30000}
	created := studioCall(t, h, author, "POST", "/api/v1/simulation-runtime/runs", start, 201, nil)
	id := created["id"].(string)
	cop := r
	cop.Partition = access.ExerciseObservation
	cop.RunID = id
	if e = identity.CreateHuman(admin, "native-operator", "operator-01", access.Grant{Resource: cop, Permissions: []string{"operational.read", "sources.read", "observations.read", "tracks.read", "assets.read", "telemetry.read", "commands.read", "commands.issue", "missions.read", "missions.create", "missions.update", "audit.read", "scenarios.read", "alerts.read", "geofences.manage", "incidents.read", "incidents.create"}}); e != nil {
		t.Fatal(e)
	}
	_, operator, e := identity.IssueCredential(admin, "native-operator", cop, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	callZone := map[string]any{"id": "native-zone", "name": "Virtual exercise zone", "type": "restricted", "severity": "high", "polygon": []map[string]float64{{"lat": 21, "lng": 39}, {"lat": 22, "lng": 39}, {"lat": 22, "lng": 40}, {"lat": 21, "lng": 40}}}
	studioCall(t, h, operator, "POST", "/api/v1/geofences", callZone, 201, nil)
	nativeList := studioCall(t, h, operator, "GET", "/api/v1/scenarios/runs", nil, 200, nil)
	if len(nativeList["items"].([]any)) != 1 || nativeList["total"].(float64) != 1 {
		t.Fatal("native run is absent from scoped list", nativeList)
	}
	nativePage := studioCall(t, h, operator, "GET", "/api/v1/scenarios/runs?offset=1", nil, 200, nil)
	if len(nativePage["items"].([]any)) != 0 || nativePage["total"].(float64) != 1 {
		t.Fatal("native pagination duplicated run", nativePage)
	}
	// Freeze virtual time until the truth/perception contrast is inspected.
	studioCall(t, h, author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/pause", nil, 200, nil)
	conn, wsctx := dialScope(t, h, operator, "")
	frames := make(chan []byte, 512)
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		defer close(frames)
		for {
			_, b, e := conn.Read(wsctx)
			if e != nil {
				return
			}
			select {
			case frames <- b:
			default:
			}
		}
	}()
	worker := scopeToken(t, h, "native-sensing-worker", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	tokenFile := filepath.Join(t.TempDir(), "credential")
	if e = os.WriteFile(tokenFile, []byte(worker), 0600); e != nil {
		t.Fatal(e)
	}
	work, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd = exec.CommandContext(work, bin, "-url", h.server.URL, "-token-file", tokenFile, "-run", id)
	log := filepath.Join(t.TempDir(), "runtime.log")
	file, e := os.Create(log)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	cmd.Stderr = file
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { cancel(); _ = cmd.Process.Kill() })
	call := func(token, method, path string, in any, want int) map[string]any {
		return studioCall(t, h, token, method, path, in, want, nil)
	}
	wait := func(fn func() bool) {
		t.Helper()
		until := time.Now().Add(12 * time.Second)
		for time.Now().Before(until) {
			if fn() {
				return
			}
			select {
			case err := <-done:
				b, _ := os.ReadFile(log)
				t.Fatalf("native runtime exited %v %s", err, b)
			default:
			}
			time.Sleep(100 * time.Millisecond)
		}
		b, _ := os.ReadFile(log)
		t.Fatalf("native condition timed out %s", b)
	}
	assets := call(operator, "GET", "/api/v1/assets", nil, 200)
	if len(assets["items"].([]any)) != 1 {
		t.Fatal(assets)
	}
	asset := assets["items"].([]any)[0].(map[string]any)["id"].(string)
	wait(func() bool {
		v := call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)
		return v["sequence"].(float64) > 0
	})
	truthResource := cop
	truthResource.Partition = access.ExerciseTruth
	evaluator := scopeTokenForTruth(t, h, truthResource)
	sealed := call(evaluator, "GET", "/api/v1/simulation-runtime/runs/"+id+"/truth", nil, 200)
	if len(sealed["actors"].([]any)) != 3 {
		t.Fatal("evaluator truth unavailable", sealed)
	}
	early := call(operator, "GET", "/api/v1/observations", nil, 200)
	if early["total"].(float64) != 0 {
		t.Fatal("delayed perception appeared as truth", early)
	}
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/resume", nil, 200)
	wait(func() bool { return call(operator, "GET", "/api/v1/observations", nil, 200)["total"].(float64) >= 3 })
	tracks := call(operator, "GET", "/api/v1/tracks", nil, 200)
	if tracks["total"].(float64) != 3 {
		t.Fatal("contacts not independently correlated", tracks)
	}
	for _, path := range []string{"/api/v1/observations", "/api/v1/tracks", "/api/v1/assets", "/api/v1/audit", "/api/v1/assets/" + asset + "/telemetry"} {
		v := call(operator, "GET", path, nil, 200)
		body, _ := json.Marshal(v)
		for _, secret := range []string{"waypoint-a", "marker-a", "marker-b", "private-observer", "runtimeBuild"} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("private value %s leaked through %s", secret, path)
			}
		}
	}
	call(operator, "GET", "/api/v1/simulation-runtime/runs/"+id+"/truth", nil, 403)
	alerts := call(operator, "GET", "/api/v1/alerts", nil, 200)
	if len(alerts["items"].([]any)) == 0 {
		t.Fatal("synthetic evidence did not enter PostGIS alert pipeline", alerts)
	}
	evidence := call(operator, "GET", "/api/v1/observations", nil, 200)["items"].([]any)[0].(map[string]any)["id"]
	incident := call(operator, "POST", "/api/v1/incidents", map[string]any{"title": "Native perception exercise incident", "priority": "high", "alertIds": []any{alerts["items"].([]any)[0].(map[string]any)["id"]}, "assetIds": []string{asset}, "observationIds": []any{evidence}}, 201)
	mission := call(operator, "POST", "/api/v1/missions", map[string]any{"name": "Virtual hold mission", "incidentId": incident["id"], "objective": "Exercise-only hold and resume", "priority": "medium", "assets": []string{asset}, "tasks": []any{map[string]any{"type": "hold", "description": "Hold controlled virtual actor"}}}, 201)
	task := mission["tasks"].([]any)[0].(map[string]any)["id"].(string)
	issue := map[string]any{"assetId": asset, "missionId": mission["id"], "incidentId": incident["id"], "type": "HOLD", "requestKey": "hold-once", "payload": map[string]any{"taskId": task}}
	command := call(operator, "POST", "/api/v1/commands", issue, 201)
	commandID := command["id"].(string)
	again := call(operator, "POST", "/api/v1/commands", issue, 201)
	if again["id"] != commandID {
		t.Fatal("duplicate command execution identity")
	}
	wait(func() bool {
		return call(operator, "GET", "/api/v1/commands/"+commandID, nil, 200)["state"] == "COMPLETED"
	})
	wait(func() bool {
		updated := call(operator, "GET", "/api/v1/missions/"+mission["id"].(string), nil, 200)
		return updated["tasks"].([]any)[0].(map[string]any)["status"] == "COMPLETED"
	})

	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/pause", nil, 200)
	wait(func() bool {
		return call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)["status"] == "PAUSED"
	})
	pausedRun := call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)
	pausedCommand := call(operator, "POST", "/api/v1/commands", map[string]any{"assetId": asset, "type": "HOLD", "requestKey": "queued-during-pause"}, 201)
	wait(func() bool {
		return call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)["sequence"].(float64) >= pausedRun["sequence"].(float64)+2
	})
	if v := call(operator, "GET", "/api/v1/commands/"+pausedCommand["id"].(string), nil, 200); v["state"] != "SENT" {
		t.Fatal("task advanced while paused", v)
	}
	if v := call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200); v["elapsedMs"] != pausedRun["elapsedMs"] {
		t.Fatal("paused clock advanced", v)
	}
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/resume", nil, 200)
	wait(func() bool {
		return call(operator, "GET", "/api/v1/commands/"+pausedCommand["id"].(string), nil, 200)["state"] == "COMPLETED"
	})
	issue["requestKey"] = "resume-once"
	issue["type"] = "RESUME_ROUTE"
	delete(issue, "missionId")
	delete(issue, "payload")
	resume := call(operator, "POST", "/api/v1/commands", issue, 201)
	wait(func() bool {
		return call(operator, "GET", "/api/v1/commands/"+resume["id"].(string), nil, 200)["state"] == "COMPLETED"
	})
	issue["requestKey"] = "unsupported-once"
	issue["type"] = "UNSUPPORTED_EXERCISE_ACTION"
	rejected := call(operator, "POST", "/api/v1/commands", issue, 201)
	wait(func() bool {
		return call(operator, "GET", "/api/v1/commands/"+rejected["id"].(string), nil, 200)["state"] == "REJECTED"
	})
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/stop", nil, 200)
	issue["requestKey"] = "after-stop"
	call(operator, "POST", "/api/v1/commands", issue, 409)
	wait(func() bool {
		return call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)["status"] == "STOPPED"
	})
	select {
	case err := <-done:
		if err != nil {
			b, _ := os.ReadFile(log)
			t.Fatal(err, string(b))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native process did not stop")
	}
	// At the maximum admitted clock scale, ACK/execution must advance on the
	// worker tick loop rather than being delayed until a publication boundary.
	start.RequestKey = "maximum-scale-tasking"
	start.Scale = 20
	start.DurationMs = 300000
	fastID := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	fastCOP := cop
	fastCOP.RunID = fastID
	if e = identity.AddGrant(admin, "native-operator", access.Grant{Resource: fastCOP, Permissions: []string{"operational.read", "assets.read", "commands.read", "commands.issue"}}); e != nil {
		t.Fatal(e)
	}
	_, fastOperator, e := identity.IssueCredential(admin, "native-operator", fastCOP, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	fastAsset := call(fastOperator, "GET", "/api/v1/assets", nil, 200)["items"].([]any)[0].(map[string]any)["id"]
	cmd = exec.CommandContext(work, bin, "-url", h.server.URL, "-token-file", tokenFile, "-run", fastID)
	cmd.Stderr = file
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done = make(chan error, 1)
	go func(process *exec.Cmd, result chan error) { result <- process.Wait() }(cmd, done)
	wait(func() bool {
		return call(author, "GET", "/api/v1/simulation-runtime/runs/"+fastID, nil, 200)["sequence"].(float64) > 0
	})
	fastCommand := call(fastOperator, "POST", "/api/v1/commands", map[string]any{"assetId": fastAsset, "type": "HOLD", "requestKey": "fast-hold"}, 201)
	wait(func() bool {
		return call(fastOperator, "GET", "/api/v1/commands/"+fastCommand["id"].(string), nil, 200)["state"] == "COMPLETED"
	})
	wait(func() bool {
		return call(author, "GET", "/api/v1/simulation-runtime/runs/"+fastID, nil, 200)["elapsedMs"].(float64) >= 60000
	})
	unavailable := call(fastOperator, "POST", "/api/v1/commands", map[string]any{"assetId": fastAsset, "type": "HOLD", "requestKey": "inactive-recipient"}, 201)
	wait(func() bool {
		return call(fastOperator, "GET", "/api/v1/commands/"+unavailable["id"].(string), nil, 200)["state"] == "FAILED"
	})
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+fastID+"/stop", nil, 200)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("maximum-scale runtime did not stop")
	}
	conn.CloseNow()
	<-streamDone
	observedFrame := false
	for raw := range frames {
		if strings.Contains(string(raw), "observation.received") {
			observedFrame = true
		}
		for _, secret := range []string{"waypoint-a", "marker-a", "marker-b", "private-observer", "runtimeBuild"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("truth leaked through serialized frame: %s", secret)
			}
		}
	}
	if !observedFrame {
		t.Fatal("no synthetic evidence reached the ordinary stream")
	}
	runtimeLog, _ := os.ReadFile(log)
	for _, secret := range []string{"waypoint-a", "private-observer", worker} {
		if strings.Contains(string(runtimeLog), secret) {
			t.Fatal("private runtime value reached process log")
		}
	}
	nativeGatewayRecoveryProof(t, h, r, author, worker, sceneID, build, commandID)

}
