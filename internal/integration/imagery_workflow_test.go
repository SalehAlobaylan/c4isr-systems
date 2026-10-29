package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/caselink"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func TestMapFirstImageryProposalsRollingIntentsAndCaseLinks(t *testing.T) {
	f := newPublicFixture(t)
	admin := access.Bootstrap(context.Background())
	store := access.NewStore(f.h.pool)
	if e := store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"imagery.jobs.submit"}}); e != nil {
		t.Fatal(e)
	}
	registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, URL: publicingest.EarthSearchURL, Adapter: publicingest.EarthSearchAdapter, MaxRequestsPerDay: 20}})
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	assets := map[string]any{}
	for _, key := range []string{"red", "green", "blue", "nir", "scl", "product_metadata"} {
		assets[key] = map[string]any{"href": "https://e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com/sentinel-2-c1-l2a/synthetic-never-fetched/" + key + ".tif", "type": "image/tiff", "raster:bands": []any{map[string]any{"nodata": 0, "scale": .0001, "offset": 0, "spatial_resolution": 10}}}
	}
	feature := map[string]any{"type": "Feature", "id": "S2A_SYNTHETIC_SCOPE_TEST", "collection": publicingest.EarthSearchDataset, "stac_version": "1.1.0", "bbox": []float64{46, 24, 47, 25}, "geometry": json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`), "properties": map[string]any{"datetime": at, "platform": "sentinel-2a", "grid:code": "SYNTHETIC", "eo:cloud_cover": 0, "s2:processing_baseline": "05.11", "s2:product_type": "S2MSI2A", "s2:product_uri": "S2A_SYNTHETIC_R005_METADATA"}, "assets": assets}
	raw, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": []any{feature}, "links": []any{}})
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		p, e := publicingest.DecodeEarthSearch(raw, in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
	})
	f.admission.Dataset = publicingest.EarthSearchDataset
	f.admission.ProductVersion = "sentinel2-c1-l2a"
	f.admission.Policy.MinIntervalSeconds = 5
	f.admission.RevisionPolicy = "content_hash"
	f.admission.Policy.MaxResponseBytes = 32768
	f.admit(t)
	if run := f.execute(t); run["status"] != "SUCCEEDED" {
		t.Fatal("synthetic C1 metadata retention", run)
	}
	code, catalog := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/imagery/scenes", nil, nil)
	if code != 200 || len(catalog["items"].([]any)) != 1 {
		t.Fatal("C1 metadata projection", code, catalog)
	}
	scene := catalog["items"].([]any)[0].(map[string]any)
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		code, body := scopeRequest(t, f.h, token, method, path, in, nil)
		if code != want {
			t.Fatalf("%s %s: got %d want %d %v", method, path, code, want, body)
		}
		return body
	}
	in := analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Synthetic holed imagery proposal", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[46.5,24],[46.5,24.5],[46,24.5],[46,24]],[[46.10,24.10],[46.13,24.10],[46.13,24.13],[46.10,24.13],[46.10,24.10]]]}`)}, Temporal: queryscope.Temporal{Mode: "monitoring", RollingSeconds: 86400, RevisionView: "latest", Timezone: "UTC"}}
	bounds := publicingest.Bounds{West: 46.2, South: 24.2, East: 46.22, North: 24.22}
	proposal := imagery.ProposalInput{Scope: in, SceneID: scene["id"].(string), AOI: &bounds}
	check := call(f.manager, "POST", "/api/v1/imagery/capture-proposal", proposal, 200)
	if check["valid"] != true || check["suggestedAoi"] == nil || check["sceneGeometry"] == nil {
		t.Fatal("missing exact proposal", check)
	}
	hole := publicingest.Bounds{West: 46.10, South: 24.10, East: 46.12, North: 24.12}
	proposal.AOI = &hole
	if check = call(f.manager, "POST", "/api/v1/imagery/capture-proposal", proposal, 200); check["valid"] != false {
		t.Fatal("hole accepted", check)
	}
	input := imagery.CaptureInput{RequestKey: "original-rolling-intent", SceneID: scene["id"].(string), AOI: bounds, Scope: &in}
	saved := call(f.manager, "POST", "/api/v1/imagery/captures", input, 201)
	time.Sleep(20 * time.Millisecond)
	retry := call(f.manager, "POST", "/api/v1/imagery/captures", input, 201)
	if retry["id"] != saved["id"] || retry["input"].(map[string]any)["scope"].(map[string]any)["scopeHash"] != saved["input"].(map[string]any)["scope"].(map[string]any)["scopeHash"] {
		t.Fatal("rolling retry reevaluated original intent", saved, retry)
	}
	// Explicitly synthetic cancelled history: exercise actual submissions past
	// the former lifetime ceiling without fabricating retained pixel captures.
	if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,status,created_by) SELECT 'synthetic-old-intent-'||i,j.scope_id,j.source_id,j.input,'CANCELLED',j.created_by FROM imagery_capture_jobs j CROSS JOIN generate_series(1,1001)i WHERE j.id=$1`, saved["id"]); e != nil {
		t.Fatal(e)
	}
	changed := input
	changed.AOI.East += .001
	call(f.manager, "POST", "/api/v1/imagery/captures", changed, 409)
	_, credential, e := store.IssueCredential(admin, "source-reviewer", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if call(credential, "POST", "/api/v1/imagery/captures", input, 201)["id"] != saved["id"] {
		t.Fatal("credential rotation changed canonical human intent")
	}
	if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO operators(id,name,role)VALUES('imagery-person','Second investigator','operator')`); e != nil {
		t.Fatal(e)
	}
	if e = store.CreateHuman(admin, "imagery-person-login", "imagery-person", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "imagery.jobs.submit", "investigations.read", "investigations.edit", "investigations.manage"}}); e != nil {
		t.Fatal(e)
	}
	_, other, e := store.IssueCredential(admin, "imagery-person-login", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if call(other, "POST", "/api/v1/imagery/captures", input, 201)["id"] == saved["id"] {
		t.Fatal("different human shared an intent identity")
	}
	if e = store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"investigations.read", "investigations.create", "investigations.edit", "investigations.manage"}}); e != nil {
		t.Fatal(e)
	}
	caseInput := investigations.Input{Title: "Synthetic imagery case", Question: "What do these retained inputs show?", State: "DRAFT", Scope: in, Evidence: []investigations.Evidence{}}
	created := call(f.manager, "POST", "/api/v1/investigations", caseInput, 201)
	caseID := created["id"].(string)
	revision := created["revision"].(map[string]any)["id"].(string)
	input.RequestKey = "case-linked-intent"
	input.CaseRevision = &caselink.Revision{CaseID: caseID, RevisionID: revision}
	linked := call(f.manager, "POST", "/api/v1/imagery/captures", input, 201)
	if linked["input"].(map[string]any)["caseRevision"].(map[string]any)["revisionId"] != revision {
		t.Fatal("link omitted exact revision", linked)
	}
	caseInput.ExpectedRevisionID = revision
	caseInput.Notes = "A new saved revision"
	edited := call(f.manager, "POST", "/api/v1/investigations/"+caseID+"/revisions", caseInput, 201)
	if call(f.manager, "POST", "/api/v1/imagery/captures", input, 201)["id"] != linked["id"] {
		t.Fatal("lost linked response revalidated changed head")
	}
	stale := input
	stale.RequestKey = "new-intent-old-case"
	call(f.manager, "POST", "/api/v1/imagery/captures", stale, 409)
	call(other, "GET", "/api/v1/imagery/captures/"+linked["id"].(string), nil, 404)
	read := call(f.manager, "POST", "/api/v1/catalog/reads", map[string]any{"kind": "captures", "limit": 1}, 200)
	members := investigations.Membership{ExpectedMembershipVersion: int(created["membershipVersion"].(float64)), ExpectedRevisionID: edited["revision"].(map[string]any)["id"].(string), OwnerHumanID: "imagery-person", Members: []string{"operator-01", "imagery-person"}}
	transfer := call(f.manager, "POST", "/api/v1/investigations/"+caseID+"/members", members, 200)
	members.ExpectedMembershipVersion = int(transfer["membershipVersion"].(float64))
	members.Members = []string{"imagery-person"}
	call(other, "POST", "/api/v1/investigations/"+caseID+"/members", members, 200)
	call(f.manager, "GET", "/api/v1/imagery/captures/"+linked["id"].(string), nil, 404)
	call(f.manager, "POST", "/api/v1/catalog/reads", map[string]any{"kind": "captures", "limit": 1, "snapshotId": read["snapshotId"], "cursor": read["nextCursor"]}, 409)
	call(other, "GET", "/api/v1/imagery/captures/"+linked["id"].(string), nil, 200)
}
