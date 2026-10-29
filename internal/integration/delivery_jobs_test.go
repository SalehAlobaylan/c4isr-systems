package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

func deliveryAuth(t *testing.T, f *publicFixture, token string) context.Context {
	t.Helper()
	store := access.NewStore(f.h.pool)
	session, e := store.Authenticate(context.Background(), token, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx, e := store.BindSession(context.Background(), session, f.resource)
	if e != nil {
		t.Fatal(e)
	}
	return ctx
}
func deliveryInput(c investigations.Case) investigations.DeliveryInput {
	return investigations.DeliveryInput{Kind: "brief", Output: investigations.OutputInput{RequestKey: "brief-intent", ExpectedRevisionID: c.Revision.ID, Purpose: "distribution", SourceUses: map[string]string{}, AssetIDs: []string{}, RendererVersion: "server-pinned", StyleVersion: "server-pinned", RetainUntil: time.Now().Add(time.Hour), ExpectedBytes: 8 << 20}}
}
func rawDelivery(t *testing.T, f *publicFixture, token, path string, want int) []byte {
	t.Helper()
	req, _ := http.NewRequest("GET", f.h.server.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(resp.Body)
	if e != nil || resp.StatusCode != want {
		t.Fatalf("raw %s: %d want %d %s %v", path, resp.StatusCode, want, string(raw), e)
	}
	return raw
}

func TestMapFirstDurableBriefRenderReviewAndOfflineBundle(t *testing.T) {
	store := newS3Fixture(t)
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	reviewer := caseReviewer(t, f)
	in := caseInput()
	in.Title = "Readable brief · تحليل"
	in.Notes = `<script>alert('not executable')</script> Authored uncertainty remains visible.`
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 201))
	shareCase(t, f, c)
	input := deliveryInput(c)
	path := "/api/v1/investigations/" + c.ID + "/delivery-jobs"
	job := caseCall(t, f, f.manager, "POST", path, input, 202)
	retry := caseCall(t, f, f.manager, "POST", path, input, 202)
	if job["id"] != retry["id"] {
		t.Fatal("lost response changed frozen job identity")
	}
	id := job["outputId"].(string)
	svc := investigations.Service{Pool: f.h.pool, Store: store}
	auth := deliveryAuth(t, f, f.manager)
	did, e := svc.WorkDelivery(auth, strings.Repeat("a", 64))
	if e != nil || !did {
		t.Fatal("render worker", did, e)
	}
	saved := caseCall(t, f, f.manager, "GET", "/api/v1/delivery-jobs/"+job["id"].(string), nil, 200)
	if saved["state"] != "SUCCEEDED" || saved["attempt"] != float64(1) {
		t.Fatal("missing durable attempt", saved)
	}
	out, html, e := svc.ReadOutput(auth, id, false)
	if e != nil || out.MediaType != "text/html" {
		t.Fatal("readable output", e, out.MediaType)
	}
	if bytes.Contains(html, []byte("<script>")) || !bytes.Contains(html, []byte("&lt;script&gt;")) {
		t.Fatal("authored markup executed or disappeared")
	}
	if dir := os.Getenv("C4ISR_TEST_DELIVERY_HTML_DIR"); dir != "" {
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "analyst-brief.html"), html, 0600); e != nil {
			t.Fatal(e)
		}
		meta, _ := json.Marshal(out)
		if e := os.WriteFile(filepath.Join(dir, "brief-output.json"), meta, 0600); e != nil {
			t.Fatal(e)
		}
	}
	caseCall(t, f, f.manager, "PUT", "/api/v1/outputs/"+id+"/content", map[string]string{"fake": "manual overwrite"}, 400)
	caseCall(t, f, f.manager, "GET", "/api/v1/outputs/"+id+"/attestation", nil, 409)
	request := investigations.ReviewInput{Action: "REVIEW_REQUESTED", ExpectedRevisionID: c.Revision.ID, ReviewerHumanID: "independent-case-human", OutputID: id, Reason: "Inspect exact readable figures, text, uncertainty and omissions"}
	event := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/review", request, 201)
	decision := investigations.ReviewInput{Action: "APPROVED", ExpectedRevisionID: c.Revision.ID, ExpectedEventID: event["id"].(string), Reason: "The exact retained rendering and its manifest were inspected"}
	approved := caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/review", decision, 201)
	bundle := rawDelivery(t, f, f.manager, "/api/v1/outputs/"+id+"/bundle", 200)
	z, e := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if e != nil || len(z.File) != 4 {
		t.Fatal("offline bundle", e)
	}
	for _, file := range z.File {
		r, _ := file.Open()
		raw, _ := io.ReadAll(r)
		_ = r.Close()
		if file.Name == "brief.html" && !bytes.Equal(raw, html) {
			t.Fatal("approval rerendered content")
		}
		if file.Name == "approval.json" {
			var a investigations.ReviewEvent
			_ = json.Unmarshal(raw, &a)
			if a.RenderSHA256 != approved["renderSha256"] || a.OutputContentDigest != out.ContentDigest {
				t.Fatal("attestation detached from frozen bytes")
			}
		}
	}
	in.RequestKey = "case-edited-after-brief"
	in.ExpectedRevisionID = c.Revision.ID
	in.Notes = "A newer content revision needs its own draft and review"
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", in, 201)
	if _, _, e = svc.ReadOutput(auth, id, false); e != nil {
		t.Fatal("historical draft was silently replaced or lost", e)
	}
	caseCall(t, f, f.manager, "GET", "/api/v1/outputs/"+id+"/attestation", nil, 409)
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE delivery_attempt_events SET document='{}' WHERE job_id=$1`, job["id"]); e == nil {
		t.Fatal("attempt history mutable")
	}
}

func TestMapFirstDeliveryCancellationDuringIOAndExpiredLeaseRecovery(t *testing.T) {
	store := &caseBlobStore{data: map[string][]byte{}}
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	input := deliveryInput(c)
	job := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", input, 202)
	store.afterPut = func() {
		caseCall(t, f, f.manager, "POST", "/api/v1/delivery-jobs/"+job["id"].(string)+"/cancel", map[string]int{"expectedAttempt": 1}, 200)
	}
	svc := investigations.Service{Pool: f.h.pool, Store: store}
	did, e := svc.WorkDelivery(deliveryAuth(t, f, f.manager), strings.Repeat("b", 64))
	if !did || e == nil {
		t.Fatal("canceled worker published bytes", did, e)
	}
	final := caseCall(t, f, f.manager, "GET", "/api/v1/delivery-jobs/"+job["id"].(string), nil, 200)
	if final["state"] != "CANCELED" {
		t.Fatal("failure overwrote cancellation", final)
	}
	caseCall(t, f, f.manager, "GET", "/api/v1/outputs/"+job["outputId"].(string)+"/content", nil, 409)
	store.afterPut = nil
	input.Output.RequestKey = "expired-worker-lease"
	second := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", input, 202)
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE delivery_jobs SET state='RUNNING',attempt=1,lease_id='abandoned-worker',lease_until=now()-interval '1 minute' WHERE id=$1`, second["id"]); e != nil {
		t.Fatal(e)
	}
	caseCall(t, f, f.manager, "POST", "/api/v1/delivery-jobs/"+second["id"].(string)+"/retry", map[string]int{"expectedAttempt": 0}, 409)
	caseCall(t, f, f.manager, "POST", "/api/v1/delivery-jobs/"+second["id"].(string)+"/retry", map[string]int{"expectedAttempt": 1}, 200)
	did, e = svc.WorkDelivery(deliveryAuth(t, f, f.manager), strings.Repeat("c", 64))
	if !did || e != nil {
		t.Fatal("frozen lease recovery failed", did, e)
	}
	final = caseCall(t, f, f.manager, "GET", "/api/v1/delivery-jobs/"+second["id"].(string), nil, 200)
	if final["attempt"] != float64(2) {
		t.Fatal("retry discarded attempt identity")
	}
	input.Output.RequestKey = "automatic-abandoned-lease"
	automatic := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", input, 202)
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE delivery_jobs SET state='RUNNING',attempt=1,lease_id='abandoned-auto',lease_until=now()-interval '1 minute' WHERE id=$1`, automatic["id"]); e != nil {
		t.Fatal(e)
	}
	if did, e := svc.WorkDelivery(deliveryAuth(t, f, f.manager), strings.Repeat("e", 64)); !did || e != nil {
		t.Fatal("automatic expired lease recovery", did, e)
	}
	var events int
	if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM delivery_attempt_events WHERE job_id=$1 AND document->>'action'='LEASE_EXPIRED'`, automatic["id"]).Scan(&events); e != nil || events != 1 {
		t.Fatal("abandoned attempt lost its audit identity", events, e)
	}
}

type deliveryFaultStore struct {
	caseBlobStore
	fail bool
}

func (s *deliveryFaultStore) Put(ctx context.Context, key string, b []byte, media string) error {
	if e := s.caseBlobStore.Put(ctx, key, b, media); e != nil {
		return e
	}
	if s.fail {
		return errors.New("injected uncertain upload response")
	}
	return nil
}
func TestMapFirstDeliveryUnknownUploadIdentityAndExpiryReconciliation(t *testing.T) {
	store := &deliveryFaultStore{caseBlobStore: caseBlobStore{data: map[string][]byte{}}, fail: true}
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	auth := deliveryAuth(t, f, f.manager)
	svc := investigations.Service{Pool: f.h.pool, Store: store}
	input := investigations.OutputInput{RequestKey: "uncertain-upload", ExpectedRevisionID: c.Revision.ID, Purpose: "internal", RendererVersion: "fixture-v1", StyleVersion: "v1", RetainUntil: time.Now().Add(400 * time.Millisecond), ExpectedBytes: 1024}
	out, e := svc.CreateOutput(auth, c.ID, input)
	if e != nil {
		t.Fatal(e)
	}
	work, stop := context.WithTimeout(auth, time.Second)
	deadline, _ := work.Deadline()
	_, e = svc.StageOutput(work, out.ID, "application/json", []byte(`{"version":1}`))
	stop()
	if e == nil {
		t.Fatal("uncertain upload was published")
	}
	if _, e = svc.StageOutput(auth, out.ID, "application/json", []byte(`{"version":2}`)); e == nil {
		t.Fatal("failed upload changed its immutable candidate identity")
	}
	time.Sleep(time.Until(input.RetainUntil) + 10*time.Millisecond)
	if e := svc.ExpireOutput(auth, out.ID); e == nil {
		t.Fatal("uncertain in-flight write was treated as settled")
	}
	time.Sleep(time.Until(deadline) + 10*time.Millisecond)
	did, e := svc.ReconcileDeliveryOutput(auth)
	if e != nil || !did {
		t.Fatal("idle cleanup failed", did, e)
	}
	var state string
	if e := f.h.pool.QueryRow(context.Background(), `SELECT state FROM composite_outputs WHERE id=$1`, out.ID).Scan(&state); e != nil || state != "TOMBSTONED" {
		t.Fatal(state, e)
	}
	store.mu.Lock()
	remaining := len(store.data)
	store.mu.Unlock()
	if remaining != 0 {
		t.Fatal("uncommitted upload escaped expiry reconciliation")
	}
}

func TestMapFirstPortableEnvironmentProductFlowAndIndependentReader(t *testing.T) {
	for _, area := range []struct {
		name        string
		west, south float64
		east, north float64
		controls    []environmentprep.Control
	}{
		{"saudi-fixture", 46, 24, 47, 25, []environmentprep.Control{{Longitude: 46.5, Latitude: 24.5, EllipsoidHeight: 100, East: 0, North: 0, Up: 100, Provenance: "Independent PROJ fixture, not a surveyed control"}, {Longitude: 46.5001, Latitude: 24.5, EllipsoidHeight: 100, East: 10.135637192357791, North: .0000036677303683, Up: 99.99999195185877, Provenance: "Independent PROJ fixture, not a surveyed control"}, {Longitude: 46.5, Latitude: 24.5001, EllipsoidHeight: 100, East: 0, North: 11.076724262114205, Up: 99.99999033398097, Provenance: "Independent PROJ fixture, not a surveyed control"}}},
		{"france-fixture", 2, 48, 3, 49, []environmentprep.Control{{Longitude: 2.5, Latitude: 48.5, EllipsoidHeight: 100, East: 0, North: 0, Up: 100, Provenance: "Independent PROJ fixture, not a surveyed control"}, {Longitude: 2.5001, Latitude: 48.5, EllipsoidHeight: 100, East: 7.390256730895686, North: .000004830523089, Up: 99.99999572690885, Provenance: "Independent PROJ fixture, not a surveyed control"}, {Longitude: 2.5, Latitude: 48.5001, EllipsoidHeight: 100, East: 0, North: 11.120178630030267, Up: 99.9999902983518, Provenance: "Independent PROJ fixture, not a surveyed control"}}},
	} {
		t.Run(area.name, func(t *testing.T) {
			store := &caseBlobStore{data: map[string][]byte{}}
			f := newPublicFixture(t, app.WithArtifactStore(store))
			grantCase(t, f)
			values := []*float64{}
			for i := 0; i < 81; i++ {
				v := 100.
				values = append(values, &v)
			}
			grid := reference.Grid{Width: 9, Height: 9, West: area.west, South: area.south, StepX: .125, StepY: .125, Registration: "point_samples", Values: values}
			artifact, _ := json.Marshal(grid)
			yes := true
			dataset := reference.Dataset{ID: "portable-dem", Input: reference.Input{SourceID: f.source, DatasetKey: "portable-dem-fixture", Version: "v1", Label: "Synthetic grid; no geographic accuracy claim", Kind: "elevation", CRS: "EPSG:4326", VerticalDatum: "WGS84_ELLIPSOID", Accuracy: &reference.Accuracy{HorizontalM: 10, VerticalM: 2, Provenance: "Synthetic source uncertainty for protocol checks only"}, AOI: publicingest.Bounds{West: area.west, South: area.south, East: area.east, North: area.north}, SHA256: reference.Hash(artifact), Rights: publicingest.Rights{RetainRaw: true, AllowAnalysis: true, AllowSimulationExport: true, Attribution: "Owned synthetic workflow fixture", OutputPermissions: &publicingest.OutputPermissions{Citation: &yes, GeographicEnvironment: &yes, MetadataRetention: &yes}}}, ApprovedAt: time.Now().UTC()}
			doc, _ := json.Marshal(dataset)
			if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO reference_datasets(id,scope_id,source_id,dataset_key,version,kind,document,artifact,spec_hash)VALUES($1,$2,$3,$4,'v1','elevation',$5,$6,'fixture')`, dataset.ID, f.resource.ID(), f.source, dataset.DatasetKey, doc, artifact); e != nil {
				t.Fatal(e)
			}
			caseCall(t, f, f.manager, "POST", "/api/v1/content-purpose", map[string]any{"kind": "reference", "entityId": dataset.ID, "purpose": "working", "reason": "Owned synthetic protocol fixture only, no production terrain claim", "receiptReference": "isolated-test-fixture", "expectedVersion": 0}, 201)
			var imageID string
			var imageRaw []byte
			if area.name == "saudi-fixture" {
				f.admission.Rights.AllowSimulationExport = true
				f.admission.Rights.OutputPermissions = &publicingest.OutputPermissions{GeographicEnvironment: &yes, MetadataRetention: &yes}
				f.admit(t)
				v := .2
				tile := imageryanalysis.Tile{Schema: "emad.multispectral-subset.v1", SceneID: "protocol-scene", SceneHash: strings.Repeat("f", 64), AcquiredAt: time.Now().Add(-time.Hour), PreparationMethod: "Owned synthetic source grid; no real acquisition claimed", ReflectanceUnit: "surface_reflectance", Grid: imageryanalysis.Grid{CRS: "EPSG:4326", Width: 2, Height: 2, West: 46.499, North: 24.501, StepLng: .0005, StepLat: .0005, Registration: "pixel_edges_north_to_south"}, Red: []*float64{&v, &v, &v, &v}, NIR: []*float64{&v, &v, &v, &v}, SCL: []int{4, 4, 4, 4}, SourceHashes: map[string]string{"B04_10m": strings.Repeat("a", 64), "B08_10m": strings.Repeat("b", 64), "SCL_20m": strings.Repeat("c", 64)}}
				imageRaw, _ = json.MarshalIndent(tile, "", " ")
				auth := deliveryAuth(t, f, f.manager)
				aSvc := artifacts.NewService(f.h.pool, store)
				accuracy := 10.
				a, e := aSvc.Reserve(auth, artifacts.Input{RequestKey: "qualified-image", SourceID: f.source, SHA256: artifacts.Hash(imageRaw), Bytes: int64(len(imageRaw)), Kind: "imagery_subset", MediaType: "application/json", HorizontalAccuracyM: &accuracy, RetainUntil: time.Now().Add(2 * time.Hour), Provenance: "Synthetic source-backed horizontal accuracy for protocol fixture"})
				if e != nil {
					t.Fatal(e)
				}
				if _, e = aSvc.Upload(auth, a.ID, imageRaw); e != nil {
					t.Fatal(e)
				}
				imageID = a.ID
				caseCall(t, f, f.manager, "POST", "/api/v1/content-purpose", map[string]any{"kind": "artifact", "entityId": imageID, "purpose": "working", "reason": "Owned synthetic imagery protocol fixture only", "receiptReference": "isolated-image-fixture", "expectedVersion": 0}, 201)
			}
			in := caseInput()
			in.Scope.Area.Geometry = deliveryJSON(t, map[string]any{"type": "Polygon", "coordinates": [][][]float64{{{area.west, area.south}, {area.east, area.south}, {area.east, area.north}, {area.west, area.north}, {area.west, area.south}}}})
			in.Evidence = []investigations.Evidence{{Kind: "reference", ID: dataset.ID}}
			if imageID != "" {
				in.Evidence = append(in.Evidence, investigations.Evidence{Kind: "artifact", ID: imageID})
			}
			c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 201))
			input := deliveryInput(c)
			input.Kind = "environment"
			input.Output.SourceUses = map[string]string{f.source: "environment"}
			if imageID != "" {
				input.Output.AssetIDs = []string{imageID}
			}
			input.Environment = &environmentprep.Input{DatasetIDs: []string{dataset.ID}, HorizontalCRS: "EPSG:4326", OutputVerticalDatum: "WGS84_ELLIPSOID", Origin: environmentprep.Origin{Longitude: (area.west + area.east) / 2, Latitude: (area.south + area.north) / 2}, Qualification: environmentprep.Qualification{HorizontalToleranceM: 10, VerticalToleranceM: 2, Method: "Independent PROJ algorithm fixtures, not ground survey", Controls: area.controls, MaxPixels: 1024, MaxFeatures: 100, MaxTriangles: 2048, MaxBytes: 2 << 20, MaxRAMBytes: 32 << 20, MaxTempBytes: 0, MaxSeconds: 10}}
			job := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", input, 202)
			svc := investigations.Service{Pool: f.h.pool, Store: store}
			auth := deliveryAuth(t, f, f.manager)
			did, e := svc.WorkDelivery(auth, strings.Repeat("d", 64))
			if e != nil || !did {
				t.Fatal("geographic preparation", did, e)
			}
			_, raw, e := svc.ReadOutput(auth, job["outputId"].(string), false)
			if e != nil {
				t.Fatal(e)
			}
			if imageID != "" {
				z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, file := range z.File {
					if file.Name == "sources/imagery-00.json" {
						r, _ := file.Open()
						original, _ := io.ReadAll(r)
						_ = r.Close()
						if !bytes.Equal(original, imageRaw) {
							t.Fatal("original image bytes changed")
						}
						found = true
					}
				}
				if !found {
					t.Fatal("qualified imagery omitted")
				}
			}
			output := os.Getenv("C4ISR_TEST_ENV_OUTPUT_DIR")
			if output == "" {
				output = t.TempDir()
			}
			if e := os.MkdirAll(output, 0700); e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(output, area.name+".zip")
			if e := os.WriteFile(file, raw, 0600); e != nil {
				t.Fatal(e)
			}
			python, _ := filepath.Abs("../../.venv-recovery/Scripts/python.exe")
			if _, e := os.Stat(python); e == nil {
				cmd := exec.Command(python, "../../scripts/data/check_portable_environment.py", file)
				b, e := cmd.CombinedOutput()
				if e != nil {
					t.Fatalf("independent offline reader: %v %s", e, b)
				}
				if e := os.WriteFile(filepath.Join(output, area.name+"-reader.json"), b, 0600); e != nil {
					t.Fatal(e)
				}
			} else {
				t.Log("Independent Python/PROJ reader not installed in this environment; Go protocol checks only")
			}
			input.Output.RequestKey = "missing-grid-qualification"
			input.Environment.OutputVerticalDatum = "UNKNOWN"
			caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", input, 400)
			var count int
			if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM delivery_jobs`).Scan(&count); e != nil || count != 1 {
				t.Fatal("failed preflight consumed a full job/reservation", count, e)
			}
		})
	}
}
func deliveryJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
