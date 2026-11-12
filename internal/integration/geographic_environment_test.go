package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

type geographicFixtureStore struct {
	artifacts.Unavailable
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *geographicFixtureStore) PutGeographic(_ context.Context, key string, source io.ReadSeeker, size int64, digest, _ string) error {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(source, size+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) != size || artifacts.Hash(raw) != digest {
		return os.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = raw
	return nil
}
func (s *geographicFixtureStore) OpenGeographic(_ context.Context, key string, size int64, r *artifacts.ByteRange) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.objects[key]
	if !ok || int64(len(raw)) != size {
		return nil, os.ErrNotExist
	}
	if r != nil {
		raw = raw[r.Start : r.End+1]
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}
func (s *geographicFixtureStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func TestGeographicEnvironmentAdmissionBuildReleaseAndRevocation(t *testing.T) {
	objects := &geographicFixtureStore{objects: map[string][]byte{}}
	toolchain := strings.Repeat("a", 64)
	h := newConfiguredHarness(t, func(cfg *config.Config) {
		cfg.EnvironmentLibraryEnabled = true
		cfg.EnvironmentToolchain = toolchain
		cfg.ArtifactS3Endpoint = "https://test.example.invalid"
		cfg.ArtifactS3Bucket = "test-bucket"
		cfg.ArtifactS3AccessKey, cfg.ArtifactS3SecretKey = "test", "test"
	}, app.WithArtifactStore(objects))
	ctx := context.Background()
	resource := access.Resource{WorkspaceID: "environment-fixture", EnvironmentID: "reference", Partition: access.PublicEvidence}
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	if err := store.CreateHuman(admin, "environment-steward", "operator-01", access.Grant{Resource: resource, Permissions: []string{"operational.read", "sources.manage", "sources.read", geoenv.Manage, geoenv.Read, geoenv.Build, geoenv.Qualify, geoenv.Release, geoenv.Download}}); err != nil {
		t.Fatal(err)
	}
	_, human, err := store.IssueCredential(admin, "environment-steward", resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	call := func(token, method, path string, body any, want int) map[string]any {
		t.Helper()
		code, out := scopeRequest(t, h, token, method, "/api/v1"+path, body, nil)
		if code != want {
			t.Fatalf("%s %s returned %d, want %d: %v", method, path, code, want, out)
		}
		return out
	}
	source := call(human, "POST", "/sources", map[string]any{"id": "reference", "name": "Owned synthetic input", "type": "external"}, 201)["id"].(string)
	yes, no := true, false
	admit := geoenv.RetainedAdmission{OriginalProducer: "Fixture producer", UpstreamProvider: "Fixture provider", Dataset: "fixture-geography", ProductVersion: "v1", CoverageDescription: "Owned input for scoped integration verification", Area: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, Enabled: true,
		Rights: publicingest.Rights{ID: "fixture-geographic-rights", LegalBasis: "Test-owned data", TermsURL: "https://test.example.invalid/terms", Attribution: "Fixture attribution", RetainRaw: true, AllowAnalysis: true, OutputPermissions: &publicingest.OutputPermissions{GeographicEnvironment: &yes, RawDistribution: &yes}}}
	admission := call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	worker := scopeToken(t, h, "geographic-fixture-worker", resource, []string{"operational.read", geoenv.Execute}, []string{source})
	call(human, "POST", "/sources/"+source+"/profiles/geographic-retained/execution", map[string]any{
		"profileRevisionId": admission["profileRevisionId"], "principalId": "geographic-fixture-worker", "enabled": true,
	}, 200)
	footprint := json.RawMessage(`{"type":"Polygon","coordinates":[[[39.2,21.2],[39.21,21.2],[39.21,21.21],[39.2,21.21],[39.2,21.2]]]}`)
	raw := bytes.Repeat([]byte("G"), 17<<20)
	reserved := geoenv.AssetInput{RequestKey: "source-intent", SourceID: source, AdmissionID: admission["id"].(string), SHA256: artifacts.Hash(raw), Bytes: int64(len(raw)), RetainUntil: time.Now().Add(time.Hour),
		Representation: geoenv.Representation{Layer: "surface", Format: "geotiff", Producer: "Fixture producer", ProductEdition: "v1", DataTime: "Fixture data", RetrievedAt: time.Now().UTC(), CRS: "EPSG:4326", VerticalDatum: "ellipsoid", Resolution: "1 arc-second", Semantics: "Synthetic surface for protocol verification", Uncertainty: "Declared test-only source uncertainty", Evidence: "Fixture", Footprint: footprint}}
	asset := call(human, "POST", "/environment-assets", reserved, 201)["id"].(string)
	upload, err := http.NewRequest("PUT", h.server.URL+"/api/v1/environment-assets/"+asset+"/content", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	upload.Header.Set("Authorization", "Bearer "+human)
	resp, err := http.DefaultClient.Do(upload)
	if err != nil {
		t.Fatal(err)
	}
	responseBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(responseBytes, []byte(`"state":"READY"`)) {
		t.Fatal(resp.StatusCode, string(responseBytes))
	}
	area := call(human, "POST", "/areas/revisions", map[string]any{"kind": "drawn", "name": "Fixture area", "geometry": footprint}, 201)
	definition := call(human, "POST", "/geographic-environments", geoenv.DefinitionInput{Name: "Fixture", AreaRevisionID: area["revisionId"].(string), ProfileID: "reference-surface-v1", AssetIDs: []string{asset}}, 201)
	definitionID := definition["id"].(string)
	preflight := call(human, "GET", "/geographic-environments/"+definitionID+"/preflight", nil, 200)
	if preflight["eligible"] != true || preflight["rebuildAllowed"] != true {
		t.Fatal(preflight)
	}
	build := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "first-build", Revision: 1}, 202)
	buildID := build["id"].(string)
	if repeat := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "first-build", Revision: 1}, 202); repeat["id"] != buildID {
		t.Fatal("build idempotency lost", repeat)
	}
	session, err := store.Authenticate(ctx, worker, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	bound, err := store.BindSession(ctx, session, resource)
	if err != nil {
		t.Fatal(err)
	}
	service := geoenv.Service{Pool: h.pool, Store: objects, Toolchain: toolchain, Parallel: 1}
	did, err := service.WorkOne(bound, syntheticGeographicProcessor)
	if err != nil || !did {
		t.Fatal("run durable worker", did, err)
	}
	status := call(human, "GET", "/environment-builds/"+buildID, nil, 200)
	if status["state"] != "SUCCEEDED" {
		t.Fatal(status)
	}
	list := call(human, "GET", "/environment-releases?build="+buildID, nil, 200)
	staged := list["items"].([]any)[0].(map[string]any)
	releaseID := staged["id"].(string)
	action := geoenv.ReleaseAction{ManifestSHA256: staged["manifestSha256"].(string), Note: "Explicit fixture review tied to exact bytes"}
	call(worker, "POST", "/environment-releases/"+releaseID+"/verify", action, 403)
	call(human, "POST", "/environment-releases/"+releaseID+"/verify", action, 200)
	call(human, "POST", "/environment-releases/"+releaseID+"/release", action, 200)
	getRebuild := func() ([]byte, error) {
		t.Helper()
		req, err := http.NewRequest("GET", h.server.URL+"/api/v1/environment-releases/"+releaseID+"/rebuild", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+human)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("rebuild status %d", response.StatusCode)
		}
		return io.ReadAll(response.Body)
	}
	archive, err := getRebuild()
	if err != nil {
		t.Fatal(err)
	}
	zipped, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(zipped.File) != 3 || zipped.File[2].Name != "inputs/"+reserved.SHA256 {
		t.Fatal("rebuild must contain recipe, manifest and retained original", err)
	}
	objects.mu.Lock()
	for key, original := range objects.objects {
		if strings.HasSuffix(key, "/"+reserved.SHA256) {
			corrupt := bytes.Clone(original)
			corrupt[0] ^= 1
			objects.objects[key] = corrupt
		}
	}
	objects.mu.Unlock()
	if _, err := getRebuild(); err == nil {
		t.Fatal("corrupted rebuild ended as a successful HTTP transfer")
	}
	rangeRequest, err := http.NewRequest("GET", h.server.URL+"/api/v1/environment-releases/"+releaseID+"/prepared", nil)
	if err != nil {
		t.Fatal(err)
	}
	rangeRequest.Header.Set("Authorization", "Bearer "+human)
	rangeRequest.Header.Set("Range", "bytes=0-3")
	resp, err = http.DefaultClient.Do(rangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	signature, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 206 || !bytes.Equal(signature, []byte("PK\x03\x04")) {
		t.Fatal("authorized single range", resp.StatusCode, signature)
	}
	call(human, "PUT", "/geographic-environments/"+definitionID, geoenv.DefinitionInput{Name: "Fixture revised", AreaRevisionID: area["revisionId"].(string), ProfileID: "reference-context-v1", AssetIDs: []string{asset}, ExpectedRevision: 1}, 200)
	oldProfile := call(human, "GET", "/geographic-environments?profile=reference-surface-v1", nil, 200)
	if len(oldProfile["items"].([]any)) != 0 {
		t.Fatal("profile filter returned an obsolete definition revision", oldProfile)
	}
	if release := call(human, "GET", "/environment-releases/"+releaseID, nil, 200); release["name"] != "Fixture" {
		t.Fatal("release name must come from its frozen definition revision", release)
	}
	admit.ExpectedRevisionID = admission["profileRevisionId"].(string)
	admit.Rights.ID = "fixture-raw-redistribution-revoked"
	admit.Rights.OutputPermissions.RawDistribution = &no
	restricted := call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	call(human, "GET", "/environment-assets/"+asset+"/content", nil, 403)
	call(human, "GET", "/environment-releases/"+releaseID+"/rebuild", nil, 403)
	admit.ExpectedRevisionID = restricted["profileRevisionId"].(string)
	admit.Rights.ID = "fixture-revoked"
	admit.Rights.OutputPermissions.GeographicEnvironment = &no
	call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	call(human, "GET", "/environment-releases/"+releaseID+"/prepared", nil, 403)
}

func syntheticGeographicProcessor(_ context.Context, dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "recipe.json"))
	if err != nil {
		return err
	}
	var recipe struct {
		geoenv.Recipe
		InputSHA256 string `json:"inputSha256"`
	}
	if err = json.Unmarshal(raw, &recipe); err != nil {
		return err
	}
	part := []byte("Synthetic member exercises release protocol only.")
	manifest := geoenv.Manifest{Schema: "emad.environment-release.v1", InputSHA256: recipe.InputSHA256, Toolchain: recipe.Toolchain, ProfileID: recipe.Profile.ID, AreaRevisionID: recipe.Area.RevisionID, Area: recipe.Area.Geometry, Inputs: recipe.Inputs,
		Members: []geoenv.Member{{Name: "surface.tif", Layer: "surface", Bytes: int64(len(part)), SHA256: artifacts.Hash(part)}}}
	manifest.Quality.PhysicalAccuracy = "UNKNOWN"
	origin := environmentprep.Origin{Longitude: 39.205, Latitude: 21.205}
	manifest.Frame = geoenv.Frame{Axes: []string{"east", "north", "up"}, Origin: []float64{origin.Longitude, origin.Latitude, 0}, HeightDatum: "WGS84 ellipsoid", AxisOrder: "longitude, latitude"}
	for _, point := range [][3]float64{{39.205, 21.205, 1}, {39.2, 21.2, 0}, {39.21, 21.21, 100}} {
		enu := environmentprep.ENU(origin, point[0], point[1], point[2])
		manifest.Frame.Controls = append(manifest.Frame.Controls, geoenv.FrameControl{Longitude: point[0], Latitude: point[1], EllipsoidHeightM: point[2], ENUM: enu[:]})
	}
	raw, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "prepared.zip"))
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	for _, m := range []struct {
		name string
		data []byte
	}{{"manifest.json", raw}, {"surface.tif", part}} {
		w, e := z.Create(m.name)
		if e != nil {
			return e
		}
		if _, e = w.Write(m.data); e != nil {
			return e
		}
	}
	return z.Close()
}
