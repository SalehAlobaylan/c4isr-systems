package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/readiness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

type geographicFixtureStore struct {
	artifacts.Unavailable
	mu            sync.Mutex
	objects       map[string][]byte
	deleteFailure string
	afterPut      func()
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
	s.objects[key] = raw
	hook := s.afterPut
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
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
	if key == s.deleteFailure {
		return os.ErrPermission
	}
	delete(s.objects, key)
	return nil
}

func TestGeographicEnvironmentAdmissionBuildReleaseAndRevocation(t *testing.T) {
	objects := &geographicFixtureStore{objects: map[string][]byte{}}
	toolchain := strings.Repeat("a", 64)
	terrainToolchain := strings.Repeat("b", 64)
	feed := publicingest.Registry{}
	fetcher := &fixtureFetcher{}
	h := newConfiguredHarness(t, func(cfg *config.Config) {
		cfg.EnvironmentLibraryEnabled = true
		cfg.EnvironmentToolchain = toolchain
		cfg.EnvironmentDisplayToolchain = terrainToolchain
		cfg.ArtifactS3Endpoint = "https://test.example.invalid"
		cfg.ArtifactS3Bucket = "test-bucket"
		cfg.ArtifactS3AccessKey, cfg.ArtifactS3SecretKey = "test", "test"
	}, app.WithArtifactStore(objects), app.WithPublicIngestion(feed, fetcher))
	ctx := context.Background()
	resource := access.Resource{WorkspaceID: "environment-fixture", EnvironmentID: "reference", Partition: access.PublicEvidence}
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	if err := store.CreateHuman(admin, "environment-steward", "operator-01", access.Grant{Resource: resource, Permissions: []string{"operational.read", "sources.manage", "sources.read", "context.export", "observations.read", "tracks.read", geoenv.Manage, geoenv.Read, geoenv.Build, geoenv.Qualify, geoenv.Release, geoenv.Download}}); err != nil {
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
	rightsExpiry := time.Now().Add(48 * time.Hour).UTC()
	admit := geoenv.RetainedAdmission{OriginalProducer: "Fixture producer", UpstreamProvider: "Fixture provider", Dataset: "fixture-geography", ProductVersion: "v1", CoverageDescription: "Owned input for scoped integration verification", Area: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, Enabled: true,
		Rights: publicingest.Rights{ID: "fixture-geographic-rights", LegalBasis: "Test-owned data", TermsURL: "https://test.example.invalid/terms", Attribution: "Fixture attribution", ExpiresAt: &rightsExpiry, RetainRaw: true, AllowAnalysis: true, OutputPermissions: &publicingest.OutputPermissions{GeographicEnvironment: &yes, GeographicViewerExport: &yes, GeographicContextExport: &yes, RawDistribution: &yes}}}
	admission := call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	activationPath := "/sources/" + source + "/activation"
	sourceAssets := "/environment-assets?source=" + source
	sourceReleases := "/environment-releases?source=" + source
	assertRetention := func(token, state string, available bool) {
		t.Helper()
		activation := call(token, "GET", activationPath, nil, 200)
		retention, ok := activation["geographicRetention"].(map[string]any)
		if !ok || retention["readable"] != true || retention["assetsAvailable"] != available || activation["state"] != state || activation["nextAction"] != "" {
			t.Fatal("incorrect activation for retained source", activation)
		}
	}
	assertRetention(human, "UNAVAILABLE", false)
	worker := scopeToken(t, h, "geographic-fixture-worker", resource, []string{"operational.read", geoenv.Execute}, []string{source})
	call(human, "POST", "/sources/"+source+"/profiles/geographic-retained/execution", map[string]any{
		"profileRevisionId": admission["profileRevisionId"], "principalId": "geographic-fixture-worker", "enabled": true,
	}, 200)
	footprint := json.RawMessage(`{"type":"Polygon","coordinates":[[[39.2,21.2],[39.21,21.2],[39.21,21.21],[39.2,21.21],[39.2,21.2]]]}`)
	raw := bytes.Repeat([]byte("G"), 17<<20)
	reserved := geoenv.AssetInput{RequestKey: "source-intent", SourceID: source, AdmissionID: admission["id"].(string), SHA256: artifacts.Hash(raw), Bytes: int64(len(raw)), RetainUntil: time.Now().Add(time.Hour),
		Representation: geoenv.Representation{Layer: "surface", Format: "geotiff", Producer: "Fixture producer", ProductEdition: "v1", DataTime: "Fixture data", RetrievedAt: time.Now().UTC(), CRS: "EPSG:4326", VerticalDatum: "ellipsoid", Resolution: "1 arc-second", Semantics: "Synthetic surface for protocol verification", Uncertainty: "Declared test-only source uncertainty", Evidence: "Fixture", Footprint: footprint}}
	asset := call(human, "POST", "/environment-assets", reserved, 201)["id"].(string)
	storage := call(human, "GET", "/environment-operations/storage", nil, 200)
	if items := storage["items"].([]any); len(items) != 1 || items[0].(map[string]any)["assetId"] != asset {
		t.Fatal("expected only the pending original reservation", storage)
	}
	badUpload, err := http.NewRequest("PUT", h.server.URL+"/api/v1/environment-assets/"+asset+"/content", bytes.NewReader(bytes.Repeat([]byte("X"), len(raw))))
	if err != nil {
		t.Fatal(err)
	}
	badUpload.Header.Set("Authorization", "Bearer "+human)
	badResponse, err := http.DefaultClient.Do(badUpload)
	if err != nil {
		t.Fatal(err)
	}
	badResponse.Body.Close()
	if badResponse.StatusCode != 409 {
		t.Fatal("corrupt bytes must leave the reservation retryable", badResponse.StatusCode)
	}
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
	assertRetention(human, "RETAINED", true)
	if items := call(human, "GET", sourceAssets, nil, 200)["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != asset {
		t.Fatal("source-filtered inputs must contain the ready original", items)
	}
	if items := call(human, "GET", sourceAssets+"&after="+asset, nil, 200)["items"].([]any); len(items) != 0 {
		t.Fatal("input cursor must skip the previously returned asset", items)
	}
	area := call(human, "POST", "/areas/revisions", map[string]any{"kind": "drawn", "name": "Fixture area", "geometry": footprint}, 201)
	definition := call(human, "POST", "/geographic-environments", geoenv.DefinitionInput{Name: "Fixture", AreaRevisionID: area["revisionId"].(string), ProfileID: "reference-surface-v1", AssetIDs: []string{asset}}, 201)
	definitionID := definition["id"].(string)
	preflight := call(human, "GET", "/geographic-environments/"+definitionID+"/preflight", nil, 200)
	if preflight["eligible"] != true || preflight["rebuildAllowed"] != true {
		t.Fatal(preflight)
	}
	var profiles []struct {
		ID             string `json:"id"`
		ContractSHA256 string `json:"contractSha256"`
	}
	profileRequest := httptest.NewRequest("GET", "/api/v1/environment-profiles", nil)
	profileRequest.Header.Set("Authorization", "Bearer "+human)
	profileResponse := httptest.NewRecorder()
	h.server.Config.Handler.ServeHTTP(profileResponse, profileRequest)
	if err := json.Unmarshal(profileResponse.Body.Bytes(), &profiles); err != nil || profileResponse.Code != 200 || len(profiles) < 2 || profiles[1].ContractSHA256 != preflight["profileContractSha256"] {
		t.Fatal("preflight must bind the published profile contract", profiles, err)
	}
	polar := call(human, "POST", "/areas/revisions", map[string]any{"kind": "drawn", "name": "Polar fixture", "geometry": json.RawMessage(`{"type":"Polygon","coordinates":[[[10,86],[11,86],[11,87],[10,87],[10,86]]]}`)}, 201)
	polarCheck := call(human, "POST", "/geographic-environments", geoenv.DefinitionInput{Name: "Polar fixture", AreaRevisionID: polar["revisionId"].(string), ProfileID: "reference-surface-v1"}, 400)
	if !strings.Contains(fmt.Sprint(polarCheck), "non-polar") {
		t.Fatal("polar input must fail before preparation", polarCheck)
	}
	coverage := preflight["coverage"].([]any)
	if len(coverage) != 2 || coverage[0].(map[string]any)["status"] != "DECLARED_COVERED" || coverage[1].(map[string]any)["status"] != "NO_ELIGIBLE_INPUT" || coverage[1].(map[string]any)["required"] != false {
		t.Fatal("preflight must distinguish required surface coverage from absent optional geoid", coverage)
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
	if items := call(human, "GET", sourceReleases, nil, 200)["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != releaseID || items[0].(map[string]any)["state"] != "STAGED" {
		t.Fatal("source-filtered releases must include the staged dependent release for a reviewer", items)
	}
	action := geoenv.ReleaseAction{ManifestSHA256: staged["manifestSha256"].(string), Note: "Explicit fixture review tied to exact bytes"}
	quality := call(human, "GET", "/environment-releases/"+releaseID+"/quality-report", nil, 200)
	if quality["manifestSha256"] != staged["manifestSha256"] || quality["profileContractSha256"] != preflight["profileContractSha256"] || quality["physicalAccuracy"] != "UNKNOWN" {
		t.Fatal("quality report must bind the original release and profile", quality)
	}
	impactPath := "/environment-sources/" + source + "/impact"
	impactReport := call(human, "GET", impactPath, nil, 200)
	if impactReport["retainedInputs"] != float64(1) || impactReport["retainedBytes"] != float64(len(raw)) || len(impactReport["releases"].(map[string]any)["items"].([]any)) != 1 {
		t.Fatal("expected the authorized retained input and dependent release", impactReport)
	}
	decisionPath := "/environment-releases/" + releaseID + "/decisions"
	if _, err := h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('geographic-receiver','Fixture receiver','operator')`); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateHuman(admin, "geographic-receiver", "geographic-receiver", access.Grant{Resource: resource,
		Permissions: []string{"operational.read", "sources.read", geoenv.Read, geoenv.Download}, SourceIDs: []string{source}}); err != nil {
		t.Fatal(err)
	}
	_, consumer, err := store.IssueCredential(admin, "geographic-receiver", resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	call(consumer, "GET", "/environment-releases/"+releaseID, nil, 404)
	assertRetention(consumer, "RETAINED", true)
	call(consumer, "GET", "/environment-assets?source=out-of-scope", nil, 403)
	if items := call(consumer, "GET", sourceReleases, nil, 200)["items"].([]any); len(items) != 0 {
		t.Fatal("an environment reader must not see staged releases", items)
	}
	call(consumer, "GET", decisionPath, nil, 403)
	call(consumer, "GET", impactPath, nil, 403)
	call(consumer, "GET", "/environment-operations/storage", nil, 403)
	call(consumer, "POST", "/environment-releases/"+releaseID+"/verify", action, 403)
	call(human, "POST", "/environment-releases/"+releaseID+"/verify", geoenv.ReleaseAction{ManifestSHA256: strings.Repeat("0", 64), Note: action.Note}, 409)
	var preparedKey string
	if err := h.pool.QueryRow(ctx, `SELECT a.object_key FROM geographic_environment_releases r JOIN geographic_assets a ON (a.id,a.scope_id)=(r.bundle_id,r.scope_id) WHERE r.id=$1`, releaseID).Scan(&preparedKey); err != nil {
		t.Fatal(err)
	}
	objects.mu.Lock()
	prepared := objects.objects[preparedKey]
	if len(prepared) == 0 {
		objects.mu.Unlock()
		t.Fatal("Missing test archive")
	}
	tampered := bytes.Clone(prepared)
	tampered[len(tampered)/2] ^= 1
	objects.objects[preparedKey] = tampered
	objects.mu.Unlock()
	call(human, "POST", "/environment-releases/"+releaseID+"/verify", action, 409)
	objects.mu.Lock()
	objects.objects[preparedKey] = prepared
	objects.mu.Unlock()
	call(worker, "POST", "/environment-releases/"+releaseID+"/verify", action, 403)
	call(human, "POST", "/environment-releases/"+releaseID+"/verify", action, 200)
	call(human, "POST", "/environment-releases/"+releaseID+"/release", action, 200)
	call(human, "POST", "/environment-releases/"+releaseID+"/release", action, 200)
	call(human, "POST", "/environment-releases/"+releaseID+"/release", geoenv.ReleaseAction{ManifestSHA256: action.ManifestSHA256, Note: "Different evidence"}, 409)
	call(consumer, "GET", "/environment-releases/"+releaseID, nil, 200)
	{
		municipalSource := call(human, "POST", "/sources", map[string]any{
			"id": "riyadh-940", "name": "Riyadh municipal reports", "type": "external",
		}, 201)["id"].(string)
		municipal := admit
		municipal.Dataset, municipal.ProductVersion = "riyadh-940", "2026-q1"
		municipal.OriginalProducer, municipal.UpstreamProvider = "Riyadh Municipality", "Riyadh Municipality"
		municipal.Rights.ID = "fixture-municipal-rights"
		municipal.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		reviewAdmission := call(human, "POST", "/environment-sources/"+municipalSource+"/admissions", municipal, 201)
		query := url.Values{"sourceId": {municipalSource}, "admissionId": {reviewAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}}
		path := "/api/v1/environment-releases/" + releaseID + "/municipal-workload?"
		review := func(token string, raw []byte, want int) map[string]any {
			t.Helper()
			r, err := http.NewRequest("POST", h.server.URL+path+query.Encode(), bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Content-Type", "application/octet-stream")
			res, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			var output map[string]any
			if err := json.NewDecoder(res.Body).Decode(&output); err != nil || res.StatusCode != want {
				t.Fatalf("%s returned HTTP %d, want %d: %v (%v)", path, res.StatusCode, want, output, err)
			}
			return output
		}
		if inputPath := os.Getenv("EMAD_TEST_MUNICIPAL_Q1"); inputPath != "" {
			raw, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			out := review(human, raw, 200)
			workload := out["workload"].(map[string]any)
			if out["releaseId"] != releaseID || workload["rowCount"] != float64(6820) || workload["rowsWithoutPlaceLabel"] != float64(1) {
				t.Fatalf("unexpected municipal review: %v", out)
			}
		}
		review(human, []byte("[]"), 400)
		query.Set("manifestSha256", strings.Repeat("0", 64))
		review(human, []byte("[]"), 409)
		query.Set("manifestSha256", action.ManifestSHA256)
		municipal.ExpectedRevisionID = reviewAdmission["profileRevisionId"].(string)
		municipal.Rights.ID = "fixture-municipal-rights-revoked"
		municipal.Rights.OutputPermissions.InternalAnalysis = &no
		call(human, "POST", "/environment-sources/"+municipalSource+"/admissions", municipal, 201)
		review(human, []byte("[]"), 403)

		portSource := call(human, "POST", "/sources", map[string]any{
			"id": "portwatch", "name": "PortWatch daily activity", "type": "external",
		}, 201)["id"].(string)
		port := admit
		port.Dataset, port.ProductVersion = "imf-portwatch", "daily-ports-v1"
		port.OriginalProducer, port.UpstreamProvider = "IMF PortWatch", "IMF PortWatch"
		port.Rights.ID = "fixture-portwatch-rights"
		port.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		portAdmission := call(human, "POST", "/environment-sources/"+portSource+"/admissions", port, 201)
		portRaw := []byte(`{"features":[{"attributes":{"portid":"port24","year":2024,"month":1,"day":15,"portcalls":4}}]}`)
		if inputPath := os.Getenv("EMAD_TEST_PORTWATCH_JUBAIL"); inputPath != "" {
			portRaw, err = os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
		}
		path = "/api/v1/environment-releases/" + releaseID + "/port-activity?"
		query.Set("sourceId", portSource)
		query.Set("admissionId", portAdmission["id"].(string))
		query.Set("inputSha256", artifacts.Hash(portRaw))
		query.Set("portId", "port24")
		query.Set("baselineFrom", "2023-01-01")
		query.Set("from", "2024-01-01")
		query.Set("toExclusive", "2025-01-01")
		out := review(human, portRaw, 200)
		activity := out["activity"].(map[string]any)
		daily := activity["days"].([]any)
		if len(daily) != 366 || daily[14].(map[string]any)["portCalls"] != float64(4) {
			t.Fatalf("unexpected PortWatch review: %v", activity)
		}
		review(consumer, portRaw, 403)
		review(human, []byte("{}"), 400)

		floodSource := call(human, "POST", "/sources", map[string]any{
			"id": "cems-flood", "name": "Copernicus public flood metadata", "type": "external",
		}, 201)["id"].(string)
		flood := admit
		flood.Dataset, flood.ProductVersion = "copernicus-ems-rapid-mapping", "emergency-response-v1"
		flood.OriginalProducer, flood.UpstreamProvider = "Copernicus EMS", "Copernicus EMS"
		flood.Rights.ID = "fixture-cems-rights"
		flood.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		floodAdmission := call(human, "POST", "/environment-sources/"+floodSource+"/admissions", flood, 201)
		floodRaw := []byte(`{"count":1,"next":null,"results":[{"code":"EMSR761","category":"Flood","sensitive":false,"aois":[{"number":1,"name":"Dresden","products":[
			{"id":1061,"type":"DEL","activationCode":"EMSR761","aoiNumber":1,"version":{"uuid":"earlier","number":2,"statusCode":"F","deliveryTime":"2024-09-20T13:13:21.738919"},"images":[]},
			{"id":1084,"type":"DEL","activationCode":"EMSR761","aoiNumber":1,"version":{"uuid":"later","number":1,"statusCode":"F","deliveryTime":"2024-09-21T05:29:29.607239"},"images":[]},
			{"id":1092,"type":"DEL","activationCode":"EMSR761","aoiNumber":1,"version":{"uuid":"latest","number":1,"statusCode":"F","deliveryTime":"2024-09-23T00:16:49.913105"},"images":[]}
		]}]}]}`)
		if inputPath := os.Getenv("EMAD_TEST_CEMS_FLOOD"); inputPath != "" {
			floodRaw, err = os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
		}
		path = "/api/v1/environment-releases/" + releaseID + "/flood-mapping?"
		query = url.Values{"sourceId": {floodSource}, "admissionId": {floodAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(floodRaw)},
			"activation": {"EMSR761"}, "knownAt": {"2024-09-21T10:00:00Z"}}
		out = review(human, floodRaw, 200)
		products := out["floodMapping"].(map[string]any)["products"].([]any)
		if len(products) != 2 || products[0].(map[string]any)["versionNumber"] != float64(2) {
			t.Fatalf("expected two CEMS versions visible at the cutoff: %v", out)
		}
		query.Set("knownAt", "2024-09-20T13:13:21Z")
		out = review(human, floodRaw, 200)
		if products := out["floodMapping"].(map[string]any)["products"].([]any); len(products) != 0 {
			t.Fatalf("a later CEMS version appeared early: %v", out)
		}

		radarSource := call(human, "POST", "/sources", map[string]any{
			"id": "radar-outages", "name": "Cloudflare Radar outage annotations", "type": "external",
		}, 201)["id"].(string)
		radar := admit
		radar.Dataset, radar.ProductVersion = "cloudflare-radar-annotations", "v4"
		radar.OriginalProducer, radar.UpstreamProvider = "Cloudflare Radar", "Cloudflare Radar"
		radar.Rights.ID = "fixture-radar-rights"
		radar.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		radarAdmission := call(human, "POST", "/environment-sources/"+radarSource+"/admissions", radar, 201)
		radarRaw := []byte(`{"success":true,"result":{"annotations":[{"id":"fixture-outage","eventType":"OUTAGE",
			"locations":["SA"],"startDate":"2024-09-20T00:00:00Z","endDate":null,"outage":{"outageType":"REGIONAL"}}]}}`)
		path = "/api/v1/environment-releases/" + releaseID + "/radar-outages?"
		query = url.Values{"sourceId": {radarSource}, "admissionId": {radarAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(radarRaw)},
			"country": {"SA"}, "from": {"2024-09-20T00:00:00Z"}, "toExclusive": {"2024-09-21T00:00:00Z"}}
		out = review(human, radarRaw, 200)
		outages := out["outages"].(map[string]any)
		rows := outages["annotations"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)["endAt"] != nil || outages["availabilityAt"] != nil {
			t.Fatalf("expected unknown provider end and availability: %v", out)
		}
		review(consumer, radarRaw, 403)

		exerciseSource := call(human, "POST", "/sources", map[string]any{
			"id": "civic-access", "name": "Owned civilian road access exercise", "type": "external",
		}, 201)["id"].(string)
		exercise := admit
		exercise.Dataset, exercise.ProductVersion = "civilian-road-exercise", "v1"
		exercise.Rights.ID = "fixture-civic-access-rights"
		exercise.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		exerciseAdmission := call(human, "POST", "/environment-sources/"+exerciseSource+"/admissions", exercise, 201)
		exerciseRaw, err := os.ReadFile("../../testdata/simulation-inputs/riyadh-access-exercise.json")
		if err != nil {
			t.Fatal(err)
		}
		path = "/api/v1/environment-releases/" + releaseID + "/civilian-access-replay?"
		query = url.Values{"sourceId": {exerciseSource}, "admissionId": {exerciseAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(exerciseRaw)},
			"knownAt": {"2026-11-21T06:45:00Z"}}
		distance := func(at string, want float64) {
			t.Helper()
			query.Set("knownAt", at)
			out := review(human, exerciseRaw, 200)
			replay := out["replay"].(map[string]any)
			first := replay["destinations"].([]any)[0].(map[string]any)
			if replay["kind"] != "authored-exercise" || first["observed"].(map[string]any)["verifiedDistanceM"] != want {
				t.Fatalf("expected %.0f m plan at %s: %v", want, at, out)
			}
		}
		distance("2026-11-21T06:45:00Z", 230)
		distance("2026-11-21T07:05:00Z", 120)
		review(consumer, exerciseRaw, 403)
		var otherManifest map[string]any
		if err := json.Unmarshal(exerciseRaw, &otherManifest); err != nil {
			t.Fatal(err)
		}
		otherManifest["retainedManifestSha256"] = strings.Repeat("f", 64)
		unmatched, err := json.Marshal(otherManifest)
		if err != nil {
			t.Fatal(err)
		}
		query.Set("inputSha256", artifacts.Hash(unmatched))
		review(human, unmatched, 409)

		aerialSource := call(human, "POST", "/sources", map[string]any{
			"id": "civic-aerial", "name": "Owned civilian aerial inspection exercise", "type": "external",
		}, 201)["id"].(string)
		aerial := admit
		aerial.Dataset, aerial.ProductVersion = "civilian-aerial-review", "v1"
		aerial.Rights.ID = "fixture-civic-aerial-rights"
		aerial.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		aerialAdmission := call(human, "POST", "/environment-sources/"+aerialSource+"/admissions", aerial, 201)
		aerialRaw, err := os.ReadFile("../../testdata/simulation-inputs/civilian-aerial-exercise.json")
		if err != nil {
			t.Fatal(err)
		}
		path = "/api/v1/environment-releases/" + releaseID + "/civilian-aerial-review?"
		query = url.Values{"sourceId": {aerialSource}, "admissionId": {aerialAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(aerialRaw)},
			"knownAt": {"2026-11-18T09:25:00Z"}}
		early := review(human, aerialRaw, 200)["review"].(map[string]any)
		if early["imageStatus"] != "needs-another-observation" || early["after"] != nil {
			t.Fatalf("later aerial image appeared early: %v", early)
		}
		query.Set("knownAt", "2026-11-18T10:05:00Z")
		out = review(human, aerialRaw, 200)
		late := out["review"].(map[string]any)
		if out["manifestSha256"] != action.ManifestSHA256 || late["imageStatus"] != "paired" || late["after"].(map[string]any)["id"] != "after" {
			t.Fatalf("expected a pinned aerial pair: %v", out)
		}
		review(consumer, aerialRaw, 403)
		query.Set("inputSha256", strings.Repeat("f", 64))
		review(human, aerialRaw, 400)

		regionalSource := call(human, "POST", "/sources", map[string]any{
			"id": "civic-regional", "name": "Owned regional response exercise", "type": "external",
		}, 201)["id"].(string)
		regional := admit
		regional.Dataset, regional.ProductVersion = "regional-response-review", "v1"
		regional.Rights.ID = "fixture-civic-regional-rights"
		regional.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		regionalAdmission := call(human, "POST", "/environment-sources/"+regionalSource+"/admissions", regional, 201)
		regionalRaw, err := os.ReadFile("../../testdata/simulation-inputs/regional-response-exercise.json")
		if err != nil {
			t.Fatal(err)
		}
		path = "/api/v1/environment-releases/" + releaseID + "/regional-response-review?"
		query = url.Values{"sourceId": {regionalSource}, "admissionId": {regionalAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(regionalRaw)},
			"knownAt": {"2026-11-20T08:40:00Z"}, "staffedSlots": {"2"}}
		out = review(human, regionalRaw, 200)
		response := out["review"].(map[string]any)
		service := response["service"].(map[string]any)
		first := service["workloads"].([]any)[0].(map[string]any)
		if first["disconnected"] != float64(2) || first["queues"].(map[string]any)["analyst-override"].(map[string]any)["completed"] != float64(3) ||
			first["reference"].(map[string]any)["reachable"] != float64(5) ||
			out["manifestSha256"] != action.ManifestSHA256 {
			t.Fatalf("expected a manifest-scoped road and staffing comparison: %v", service)
		}
		review(consumer, regionalRaw, 403)
		query.Set("staffedSlots", "129")
		review(human, regionalRaw, 400)

		supportSource := call(human, "POST", "/sources", map[string]any{
			"id": "civic-support", "name": "Owned supporting evidence exercise", "type": "external",
		}, 201)["id"].(string)
		support := admit
		support.Dataset, support.ProductVersion = "supporting-source-review", "v1"
		support.Rights.ID = "fixture-civic-support-rights"
		support.Rights.OutputPermissions = &publicingest.OutputPermissions{InternalAnalysis: &yes}
		supportAdmission := call(human, "POST", "/environment-sources/"+supportSource+"/admissions", support, 201)
		supportRaw, err := os.ReadFile("../../testdata/simulation-inputs/supporting-evidence-exercise.json")
		if err != nil {
			t.Fatal(err)
		}
		path = "/api/v1/environment-releases/" + releaseID + "/supporting-source-review?"
		query = url.Values{"sourceId": {supportSource}, "admissionId": {supportAdmission["id"].(string)},
			"manifestSha256": {action.ManifestSHA256}, "inputSha256": {artifacts.Hash(supportRaw)}, "knownAt": {"2026-11-20T08:10:00Z"}}
		out = review(human, supportRaw, 200)
		replay := out["review"].(map[string]any)
		if got := replay["delivery"].([]any)[1].(map[string]any)["pending"]; got != float64(3) {
			t.Fatalf("expected three pending authored reports: %v", got)
		}
		review(consumer, supportRaw, 403)
		query.Set("inputSha256", strings.Repeat("f", 64))
		review(human, supportRaw, 400)
	}
	req, err := http.NewRequest("GET", h.server.URL+"/api/v1"+decisionPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+human)
	readDecisions := func() []geoenv.ReleaseDecision {
		t.Helper()
		response, err := http.DefaultClient.Do(req.Clone(ctx))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var decisions []geoenv.ReleaseDecision
		if err := json.NewDecoder(response.Body).Decode(&decisions); err != nil || response.StatusCode != 200 {
			t.Fatal(response.StatusCode, err)
		}
		return decisions
	}
	decisions := readDecisions()
	if len(decisions) != 3 || decisions[2].State != "RELEASED" ||
		decisions[2].ManifestSHA256 != action.ManifestSHA256 || decisions[2].Note != action.Note || decisions[2].HumanID == "" {
		t.Fatal(decisions)
	}
	detail := call(human, "GET", "/environment-releases/"+releaseID, nil, 200)
	manifestDocument := detail["manifest"].(map[string]any)
	manifestMember := manifestDocument["members"].([]any)[0].(map[string]any)
	areaRaw, err := json.Marshal(manifestDocument["area"])
	if err != nil {
		t.Fatal(err)
	}
	assessmentPath := "/environment-releases/" + releaseID + "/assessments"
	assessment := geoenv.AssessmentInput{
		ManifestSHA256: action.ManifestSHA256, MemberSHA256: manifestMember["sha256"].(string),
		Layer: manifestMember["layer"].(string), Use: "fixture-independent-surface",
		Method: "Held-out fixture controls", ControlSetSHA256: strings.Repeat("4", 64),
		StatisticM: .2, UncertaintyM: .05, Area: areaRaw, Decision: "ACCEPTED",
		Note: "Fixture reviewer accepted the held-out control comparison.",
	}
	call(consumer, "POST", assessmentPath, assessment, 403)
	call(worker, "POST", assessmentPath, assessment, 403)
	recorded := call(human, "POST", assessmentPath, assessment, 201)
	assessmentID := recorded["id"].(string)
	if recorded["sequence"] != float64(1) || recorded["releaseManifestSha256"] != action.ManifestSHA256 ||
		recorded["supersededBy"] != nil {
		t.Fatal("assessment must bind the exact release member and start a sequence", recorded)
	}
	if got := call(consumer, "GET", "/environment-assessments/"+assessmentID, nil, 200); got["evidenceSha256"] != recorded["evidenceSha256"] {
		t.Fatal("authorized reader must see the same pinned assessment", got)
	}
	if list := call(human, "GET", assessmentPath, nil, 200); len(list["items"].([]any)) != 1 {
		t.Fatal("assessment list must contain the recorded decision", list)
	}
	requirementFor := func(representations []any) map[string]any {
		return map[string]any{
			"schema": "emad.environment-requirement.v1", "requirementId": "fixture-surface",
			"operation": "read-native",
			"release": map[string]any{"manifestSha256": action.ManifestSHA256,
				"areaRevisionId": manifestDocument["areaRevisionId"]},
			"resources":       map[string]any{"maxMembers": 64, "maxBytes": 1 << 30},
			"representations": representations,
		}
	}
	accuracy := func(tolerance float64) map[string]any {
		return map[string]any{"id": "surface", "layer": manifestMember["layer"],
			"required": true, "units": "metre",
			"accuracy": map[string]any{"mode": "independent-assessment",
				"use": "fixture-independent-surface", "toleranceM": tolerance}}
	}
	requirement := requirementFor([]any{accuracy(.5)})
	suitabilityPath := "/environment-releases/" + releaseID + "/suitability"
	supported := call(human, "POST", suitabilityPath, requirement, 200)
	if supported["overall"] != "supported" || supported["evidenceSha256"] == "" ||
		len(supported["decisions"].([]any)) != 4 {
		t.Fatal("a covered accepted assessment inside tolerance must support the requirement", supported)
	}
	evidenced := supported["decisions"].([]any)[3].(map[string]any)
	if evidenced["reason"] != "accuracy-evidenced" || evidenced["assessment"] != assessmentID {
		t.Fatal("the decision must pin the assessment it used", evidenced)
	}
	exceeded := call(human, "POST", suitabilityPath, requirementFor([]any{accuracy(.1)}), 200)
	if exceeded["overall"] != "unsupported" {
		t.Fatal("a statistic above the declared tolerance must be unsupported", exceeded)
	}
	unknown := requirementFor([]any{map[string]any{"id": "surface", "layer": manifestMember["layer"],
		"required": true, "accuracy": map[string]any{"mode": "unknown-acceptable"}}})
	if declared := call(human, "POST", suitabilityPath, unknown, 200); declared["overall"] != "supported" {
		t.Fatal("a consumer accepting UNKNOWN accuracy may inspect the native surface", declared)
	}
	correction := geoenv.AssessmentCorrection{
		Method: "Corrected held-out fixture controls", ControlSetSHA256: strings.Repeat("5", 64),
		StatisticM: .9, UncertaintyM: .1, Area: areaRaw, Decision: "ACCEPTED",
		Note: "Correction after reprocessing the held-out controls.",
	}
	corrected := call(human, "POST", "/environment-assessments/"+assessmentID+"/supersede", correction, 201)
	if corrected["sequence"] != float64(2) || corrected["supersedesId"] != assessmentID {
		t.Fatal("a correction must supersede the exact prior record", corrected)
	}
	call(human, "POST", "/environment-assessments/"+assessmentID+"/supersede", correction, 409)
	correctedResult := call(human, "POST", suitabilityPath, requirement, 200)
	correctedDecision := correctedResult["decisions"].([]any)[3].(map[string]any)
	if correctedResult["overall"] != "unsupported" || correctedDecision["assessment"] != corrected["id"] ||
		correctedDecision["reason"] != "accuracy-exceeded" {
		t.Fatal("a correction must produce a new decision with the new evidence", correctedResult)
	}
	consumerRequest, err := http.NewRequest("GET", h.server.URL+"/api/v1/environment-releases/"+releaseID+"/prepared", nil)
	if err != nil {
		t.Fatal(err)
	}
	consumerRequest.Header.Set("Authorization", "Bearer "+consumer)
	consumerResponse, err := http.DefaultClient.Do(consumerRequest)
	if err != nil {
		t.Fatal(err)
	}
	consumerBytes, err := io.ReadAll(consumerResponse.Body)
	consumerResponse.Body.Close()
	if err != nil || consumerResponse.StatusCode != 200 || !bytes.Equal(consumerBytes, prepared) ||
		consumerResponse.Header.Get("ETag") != `"`+artifacts.Hash(prepared)+`"` {
		t.Fatal("receiver did not obtain the verified prepared bytes", consumerResponse.StatusCode, err)
	}
	lineagePath := "/environment-releases/" + releaseID + "/lineage"
	lineage := call(human, "GET", lineagePath, nil, 200)
	input := lineage["inputs"].([]any)[0].(map[string]any)
	member := lineage["members"].([]any)[0].(map[string]any)
	if lineage["manifestSha256"] != action.ManifestSHA256 || input["sha256"] != reserved.SHA256 || input["originalDownload"] != true || member["inputId"] != asset {
		t.Fatal("lineage must identify the frozen member, input and current rights", lineage)
	}
	impact := call(human, "GET", "/environment-releases?source="+source, nil, 200)
	if len(impact["items"].([]any)) != 1 || impact["items"].([]any)[0].(map[string]any)["id"] != releaseID {
		t.Fatal("source-filtered release list must show its dependents", impact)
	}
	call(worker, "GET", lineagePath, nil, 403)
	displayList := "/environment-releases/" + releaseID + "/displays"
	glb := fixtureTerrainGLB()
	intent := geoenv.DisplayInput{
		RequestKey: "fixture-terrain", ManifestSHA256: action.ManifestSHA256, ToolchainSHA256: toolchain,
		Recipe: "terrain-glb-v1", Bytes: int64(len(glb)), SHA256: artifacts.Hash(glb),
		Inputs: []geoenv.DisplayDependency{{Name: member["name"].(string), SHA256: member["sha256"].(string)}},
	}
	call(worker, "POST", displayList, intent, 403)
	invalid := intent
	invalid.Inputs = nil
	call(human, "POST", displayList, invalid, 409)
	display := call(human, "POST", displayList, intent, 201)
	displayID := display["id"].(string)
	displayPath := "/environment-displays/" + displayID
	stacPath := "/environment-releases/" + releaseID + "/stac"
	if display["state"] != "PENDING" || len(call(human, "GET", displayList, nil, 200)["items"].([]any)) != 0 {
		t.Fatal("reservation must not be visible before publication", display)
	}
	call(human, "GET", displayPath+"/manifest", nil, 404)
	packagePath := "/environment-releases/" + releaseID + "/viewing-packages"
	packageIntent := geoenv.ViewingPackageInput{ManifestSHA256: action.ManifestSHA256, ViewerSHA256: strings.Repeat("4", 64),
		NavigationSHA256: strings.Repeat("5", 64), Displays: []string{displayID}}
	call(human, "POST", packagePath, packageIntent, 404)
	if _, exists := call(human, "GET", stacPath, nil, 200)["assets"].(map[string]any)[displayID]; exists {
		t.Fatal("STAC must not advertise incomplete artifacts")
	}
	if prior := call(human, "POST", displayList, intent, 201); prior["id"] != displayID {
		t.Fatal("reservation retry must preserve identity", prior)
	}
	invalid = intent
	invalid.SHA256 = strings.Repeat("0", 64)
	call(human, "POST", displayList, invalid, 409)
	displayRequest := func(method, token string, data []byte, headers map[string]string, status int) ([]byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(method, h.server.URL+"/api/v1"+displayPath+"/content", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != status {
			t.Fatalf("%s display returned %d, want %d: %s (%v)", method, response.StatusCode, status, body, err)
		}
		return body, response.Header
	}
	badGLB := bytes.Clone(glb)
	badGLB[0] ^= 1
	displayRequest("PUT", human, badGLB, nil, 400)
	call(human, "GET", displayPath+"/manifest", nil, 404)
	displayRequest("PUT", human, glb, nil, 200)
	displayRequest("PUT", human, glb, nil, 200)
	if items := call(human, "GET", displayList, nil, 200)["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != displayID {
		t.Fatal("publication must expose exactly one display", items)
	}
	if ready := call(human, "GET", displayPath+"/manifest", nil, 200); ready["state"] != "READY" || ready["manifestSha256"] != action.ManifestSHA256 {
		t.Fatal("published display must identify the exact parent", ready)
	}
	if published := call(human, "GET", stacPath, nil, 200)["assets"].(map[string]any)[displayID]; published == nil || published.(map[string]any)["href"] != "/api/v1"+displayPath+"/content" {
		t.Fatal("STAC must resolve the published artifact through the scoped route", published)
	}
	displayRequest("GET", worker, nil, nil, 403)
	call(worker, "POST", packagePath, packageIntent, 403)
	issued := call(human, "POST", packagePath, packageIntent, 201)
	if issued["viewerSha256"] != packageIntent.ViewerSHA256 || issued["releaseId"] != releaseID ||
		issued["displays"].([]any)[0].(map[string]any)["id"] != displayID || len(issued["terms"].([]any)) != 1 {
		t.Fatal("portable issuance must bind the exact release, display and terms", issued)
	}
	eventSource := call(human, "POST", "/sources", map[string]any{"id": "reported-fixture", "name": "Owned reported context fixture", "type": "external"}, 201)["id"].(string)
	connector := scopeToken(t, h, "geographic-context-feed", resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{eventSource})
	locations, err := publicingest.NewRegistry([]publicingest.Location{{ID: "geographic-context-location", SourceID: eventSource, Resource: resource,
		URL: "https://test.example.invalid/feed", Adapter: "emad-json-page-v1", MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range locations {
		feed[key] = value
	}
	call(human, "POST", "/sources/"+eventSource+"/admission", publicingest.AdmissionInput{
		OriginalProducer: "Fixture producer", UpstreamProvider: "Fixture provider", ConnectorID: "geographic-context-location", ConnectorPrincipalID: "geographic-context-feed",
		Dataset: "geographic-context-events", ProductVersion: "v1", CoverageDescription: "Owned event revisions for geographic handoff integration", RevisionPolicy: "provider_version", Enabled: true,
		Rights: admit.Rights, Policy: publicingest.Policy{AOI: admit.Area, LookbackSeconds: 86400, FreshForSeconds: 60, UnavailableAfterSeconds: 120,
			MinIntervalSeconds: 1, TimeoutSeconds: 2, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 4096, MaxRequestsPerDay: 20},
	}, 201)
	observed := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
	publish := func(revision, title string, withdrawn bool) string {
		t.Helper()
		(&publicFixture{h: h, source: eventSource}).ready(t)
		at := time.Now().UTC()
		position := &geo.Point{Lat: 21.205, Lng: 39.205}
		if withdrawn {
			position = nil
		}
		fetcher.set(publicingest.Page{Records: []publicingest.Record{{ProviderRecordID: "fixture-earthquake", ProviderRevision: revision,
			ReferenceURL: "https://test.example.invalid/event", ProviderUpdatedAt: &at, OriginalRecord: json.RawMessage(`{"fixture":true}`),
			Observation: &publicingest.ReportedObservation{SemanticKind: "point_event", Type: "event.earthquake", Title: title, ObservedAt: observed,
				Position: position, GeometryRole: "reported_location", Withdrawn: withdrawn}}}}, nil)
		if run := call(connector, "POST", "/sources/"+eventSource+"/connector-runs", nil, 201); run["status"] != "SUCCEEDED" {
			t.Fatal(run)
		}
		if withdrawn {
			return ""
		}
		return call(human, "GET", "/public-events?source_id="+eventSource, nil, 200)["items"].([]any)[0].(map[string]any)["evidenceId"].(string)
	}
	first := publish("1", "Original fixture report", false)
	// A provider report remains a context record. The same scoped read can
	// select it for a handoff without promoting it into the event catalogue.
	floodSource := call(human, "POST", "/sources", map[string]any{"id": "flood-fixture", "name": "Owned flood report fixture", "type": "external"}, 201)["id"].(string)
	floodWorker := scopeToken(t, h, "geographic-flood-feed", resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{floodSource})
	floodRegistry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "geographic-flood-location", SourceID: floodSource, Resource: resource,
		URL: publicingest.EONETURL, Adapter: publicingest.EONETAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	feed["geographic-flood-location"] = floodRegistry["geographic-flood-location"]
	call(human, "POST", "/sources/"+floodSource+"/admission", publicingest.AdmissionInput{
		OriginalProducer: "Fixture producer", UpstreamProvider: "Fixture EONET", ConnectorID: "geographic-flood-location", ConnectorPrincipalID: "geographic-flood-feed",
		Dataset: publicingest.EONETDataset, ProductVersion: "v3", CoverageDescription: "Owned flood report for handoff test", RevisionPolicy: "content_hash",
		AnalystQuestion: "Which retained flood reports belong to the bounded area and time?", Enabled: true, Rights: admit.Rights,
		Policy: publicingest.Policy{AOI: admit.Area, LookbackSeconds: 86400, FreshForSeconds: 60, UnavailableAfterSeconds: 120,
			MinIntervalSeconds: 900, TimeoutSeconds: 2, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 4096, MaxRequestsPerDay: 20}}, 201)
	publishFlood := func(closed string) {
		t.Helper()
		(&publicFixture{h: h, source: floodSource}).ready(t)
		fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
			raw := fmt.Sprintf(`{"events":[{"id":"EONET_45","title":"Owned flood report","link":"https://eonet.gsfc.nasa.gov/api/v3/events/EONET_45","closed":%s,
				"categories":[{"id":"floods"}],"sources":[{"id":"GDACS","url":"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=45"}],
				"geometry":[{"date":%q,"type":"Point","coordinates":[39.205,21.205]}]}]}`, closed, observed.Format(time.RFC3339Nano))
			page, e := publicingest.DecodeEONET([]byte(raw), in)
			return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
		})
		if r := call(floodWorker, "POST", "/sources/"+floodSource+"/connector-runs", nil, 201); r["status"] != "SUCCEEDED" {
			t.Fatal(r)
		}
		fetcher.delegate = nil
	}
	publishFlood("null")
	movingSource := call(human, "POST", "/sources", map[string]any{"id": "moving-geographic-fixture", "name": "Owned moving report fixture", "type": "external"}, 201)["id"].(string)
	movingWorker := scopeToken(t, h, "geographic-moving-feed", resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{movingSource})
	movingRegistry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "geographic-moving-location", SourceID: movingSource, Resource: resource,
		URL: publicingest.OpenSkyURL, Adapter: publicingest.OpenSkyAdapter, MaxRequestsPerDay: 20}})
	if err != nil {
		t.Fatal(err)
	}
	feed["geographic-moving-location"] = movingRegistry["geographic-moving-location"]
	movingAdmission := publicingest.AdmissionInput{OriginalProducer: "Fixture producer", UpstreamProvider: "Fixture aircraft feed", ConnectorID: "geographic-moving-location", ConnectorPrincipalID: "geographic-moving-feed",
		Dataset: publicingest.OpenSkyDataset, ProductVersion: "v1", CoverageDescription: "Test-owned moving samples in the exact geographic area", RevisionPolicy: "content_hash", Enabled: true, Rights: admit.Rights,
		Policy: publicingest.Policy{AOI: admit.Area, LookbackSeconds: 300, FreshForSeconds: 60, UnavailableAfterSeconds: 120, MinIntervalSeconds: 60, TimeoutSeconds: 2, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 1048576, MaxRequestsPerDay: 20}}
	call(human, "POST", "/sources/"+movingSource+"/admission", movingAdmission, 201)
	motionAt := time.Now().UTC().Add(-4 * time.Minute).Truncate(time.Second)
	for i := range 2 {
		at := motionAt.Add(time.Duration(i) * 3 * time.Minute)
		(&publicFixture{h: h, source: movingSource}).ready(t)
		fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
			raw := fmt.Sprintf(`{"time":%d,"states":[["abcdef","OWNED TEST","Provider inference",%d,%d,39.205,21.205,8000,false,200,90,null,null,null,null,false,0]]}`, in.Now.Unix(), at.Unix(), at.Unix())
			page, e := publicingest.DecodeOpenSky([]byte(raw), in)
			return page, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
		})
		if r := call(movingWorker, "POST", "/sources/"+movingSource+"/connector-runs", nil, 201); r["newEvidence"] != float64(1) {
			t.Fatal(r)
		}
	}
	fetcher.delegate = nil
	trackID := call(human, "GET", "/external-tracks?source="+movingSource, nil, 200)["items"].([]any)[0].(map[string]any)["id"].(string)
	var now time.Time
	if err := h.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	later := publish("2", "Corrected fixture report", false)
	read := call(human, "POST", "/analyst-scope/read", map[string]any{
		"area": map[string]any{"revisionId": area["revisionId"]},
		"temporal": map[string]any{"mode": "historical", "timezone": "UTC", "revisionView": "known_at", "asOf": now,
			"interval": map[string]any{"from": now.Add(-time.Hour), "to": now}},
		"families": []string{}, "includeEvents": true, "zoom": 14,
	}, 200)
	readEvents := read["events"].(map[string]any)["items"].([]any)
	if len(readEvents) != 1 || readEvents[0].(map[string]any)["evidenceId"] != first {
		t.Fatalf("known-by read must exclude the later correction: %v", readEvents)
	}
	preview := call(human, "POST", "/analyst-scope/read", map[string]any{
		"area": map[string]any{"revisionId": area["revisionId"]},
		"temporal": map[string]any{"mode": "historical", "timezone": "UTC", "revisionView": "known_at", "asOf": now,
			"interval": map[string]any{"from": now.Add(-time.Hour), "to": now}},
		"families": []string{"earthquakes"}, "purpose": "geographic_preview", "zoom": 14,
	}, 200)
	if preview["id"] != "" || preview["layers"].(map[string]any)["earthquakes"].(map[string]any)["total"] != float64(1) {
		t.Fatal("bounded geographic overlay must expose its scoped layer without consuming a read handle", preview)
	}
	freezePath := "/context-packages/geographic"
	frozenInput := contextpackage.GeographicInput{RequestKey: "fixture-geographic-handoff", ReadID: read["id"].(string), MissingData: "explicit_unavailable", EvidenceIDs: []string{first},
		Geography: geoenv.ContextDependency{ReleaseID: releaseID, ManifestSHA256: action.ManifestSHA256,
			Displays: []geoenv.ContextDisplay{{ID: displayID, SHA256: intent.SHA256}}}}
	movingPath := "/external-tracks/" + trackID + "/history?"
	q := url.Values{"areaRevisionId": {area["revisionId"].(string)}, "from": {now.Add(-time.Hour).Format(time.RFC3339Nano)}, "to": {now.Format(time.RFC3339Nano)}}
	bounded := call(human, "GET", movingPath+q.Encode(), nil, 200)
	movingItems := bounded["items"].([]any)
	if len(movingItems) != 2 || bounded["gaps"].([]any)[1].(map[string]any)["reason"] != "report_gap" {
		t.Fatal(bounded)
	}
	q.Set("cutoff", motionAt.Format(time.RFC3339Nano))
	if v := call(human, "GET", movingPath+q.Encode(), nil, 200); len(v["items"].([]any)) != 0 {
		t.Fatal("cutoff exposed a report before platform publication", v)
	}
	q.Del("cutoff")
	without := scopeToken(t, h, "moving-history-other", resource, []string{"tracks.read", "observations.read"}, []string{source})
	call(without, "GET", movingPath+q.Encode(), nil, 404)
	movingInput := frozenInput
	movingInput.RequestKey = "fixture-moving-handoff"
	movingInput.MovingTrackID = trackID
	movingInput.MovingEvidenceIDs = []string{movingItems[0].(map[string]any)["evidenceId"].(string), movingItems[1].(map[string]any)["evidenceId"].(string)}
	movingHandoff := call(human, "POST", freezePath, movingInput, 201)
	movingReopen := "/context-packages/" + movingHandoff["id"].(string) + "/geographic"
	if v := call(human, "GET", movingReopen, nil, 200)["moving"].(map[string]any); !reflect.DeepEqual(v["items"], movingItems) {
		t.Fatal("frozen revisions changed", v, movingItems)
	}
	if directory := os.Getenv("C4ISR_GEOGRAPHIC_HANDOFF_EVIDENCE_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(movingHandoff)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "moving-handoff.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	movingAdmission.Rights.OutputPermissions = &publicingest.OutputPermissions{GeographicContextExport: &no}
	movingAdmission.Rights.ID = "fixture-moving-rights-v2"
	call(human, "POST", "/sources/"+movingSource+"/admission", movingAdmission, 201)
	call(human, "GET", movingReopen, nil, 403)
	call(human, "GET", movingPath+q.Encode(), nil, 200)
	floodRead := call(human, "POST", "/analyst-scope/read", map[string]any{
		"area": map[string]any{"revisionId": area["revisionId"]},
		"temporal": map[string]any{"mode": "historical", "timezone": "UTC", "revisionView": "known_at", "asOf": now,
			"interval": map[string]any{"from": now.Add(-time.Hour), "to": now}},
		"families": []string{"bulletins"}, "purpose": "geographic_handoff", "includeEvents": true, "zoom": 14}, 200)
	floodLayers := floodRead["layers"].(map[string]any)["bulletins"].(map[string]any)
	if floodLayers["total"] != float64(1) {
		t.Fatal(floodLayers)
	}
	floodID := floodLayers["features"].([]any)[0].(map[string]any)["id"].(string)
	floodInput := frozenInput
	floodInput.ReadID = floodRead["id"].(string)
	floodInput.EvidenceIDs = []string{floodID}
	floodInput.RequestKey = "fixture-flood-handoff"
	floodHandoff := call(human, "POST", freezePath, floodInput, 201)
	publishFlood(fmt.Sprintf("%q", observed.Add(time.Minute).Format(time.RFC3339Nano)))
	floodReload := call(human, "GET", "/context-packages/"+floodHandoff["id"].(string)+"/geographic", nil, 200)
	frozenFlood := floodReload["observations"].([]any)[0].(map[string]any)
	history := frozenFlood["providerReport"].(map[string]any)["report"].(map[string]any)["eventHistory"].(map[string]any)
	if frozenFlood["evidenceId"] != floodID || history["closedAt"] != nil || len(history["geometries"].([]any)) != 1 {
		t.Fatal(frozenFlood)
	}
	badHandoff := frozenInput
	badHandoff.EvidenceIDs = []string{later}
	call(human, "POST", freezePath, badHandoff, 409)
	type handoffReply struct {
		status int
		body   map[string]any
	}
	replies := make(chan handoffReply, 2)
	for range 2 {
		go func() {
			status, body := scopeRequest(t, h, human, "POST", "/api/v1"+freezePath, frozenInput, nil)
			replies <- handoffReply{status, body}
		}()
	}
	a, b := <-replies, <-replies
	if a.status != 201 || b.status != 201 || a.body["id"] != b.body["id"] || a.body["createdAt"] != b.body["createdAt"] {
		t.Fatalf("concurrent handoffs did not converge: %v %v", a, b)
	}
	frozen := a.body
	frozenPath := "/context-packages/" + frozen["id"].(string) + "/geographic"
	call(human, "POST", "/analyst-scope/reads/"+frozenInput.ReadID+"/release", nil, 200)
	if saved := call(human, "POST", freezePath, frozenInput, 201); saved["id"] != frozen["id"] || saved["createdAt"] != frozen["createdAt"] {
		t.Fatal("idempotent handoff must survive expired read membership", saved)
	}
	if loaded := call(human, "GET", frozenPath, nil, 200); loaded["availabilityCutoff"] != frozen["availabilityCutoff"] || loaded["queryFingerprint"] != frozen["queryFingerprint"] ||
		loaded["observations"].([]any)[0].(map[string]any)["providerRevision"] != "1" {
		t.Fatal("reopened handoff changed its time predicate", loaded)
	}
	call(human, "GET", "/context-packages/"+frozen["id"].(string), nil, 404)
	call(worker, "GET", frozenPath, nil, 403)
	if directory := os.Getenv("C4ISR_GEOGRAPHIC_HANDOFF_EVIDENCE_DIR"); directory != "" {
		if err := os.MkdirAll(filepath.Join(directory, "displays"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "displays", displayID), glb, 0600); err != nil {
			t.Fatal(err)
		}
		for endpoint, name := range map[string]string{frozenPath: "handoff.json", "/environment-releases/" + releaseID + "/manifest": "release.json"} {
			req, err := http.NewRequest("GET", h.server.URL+"/api/v1"+endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+human)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil || res.StatusCode != 200 {
				t.Fatal(res.StatusCode, err)
			}
			if err := os.WriteFile(filepath.Join(directory, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	publish("3", "Withdrawn fixture report", true)
	call(human, "GET", frozenPath, nil, 409)
	mesh, headers := displayRequest("GET", human, nil, map[string]string{"Range": "bytes=0-3"}, 206)
	if !bytes.Equal(mesh, glb[:4]) || headers.Get("ETag") != `"`+intent.SHA256+`"` {
		t.Fatal("terrain range must identify and return verified bytes", headers, mesh)
	}
	queuePath := "/environment-releases/" + releaseID + "/display-builds"
	buildRequest := geoenv.DisplayBuildInput{RequestKey: "terrain-job", ManifestSHA256: action.ManifestSHA256, Recipe: "terrain-glb-v1"}
	call(worker, "POST", queuePath, buildRequest, 403)
	queued := call(human, "POST", queuePath, buildRequest, 202)
	terrainJob := queued["id"].(string)
	jobPath := "/environment-builds/" + terrainJob
	if retry := call(human, "POST", queuePath, buildRequest, 202); retry["id"] != terrainJob {
		t.Fatal("display job submission must be idempotent", retry)
	}
	if did, err := service.WorkOne(bound, syntheticGeographicProcessor); err != nil || did {
		t.Fatal("worker without the terrain processor must skip display jobs", did, err)
	}
	service.DisplayToolchain = terrainToolchain
	var terrainRuns int
	service.DisplayProcessor = func(_ context.Context, dir, digest string) error {
		terrainRuns++
		retained, err := os.ReadFile(filepath.Join(dir, "prepared.zip"))
		if err != nil || !bytes.HasPrefix(retained, []byte("PK\x03\x04")) || digest != action.ManifestSHA256 {
			t.Fatal("display worker must receive the pinned prepared bundle", err)
		}
		produced := intent
		produced.ToolchainSHA256 = terrainToolchain
		data, err := json.Marshal(produced)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "display-request.json"), data, 0600); err != nil {
			return err
		}
		if terrainRuns == 1 {
			if _, err := h.pool.Exec(ctx, `UPDATE geographic_environment_builds SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, terrainJob); err != nil {
				return err
			}
		}
		return os.WriteFile(filepath.Join(dir, "terrain.glb"), glb, 0600)
	}
	for range 2 {
		if did, err := service.WorkOne(bound, syntheticGeographicProcessor); err != nil || !did {
			t.Fatal("terrain worker must recover an expired attempt", did, err)
		}
		if terrainRuns == 1 && len(call(human, "GET", displayList, nil, 200)["items"].([]any)) != 1 {
			t.Fatal("an expired attempt must not publish")
		}
	}
	built := call(human, "GET", jobPath, nil, 200)
	if built["state"] != "SUCCEEDED" || built["attempt"] != float64(2) {
		t.Fatal("terrain job did not publish on recovered lease", built)
	}
	builtDisplay := call(human, "GET", "/environment-displays/"+built["displayId"].(string)+"/manifest", nil, 200)
	if builtDisplay["sha256"] != intent.SHA256 || builtDisplay["manifestSha256"] != action.ManifestSHA256 {
		t.Fatal("terrain job result does not match the exact artifact and parent", builtDisplay)
	}
	buildRequest.RequestKey = "retry-after-display-store-write"
	interrupted := call(human, "POST", queuePath, buildRequest, 202)["id"].(string)
	beforeInterruption := len(call(human, "GET", displayList, nil, 200)["items"].([]any))
	objects.mu.Lock()
	objects.afterPut = func() {
		if _, err := h.pool.Exec(ctx, `UPDATE geographic_environment_builds SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, interrupted); err != nil {
			t.Fatal(err)
		}
	}
	objects.mu.Unlock()
	if did, err := service.WorkOne(bound, syntheticGeographicProcessor); err != nil || !did {
		t.Fatal("interrupt display publication after the verified write", did, err)
	}
	objects.mu.Lock()
	objects.afterPut = nil
	objects.mu.Unlock()
	if len(call(human, "GET", displayList, nil, 200)["items"].([]any)) != beforeInterruption {
		t.Fatal("an object written by an expired lease must remain privately reserved")
	}
	if did, err := service.WorkOne(bound, syntheticGeographicProcessor); err != nil || !did {
		t.Fatal("display lease retry must reuse the reserved artifact", did, err)
	}
	if j := call(human, "GET", "/environment-builds/"+interrupted, nil, 200); j["state"] != "SUCCEEDED" || j["attempt"] != float64(2) || j["displayId"] == nil {
		t.Fatal("retry after the object-store write must publish exactly once", j)
	}
	if len(call(human, "GET", displayList, nil, 200)["items"].([]any)) != beforeInterruption+1 {
		t.Fatal("the recovered job must publish one derivative")
	}
	buildRequest.RequestKey = "cancel-terrain"
	canceled := call(human, "POST", queuePath, buildRequest, 202)["id"].(string)
	processor := service.DisplayProcessor
	service.DisplayProcessor = func(ctx context.Context, dir, digest string) error {
		call(human, "POST", "/environment-display-builds/"+canceled+"/cancel", nil, 200)
		return processor(ctx, dir, digest)
	}
	if did, err := service.WorkOne(bound, syntheticGeographicProcessor); err != nil || !did {
		t.Fatal("test active cancellation", did, err)
	}
	if j := call(human, "GET", "/environment-builds/"+canceled, nil, 200); j["state"] != "CANCELED" || j["displayId"] != nil {
		t.Fatal("a canceled lease must not publish a display", j)
	}
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
	objects.mu.Lock()
	for key := range objects.objects {
		if strings.HasSuffix(key, "/"+reserved.SHA256) {
			objects.objects[key] = bytes.Clone(raw)
		}
	}
	objects.mu.Unlock()
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
	tag := resp.Header.Get("ETag")
	for _, check := range []struct {
		validator string
		status    int
	}{{tag, 206}, {`"different"`, 200}} {
		req := rangeRequest.Clone(context.Background())
		req.Header.Set("If-Range", check.validator)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != check.status || res.Header.Get("ETag") != tag || (len(body) == 4) != (check.status == 206) {
			t.Fatal("unexpected conditional range response", res.StatusCode, len(body), err)
		}
	}
	remoteBuild := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "http-build", Revision: 1}, 202)["id"].(string)
	var completions atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/complete") && completions.Add(1) == 1 {
			recorder := httptest.NewRecorder()
			h.server.Config.Handler.ServeHTTP(recorder, r)
			if recorder.Code == http.StatusNoContent {
				// Emulate a gateway that lost the committed upstream response.
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(recorder.Code)
			return
		}
		h.server.Config.Handler.ServeHTTP(w, r)
	}))
	defer proxy.Close()
	remote, err := geoenv.NewRemoteWorker(proxy.URL, worker, toolchain, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scratch := int64(4096)
	if err := remote.Beat(ctx, readiness.WorkerHeartbeat{Feature: "environment-library", ToolchainSHA256: &toolchain, ScratchBytes: &scratch}); err != nil {
		t.Fatal(err)
	}
	workers := call(human, "GET", "/platform/readiness", nil, 200)["workers"].([]any)
	if len(workers) != 1 || workers[0].(map[string]any)["toolchainSha256"] != toolchain {
		t.Fatal("worker toolchain must be visible in scoped readiness", workers)
	}
	call(worker, "POST", "/platform/worker-heartbeats", map[string]any{"feature": "environment-library", "scratchBytes": -1}, 400)
	if did, err := remote.WorkOne(ctx, syntheticGeographicProcessor); err != nil || !did || completions.Load() != 2 {
		t.Fatal("remote worker must recover the lost completion response", did, err, completions.Load())
	}
	if releases := call(human, "GET", "/environment-releases?build="+remoteBuild, nil, 200); len(releases["items"].([]any)) != 1 {
		t.Fatal("remote completion must publish exactly one release", releases)
	}
	call(human, "POST", "/environment-worker/claim", map[string]string{"toolchainSha256": toolchain}, 403)
	retryBuild := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "http-expired-lease", Revision: 1}, 202)["id"].(string)
	claim := call(worker, "POST", "/environment-worker/claim", map[string]string{"toolchainSha256": toolchain}, 200)
	if claim["jobId"] != retryBuild {
		t.Fatal("worker must claim the queued build")
	}
	lease := map[string]string{"X-Environment-Lease": claim["lease"].(string)}
	beatPath := "/api/v1/environment-worker/builds/" + retryBuild + "/heartbeat"
	if status, _ := scopeRequest(t, h, worker, "POST", beatPath, nil, lease); status != 204 {
		t.Fatal("active lease should renew", status)
	}
	complete := func(job, leaseID string, body []byte) int {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/v1/environment-worker/builds/"+job+"/complete", bytes.NewReader(body))
		r.ContentLength = int64(len(prepared))
		r.Header.Set("Authorization", "Bearer "+worker)
		r.Header.Set("X-Environment-Lease", leaseID)
		r.Header.Set("X-Artifact-SHA256", artifacts.Hash(prepared))
		r.Header.Set("Content-Type", "application/zip")
		w := httptest.NewRecorder()
		h.server.Config.Handler.ServeHTTP(w, r)
		return w.Code
	}
	if code := complete(retryBuild, lease["X-Environment-Lease"], prepared[:len(prepared)/2]); code != 400 {
		t.Fatal("an interrupted upload must fail before staging", code)
	}
	if releases := call(human, "GET", "/environment-releases?build="+retryBuild, nil, 200); len(releases["items"].([]any)) != 0 {
		t.Fatal("an interrupted upload must leave no publication", releases)
	}
	// The first worker cannot see another source's job, but that job still
	// consumes the same resource capacity while its lease is active.
	otherSource := call(human, "POST", "/sources", map[string]any{"id": "separate-fixture", "name": "Separate worker source", "type": "external"}, 201)["id"].(string)
	if err := store.CreateHuman(admin, "restricted-environment-steward", "operator-01", access.Grant{Resource: resource,
		Permissions: []string{"sources.manage", geoenv.Manage, geoenv.Read}, SourceIDs: []string{otherSource}}); err != nil {
		t.Fatal(err)
	}
	_, restrictedSteward, err := store.IssueCredential(admin, "restricted-environment-steward", resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	call(restrictedSteward, "GET", impactPath, nil, 403)
	otherWorker := scopeToken(t, h, "separate-environment-worker", resource, []string{"operational.read", geoenv.Execute}, []string{otherSource})
	otherSession, err := store.Authenticate(ctx, otherWorker, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	otherContext, err := store.BindSession(ctx, otherSession, resource)
	if err != nil {
		t.Fatal(err)
	}
	var visible int
	var capacity bool
	if err := platformdb.Scoped(h.pool).QueryRow(otherContext, `SELECT (SELECT count(*) FROM geographic_environment_builds WHERE state='RUNNING'),emad_environment_claim_capacity(1)`).Scan(&visible, &capacity); err != nil || visible != 0 || capacity {
		t.Fatal("a lease hidden by source policy must still consume scoped capacity", visible, capacity, err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO geographic_assets
	 (id,scope_id,source_id,admission_id,owner_id,required_sources,document,object_key,size_bytes,state,retain_until)
	 SELECT 'geoasset_quota_'||n,scope_id,source_id,admission_id,'separate-environment-worker',required_sources,document,
	 'isolated-test/quota/'||n,1073741824,'EXPIRED',retain_until FROM geographic_assets CROSS JOIN generate_series(1,16) n WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	if err := platformdb.Scoped(h.pool).QueryRow(otherContext, `SELECT (SELECT count(*) FROM geographic_assets WHERE id LIKE 'geoasset_quota_%'),emad_geographic_owner_capacity('separate-environment-worker',1)`).Scan(&visible, &capacity); err != nil || visible != 0 || capacity {
		t.Fatal("unpurged bytes on hidden sources must still consume owner capacity", visible, capacity, err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE geographic_assets SET purged_at=clock_timestamp() WHERE id='geoasset_quota_1'`); err != nil {
		t.Fatal(err)
	}
	if err := platformdb.Scoped(h.pool).QueryRow(otherContext, `SELECT emad_geographic_owner_capacity('separate-environment-worker',1073741824)`).Scan(&capacity); err != nil || !capacity {
		t.Fatal("successful purge must free capacity", capacity, err)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM geographic_assets WHERE id LIKE 'geoasset_quota_%'`); err != nil {
		t.Fatal(err)
	}
	t.Run("concurrent original reservations share quota", func(t *testing.T) {
		if _, err := h.pool.Exec(ctx, `INSERT INTO geographic_assets
		 (id,scope_id,source_id,admission_id,owner_id,required_sources,document,object_key,size_bytes,state,retain_until)
		 SELECT 'geoasset_quota_'||n,scope_id,source_id,admission_id,owner_id,required_sources,document,
		 'isolated-test/quota/'||n,1073741824,'EXPIRED',retain_until FROM geographic_assets CROSS JOIN generate_series(1,15) n WHERE id=$1`, asset); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.pool.Exec(ctx, `DELETE FROM geographic_assets WHERE id LIKE 'geoasset_quota_%' OR document->>'requestKey' LIKE 'concurrent-storage-%'`); err != nil {
				t.Error(err)
			}
		})
		var occupied int64
		if err := h.pool.QueryRow(ctx, `SELECT sum(size_bytes) FROM geographic_assets WHERE scope_id=$1 AND owner_id='environment-steward' AND purged_at IS NULL`, resource.ID()).Scan(&occupied); err != nil {
			t.Fatal(err)
		}
		remaining := int64(16<<30) - occupied
		if remaining < 2 || remaining > 1<<30 {
			t.Fatal("fixture must leave part of the final GiB available", remaining)
		}
		start, statuses := make(chan struct{}), make(chan int, 2)
		for n := range 2 {
			go func() {
				var status int
				defer func() { statuses <- status }()
				intent := reserved
				intent.RequestKey = fmt.Sprintf("concurrent-storage-%d", n)
				intent.Bytes = remaining/2 + 1
				<-start
				status, _ = scopeRequest(t, h, human, "POST", "/api/v1/environment-assets", intent, nil)
			}()
		}
		close(start)
		actual := []int{<-statuses, <-statuses}
		slices.Sort(actual)
		if !slices.Equal(actual, []int{201, 409}) {
			t.Fatal("one reservation should consume the available capacity", actual)
		}
	})
	if _, err := h.pool.Exec(ctx, `UPDATE geographic_environment_builds SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, retryBuild); err != nil {
		t.Fatal(err)
	}
	if status, _ := scopeRequest(t, h, worker, "POST", beatPath, nil, lease); status != 409 {
		t.Fatal("expired lease cannot renew", status)
	}
	metricsReq, err := http.NewRequest("GET", h.server.URL+"/api/v1/environment-operations/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	monitor := scopeToken(t, h, "environment-metrics-reader", resource, []string{"operational.read", geoenv.Monitor}, []string{source})
	metricsReq.Header.Set("Authorization", "Bearer "+monitor)
	call(monitor, "POST", "/environment-worker/claim", map[string]string{"toolchainSha256": toolchain}, 403)
	if err := remote.Beat(ctx, readiness.WorkerHeartbeat{Feature: "environment-library", ToolchainSHA256: &toolchain, ScratchBytes: &scratch}); err != nil {
		t.Fatal(err)
	}
	metricsRes, err := http.DefaultClient.Do(metricsReq)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := io.ReadAll(metricsRes.Body)
	metricsRes.Body.Close()
	if err != nil || metricsRes.StatusCode != 200 || !bytes.Contains(sample, []byte("c4isr_environment_expired_leases 1\n")) ||
		!bytes.Contains(sample, []byte("c4isr_environment_object_read_ok 1\n")) ||
		!bytes.Contains(sample, []byte("c4isr_environment_source_admissions_expiring_7d 1\n")) ||
		!bytes.Contains(sample, []byte("c4isr_environment_fresh_worker_scratch_samples 1\n")) ||
		!bytes.Contains(sample, []byte("c4isr_environment_largest_worker_scratch_bytes 4096\n")) ||
		bytes.Contains(sample, []byte(retryBuild)) || bytes.Contains(sample, []byte(toolchain)) {
		t.Fatal("scoped metrics must report expired leases without build identifiers", metricsRes.StatusCode, err)
	}
	if did, err := remote.WorkOne(ctx, syntheticGeographicProcessor); err != nil || !did {
		t.Fatal("another attempt must recover the expired lease", did, err)
	}
	if result := call(human, "GET", "/environment-builds/"+retryBuild, nil, 200); result["state"] != "SUCCEEDED" || result["attempt"] != float64(2) {
		t.Fatal("expired claim must be recovered by a second attempt", result)
	}
	if releases := call(human, "GET", "/environment-releases?build="+retryBuild, nil, 200); len(releases["items"].([]any)) != 1 {
		t.Fatal("recovered upload must publish exactly once", releases)
	}
	canceledUpload := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "http-canceled-upload", Revision: 1}, 202)["id"].(string)
	displayWorker := service
	displayWorker.DisplayOnly = true
	displayWorker.DisplayToolchain = strings.Repeat("d", 64)
	displayWorker.DisplayProcessor = func(context.Context, string, string) error { return nil }
	if did, err := displayWorker.WorkOne(bound, syntheticGeographicProcessor); err != nil || did {
		t.Fatal("display-only worker must leave ordinary preparation jobs for HTTP workers", did, err)
	}
	canceledClaim := call(worker, "POST", "/environment-worker/claim", map[string]string{"toolchainSha256": toolchain}, 200)
	call(human, "POST", "/environment-builds/"+canceledUpload+"/cancel", nil, 200)
	if canceledClaim["jobId"] != canceledUpload || complete(canceledUpload, canceledClaim["lease"].(string), prepared) != 409 {
		t.Fatal("a canceled HTTP lease must not publish")
	}
	scratchBuild := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "http-scratch-limit", Revision: 1}, 202)["id"].(string)
	if did, err := remote.WorkOne(ctx, func(context.Context, string) error { return geoenv.ErrScratchBudget }); err != nil || !did {
		t.Fatal("remote worker must record an exhausted scratch budget", did, err)
	}
	if result := call(human, "GET", "/environment-builds/"+scratchBuild, nil, 200); result["state"] != "FAILED" || result["failureCode"] != "scratch_budget_exceeded" {
		t.Fatal("scratch exhaustion must leave a bounded terminal failure", result)
	}
	revokedUpload := call(human, "POST", "/geographic-environments/"+definitionID+"/builds", geoenv.BuildInput{RequestKey: "http-revoked-upload", Revision: 1}, 202)["id"].(string)
	revokedClaim := call(worker, "POST", "/environment-worker/claim", map[string]string{"toolchainSha256": toolchain}, 200)
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
	admit.Rights.OutputPermissions.GeographicViewerExport = &no
	admit.Rights.OutputPermissions.GeographicContextExport = &no
	admit.IntendedUse = "Retain geographic preparation with tighter export permissions."
	restricted := call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 409)
	if p := call(human, "GET", impactPath, nil, 200)["currentPolicy"].(map[string]any); p["id"] != restricted["id"] || p["analystQuestion"] != admit.IntendedUse {
		t.Fatal("renewal must bind the latest approved policy and intended use", p)
	}
	lineage = call(human, "GET", lineagePath, nil, 200)
	input = lineage["inputs"].([]any)[0].(map[string]any)
	if input["admissionId"] != admission["id"] || input["currentAdmissionId"] != restricted["id"] || input["originalDownload"] != false {
		t.Fatal("policy revision must preserve frozen lineage and limit current rights", input)
	}
	call(human, "GET", "/environment-assets/"+asset+"/content", nil, 403)
	call(human, "GET", "/environment-releases/"+releaseID+"/rebuild", nil, 403)
	call(human, "POST", packagePath, packageIntent, 403)
	call(human, "GET", frozenPath, nil, 403)
	admit.ExpectedRevisionID = restricted["profileRevisionId"].(string)
	admit.Rights.ID = "fixture-revoked"
	admit.Rights.OutputPermissions.GeographicEnvironment = &no
	call(human, "POST", "/environment-sources/"+source+"/admissions", admit, 201)
	if revokedClaim["jobId"] != revokedUpload || complete(revokedUpload, revokedClaim["lease"].(string), prepared) != 409 {
		t.Fatal("current source revocation must prevent HTTP publication")
	}
	call(human, "GET", "/environment-releases/"+releaseID+"/prepared", nil, 403)
	call(human, "POST", suitabilityPath, requirement, 403)
	call(human, "GET", lineagePath, nil, 403)
	call(human, "GET", displayList, nil, 403)
	call(human, "GET", displayPath+"/manifest", nil, 403)
	call(human, "GET", stacPath, nil, 403)
	displayRequest("GET", human, nil, nil, 403)
	call(human, "POST", "/environment-releases/"+releaseID+"/release", action, 403)
	call(human, "POST", "/environment-releases/"+releaseID+"/withdraw", action, 200)
	call(consumer, "GET", "/environment-releases/"+releaseID+"/prepared", nil, 404)
	if entries := call(human, "GET", impactPath, nil, 200)["releases"].(map[string]any)["items"].([]any); len(entries) == 0 {
		t.Fatal("revoked data must remain discoverable by an authorized source steward")
	}
	if _, err := h.pool.Exec(ctx, `UPDATE geographic_assets SET retain_until=now()-interval '1 second' WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	assertRetention(human, "UNAVAILABLE", false)
	var originalKey string
	if err := h.pool.QueryRow(ctx, `SELECT object_key FROM geographic_assets WHERE id=$1`, asset).Scan(&originalKey); err != nil {
		t.Fatal(err)
	}
	objects.mu.Lock()
	objects.deleteFailure = originalKey
	objects.mu.Unlock()
	if err := service.ReconcileOne(bound); err == nil {
		t.Fatal("expected injected deletion failure")
	}
	if entries := call(human, "GET", "/environment-operations/storage", nil, 200)["items"].([]any); !slices.ContainsFunc(entries, func(item any) bool {
		entry := item.(map[string]any)
		return entry["assetId"] == asset && entry["state"] == "EXPIRED" && entry["purgeAttempts"] == float64(1) && entry["purgeRetryAt"] != nil
	}) {
		t.Fatal("failed deletion must preserve the tombstone and retry deadline", entries)
	}
	objects.mu.Lock()
	objects.deleteFailure = ""
	objects.mu.Unlock()
	if _, err := h.pool.Exec(ctx, `UPDATE geographic_assets SET purge_retry_at=now()-interval '1 second' WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileOne(bound); err != nil {
		t.Fatal(err)
	}
	var purged bool
	if err := h.pool.QueryRow(ctx, `SELECT purged_at IS NOT NULL FROM geographic_assets WHERE id=$1 AND state='EXPIRED'`, asset).Scan(&purged); err != nil || !purged {
		t.Fatal("retry must purge bytes and retain the tombstone", purged, err)
	}
	if history := readDecisions(); len(history) != 4 || history[3].State != "WITHDRAWN" {
		t.Fatal("withdrawal must retain review history", history)
	}
}

func fixtureTerrainGLB() []byte {
	doc := []byte(`{"asset":{"version":"2.0"},"scene":0,"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],"meshes":[{"primitives":[{"attributes":{"POSITION":0,"NORMAL":1},"indices":2,"material":0,"mode":4}]}],"materials":[{"pbrMetallicRoughness":{"baseColorFactor":[0.5,0.5,0.5,1],"metallicFactor":0,"roughnessFactor":1}}],"buffers":[{"byteLength":84}],"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36,"target":34962},{"buffer":0,"byteOffset":36,"byteLength":36,"target":34962},{"buffer":0,"byteOffset":72,"byteLength":12,"target":34963}],"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3","min":[0,0,0],"max":[1,0,1]},{"bufferView":1,"componentType":5126,"count":3,"type":"VEC3"},{"bufferView":2,"componentType":5125,"count":3,"type":"SCALAR"}]}`)
	doc = append(doc, bytes.Repeat([]byte(" "), (4-len(doc)%4)%4)...)
	var glb bytes.Buffer
	glb.WriteString("glTF")
	_ = binary.Write(&glb, binary.LittleEndian, []uint32{2, uint32(28 + len(doc) + 84), uint32(len(doc))})
	glb.WriteString("JSON")
	glb.Write(doc)
	_ = binary.Write(&glb, binary.LittleEndian, uint32(84))
	glb.WriteString("BIN\x00")
	_ = binary.Write(&glb, binary.LittleEndian, []float32{0, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0})
	_ = binary.Write(&glb, binary.LittleEndian, []uint32{0, 1, 2})
	return glb.Bytes()
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
		Members: []geoenv.Member{{Name: "surface.tif", Layer: "surface", Bytes: int64(len(part)), SHA256: artifacts.Hash(part), InputID: recipe.Inputs[0].ID}}}
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
