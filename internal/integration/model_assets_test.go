package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func assetLicense() modelassets.License {
	return modelassets.License{ID: "fixture-terms-v1", LicenseID: "LicenseRef-EMAD-TestFixture", TermsURL: "https://example.invalid/emad-fixtures", Attribution: "EMAD project-authored abstract fixture", LegalBasis: "Project-authored geometry for internal development and synthetic simulation; no external model inputs.", AllowStorage: true, AllowSimulation: true, AllowRedistribution: false, ArchivePermission: "perpetual"}
}
func assetInput(key, media string, b []byte) modelassets.UploadInput {
	return modelassets.UploadInput{SourceVersion: "fixture-v1", RequestKey: key, Name: key, Kind: "MODEL", SHA256: artifacts.Hash(b), Bytes: int64(len(b)), MediaType: media, SourceReference: "https://example.invalid/emad-fixtures", Provenance: "Project-authored synthetic geometry; no physical performance validation.", License: assetLicense()}
}
func assetBytes(t *testing.T, h *harness, token, id string, b []byte, want int) map[string]any {
	t.Helper()
	req, e := http.NewRequest("PUT", h.server.URL+"/api/v1/simulation-assets/uploads/"+id+"/content", bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	r, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var out map[string]any
	if e = json.NewDecoder(r.Body).Decode(&out); e != nil {
		t.Fatal(e)
	}
	if r.StatusCode != want {
		t.Fatal(r.StatusCode, out)
	}
	return out
}
func workerContext(t *testing.T, h *harness, token string) context.Context {
	t.Helper()
	store := access.NewStore(h.pool)
	session, e := store.Authenticate(context.Background(), token, httpx.PermissionsForRole)
	if e != nil {
		t.Fatal(e)
	}
	ctx, e := store.BindSession(context.Background(), session, session.Resource)
	if e != nil {
		t.Fatal(e)
	}
	return ctx
}
func TestS11ModelAssetRegistryAndProtectedAuthoring(t *testing.T) {
	store := newS3Fixture(t)
	h := newHarness(t, app.WithArtifactStore(store))
	ctx := context.Background()
	identities := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	r := access.Resource{WorkspaceID: "asset-fixture", EnvironmentID: "studio", Partition: access.ExerciseInput}
	if e := identities.CreateHuman(admin, "asset-author", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "audit.read"}}); e != nil {
		t.Fatal(e)
	}
	_, author, e := identities.IssueCredential(admin, "asset-author", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	worker := scopeToken(t, h, "asset-import-worker", r, []string{"operational.read", "sim.assets.import"}, nil)
	svc := modelassets.New(h.pool, store)
	validator, e := modelassets.NewNodeValidator("../../apps/emad-asset-validator/validate.mjs")
	if e != nil {
		t.Fatal(e)
	}
	build := artifacts.Hash([]byte("s11-fixture-worker"))
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		c, b := scopeRequest(t, h, token, method, path, in, nil)
		if c != want {
			t.Fatalf("%s %s %d %v", method, path, c, b)
		}
		return b
	}
	raw, e := os.ReadFile("../../testdata/models/abstract-wing.glb")
	if e != nil {
		t.Fatal(e)
	}
	in := assetInput("wing", "model/gltf-binary", raw)
	bad := in
	bad.License.AllowSimulation = false
	call(author, "POST", "/api/v1/simulation-assets/uploads", bad, 400)
	bad = in
	bad.Bytes = 17 * 1024 * 1024
	call(author, "POST", "/api/v1/simulation-assets/uploads", bad, 400)
	sourceBytes := []byte("Project-authored fixture generator source reference, retained independently.")
	source := assetInput("authoring-source", "application/octet-stream", sourceBytes)
	source.Kind = "SOURCE"
	sourceID := call(author, "POST", "/api/v1/simulation-assets/uploads", source, 201)["id"].(string)
	assetBytes(t, h, author, sourceID, sourceBytes, 200)
	in.SourceFileID = sourceID
	reserved := call(author, "POST", "/api/v1/simulation-assets/uploads", in, 201)
	id := reserved["id"].(string)
	if again := call(author, "POST", "/api/v1/simulation-assets/uploads", in, 201); again["id"] != id {
		t.Fatal("idempotency lost")
	}
	changed := in
	changed.Name = "different"
	call(author, "POST", "/api/v1/simulation-assets/uploads", changed, 409)
	assetBytes(t, h, author, id, []byte("wrong hash"), 400)
	assetBytes(t, h, author, id, raw, 200)
	if did, e := svc.WorkOne(workerContext(t, h, worker), validator, build); e != nil || !did {
		t.Fatal(did, e)
	}
	models := call(author, "GET", "/api/v1/simulation-assets/models", nil, 200)
	if models["total"] != float64(1) {
		t.Fatal(models)
	}
	model := models["items"].([]any)[0].(map[string]any)
	version := model["id"].(string)
	validation := model["validation"].(map[string]any)
	if validation["triangles"] != float64(1) || validation["units"] != "metres" || validation["fidelity"] != "visual_only" || model["validatorBuild"] != validator.Build() || model["manifestKey"] != nil {
		t.Fatal(model)
	}
	if state := call(author, "GET", "/api/v1/simulation-assets/uploads/"+id, nil, 200); state["state"] != "READY" || state["attempts"] != float64(1) {
		t.Fatal(state)
	}
	// Download both exact source bytes and the independently retained import report.
	for _, test := range []struct{ path, hash string }{{"uploads/" + id + "/content", in.SHA256}, {"models/" + version + "/manifest", model["manifestSha256"].(string)}} {
		req, _ := http.NewRequest("GET", h.server.URL+"/api/v1/simulation-assets/"+test.path, nil)
		req.Header.Set("Authorization", "Bearer "+author)
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 || artifacts.Hash(b) != test.hash {
			t.Fatalf("artifact integrity %s %d %s", test.path, res.StatusCode, b)
		}
	}
	template := modelassets.ActorTemplate{Name: "Abstract authored aircraft shape", ModelVersionID: version, Behavior: "WAYPOINT_KINEMATIC", MaxSpeedMPS: 30, Fidelity: "abstract_behavior", Limitations: "Template only; no instantiated actor and no validated aerodynamics or sensor capability."}
	tpl := call(author, "POST", "/api/v1/simulation-assets/templates", template, 201)
	tplAgain := call(author, "POST", "/api/v1/simulation-assets/templates", template, 201)
	if tpl["id"] != tplAgain["id"] || tpl["createdAt"] != tplAgain["createdAt"] || tpl["createdBy"] != "asset-author" {
		t.Fatal(tpl, tplAgain)
	}
	height := 10.0
	env := modelassets.EnvironmentAsset{Name: "Synthetic local reference", ModelVersionID: version, Frame: simulation.DefaultFrame(simulation.Position{Longitude: 39.2, Latitude: 21.5, Height: &height, VerticalDatum: "WGS84_ELLIPSOID"}), ModelAxesToENU: "x_east_y_up_z_south", Provenance: "Project-authored synthetic geometry placed at an explicitly declared origin; not surveyed terrain.", Fidelity: "visual_only"}
	environment := call(author, "POST", "/api/v1/simulation-assets/environments", env, 201)
	env.Frame.Origin.VerticalDatum = "MSL"
	call(author, "POST", "/api/v1/simulation-assets/environments", env, 400)
	variantBytes := []byte("Opaque test-engine output; no engine execution or validation claimed.")
	variant := assetInput("native-fixture", "application/octet-stream", variantBytes)
	variant.Kind = "ENGINE_VARIANT"
	variant.ParentVersionID = version
	variant.Engine = "fixture-engine"
	variant.EngineBuild = "synthetic-not-selected"
	variantID := call(author, "POST", "/api/v1/simulation-assets/uploads", variant, 201)["id"].(string)
	assetBytes(t, h, author, variantID, variantBytes, 200)
	if b := call(author, "GET", "/api/v1/simulation-assets/models", nil, 200); b["total"] != float64(1) {
		t.Fatal("opaque variant became validated model")
	}
	// Invalid model input remains inspectable but cannot acquire a model version.
	invalid := assetInput("archive-as-glb", "model/gltf-binary", []byte("PK invalid archive"))
	invalidID := call(author, "POST", "/api/v1/simulation-assets/uploads", invalid, 201)["id"].(string)
	assetBytes(t, h, author, invalidID, []byte("PK invalid archive"), 200)
	if did, e := svc.WorkOne(workerContext(t, h, worker), validator, build); e != nil || !did {
		t.Fatal(did, e)
	}
	rejected := call(author, "GET", "/api/v1/simulation-assets/uploads/"+invalidID, nil, 200)
	if rejected["state"] != "REJECTED" {
		t.Fatal(rejected)
	}
	if history := call(author, "GET", "/api/v1/simulation-assets/uploads/"+invalidID+"/attempts", nil, 200); history["total"] != float64(1) {
		t.Fatal(history)
	}

	// Exercise the embedded glTF/texture path and exact manifest retention even
	// when JSONB normalizes tiny material numbers into a different textual form.
	gltf, e := os.ReadFile("../../testdata/models/ground-tile.gltf")
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	if e = json.Unmarshal(gltf, &doc); e != nil {
		t.Fatal(e)
	}
	var pngBytes bytes.Buffer
	if e = png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	doc["images"] = []any{map[string]any{"uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes.Bytes())}}
	doc["textures"] = []any{map[string]any{"source": 0}}
	doc["materials"].([]any)[0].(map[string]any)["pbrMetallicRoughness"].(map[string]any)["baseColorFactor"] = []float64{1e-7, .5, .4, 1}
	textured, _ := json.Marshal(doc)
	texturedID := call(author, "POST", "/api/v1/simulation-assets/uploads", assetInput("textured-ground", "model/gltf+json", textured), 201)["id"].(string)
	assetBytes(t, h, author, texturedID, textured, 200)
	if did, e := svc.WorkOne(workerContext(t, h, worker), validator, build); e != nil || !did {
		t.Fatal(did, e)
	}
	found := false
	for _, item := range call(author, "GET", "/api/v1/simulation-assets/models", nil, 200)["items"].([]any) {
		m := item.(map[string]any)
		if m["upload"].(map[string]any)["id"] == texturedID {
			found = true
			manifest, e := svc.Manifest(workerContext(t, h, author), m["id"].(string))
			if e != nil || artifacts.Hash(manifest) != m["manifestSha256"] {
				t.Fatal("textured manifest integrity lost", e)
			}
			if m["validation"].(map[string]any)["decodedTexturePixels"] != float64(4) {
				t.Fatal(m)
			}
		}
	}
	if !found {
		t.Fatal("embedded textured model was not validated")
	}
	other := scopeToken(t, h, "asset-other-env", access.Resource{WorkspaceID: r.WorkspaceID, EnvironmentID: "other", Partition: access.ExerciseInput}, []string{"operational.read", "sim.scenarios.author"}, nil)
	for _, path := range []string{"uploads/" + id, "uploads/" + id + "/content", "models/" + version, "templates/" + tpl["id"].(string), "environments/" + environment["id"].(string)} {
		call(other, "GET", "/api/v1/simulation-assets/"+path, nil, 404)
	}
	public := scopeToken(t, h, "asset-public-reader", access.Resource{WorkspaceID: r.WorkspaceID, EnvironmentID: "world", Partition: access.PublicEvidence}, []string{"operational.read", "sim.scenarios.author"}, nil)
	call(public, "GET", "/api/v1/simulation-assets/models", nil, 403)
	operator := scopeToken(t, h, "asset-exercise-operator", access.LegacyResource(), []string{"operational.read", "sim.scenarios.author"}, nil)
	call(operator, "GET", "/api/v1/simulation-assets/models", nil, 403)
	call(worker, "POST", "/api/v1/simulation-assets/uploads", in, 403)
	if _, e := h.pool.Exec(ctx, `UPDATE model_asset_versions SET document='{}' WHERE id=$1`, version); e == nil {
		t.Fatal("mutable model version")
	}
	if _, e := h.pool.Exec(ctx, `UPDATE model_asset_uploads SET document='{}' WHERE id=$1`, id); e == nil {
		t.Fatal("mutable source/rights snapshot")
	}
	for _, table := range []string{"assets", "tracks", "observations", "simulation_actor_mappings"} {
		var n int
		if e := h.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); e != nil || n != 0 {
			t.Fatal("registry instantiated operational state", table, n, e)
		}
	}
	var audits int
	if e := h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE scope_id=$1 AND subject_type='simulation_asset'`, r.ID()).Scan(&audits); e != nil || audits < 4 {
		t.Fatal("registry provenance audit missing", audits, e)
	}
}

type assetValidationHook struct {
	modelassets.Validator
	entered, release chan struct{}
}

func (v assetValidationHook) Validate(ctx context.Context, b []byte, media string) (modelassets.Validation, error) {
	close(v.entered)
	select {
	case <-v.release:
		return v.Validator.Validate(ctx, b, media)
	case <-ctx.Done():
		return modelassets.Validation{}, ctx.Err()
	}
}
func TestS11RevokedImportLeaseRecoversWithoutDuplicateVersions(t *testing.T) {
	store := newS3Fixture(t)
	h := newHarness(t, app.WithArtifactStore(store))
	ctx := context.Background()
	identities := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	r := access.Resource{WorkspaceID: "asset-recovery", EnvironmentID: "studio", Partition: access.ExerciseInput}
	if e := identities.CreateHuman(admin, "recovery-author", "operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author"}}); e != nil {
		t.Fatal(e)
	}
	_, author, e := identities.IssueCredential(admin, "recovery-author", r, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	token := scopeToken(t, h, "recovery-worker-old", r, []string{"operational.read", "sim.assets.import"}, nil)
	bound := workerContext(t, h, token)
	validator, e := modelassets.NewNodeValidator("../../apps/emad-asset-validator/validate.mjs")
	if e != nil {
		t.Fatal(e)
	}
	svc := modelassets.New(h.pool, store)
	raw, e := os.ReadFile("../../testdata/models/marker.glb")
	if e != nil {
		t.Fatal(e)
	}
	code, a := scopeRequest(t, h, author, "POST", "/api/v1/simulation-assets/uploads", assetInput("recovery-model", "model/gltf-binary", raw), nil)
	if code != 201 {
		t.Fatal(code, a)
	}
	id := a["id"].(string)
	assetBytes(t, h, author, id, raw, 200)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := svc.WorkOne(bound, assetValidationHook{validator, entered, release}, artifacts.Hash([]byte("recovery-worker")))
		done <- e
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("validation not entered")
	}
	p, _, _ := access.From(bound)
	if e := identities.RevokeCredential(admin, p.CredentialID); e != nil {
		t.Fatal(e)
	}
	close(release)
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("revoked worker finalized version")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not stop")
	}
	var n int
	if e := h.pool.QueryRow(ctx, `SELECT count(*) FROM model_asset_versions WHERE scope_id=$1`, r.ID()).Scan(&n); e != nil || n != 0 {
		t.Fatal("revoked validation was published", n, e)
	}
	if _, e := h.pool.Exec(ctx, `UPDATE model_asset_uploads SET lease_until=now()-interval '1 second' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	fresh := scopeToken(t, h, "recovery-worker-new", r, []string{"operational.read", "sim.assets.import"}, nil)
	if did, e := svc.WorkOne(workerContext(t, h, fresh), validator, artifacts.Hash([]byte("recovery-worker"))); e != nil || !did {
		t.Fatal(did, e)
	}
	code, attempts := scopeRequest(t, h, author, "GET", "/api/v1/simulation-assets/uploads/"+id+"/attempts", nil, nil)
	if code != 200 || attempts["total"] != float64(2) {
		t.Fatal(code, attempts)
	}
	statuses := map[string]bool{}
	for _, v := range attempts["items"].([]any) {
		statuses[v.(map[string]any)["status"].(string)] = true
	}
	if !statuses["INTERRUPTED"] || !statuses["READY"] {
		t.Fatal(statuses)
	}
	if e := h.pool.QueryRow(ctx, `SELECT count(*) FROM model_asset_versions WHERE scope_id=$1`, r.ID()).Scan(&n); e != nil || n != 1 {
		t.Fatal("recovery duplicated model versions", n, e)
	}
	// Input authority remains confined to the dedicated authoring routes.
	if code, _ := scopeRequest(t, h, author, "GET", "/api/v1/picture", nil, nil); code != 403 {
		t.Fatal("input grant reached ordinary COP", code)
	}
}
