package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

// Extends the real catalogue/S3/worker/review workflow into the M3 exit gate.
func assertM3ApprovedExport(t *testing.T, f *publicFixture, published, draft, scenarioPath string) {
	t.Helper()
	call := func(token, method, path string, input any, want int) map[string]any {
		t.Helper()
		code, b := scopeRequest(t, f.h, token, method, path, input, nil)
		t.Logf("M3 %s %s want %d got %d", method, path, want, code)
		if code != want {
			t.Fatalf("%s %s %d %v", method, path, code, b)
		}
		return b
	}
	in := contextpackage.BuildInput{AssessmentRevisionIDs: []string{published}, AOI: publicingest.Bounds{West: 39.1601, South: 21.4893, East: 39.1607, North: 21.4899}, TimeWindow: contextpackage.Window{From: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 2, 28, 23, 59, 59, 0, time.UTC)}, AcceptedQuality: "Synthetic angular spectral subsets; no assertion of real vegetation loss", Approval: "Approve the pinned synthetic environmental context"}
	in.Scope = &analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Pinned export fixture", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[39.16,21.489],[39.161,21.489],[39.161,21.49],[39.16,21.49],[39.16,21.489]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: in.TimeWindow.From, To: in.TimeWindow.To}, RevisionView: "latest", Timezone: "UTC"}}
	call(f.manager, "POST", "/api/v1/context-packages", in, 400)
	bad := in
	bad.AssessmentRevisionIDs = []string{draft}
	call(f.manager, "POST", "/api/v1/context-packages/preview", bad, 404)
	bad = in
	bad.AOI.East = 39.17
	call(f.manager, "POST", "/api/v1/context-packages/preview", bad, 400)
	bad = in
	bad.TimeWindow.To = bad.TimeWindow.From
	call(f.manager, "POST", "/api/v1/context-packages/preview", bad, 400)
	preview := call(f.manager, "POST", "/api/v1/context-packages/preview", in, 200)
	manifest := preview["manifest"].(map[string]any)
	kinds := map[string]int{}
	for _, v := range manifest["assets"].([]any) {
		kinds[v.(map[string]any)["kind"].(string)]++
	}
	if kinds["imagery"] != 2 || kinds["analysis"] != 1 || kinds["assessment"] != 1 || kinds["derived_geometry"] != 1 || manifest["syntheticDerived"] != true {
		t.Fatal("export selection lost lineage", kinds, manifest)
	}
	if items := call(f.manager, "GET", "/api/v1/context-packages", nil, 200); items["total"] != float64(0) {
		t.Fatal("preview created approved package", items)
	}
	freeze := contextpackage.BuildInput{PreviewID: preview["id"].(string), PreviewSHA256: preview["sha256"].(string), Approval: in.Approval}
	wrong := freeze
	wrong.PreviewSHA256 = "changed"
	call(f.manager, "POST", "/api/v1/context-packages", wrong, 409)
	wrong = freeze
	wrong.Approval = ""
	call(f.manager, "POST", "/api/v1/context-packages", wrong, 400)
	hiddenStore := access.NewStore(f.h.pool)
	hiddenAdmin := access.Bootstrap(context.Background())
	if err := hiddenStore.CreateHuman(hiddenAdmin, "m3-hidden-source", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "context.export", "assessments.read"}, SourceIDs: []string{"unrelated"}}); err != nil {
		t.Fatal(err)
	}
	_, restricted, err := hiddenStore.IssueCredential(hiddenAdmin, "m3-hidden-source", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	call(restricted, "POST", "/api/v1/context-packages", freeze, 404)
	if err := hiddenStore.CreateHuman(hiddenAdmin, "m3-other-approver", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "context.export"}, SourceIDs: []string{f.source}}); err != nil {
		t.Fatal(err)
	}
	_, other, err := hiddenStore.IssueCredential(hiddenAdmin, "m3-other-approver", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	call(other, "POST", "/api/v1/context-packages", freeze, 409)
	service := scopeToken(t, f.h, "m3-service-cannot-approve", f.resource, []string{"operational.read", "sources.read", "context.export"}, []string{f.source})
	call(service, "POST", "/api/v1/context-packages", freeze, 403)
	// Rights/admission changes between preview and approval require a fresh preview.
	f.admission.Rights.ID = "m3-export-withdrawn"
	f.admission.Rights.AllowSimulationExport = false
	f.admit(t)
	call(f.manager, "POST", "/api/v1/context-packages", freeze, 409)
	call(f.manager, "POST", "/api/v1/context-packages/preview", in, 403)
	f.admission.Rights.ID = "m3-export-restored"
	f.admission.Rights.AllowSimulationExport = true
	f.admit(t)
	preview = call(f.manager, "POST", "/api/v1/context-packages/preview", in, 200)
	freeze.PreviewID = preview["id"].(string)
	freeze.PreviewSHA256 = preview["sha256"].(string)
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE context_export_previews SET expires_at=$2 WHERE id=$1`, freeze.PreviewID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	call(f.manager, "POST", "/api/v1/context-packages", freeze, 409)
	preview = call(f.manager, "POST", "/api/v1/context-packages/preview", in, 200)
	freeze.PreviewID = preview["id"].(string)
	freeze.PreviewSHA256 = preview["sha256"].(string)
	frozen := call(f.manager, "POST", "/api/v1/context-packages", freeze, 201)
	id := frozen["id"].(string)
	download := call(f.manager, "GET", "/api/v1/context-packages/"+id+"/bundle", nil, 200)
	var b contextpackage.Bundle
	if err := json.Unmarshal(p2JSON(download), &b); err != nil {
		t.Fatal(err)
	}
	if err := b.Validate(time.Now(), true); err != nil {
		t.Fatal("offline validator rejected frozen context", err)
	}
	if b.Manifest.Scope == nil || b.Manifest.Scope.ScopeHash == "" {
		t.Fatal("export omitted its exact area/period")
	}
	if _, err := (analystscope.Service{Pool: f.h.pool}).Get(scopeContext(t, f.h, f.resource), b.Manifest.Scope.ID); err != nil {
		t.Fatal("approved package did not atomically retain its scope", err)
	}
	for _, a := range b.Manifest.Assets {
		if a.Kind == "assessment" {
			var revision map[string]any
			json.Unmarshal(b.Assets[a.SHA256], &revision)
			if revision["id"] != published || revision["status"] != "PUBLISHED" {
				t.Fatal("export used newer unreviewed draft", revision)
			}
		}
		if a.Kind == "derived_geometry" && !a.SyntheticDerived {
			t.Fatal("derived geometry lost synthetic marker")
		}
	}
	call(restricted, "GET", "/api/v1/context-packages/"+id+"/bundle", nil, 404)
	// The existing S0 adapter consumes the extended manifest and freezes its bytes;
	// this is an explicit synthetic actor, never an imported public entity.
	identities := access.NewStore(f.h.pool)
	exercise := f.resource
	exercise.Partition = access.ExerciseObservation
	exercise.RunID = "authoring"
	admin := access.Bootstrap(context.Background())
	if err := identities.EnsureResource(admin, exercise); err != nil {
		t.Fatal(err)
	}
	exercise.RunID = "*"
	if err := identities.CreateHuman(admin, "m3-scenario-author", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "context.export"}}); err != nil {
		t.Fatal(err)
	}
	_, authorToken, err := identities.IssueCredential(admin, "m3-scenario-author", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := identities.AddGrant(admin, "m3-scenario-author", access.Grant{Resource: exercise, Permissions: []string{"operational.read", "scenarios.read", "scenarios.control", "sim.scenarios.author"}}); err != nil {
		t.Fatal(err)
	}
	definition := fmt.Sprintf(`scenario: m3-export
seed: 73
playback_speed: 1000
context_packages: [%s]
sources: [{id: synthetic-context-sensor, name: Synthetic sensor, type: synthetic}]
tracks: [{id: EXPLICIT_SYNTHETIC_ACTOR}]
events:
  - {after: 0ms, action: observe, source: synthetic-context-sensor, track: EXPLICIT_SYNTHETIC_ACTOR, position: {lat: 21.4896, lng: 39.1604}}
`, id)
	if err := os.WriteFile(scenarioPath, []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-EMAD-Partition": string(access.ExerciseObservation), "X-EMAD-Run-ID": "authoring"}
	code, run := scopeRequest(t, f.h, authorToken, "POST", "/api/v1/scenarios/definitions/m3-export/start", nil, headers)
	if code != 201 {
		t.Fatalf("M3 headless context %d %v", code, run)
	}
	waitFor(t, 10*time.Second, 20*time.Millisecond, "M3 frozen-context completion", func() bool {
		_, state := scopeRequest(t, f.h, authorToken, "GET", "/api/v1/scenarios/runs/"+run["id"].(string), nil, nil)
		return state["status"] == "COMPLETED"
	})
	var count int
	if err := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM context_packages WHERE id=$1 AND document->'approval'->>'statement'=$2`, id, in.Approval).Scan(&count); err != nil || count != 1 {
		t.Fatal("scenario mutated public package", count, err)
	}
	// Metadata-only exports explicitly refuse an offline reproduction claim.
	in.ReferenceOnly = true
	preview = call(f.manager, "POST", "/api/v1/context-packages/preview", in, 200)
	freeze.PreviewID = preview["id"].(string)
	freeze.PreviewSHA256 = preview["sha256"].(string)
	ref := call(f.manager, "POST", "/api/v1/context-packages", freeze, 201)
	ref = call(f.manager, "GET", "/api/v1/context-packages/"+ref["id"].(string)+"/bundle", nil, 200)
	if err := json.Unmarshal(p2JSON(ref), &b); err != nil {
		t.Fatal(err)
	}
	if err := b.Validate(time.Now(), true); err == nil {
		t.Fatal("reference-only package claimed offline reproduction")
	}
}
