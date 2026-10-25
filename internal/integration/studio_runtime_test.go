package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

func studioCall(t *testing.T, h *harness, token, method, path string, in any, want int, headers map[string]string) map[string]any {
	t.Helper()
	c, v := scopeRequest(t, h, token, method, path, in, headers)
	if c != want {
		t.Fatalf("studio %s %s: %d %v", method, path, c, v)
	}
	return v
}

// The real reference point is an explicit subset of the accepted Natural Earth
// fixture. Terrain is project-authored synthetic ellipsoid data, not ETOPO or a
// silently approximated geoid conversion. Both pass the normal approval flow.
func seedStudioScene(t *testing.T, h *harness, blobs artifacts.Store, r access.Resource, author, principal string) simscene.Scene {
	t.Helper()
	admin := access.Bootstrap(context.Background())
	identity := access.NewStore(h.pool)
	pub := r
	pub.Partition = access.PublicEvidence
	if e := identity.AddGrant(admin, principal, access.Grant{Resource: pub, Permissions: []string{"operational.read", "sources.read", "sources.manage", "context.export"}}); e != nil {
		t.Fatal(e)
	}
	headers := map[string]string{"X-EMAD-Workspace-ID": pub.WorkspaceID, "X-EMAD-Environment-ID": pub.EnvironmentID, "X-EMAD-Partition": string(pub.Partition)}
	call := func(method, path string, in any, want int) map[string]any {
		return studioCall(t, h, author, method, path, in, want, headers)
	}
	source := call("POST", "/api/v1/sources", map[string]any{"name": "Studio accepted reference fixture", "type": "external"}, 201)["id"].(string)
	raw, e := os.ReadFile("../../testdata/reference/jeddah-makkah/places.json")
	if e != nil {
		t.Fatal(e)
	}
	var places reference.Input
	if e = json.Unmarshal(raw, &places); e != nil {
		t.Fatal(e)
	}
	var collection reference.Collection
	if e = json.Unmarshal(places.Artifact, &collection); e != nil {
		t.Fatal(e)
	}
	collection.Features = collection.Features[:1]
	places.ParentArtifactSHA256 = places.SHA256
	places.Artifact, _ = json.Marshal(collection)
	places.SHA256 = artifacts.Hash(places.Artifact)
	places.SourceID = source
	places.Version += "-jeddah-scene-subset"
	places.Transformation += "; retain Jeddah point only, exact coordinates unchanged"
	places.AOI = publicingest.Bounds{West: 39.18, East: 39.22, South: 21.48, North: 21.52}
	pointID := call("POST", "/api/v1/reference-datasets", places, 201)["id"].(string)
	grid := reference.Grid{Width: 3, Height: 3, West: 39.19, South: 21.49, StepX: .01, StepY: .01, Registration: "point_samples"}
	for _, n := range []float64{10, 10, 10, 10, 30, 10, 10, 10, 10} {
		v := n
		grid.Values = append(grid.Values, &v)
	}
	terrain := places
	terrain.DatasetKey = "emad-synthetic-scene-terrain"
	terrain.Version = "v1"
	terrain.Label = "Synthetic ellipsoidal terrain"
	terrain.Kind = "elevation"
	terrain.OriginalProducer = "EMAD synthetic fixture"
	terrain.VerticalDatum = "WGS84_ELLIPSOID"
	terrain.Resolution = reference.Resolution{Value: 36, Unit: "arc_seconds"}
	terrain.Quality = "Synthetic 3 by 3 height grid; no measured terrain or physical validation"
	terrain.Transformation = "Project-authored ellipsoidal heights; no source terrain or geoid conversion"
	terrain.ParentArtifactSHA256 = ""
	terrain.Artifact, _ = json.Marshal(grid)
	terrain.SHA256 = artifacts.Hash(terrain.Artifact)
	terrain.Rights = publicingest.Rights{ID: "emad-synthetic-terrain-v1", TermsURL: "https://example.invalid/emad-fixtures", LegalBasis: "Project-authored synthetic test terrain", Attribution: "EMAD synthetic terrain fixture", RetainRaw: true, AllowSimulationExport: true}
	terrainID := call("POST", "/api/v1/reference-datasets", terrain, 201)["id"].(string)
	now := time.Now().UTC()
	bundle := call("POST", "/api/v1/context-packages", contextpackage.BuildInput{DatasetIDs: []string{pointID, terrainID}, AOI: places.AOI, TimeWindow: contextpackage.Window{From: now.Add(-time.Hour), To: now}, AcceptedQuality: "Natural Earth reference point and explicitly synthetic ellipsoidal relief; no physics", Approval: "Accept this bounded retained package for visual scene validation"}, 201)
	worker := scopeToken(t, h, "studio-import-"+r.WorkspaceID, r, []string{"operational.read", "sim.assets.import"}, nil)
	validator, e := modelassets.NewNodeValidator("../../apps/emad-asset-validator/validate.mjs")
	if e != nil {
		t.Fatal(e)
	}
	assets := modelassets.New(h.pool, blobs)
	models := map[string]string{}
	for _, name := range []string{"ground-tile.gltf", "abstract-wing.glb", "marker.glb"} {
		bytes, e := os.ReadFile("../../testdata/models/" + name)
		if e != nil {
			t.Fatal(e)
		}
		media := "model/gltf-binary"
		if name == "ground-tile.gltf" {
			media = "model/gltf+json"
		}
		upload := studioCall(t, h, author, "POST", "/api/v1/simulation-assets/uploads", assetInput(name, media, bytes), 201, nil)["id"].(string)
		assetBytes(t, h, author, upload, bytes, 200)
		if did, e := assets.WorkOne(workerContext(t, h, worker), validator, artifacts.Hash([]byte("studio-import-fixture-v1"))); e != nil || !did {
			t.Fatal(did, e)
		}
	}
	authorCtx := workerContext(t, h, author)
	list, e := assets.List(authorCtx, "models")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range list {
		var m modelassets.ModelAsset
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		models[m.Upload.Name] = m.ID
	}
	height := 10.0
	frame := simulation.DefaultFrame(simulation.Position{Longitude: 39.2, Latitude: 21.5, Height: &height, VerticalDatum: "WGS84_ELLIPSOID"})
	env, e := assets.Environment(authorCtx, modelassets.EnvironmentAsset{Name: "Synthetic scene ground", ModelVersionID: models["ground-tile.gltf"], Frame: frame, ModelAxesToENU: "x_east_y_up_z_south", Provenance: "Project-authored visual mesh at resolved S0 origin", Fidelity: "visual_only"})
	if e != nil {
		t.Fatal(e)
	}
	static, e := assets.Template(authorCtx, modelassets.ActorTemplate{Name: "Abstract static marker", ModelVersionID: models["marker.glb"], Behavior: "STATIC", Fidelity: "abstract_behavior", Limitations: "Synthetic marker; no physical dynamics or sensing"})
	if e != nil {
		t.Fatal(e)
	}
	moving, e := assets.Template(authorCtx, modelassets.ActorTemplate{Name: "Abstract waypoint actor", ModelVersionID: models["abstract-wing.glb"], Behavior: "WAYPOINT_KINEMATIC", MaxSpeedMPS: 20, Fidelity: "abstract_behavior", Limitations: "Constant ENU speed, no forces, collision or sensor model"})
	if e != nil {
		t.Fatal(e)
	}
	position := func(x, y, z float64) simulation.Position {
		p, e := frame.FromLocal(simulation.Vector{X: x, Y: y, Z: z})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	in := simscene.Input{Name: "Jeddah reference and synthetic terrain proof", ContextPackageID: bundle["id"].(string), EnvironmentID: env.ID, Actors: []simscene.Placement{{ID: "marker-a", TemplateID: static.ID, Position: position(0, 0, 0), Heading: 0, Waypoints: []simulation.Position{}, DespawnMs: 60000}, {ID: "waypoint-a", TemplateID: moving.ID, Position: position(100, 0, 20), SpeedMPS: 10, Heading: 90, Waypoints: []simulation.Position{position(1000, 0, 20)}, DespawnMs: 60000}, {ID: "marker-b", TemplateID: static.ID, Position: position(-100, 100, 0), Waypoints: []simulation.Position{}, DespawnMs: 60000}}}
	result := studioCall(t, h, author, "POST", "/api/v1/simulation-studio/scenes", in, 201, nil)
	// Preserve embedded raw material numbers; a generic map round-trip can
	// rewrite 1.0 to 1 and must never be used for a content-addressed artifact.
	req, e := http.NewRequest("GET", h.server.URL+"/api/v1/simulation-studio/scenes/"+result["id"].(string), nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+author)
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	var scene simscene.Scene
	if e = json.NewDecoder(response.Body).Decode(&scene); e != nil {
		t.Fatal(e)
	}
	return scene
}
func TestS13S21FrozenSceneNativeRuntimeAndTruthBoundary(t *testing.T) {
	blobs := newS3Fixture(t)
	h := newHarness(t, app.WithArtifactStore(blobs))
	admin := access.Bootstrap(context.Background())
	identity := access.NewStore(h.pool)
	r := access.Resource{WorkspaceID: "studio-proof", EnvironmentID: "local", Partition: access.ExerciseInput}
	if e := identity.CreateHuman(admin, "studio-author", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "sim.runtime.control"}}); e != nil {
		t.Fatal(e)
	}
	_, author, e := identity.IssueCredential(admin, "studio-author", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	scene := seedStudioScene(t, h, blobs, r, author, "studio-author")
	call := func(token, method, path string, in any, want int) map[string]any {
		return studioCall(t, h, token, method, path, in, want, nil)
	}
	again := call(author, "POST", "/api/v1/simulation-studio/scenes", scene.Frozen.Input, 201)
	if again["id"] != scene.ID {
		t.Fatal("scene not reproducible")
	}
	if len(scene.Frozen.Geometry.Terrain) != 9 || len(scene.Frozen.Geometry.Points) != 1 || !scene.Frozen.Geometry.Points[0].HeightAssumed {
		t.Fatal("terrain/reference provenance lost")
	}
	call(integrationToken, "GET", "/api/v1/simulation-studio/scenes/"+scene.ID, nil, 403)
	call(integrationToken, "GET", "/api/v1/simulation-runtime/runs", nil, 403)
	// Unresolved ETOPO heights must not be silently reinterpreted as ellipsoidal.
	bad := scene.Frozen
	bad.Context.Manifest.Assets = append([]contextpackage.Asset(nil), bad.Context.Manifest.Assets...)
	for i := range bad.Context.Manifest.Assets {
		if bad.Context.Manifest.Assets[i].Kind == "elevation" {
			bad.Context.Manifest.Assets[i].VerticalDatum = "EGM2008"
		}
	}
	if _, e := simscene.GeometryFor(bad.Context, bad.Environment.Frame); e == nil {
		t.Fatal("unresolved height datum accepted")
	}
	authorCtx := workerContext(t, h, author)
	if _, e = platformdb.Scoped(h.pool).Exec(authorCtx, `UPDATE simulation_scenes SET name='tampered' WHERE id=$1`, scene.ID); e == nil {
		t.Fatal("scene mutable")
	}
	bin := filepath.Join(t.TempDir(), "emad-runtime.exe")
	cmd := exec.Command("go", "build", "-o", bin, "./apps/emad-runtime")
	cmd.Dir = "../.."
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(output), e)
	}
	binary, e := os.ReadFile(bin)
	if e != nil {
		t.Fatal(e)
	}
	build := artifacts.Hash(binary)
	scenePath := filepath.Join(t.TempDir(), "scene.json")
	sceneRaw, _ := json.Marshal(scene)
	if e = os.WriteFile(scenePath, sceneRaw, 0600); e != nil {
		t.Fatal(e)
	}
	// Offline and attached execution share the exact frozen scene contract.
	offline := filepath.Join(t.TempDir(), "truth.json")
	cmd = exec.Command(bin, "-offline-scene", scenePath, "-elapsed-ms", "5000", "-out", offline)
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(output), e)
	}
	offlineRaw, e := os.ReadFile(offline)
	if e != nil {
		t.Fatal(e)
	}
	var snap simruntime.Snapshot
	if e = json.Unmarshal(offlineRaw, &snap); e != nil {
		t.Fatal(e)
	}
	if math.Abs(snap.Actors[1].Local.X-150) > 1e-5 {
		t.Fatal("offline kinematics", snap)
	}
	runtimeToken := scopeToken(t, h, "native-runtime", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	start := simruntime.Start{RequestKey: "native-proof", SceneID: scene.ID, RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build, Seed: 51, Epoch: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Scale: 1, DurationMs: 60000}
	probe := start
	probe.RequestKey = "protocol-probe"
	probeID := call(author, "POST", "/api/v1/simulation-runtime/runs", probe, 201)["id"].(string)
	probeLease := call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 200)
	world, e := simruntime.NewWorld(scene.Frozen)
	if e != nil {
		t.Fatal(e)
	}
	states, e := world.At(0)
	if e != nil {
		t.Fatal(e)
	}
	broker := &simruntime.Broker{Pool: h.pool, Scenes: &simscene.Service{Pool: h.pool, Assets: modelassets.New(h.pool, blobs), Packages: contextpackage.NewService(h.pool)}}
	beat := simruntime.Beat{Token: probeLease["token"].(string), Status: "RUNNING", Snapshot: simruntime.Snapshot{RunID: probeID, SceneID: scene.ID, RuntimeBuild: build, Sequence: 1, ElapsedMs: 0, Actors: states}}
	if _, e = broker.Heartbeat(workerContext(t, h, runtimeToken), probeID, beat); e != nil {
		t.Fatalf("direct heartbeat: %v", e)
	}
	// A retried accepted message is idempotent. A reused or skipped sequence,
	// another credential owner and a human author cannot publish private state.
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/heartbeat", beat, 200)
	altered := beat
	altered.Snapshot.ElapsedMs = 1
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/heartbeat", altered, 409)
	altered = beat
	altered.Snapshot.Sequence = 3
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/heartbeat", altered, 409)
	otherWorker := scopeToken(t, h, "other-native-runtime", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	call(otherWorker, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/heartbeat", beat, 403)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/heartbeat", beat, 403)
	if _, e = h.pool.Exec(context.Background(), `INSERT INTO simulation_runtime_events(scope_id,run_id,kind)SELECT $1,$2,'PAUSE' FROM generate_series(1,126)`, r.ID(), probeID); e != nil {
		t.Fatal(e)
	}
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/pause", nil, 409)
	otherScope := r
	otherScope.EnvironmentID = "unrelated"
	outsider := scopeToken(t, h, "unrelated-runtime", otherScope, []string{"operational.read", "sim.runtime.execute"}, nil)
	call(outsider, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 404)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+probeID+"/stop", nil, 200)
	beat.Status = "STOPPED"
	beat.Snapshot.Sequence++
	if _, e = broker.Heartbeat(workerContext(t, h, runtimeToken), probeID, beat); e != nil {
		t.Fatal(e)
	}
	run := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)
	id := run["id"].(string)
	start.Scale = 2
	call(author, "POST", "/api/v1/simulation-runtime/runs", start, 409)
	start.Scale = 1
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+id+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: artifacts.Hash([]byte("wrong-build"))}, 409)
	tokenPath := filepath.Join(t.TempDir(), "credential")
	if e = os.WriteFile(tokenPath, []byte(runtimeToken), 0600); e != nil {
		t.Fatal(e)
	}
	cmd = exec.Command(bin, "-url", h.server.URL, "-run", id, "-token-file", tokenPath)
	var processLog bytes.Buffer
	cmd.Stderr = &processLog
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})
	wait := func(status string) map[string]any {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			v := call(author, "GET", "/api/v1/simulation-runtime/runs/"+id, nil, 200)
			if v["status"] == status && v["sequence"].(float64) > 0 {
				return v
			}
			time.Sleep(100 * time.Millisecond)
		}
		select {
		case err := <-done:
			t.Fatal("runtime exited", err, processLog.String())
		default:
		}
		t.Fatal("runtime did not reach", status)
		return nil
	}
	wait("RUNNING")
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+id+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 409)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/pause", nil, 200)
	paused := wait("PAUSED")
	time.Sleep(800 * time.Millisecond)
	later := wait("PAUSED")
	if later["elapsedMs"] != paused["elapsedMs"] {
		t.Fatal("paused clock advanced")
	}
	call(author, "GET", "/api/v1/simulation-runtime/runs/"+id+"/truth", nil, 403)
	truthResource := r
	truthResource.Partition = access.ExerciseTruth
	truthResource.RunID = id
	evaluator := scopeTokenForTruth(t, h, truthResource)
	truth := call(evaluator, "GET", "/api/v1/simulation-runtime/runs/"+id+"/truth", nil, 200)
	if len(truth["actors"].([]any)) != 3 {
		t.Fatal("sealed truth missing")
	}
	call(evaluator, "GET", "/api/v1/tracks", nil, 403)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/resume", nil, 200)
	wait("RUNNING")
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/stop", nil, 200)
	wait("STOPPED")
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runtime did not terminate")
	}
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/resume", nil, 409)
	// Killing a process does not enable another worker to take over its run.
	start.RequestKey = "crash-proof"
	crash := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	lease := call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+crash+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 200)
	_ = lease
	if _, e = h.pool.Exec(context.Background(), `UPDATE simulation_runtime_runs SET lease_until=now()-interval '1 second' WHERE id=$1`, crash); e != nil {
		t.Fatal(e)
	}
	failed := call(author, "GET", "/api/v1/simulation-runtime/runs/"+crash, nil, 200)
	if failed["status"] != "FAILED" || failed["failureCode"] != "HEARTBEAT_TIMEOUT" {
		t.Fatal(failed)
	}
	call(runtimeToken, "POST", "/api/v1/simulation-runtime/runs/"+crash+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 409)
	// Cancellation reports a typed failure before dropping private world state.
	start.RequestKey = "cancellation-proof"
	cancelID := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	client, e := simruntime.NewClient(h.server.URL, runtimeToken)
	if e != nil {
		t.Fatal(e)
	}
	work, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- client.Execute(work, cancelID, build) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v := call(author, "GET", "/api/v1/simulation-runtime/runs/"+cancelID, nil, 200)
		if v["sequence"].(float64) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("canceled runtime did not exit")
	}
	canceled := call(author, "GET", "/api/v1/simulation-runtime/runs/"+cancelID, nil, 200)
	if canceled["status"] != "FAILED" || canceled["failureCode"] != "INTERRUPTED" {
		t.Fatal(canceled)
	}
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+cancelID+"/resume", nil, 409)
	// Revoked credentials cannot publish even with a still-current run lease.
	start.RequestKey = "revocation-proof"
	revokedID := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	revocable := scopeToken(t, h, "revocable-native-runtime", r, []string{"operational.read", "sim.runtime.execute"}, nil)
	revokedLease := call(revocable, "POST", "/api/v1/simulation-runtime/runs/"+revokedID+"/attach", simruntime.Attach{RuntimeVersion: simscene.RuntimeVersion, RuntimeBuild: build}, 200)
	session, e := identity.Authenticate(context.Background(), revocable, func(string) []string { return nil })
	if e != nil {
		t.Fatal(e)
	}
	if e = identity.RevokeCredential(admin, session.Principal.CredentialID); e != nil {
		t.Fatal(e)
	}
	beat.Token = revokedLease["token"].(string)
	beat.Snapshot.RunID = revokedID
	beat.Snapshot.Sequence = 1
	beat.Status = "RUNNING"
	call(revocable, "POST", "/api/v1/simulation-runtime/runs/"+revokedID+"/heartbeat", beat, 401)
	var observations int
	if e = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM observations WHERE scope_id LIKE 'studio-proof/%'`).Scan(&observations); e != nil || observations != 0 {
		t.Fatal("runtime leaked into perception", observations, e)
	}
	if os.Getenv("EMAD_CAPTURE_STUDIO_EVIDENCE") == "1" {
		if e = os.MkdirAll("../../tmp/s1-s2", 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile("../../tmp/s1-s2/frozen-scene.json", sceneRaw, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile("../../tmp/s1-s2/windows-offline-truth.json", offlineRaw, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func scopeTokenForTruth(t *testing.T, h *harness, r access.Resource) string {
	t.Helper()
	admin := access.Bootstrap(context.Background())
	p, ar, _ := access.From(admin)
	p.Grants[0].Permissions = append(p.Grants[0].Permissions, "sim.truth.grant")
	admin, e := access.Bind(admin, p, ar, nil)
	if e != nil {
		t.Fatal(e)
	}
	identity := access.NewStore(h.pool)
	if e = identity.CreateHuman(admin, "studio-evaluator", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.truth.read"}}); e != nil {
		t.Fatal(e)
	}
	_, token, e := identity.IssueCredential(admin, "studio-evaluator", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	return token
}
