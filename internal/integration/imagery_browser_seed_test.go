package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"os"
	"testing"
	"time"
)

func seedImageryBrowser(t *testing.T, h *harness, store artifacts.Store, locations publicingest.Registry, fetcher *fixtureFetcher, resource access.Resource, headers map[string]string) map[string]any {
	t.Helper()
	ctx := context.Background()
	identities := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	status, created := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources", map[string]any{"name": "Recorded Copernicus catalogue", "type": "external"}, headers)
	if status != 201 {
		t.Fatal(created)
	}
	source := created["id"].(string)
	connector := scopeToken(t, h, "browser-cdse-connector", resource, []string{"operational.read", "sources.read", "connectors.execute"}, []string{source})
	registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: "browser-cdse", SourceID: source, Resource: resource, URL: publicingest.STACURL, Adapter: publicingest.STACAdapter, MaxRequestsPerDay: 20}})
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range registry {
		locations[k] = v
	}
	in := publicingest.AdmissionInput{OriginalProducer: "Copernicus Sentinel-2", UpstreamProvider: "Recorded CDSE STAC", ConnectorID: "browser-cdse", ConnectorPrincipalID: "browser-cdse-connector", Dataset: publicingest.STACDataset, ProductVersion: "stac-1.1-l2a", CoverageDescription: "Recorded regional metadata; synthetic pixel values explicitly labeled", RevisionPolicy: "content_hash", Enabled: true, Rights: publicingest.Rights{ID: "browser-cdse-rights", LegalBasis: "Copernicus public catalogue metadata; spectral pixel fixture owned by test", TermsURL: "https://dataspace.copernicus.eu/terms-and-conditions", Attribution: "Copernicus Sentinel-2 catalogue; synthetic spectral fixture", RetainRaw: true, AllowAnalysis: true, AllowSimulationExport: true}, Policy: publicingest.Policy{AOI: publicingest.Bounds{West: 39.15, South: 21.45, East: 39.2, North: 21.5}, LookbackSeconds: 31 * 86400, FreshForSeconds: 60, UnavailableAfterSeconds: 120, MinIntervalSeconds: 5, TimeoutSeconds: 5, MaxPages: 1, MaxRecordsPerPage: 2, MaxResponseBytes: 1024 * 1024, MaxRequestsPerDay: 20}}
	if s, b := scopeRequest(t, h, integrationToken, "POST", "/api/v1/sources/"+source+"/admission", in, headers); s != 201 {
		t.Fatalf("browser CDSE admission %d %v", s, b)
	}
	raw, e := os.ReadFile("../../testdata/imagery/cdse-two-scenes.json")
	if e != nil {
		t.Fatal(e)
	}
	var collection map[string]json.RawMessage
	if e = json.Unmarshal(raw, &collection); e != nil {
		t.Fatal(e)
	}
	collection["links"] = json.RawMessage("[]")
	raw, _ = json.Marshal(collection)
	fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		p, e := publicingest.DecodeSTAC(raw, in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200, ResponseHash: artifacts.Hash(raw)}, e
	})
	if s, b := scopeRequest(t, h, connector, "POST", "/api/v1/sources/"+source+"/connector-runs", map[string]string{"from": "2025-02-01T00:00:00Z", "to": "2025-02-28T23:59:59Z"}, nil); s != 201 || b["newEvidence"] != float64(2) {
		t.Fatalf("browser CDSE discovery %d %v", s, b)
	}
	fetcher.delegate = nil
	human, e := identities.Authenticate(ctx, integrationToken, httpx.PermissionsForRole)
	if e != nil {
		t.Fatal(e)
	}
	author, e := identities.BindSession(ctx, human, resource)
	if e != nil {
		t.Fatal(e)
	}
	workerToken := scopeToken(t, h, "browser-imagery-worker", resource, []string{"operational.read", "sources.read", "imagery.jobs.execute"}, []string{source})
	session, e := identities.Authenticate(ctx, workerToken, nil)
	if e != nil {
		t.Fatal(e)
	}
	worker, e := identities.BindSession(ctx, session, resource)
	if e != nil {
		t.Fatal(e)
	}
	svc := imagery.NewService(h.pool, artifacts.NewService(h.pool, store))
	scenes, e := svc.Scenes(author, nil)
	if e != nil {
		t.Fatal(e)
	}
	var selected []imagery.Scene
	for _, scene := range scenes {
		if scene.SourceID == source {
			selected = append(selected, scene)
		}
	}
	if len(selected) != 2 {
		t.Fatal("two recorded scene revisions required")
	}
	build := artifacts.Hash([]byte("browser-fixture-worker"))
	preparations := []string{}
	for index := 1; index >= 0; index-- {
		scene := selected[index]
		tile := imageryanalysis.Tile{Schema: "emad.multispectral-subset.v1", SceneID: scene.ID, SceneHash: scene.ContentHash, Grid: imageryanalysis.Grid{CRS: "EPSG:4326", Width: 4, Height: 4, West: 39.16, North: 21.49, StepLng: .0002, StepLat: .0002, Registration: "pixel_edges_north_to_south"}, AcquiredAt: scene.AcquiredAt, Platform: scene.Platform, Tile: scene.Tile, Orbit: scene.Orbit, ProcessingVersion: scene.ProcessingVersion, ReflectanceUnit: "surface_reflectance", PreparationMethod: "synthetic-browser-fixture.v1", Synthetic: true, SourceHashes: map[string]string{}}
		for _, band := range []string{"B04_10m", "B08_10m", "SCL_20m"} {
			tile.SourceHashes[band] = artifacts.Hash([]byte("synthetic:" + scene.ID + band))
		}
		for cell := 0; cell < 16; cell++ {
			r, n := .2, .6
			if index == 0 && cell < 3 {
				n = .2
			}
			tile.Red = append(tile.Red, &r)
			tile.NIR = append(tile.NIR, &n)
			tile.SCL = append(tile.SCL, 4)
		}
		tile.SCL[15] = 9
		content, _ := json.Marshal(tile)
		a, e := svc.Artifacts.Reserve(author, artifacts.Input{RequestKey: scene.ID, SourceID: source, SHA256: artifacts.Hash(content), Bytes: int64(len(content)), MediaType: "application/json", Kind: "imagery_subset", RetainUntil: time.Now().UTC().Add(24 * time.Hour), Provenance: "Synthetic browser spectral fixture; not observed vegetation"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = svc.Artifacts.Upload(author, a.ID, content); e != nil {
			t.Fatal(e)
		}
		j, e := svc.Submit(author, imagery.SubmitInput{RequestKey: "prepare:" + scene.ID, Kind: "PREPARE", SceneID: scene.ID, ArtifactID: a.ID})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = svc.WorkOne(worker, build); e != nil {
			t.Fatal(e)
		}
		preparations = append(preparations, j.ID)
	}
	if _, e = h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('browser-imagery-human','Independent development reviewer','supervisor') ON CONFLICT DO NOTHING`); e != nil {
		t.Fatal(e)
	}
	if e = identities.CreateHuman(admin, "browser-independent-reviewer", "browser-imagery-human", access.Grant{Resource: resource, Permissions: []string{"operational.read", "sources.read", "assessments.read", "assessments.review"}, SourceIDs: []string{source}}); e != nil {
		t.Fatal(e)
	}
	_, reviewer, e := identities.IssueCredential(admin, "browser-independent-reviewer", resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	work, cancel := context.WithCancel(worker)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() {
		defer close(done)
		for {
			select {
			case <-work.Done():
				return
			case <-time.After(100 * time.Millisecond):
				if _, err := svc.WorkOne(work, build); err != nil && work.Err() == nil {
					t.Errorf("browser imagery processor: %v", err)
					return
				}
			}
		}
	}()
	return map[string]any{"before": preparations[0], "after": preparations[1], "reviewerToken": reviewer}
}
