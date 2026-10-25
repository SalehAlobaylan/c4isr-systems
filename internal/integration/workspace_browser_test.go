package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

// This gate serves the production UI and assembled Go app in one origin. No
// domain API or WebSocket is mocked, and the database is a disposable PostGIS.
func TestWorkspaceBrowserCompatibilityGate(t *testing.T) {
	if os.Getenv("C4ISR_TEST_BROWSER") != "1" {
		t.Skip("set C4ISR_TEST_BROWSER=1 after building the UI and installing Chromium")
	}
	locations := publicingest.Registry{}
	fetcher := &fixtureFetcher{}
	artifactStore := newS3Fixture(t)
	bin, binary := buildStudioBrowserRuntime(t)
	h := newConfiguredHarness(t, func(cfg *config.Config) { cfg.RuntimeBuilds = []string{artifacts.Hash(binary)} }, app.WithPublicIngestion(locations, fetcher), app.WithArtifactStore(artifactStore))
	studioFixture := seedStudioBrowser(t, h, artifactStore, bin, binary)
	seedAssetBrowser(t, h, artifactStore)
	ctx := context.Background()
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	public := access.Resource{WorkspaceID: "public-context", EnvironmentID: "world", Partition: access.PublicEvidence}
	if err := store.AddGrant(admin, "human:operator-01", access.Grant{Resource: public, Permissions: httpx.PermissionsForRole("supervisor")}); err != nil {
		t.Fatal(err)
	}
	publicHeaders := map[string]string{"X-EMAD-Workspace-ID": public.WorkspaceID, "X-EMAD-Environment-ID": public.EnvironmentID, "X-EMAD-Partition": string(public.Partition)}
	status, source := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources", map[string]any{"name": "Public browser reference", "type": "external"}, publicHeaders)
	if status != 201 {
		t.Fatalf("seed public source: %d %v", status, source)
	}
	status, observation := scopeRequest(t, h, integrationToken, "POST", "/api/v1/observations", map[string]any{
		"sourceId": source["id"], "type": "track.observation", "observedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"position": map[string]float64{"lat": 24.7, "lng": 46.7}, "trackHint": "PUBLIC_BROWSER_TRACK",
		"payload": map[string]string{"note": "PUBLIC_BROWSER_EVIDENCE"},
	}, publicHeaders)
	if status != 201 {
		t.Fatalf("seed public evidence: %d %v", status, observation)
	}
	// Exercise the new public UI with durable reports and real reference
	// artifacts, rather than intercepting any browser API responses.
	for _, name := range []string{"places", "elevation"} {
		raw, err := os.ReadFile("../../testdata/reference/jeddah-makkah/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var in reference.Input
		if err = json.Unmarshal(raw, &in); err != nil {
			t.Fatal(err)
		}
		in.SourceID = source["id"].(string)
		if status, body := scopeRequest(t, h, integrationToken, "POST", "/api/v1/reference-datasets", in, publicHeaders); status != 201 {
			t.Fatalf("browser reference seed: %d %v", status, body)
		}
	}
	worker := scopeToken(t, h, "browser-public-connector", public, []string{"operational.read", "sources.read", "connectors.execute"}, []string{source["id"].(string)})
	registry, err := publicingest.NewRegistry([]publicingest.Location{{ID: "browser-public", SourceID: source["id"].(string), Resource: public, URL: "https://provider.example.invalid/feed", Adapter: "emad-json-page-v1", MaxRequestsPerDay: 2}})
	if err != nil {
		t.Fatal(err)
	}
	for id, location := range registry {
		locations[id] = location
	}
	admission := publicingest.AdmissionInput{OriginalProducer: "Browser report fixture", UpstreamProvider: "Fixture provider", ConnectorID: "browser-public", ConnectorPrincipalID: "browser-public-connector", Dataset: "browser-reports", ProductVersion: "v1", CoverageDescription: "Bounded browser fixture", RevisionPolicy: "provider_version", Enabled: true, Rights: publicingest.Rights{ID: "browser-rights", LegalBasis: "Synthetic fixture owned by test", TermsURL: "https://provider.example.invalid/terms", Attribution: "Browser report fixture", RetainRaw: true}, Policy: publicingest.Policy{AOI: publicingest.Bounds{West: 39, East: 40, South: 21, North: 22}, LookbackSeconds: 86400, FreshForSeconds: 60, UnavailableAfterSeconds: 120, MinIntervalSeconds: 1, TimeoutSeconds: 2, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 4096, MaxRequestsPerDay: 2}}
	if status, body := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources/"+source["id"].(string)+"/admission", admission, publicHeaders); status != 201 {
		t.Fatalf("browser admission seed: %d %v", status, body)
	}
	at := time.Now().UTC().Add(-time.Minute)
	fetcher.set(publicingest.Page{Records: []publicingest.Record{{ProviderRecordID: "browser-quake", ProviderRevision: "1", ProviderUpdatedAt: &at, ReferenceURL: "https://provider.example.invalid/record", OriginalRecord: json.RawMessage(`{ "kind": "reported earthquake fixture", "magnitude": 3.5 }`), Observation: &publicingest.ReportedObservation{SemanticKind: "point_event", Type: "event.earthquake", Title: "Reported browser earthquake", ObservedAt: at, Position: &geo.Point{Lat: 21.5, Lng: 39.2}, GeometryRole: "reported_location", Properties: map[string]any{"magnitude": 3.5}}}}}, nil)
	var beforeReport time.Time
	if err := h.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&beforeReport); err != nil {
		t.Fatal(err)
	}
	if status, body := scopeRequest(t, h, worker, "POST", "/api/v1/sources/"+source["id"].(string)+"/connector-runs", nil, nil); status != 201 || body["newEvidence"] != float64(1) {
		t.Fatalf("browser event seed: %d %v", status, body)
	}
	imageryFixture := seedImageryBrowser(t, h, artifactStore, locations, fetcher, public, publicHeaders)
	contextFixture := seedContextBrowser(t, h, locations, fetcher, public, publicHeaders)
	movingFixture := seedMovingBrowser(t, h, locations, fetcher, public, publicHeaders)
	viewerPermissions := httpx.PermissionsForRole("supervisor")
	viewer := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseObservation, RunID: "legacy"}
	if err := store.CreateHuman(admin, "browser-viewer", "operator-01", access.Grant{Resource: viewer, Permissions: viewerPermissions}); err != nil {
		t.Fatal(err)
	}
	keyID, viewerToken, err := store.IssueCredential(admin, "browser-viewer", viewer, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Author a fixture whose private correlation and payload must never be COP data.
	h.createSource("browser-private-source")
	if status := h.do("POST", "/api/v1/observations", map[string]any{
		"sourceId": "browser-private-source", "type": "track.observation", "trackHint": "PRIVATE_ACTOR_SENTINEL",
		"observedAt": time.Now().UTC().Format(time.RFC3339Nano), "position": map[string]float64{"lat": 24.71, "lng": 46.71},
		"payload": map[string]string{"actor": "PRIVATE_ACTOR_SENTINEL"},
	}, nil); status != 201 {
		t.Fatalf("private fixture: %d", status)
	}
	dist, _ := filepath.Abs("../../apps/operator-ui/dist")
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		t.Fatal("build apps/operator-ui before running the browser gate: ", err)
	}
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/__fixture/revoke-viewer":
			if r.Method != http.MethodPost {
				w.WriteHeader(405)
				return
			}
			if err := store.RevokeCredential(admin, keyID); err != nil {
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(204)
		case strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health":
			h.app.Handler().ServeHTTP(w, r)
		case r.URL.Path == "/config.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte("window.__C4ISR_CONFIG__=Object.assign({apiBaseUrl:'/api/v1'},window.__C4ISR_CONFIG__);"))
		default:
			file := filepath.Join(dist, filepath.Clean("/"+r.URL.Path))
			if r.URL.Path == "/" {
				file = filepath.Join(dist, "index.html")
			}
			if _, err := os.Stat(file); err != nil {
				file = filepath.Join(dist, "index.html")
			}
			http.ServeFile(w, r, file)
		}
	}))
	defer web.Close()
	command := exec.Command("pnpm", "exec", "playwright", "test", "--config", "playwright.integration.config.ts")
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd", "/c", "pnpm", "exec", "playwright", "test", "--config", "playwright.integration.config.ts")
	}
	command.Dir = filepath.Join(dist, "..")
	fixture, _ := json.Marshal(map[string]any{"url": web.URL, "token": integrationToken, "viewerToken": viewerToken, "public": public, "beforeReport": beforeReport, "imagery": imageryFixture, "moving": movingFixture, "context": contextFixture, "studio": studioFixture})
	command.Env = append(os.Environ(), "EMAD_BROWSER_FIXTURE="+string(fixture))
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		t.Fatal("real-backend browser gate: ", err)
	}
}
