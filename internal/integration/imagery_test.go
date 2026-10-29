package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestP2CatalogueS3WorkerReproducibilityAndIndependentReview(t *testing.T) {
	store := newS3Fixture(t)
	readinessKey := "fixture-readiness/" + artifacts.Hash([]byte("ready"))
	if err := store.Put(context.Background(), readinessKey, []byte("altered"), "application/json"); err == nil {
		t.Fatal("S3 conditional put overwrote an immutable key")
	}
	if data, err := store.Get(context.Background(), readinessKey, 5); err != nil || string(data) != "ready" {
		t.Fatal("S3 conditional write changed retained bytes", err)
	}
	scenarioDir := t.TempDir()
	f := newPublicFixture(t, app.WithArtifactStore(store), app.WithScenariosDirectory(scenarioDir))
	ctx := context.Background()
	identities := access.NewStore(f.h.pool)
	admin := access.Bootstrap(ctx)
	if e := identities.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"imagery.jobs.submit", "assessments.create", "assessments.read", "incidents.create", "incidents.read", "context.export"}}); e != nil {
		t.Fatal(e)
	}
	registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.STACURL, Adapter: publicingest.STACAdapter, MaxRequestsPerDay: 20}})
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	raw, e := os.ReadFile("../../testdata/imagery/cdse-two-scenes.json")
	if e != nil {
		t.Fatal(e)
	}
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		b := raw
		if in.Checkpoint != "" {
			b = []byte(`{"type":"FeatureCollection","features":[],"links":[]}`)
		}
		page, e := publicingest.DecodeSTAC(b, in)
		return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200, ResponseHash: artifacts.Hash(b)}, e
	})
	f.admission.Dataset = publicingest.STACDataset
	f.admission.ProductVersion = "stac-1.1-l2a"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.OriginalProducer = "Copernicus Sentinel-2 catalogue"
	f.admission.UpstreamProvider = "CDSE STAC recorded fixture"
	f.admission.Policy.AOI = publicingest.Bounds{West: 39.15, South: 21.45, East: 39.2, North: 21.5}
	f.admission.Policy.LookbackSeconds = 31 * 86400
	f.admission.Policy.MinIntervalSeconds = 5
	f.admission.Policy.MaxRecordsPerPage = 2
	f.admission.Policy.MaxResponseBytes = 1024 * 1024
	f.admission.Rights.AllowAnalysis = true
	f.admission.Rights.AllowSimulationExport = true
	f.admit(t)
	window := map[string]any{"from": "2025-02-01T00:00:00Z", "to": "2025-02-28T23:59:59Z"}
	if status, b := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", window, nil); status != 201 || b["newEvidence"] != float64(2) || b["status"] != "PARTIAL" {
		t.Fatalf("native discovery %d %v", status, b)
	}
	f.ready(t)
	if status, b := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/sources/"+f.source+"/connector-runs", window, nil); status != 201 || b["status"] != "SUCCEEDED" {
		t.Fatalf("cursor completion %d %v", status, b)
	}
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		s, b := scopeRequest(t, f.h, token, method, path, in, nil)
		if s != want {
			t.Fatalf("%s %s: %d %v", method, path, s, b)
		}
		return b
	}
	scenes := call(f.manager, "GET", "/api/v1/imagery/scenes?bbox=39.15,21.45,39.2,21.5", nil, 200)["items"].([]any)
	if len(scenes) != 2 {
		t.Fatalf("two eligible scenes missing %v", scenes)
	}
	prepared := []string{}
	workerToken := scopeToken(t, f.h, "p2-analysis-worker", f.resource, []string{"operational.read", "sources.read", "imagery.jobs.execute"}, []string{f.source})
	session, e := identities.Authenticate(ctx, workerToken, nil)
	if e != nil {
		t.Fatal(e)
	}
	bound, e := identities.BindSession(ctx, session, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	svc := imagery.NewService(f.h.pool, artifacts.NewService(f.h.pool, store))
	build := artifacts.Hash([]byte("test-worker-build"))
	var firstArtifact string
	retention := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	for i, index := range []int{1, 0} {
		document, _ := json.Marshal(scenes[index])
		var scene imagery.Scene
		if json.Unmarshal(document, &scene) != nil {
			t.Fatal("scene DTO")
		}
		red, nir := .2, .6
		tile := imageryanalysis.Tile{Schema: "emad.multispectral-subset.v1", SceneID: scene.ID, SceneHash: scene.ContentHash, Grid: imageryanalysis.Grid{CRS: "EPSG:4326", Width: 4, Height: 4, West: 39.16, North: 21.49, StepLng: .0002, StepLat: .0002, Registration: "pixel_edges_north_to_south"}, AcquiredAt: scene.AcquiredAt, Platform: scene.Platform, Tile: scene.Tile, Orbit: scene.Orbit, ProcessingVersion: scene.ProcessingVersion, ReflectanceUnit: "surface_reflectance", PreparationMethod: "synthetic-fixture.v1", Synthetic: true, SourceHashes: map[string]string{}}
		for _, key := range []string{"B04_10m", "B08_10m", "SCL_20m"} {
			tile.SourceHashes[key] = artifacts.Hash([]byte("synthetic:" + scene.ID + key))
		}
		for cell := 0; cell < 16; cell++ {
			r, n := red, nir
			if i == 1 && cell < 3 {
				n = .2
			}
			tile.Red = append(tile.Red, &r)
			tile.NIR = append(tile.NIR, &n)
			tile.SCL = append(tile.SCL, 4)
		}
		tile.SCL[15] = 9
		content, _ := json.Marshal(tile)
		in := artifacts.Input{RequestKey: fmt.Sprintf("subset-%d", i), SourceID: f.source, SHA256: artifacts.Hash(content), Bytes: int64(len(content)), MediaType: "application/json", Kind: "imagery_subset", RetainUntil: retention, Provenance: "Synthetic spectral fixture; recorded catalogue metadata is real but pixel values are not observations"}
		artifact := call(f.manager, "POST", "/api/v1/artifacts", in, 201)
		id := artifact["id"].(string)
		if i == 0 {
			firstArtifact = id
		}
		if artifact["state"] != "PENDING" || artifact["objectKey"] != nil {
			t.Fatal("incomplete upload exposed as ready or object key disclosed")
		}
		call(f.manager, "POST", "/api/v1/imagery/jobs", map[string]any{"requestKey": "premature", "kind": "PREPARE", "sceneId": scene.ID, "artifactId": id}, 409)
		req, _ := http.NewRequest("PUT", f.h.server.URL+"/api/v1/artifacts/"+id+"/content", bytes.NewReader(content))
		req.Header.Set("Authorization", "Bearer "+f.manager)
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("S3 upload %d %s", response.StatusCode, data)
		}
		a, download, e := svc.Artifacts.Read(bound, id)
		if e != nil || !bytes.Equal(download, content) || a.SHA256 != artifacts.Hash(content) {
			t.Fatalf("S3 exact bytes %v", e)
		}
		if _, e = svc.Artifacts.Upload(bound, id, append([]byte{}, []byte("corrupt")...)); !apperr.Is(e, apperr.CodeValidation) {
			t.Fatal("hash mutation accepted", e)
		}
		job := call(f.manager, "POST", "/api/v1/imagery/jobs", map[string]any{"requestKey": fmt.Sprintf("prepare-%d", i), "kind": "PREPARE", "sceneId": scene.ID, "artifactId": id}, 201)
		jid := job["id"].(string)
		if did, e := svc.WorkOne(bound, build); e != nil || !did {
			t.Fatalf("worker preparation %v", e)
		}
		done := call(f.manager, "GET", "/api/v1/imagery/jobs/"+jid, nil, 200)
		if done["status"] != "SUCCEEDED" {
			t.Fatal(done)
		}
		prepared = append(prepared, jid)
	}
	request := map[string]any{"requestKey": "change-1", "kind": "NDVI_CHANGE", "preparationIds": prepared, "parameters": map[string]any{"declineThreshold": .2, "minValidFraction": .5}}
	scopeInput := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Scoped imagery fixture", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[39.15,21.48],[39.18,21.48],[39.18,21.50],[39.15,21.50],[39.15,21.48]]]}`)}, Temporal: queryscope.Temporal{Mode: "historical", Interval: &queryscope.Interval{From: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)}, RevisionView: "latest", Timezone: "UTC"}}
	scopeResolved, scopeErr := (analystscope.Service{Pool: f.h.pool}).Resolve(scopeContext(t, f.h, f.resource), scopeInput)
	if scopeErr != nil {
		t.Fatal(scopeErr)
	}
	for _, path := range []string{"/api/v1/imagery/scenes", "/api/v1/imagery/jobs", "/api/v1/imagery/captures", "/api/v1/assessments/intelligence"} {
		call(f.manager, "GET", scopedCatalogURL(path, scopeResolved), nil, 200)
	}
	pairCheck := call(f.manager, "POST", "/api/v1/imagery/comparison-check", imagery.PairInput{Scope: scopeInput, BeforeID: prepared[0], AfterID: prepared[1], Parameters: imageryanalysis.Parameters{DeclineThreshold: .2, MinValidFraction: .5}}, 200)
	if pairCheck["compatible"] != true || pairCheck["validCells"] != float64(15) {
		t.Fatal("exact retained pair preflight", pairCheck)
	}
	insufficient := call(f.manager, "POST", "/api/v1/imagery/comparison-check", imagery.PairInput{Scope: scopeInput, BeforeID: prepared[0], AfterID: prepared[1], Parameters: imageryanalysis.Parameters{DeclineThreshold: .2, MinValidFraction: .99}}, 200)
	if insufficient["compatible"] != false || insufficient["validCells"] != float64(15) {
		t.Fatal("clear mask preflight accepted insufficient coverage", insufficient)
	}
	scopedRequest := map[string]any{"requestKey": "scope-contract-check", "kind": "NDVI_CHANGE", "preparationIds": prepared, "parameters": map[string]any{"declineThreshold": .2, "minValidFraction": .5}, "scope": scopeInput}
	queuedScoped := call(f.manager, "POST", "/api/v1/imagery/jobs", scopedRequest, 201)
	if queuedScoped["input"].(map[string]any)["scope"].(map[string]any)["scopeHash"] != scopeResolved.ScopeHash {
		t.Fatalf("job lost area/period lineage: expected %s area %s got %s", p2JSON(scopeResolved), scopeResolved.Area.RevisionID, p2JSON(queuedScoped["input"].(map[string]any)["scope"]))
	}
	call(f.manager, "POST", "/api/v1/imagery/jobs/"+queuedScoped["id"].(string)+"/cancel", nil, 200)
	badScope := scopeInput
	badScope.Area = areas.Selection{Kind: "drawn", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}
	scopedRequest["requestKey"] = "wrong-scope-check"
	scopedRequest["scope"] = badScope
	call(f.manager, "POST", "/api/v1/imagery/jobs", scopedRequest, 400)
	submitted := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)
	jobID := submitted["id"].(string)
	if again := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201); again["id"] != jobID {
		t.Fatal("submission idempotency lost")
	}
	if did, e := svc.WorkOne(bound, build); e != nil || !did {
		t.Fatalf("worker comparison %v", e)
	}
	result := call(f.manager, "GET", "/api/v1/imagery/jobs/"+jobID, nil, 200)
	if result["status"] != "SUCCEEDED" {
		t.Fatal(result)
	}
	output := result["result"].(map[string]any)
	if output["validCells"] != float64(15) || output["declineCells"] != float64(3) || output["synthetic"] != true {
		t.Fatalf("NDVI evidence %v", output)
	}
	// PostGIS normalizes adjacent changed-cell footprints into valid geometry.
	var valid bool
	if e = f.h.pool.QueryRow(ctx, `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON($1),4326))`, string(p2JSON(output["findingGeometry"]))).Scan(&valid); e != nil || !valid {
		t.Fatal("invalid derived finding geometry", e)
	}
	request["requestKey"] = "change-2"
	second := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)
	if _, e = svc.WorkOne(bound, build); e != nil {
		t.Fatal(e)
	}
	repeated := call(f.manager, "GET", "/api/v1/imagery/jobs/"+second["id"].(string), nil, 200)
	if repeated["result"].(map[string]any)["sha256"] != output["sha256"] {
		t.Fatal("pinned calculation did not reproduce identical result bytes")
	}
	request["requestKey"] = "cancel-me"
	pending := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)
	if canceled := call(f.manager, "POST", "/api/v1/imagery/jobs/"+pending["id"].(string)+"/cancel", nil, 200); canceled["status"] != "CANCELED" {
		t.Fatal(canceled)
	}
	// A dead process leaves frozen inputs intact and records an interrupted
	// attempt before a new lease resumes the same job.
	request["requestKey"] = "recover-process"
	abandoned := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)
	abandonedID := abandoned["id"].(string)
	if _, e = f.h.pool.Exec(ctx, `UPDATE imagery_jobs SET status='RUNNING',attempt=1,lease_id='dead-worker',lease_until=now()-interval '1 second',worker_build=$2 WHERE id=$1`, abandonedID, build); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.WorkOne(bound, build); e != nil {
		t.Fatal(e)
	}
	recovered := call(f.manager, "GET", "/api/v1/imagery/jobs/"+abandonedID, nil, 200)
	if recovered["status"] != "SUCCEEDED" || recovered["attempt"] != float64(2) {
		t.Fatalf("lease recovery lost %v", recovered)
	}
	if attempts := call(f.manager, "GET", "/api/v1/imagery/jobs/"+abandonedID+"/attempts", nil, 200); attempts["total"] != float64(2) {
		t.Fatal("interrupted attempt history lost", attempts)
	}

	// Running cancellation interrupts I/O and cannot publish a result.
	request["requestKey"] = "cancel-running"
	running := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)["id"].(string)
	entered, release := make(chan struct{}), make(chan struct{})
	gated := p2Store{Store: store, get: func(c context.Context, key string, size int64) ([]byte, error) {
		close(entered)
		select {
		case <-release:
			return store.Get(c, key, size)
		case <-c.Done():
			return nil, c.Err()
		}
	}}
	done := make(chan error, 1)
	go func() {
		_, err := imagery.NewService(f.h.pool, artifacts.NewService(f.h.pool, gated)).WorkOne(bound, build)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not enter retained object read")
	}
	if b := call(f.manager, "POST", "/api/v1/imagery/jobs/"+running+"/cancel", nil, 200); b["status"] != "CANCEL_REQUESTED" {
		t.Fatal(b)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("running cancellation did not stop work")
	}
	if b := call(f.manager, "GET", "/api/v1/imagery/jobs/"+running, nil, 200); b["status"] != "CANCELED" || b["result"] != nil {
		t.Fatal("canceled result became visible", b)
	}
	// A temporary object-store failure can retry the exact same frozen inputs.
	request["requestKey"] = "retry-storage"
	retryID := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)["id"].(string)
	broken := p2Store{Store: store, get: func(context.Context, string, int64) ([]byte, error) {
		return nil, errors.New("temporary fixture outage")
	}}
	if _, err := imagery.NewService(f.h.pool, artifacts.NewService(f.h.pool, broken)).WorkOne(bound, build); err != nil {
		t.Fatal(err)
	}
	failed := call(f.manager, "GET", "/api/v1/imagery/jobs/"+retryID, nil, 200)
	if failed["status"] != "FAILED" || failed["errorCode"] != "artifact_or_processing_unavailable" {
		t.Fatal(failed)
	}
	call(f.manager, "POST", "/api/v1/imagery/jobs/"+retryID+"/retry", nil, 200)
	if _, err := svc.WorkOne(bound, build); err != nil {
		t.Fatal(err)
	}
	if b := call(f.manager, "GET", "/api/v1/imagery/jobs/"+retryID, nil, 200); b["status"] != "SUCCEEDED" || b["attempt"] != float64(2) || string(p2JSON(b["input"])) != string(p2JSON(failed["input"])) {
		t.Fatal("frozen retry lost", b)
	}
	// Revoked analysis rights fail a queued job; no result is manufactured.
	request["requestKey"] = "rights-withdrawn"
	rightsID := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)["id"].(string)
	f.admission.Rights.ID = "p2-analysis-withdrawn"
	f.admission.Rights.AllowAnalysis = false
	f.admit(t)
	if _, err := svc.WorkOne(bound, build); err != nil {
		t.Fatal(err)
	}
	if b := call(f.manager, "GET", "/api/v1/imagery/jobs/"+rightsID, nil, 200); b["status"] != "FAILED" || b["errorCode"] != "rights_unavailable" {
		t.Fatal(b)
	}
	f.admission.Rights.ID = "p2-analysis-restored"
	f.admission.Rights.AllowAnalysis = true
	f.admit(t)
	// A reverse-time pair is rejected before work is queued.
	call(f.manager, "POST", "/api/v1/imagery/jobs", map[string]any{"requestKey": "backwards", "kind": "NDVI_CHANGE", "preparationIds": []string{prepared[1], prepared[0]}, "parameters": map[string]any{"declineThreshold": .2, "minValidFraction": .5}}, 400)
	proposal := call(f.manager, "POST", "/api/v1/assessments/imagery-proposals", map[string]any{"jobId": jobID, "conclusion": "Synthetic NDVI decline indicator requires human interpretation.", "method": "AI", "modelScore": .9}, 201)
	assessmentID := proposal["assessmentId"].(string)
	revisionID := proposal["id"].(string)
	publicIncident := call(f.manager, "POST", "/api/v1/incidents", map[string]any{"title": "Explicitly synthetic imagery review fixture", "assessmentIds": []string{assessmentID}}, 201)["id"].(string)
	attached := call(f.manager, "GET", "/api/v1/incidents/"+publicIncident, nil, 200)["assessments"].([]any)[0].(map[string]any)
	if attached["reviewStatus"] != "PROPOSED" || attached["confidence"] != nil {
		t.Fatal("incident hid unreviewed interpretation", attached)
	}

	legacy := call(f.manager, "GET", "/api/v1/assessments/"+assessmentID, nil, 200)
	if legacy["reviewStatus"] != "PROPOSED" || legacy["method"] != "AI" || legacy["confidence"] != nil {
		t.Fatal("unreviewed score became analyst confidence or published fact", legacy)
	}
	if e = identities.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"assessments.review"}}); e != nil {
		t.Fatal(e)
	}
	action := map[string]any{"expectedRevisionId": revisionID, "action": "review", "rationale": "Independent evidence review with declared caveats."}
	call(f.manager, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 403)
	if e = identities.CreateHuman(admin, "p2-independent-reviewer", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "assessments.read", "assessments.review"}, SourceIDs: []string{f.source}}); e != nil {
		t.Fatal(e)
	}
	_, reviewer, e := identities.IssueCredential(admin, "p2-independent-reviewer", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	// An alias of the same registered human is not independent.
	call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 403)
	aliasReviewer := reviewer
	if _, e = f.h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('p2-review-human','Independent development reviewer','supervisor')`); e != nil {
		t.Fatal(e)
	}
	if e = identities.CreateHuman(admin, "p2-distinct-reviewer", "p2-review-human", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "assessments.read", "assessments.review"}, SourceIDs: []string{f.source}}); e != nil {
		t.Fatal(e)
	}
	_, reviewer, e = identities.IssueCredential(admin, "p2-distinct-reviewer", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	reviewing := call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 201)
	action["expectedRevisionId"] = reviewing["id"]
	action["action"] = "publish"
	call(aliasReviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 403)
	action["action"] = "reject"
	call(aliasReviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 403)
	action["action"] = "publish"
	call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 400)
	action["confidence"] = .6
	action["confidenceRationale"] = "Synthetic subset calculation verified; no claim of real vegetation loss."
	published := call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", action, 201)
	if published["status"] != "PUBLISHED" || published["modelScore"] != .9 || published["confidence"] != .6 || published["publishedBy"] != "p2-distinct-reviewer" {
		t.Fatal("review state/score distinction lost", published)
	}
	history := call(reviewer, "GET", "/api/v1/assessments/"+assessmentID+"/revisions", nil, 200)
	if history["total"] != float64(3) {
		t.Fatal("immutable review history missing", history)
	}

	// A revised draft never overwrites the last published evidence or confidence.
	draftInput := map[string]any{"jobId": jobID, "conclusion": "Revised synthetic interpretation awaiting another independent review.", "method": "ALGORITHM", "expectedRevisionId": published["id"]}
	draft := call(f.manager, "POST", "/api/v1/assessments/"+assessmentID+"/revisions", draftInput, 201)
	call(f.manager, "POST", "/api/v1/assessments/"+assessmentID+"/revisions", draftInput, 409)
	if draft["status"] != "DRAFT" || draft["publishedBy"] != nil || draft["confidence"] != nil {
		t.Fatal("draft inherited publication", draft)
	}
	latest := call(f.manager, "GET", "/api/v1/assessments/"+assessmentID, nil, 200)
	attached = call(f.manager, "GET", "/api/v1/incidents/"+publicIncident, nil, 200)["assessments"].([]any)[0].(map[string]any)
	if attached["reviewStatus"] != "DRAFT" || attached["conclusion"] != draft["conclusion"] || attached["confidence"] != nil || attached["publishedRevisionId"] != published["id"] {
		t.Fatal("incident hid revised draft or used stale root", attached)
	}
	if latest["reviewStatus"] != "DRAFT" || latest["publishedRevisionId"] != published["id"] {
		t.Fatal("published pointer lost", latest)
	}
	exact := call(reviewer, "GET", "/api/v1/assessments/"+assessmentID+"/revisions/"+published["id"].(string), nil, 200)
	if exact["confidence"] != .6 || exact["status"] != "PUBLISHED" {
		t.Fatal("published history mutated", exact)
	}
	assertM3ApprovedExport(t, f, published["id"].(string), draft["id"].(string), filepath.Join(scenarioDir, "m3-export.yaml"))
	next := call(f.manager, "POST", "/api/v1/assessments/"+assessmentID+"/submit", map[string]any{"expectedRevisionId": draft["id"], "rationale": "Submit revised interpretation with explicit limitations."}, 201)
	next = call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", map[string]any{"expectedRevisionId": next["id"], "action": "review", "rationale": "Independent assessment of the revised wording."}, 201)
	rejected := call(reviewer, "POST", "/api/v1/assessments/"+assessmentID+"/review", map[string]any{"expectedRevisionId": next["id"], "action": "reject", "rationale": "The revised interpretation exceeds what synthetic evidence supports."}, 201)
	if rejected["status"] != "REJECTED" {
		t.Fatal(rejected)
	}
	var auditCount int
	if err := f.h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE subject_id=$1 AND action='assessment.revision.recorded'`, assessmentID).Scan(&auditCount); err != nil || auditCount != 7 {
		t.Fatal("review audit not atomic", auditCount, err)
	}
	// Expiry blocks reads; authorized cleanup removes bytes but preserves metadata.
	expiring, err := svc.Artifacts.Reserve(bound, artifacts.Input{RequestKey: "expires", SourceID: f.source, SHA256: artifacts.Hash([]byte("{}")), Bytes: 2, MediaType: "application/json", Kind: "imagery_subset", RetainUntil: time.Now().UTC().Add(500 * time.Millisecond), Provenance: "Expiry fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Artifacts.Upload(bound, expiring.ID, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	call(f.manager, "DELETE", "/api/v1/artifacts/"+expiring.ID, nil, 409)
	time.Sleep(time.Until(expiring.RetainUntil.Add(10 * time.Millisecond)))
	call(f.manager, "GET", "/api/v1/artifacts/"+expiring.ID+"/content", nil, 409)
	call(f.manager, "DELETE", "/api/v1/artifacts/"+expiring.ID, nil, 204)
	if b := call(f.manager, "GET", "/api/v1/artifacts/"+expiring.ID, nil, 200); b["state"] != "DELETED" {
		t.Fatal(b)
	}
	if _, err = store.Get(ctx, expiring.ObjectKey, 2); err == nil {
		t.Fatal("expired object bytes retained after cleanup")
	}
	restricted := scopeToken(t, f.h, "p2-other-source", f.resource, []string{"operational.read", "sources.read", "assessments.read"}, []string{"unrelated-source"})
	for _, path := range []string{"/api/v1/artifacts/" + firstArtifact, "/api/v1/artifacts/" + firstArtifact + "/content", "/api/v1/imagery/jobs/" + jobID, "/api/v1/assessments/" + assessmentID, "/api/v1/assessments/" + assessmentID + "/revisions/" + revisionID} {
		call(restricted, "GET", path, nil, 404)
	}
	if b := call(restricted, "GET", "/api/v1/imagery/scenes", nil, 200); b["total"] != float64(0) {
		t.Fatal("source visibility leak")
	}
	otherResource := access.Resource{WorkspaceID: "p2-isolated-workspace", EnvironmentID: "world", Partition: access.PublicEvidence}
	otherToken := scopeToken(t, f.h, "p2-other-workspace-reader", otherResource, []string{"operational.read", "sources.read", "assessments.read"}, []string{f.source})
	for _, path := range []string{"/api/v1/artifacts/" + firstArtifact, "/api/v1/imagery/jobs/" + jobID, "/api/v1/assessments/" + assessmentID + "/revisions/" + revisionID} {
		call(otherToken, "GET", path, nil, 404)
	}
	exerciseToken := scopeToken(t, f.h, "p2-exercise-reader", access.LegacyResource(), []string{"operational.read", "sources.read", "assessments.read"}, nil)
	call(exerciseToken, "GET", "/api/v1/artifacts/"+firstArtifact, nil, 403)
	if _, e = f.h.pool.Exec(ctx, `UPDATE assessment_revisions SET status='PUBLISHED' WHERE id=$1`, revisionID); e == nil {
		t.Fatal("review history mutable")
	}
	for _, table := range []string{"tracks", "assets", "observations"} {
		var n int
		if e = f.h.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE scope_id=$1`, f.resource.ID()).Scan(&n); e != nil || n != 0 {
			t.Fatalf("imagery workflow mutated operational %s %d %v", table, n, e)
		}
	}

	// Revocation during object I/O prevents finalization. Recovery requires a new
	// authorized identity and an expired lease, preserving the interrupted attempt.
	request["requestKey"] = "revoke-active-worker"
	revokedJob := call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)["id"].(string)
	revokeEntered, revokeRelease := make(chan struct{}, 1), make(chan struct{})
	revocationStore := p2Store{Store: store, get: func(c context.Context, k string, n int64) ([]byte, error) {
		select {
		case revokeEntered <- struct{}{}:
		default:
		}
		select {
		case <-revokeRelease:
			return store.Get(c, k, n)
		case <-c.Done():
			return nil, c.Err()
		}
	}}
	revokedDone := make(chan error, 1)
	go func() {
		_, err := imagery.NewService(f.h.pool, artifacts.NewService(f.h.pool, revocationStore)).WorkOne(bound, build)
		revokedDone <- err
	}()
	select {
	case <-revokeEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("revocation worker did not start")
	}
	if err := identities.RevokeCredential(admin, session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	close(revokeRelease)
	select {
	case err := <-revokedDone:
		if err == nil {
			t.Fatal("revoked worker finalized job")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked worker did not stop")
	}
	if b := call(f.manager, "GET", "/api/v1/imagery/jobs/"+revokedJob, nil, 200); b["status"] != "RUNNING" || b["result"] != nil {
		t.Fatal("revoked job published", b)
	}
	if _, err := f.h.pool.Exec(ctx, `UPDATE imagery_jobs SET lease_until=now()-interval '1 second' WHERE id=$1`, revokedJob); err != nil {
		t.Fatal(err)
	}
	replacementToken := scopeToken(t, f.h, "replacement-p2-worker", f.resource, []string{"operational.read", "sources.read", "imagery.jobs.execute"}, []string{f.source})
	replacement, err := identities.Authenticate(ctx, replacementToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacementCtx, err := identities.BindSession(ctx, replacement, f.resource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WorkOne(replacementCtx, build); err != nil {
		t.Fatal(err)
	}
	if b := call(f.manager, "GET", "/api/v1/imagery/jobs/"+revokedJob, nil, 200); b["status"] != "SUCCEEDED" || b["attempt"] != float64(2) {
		t.Fatal("authorized recovery lost", b)
	}
	// Failed retries share the active-source quota with new submissions.
	for i := 0; i < 32; i++ {
		request["requestKey"] = fmt.Sprintf("quota-%d", i)
		call(f.manager, "POST", "/api/v1/imagery/jobs", request, 201)
	}
	request["requestKey"] = "quota-overflow"
	call(f.manager, "POST", "/api/v1/imagery/jobs", request, 409)
	call(f.manager, "POST", "/api/v1/imagery/jobs/"+rightsID+"/retry", nil, 409)
	if strings.Contains(string(p2JSON(result)), "objectKey") {
		t.Fatal("private object path leaked")
	}
}
func p2JSON(v any) []byte { raw, _ := json.Marshal(v); return raw }

// p2Store injects an I/O failure or pause around a real S3 fixture.
type p2Store struct {
	artifacts.Store
	get func(context.Context, string, int64) ([]byte, error)
}

func (s p2Store) Get(c context.Context, k string, n int64) ([]byte, error) { return s.get(c, k, n) }
