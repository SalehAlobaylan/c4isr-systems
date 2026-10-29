package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"os"
	"testing"
)

func TestMapFirstSourceBackedSevenRegionalImportsAndAtomicActivation(t *testing.T) {
	path := os.Getenv("C4ISR_TEST_GEOGRAPHY_PREPARED")
	if path == "" {
		t.Skip("requires source-backed prepared regional manifest")
	}
	f := newPublicFixture(t)
	ctx := scopeContext(t, f.h, f.resource)
	svc := reference.NewService(f.h.pool)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var plan struct {
		Version string                     `json:"version"`
		Chunks  []reference.GeographyInput `json:"chunks"`
		Depth   map[string]struct {
			Regions int `json:"regions"`
			Places  int `json:"places"`
		} `json:"depth"`
	}
	if e = json.Unmarshal(raw, &plan); e != nil {
		t.Fatal(e)
	}
	if len(plan.Chunks) < 2 || len(plan.Depth) != 7 {
		t.Fatal("source-backed seven-area manifest required")
	}
	ids := []string{}
	for index, in := range plan.Chunks {
		in.SourceID = f.source
		in.Staged = true
		in.Rights = publicingest.Rights{ID: "naturalearth-public-domain-qualification-v1", LegalBasis: "Natural Earth published public-domain terms", TermsURL: "https://www.naturalearthdata.com/about/terms-of-use/", Attribution: "Made with Natural Earth. Source-backed navigation qualification.", RetainRaw: true, AllowAnalysis: true, AllowSimulationExport: true}
		actual, duplicate, e := svc.AdmitGeography(ctx, in)
		if e != nil || duplicate {
			t.Fatal("real source chunk", index, e)
		}
		again, duplicate, e := svc.AdmitGeography(ctx, in)
		if e != nil || !duplicate || again.ID != actual.ID {
			t.Fatal("resumable chunk", e)
		}
		ids = append(ids, actual.ID)
		staged, e := svc.Geography(ctx, "", "", "country", "", 0)
		if e != nil || staged.Matched != 0 {
			t.Fatal("partial initial release became discoverable", staged.Matched, e)
		}
	}
	release, e := svc.Release(ctx, reference.ReleaseInput{SourceID: f.source, Key: "regional-qualification", Version: plan.Version, EditionIDs: ids, Activate: true})
	if e != nil {
		t.Fatal("real source release", e)
	}
	for _, code := range []string{"SAU", "ARE", "EGY", "TUR", "FRA", "JPN", "USA"} {
		area, e := (areas.Service{Pool: f.h.pool}).Resolve(ctx, areas.Selection{Kind: "administrative", SourceID: f.source, LogicalKey: "country:" + code, ReleaseID: release.ID})
		if e != nil {
			t.Fatal(code, "exact source country", e)
		}
		availability, e := svc.GeographyAvailability(ctx, area.Geography.RegionID, release.ID)
		if e != nil || availability.Regions != plan.Depth[code].Regions || availability.Places != plan.Depth[code].Places || availability.Regions == 0 || availability.Places == 0 {
			t.Fatal(code, "reference depth", availability, e)
		}
		t.Logf("%s: %d source subdivisions, %d source place points", code, availability.Regions, availability.Places)
	}
	// Source aliases are retained while normalized discovery accepts accents.
	french, e := svc.Geography(ctx, "cotes", "", "region", "", 0, f.source, release.ID)
	if e != nil || french.Matched == 0 {
		t.Fatal("accent-normalized source search", french.Matched, e)
	}
	var sourceBytes int
	if e = f.h.pool.QueryRow(context.Background(), `SELECT sum(octet_length(artifact)) FROM geographic_editions WHERE id=ANY($1::text[])`, ids).Scan(&sourceBytes); e != nil || sourceBytes == 0 {
		t.Fatal("retained source bytes", e)
	}
	// The same logical name/key in another source must stay a distinct area.
	code, created := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources", map[string]any{"id": "second-navigation-namespace", "name": "Synthetic second navigation source", "type": "external"}, nil)
	if code != 201 {
		t.Fatal(code, created)
	}
	otherSource := created["id"].(string)
	otherRaw := []byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"key":"country:FRA","kind":"country","name":"France"},"geometry":{"type":"Polygon","coordinates":[[[1,45],[2,45],[2,46],[1,46],[1,45]]]}}]}`)
	other, _, e := svc.AdmitGeography(ctx, reference.GeographyInput{Staged: true, SourceID: otherSource, Label: "Synthetic namespace test", Version: "one", Rights: f.admission.Rights, Receipt: json.RawMessage(`{"synthetic":true}`), SHA256: reference.Hash(otherRaw), Artifact: otherRaw})
	if e != nil {
		t.Fatal(e)
	}
	otherRelease, e := svc.Release(ctx, reference.ReleaseInput{SourceID: otherSource, Key: "namespace", Version: "one", EditionIDs: []string{other.ID}, Activate: true})
	if e != nil {
		t.Fatal(e)
	}
	firstArea, e := (areas.Service{Pool: f.h.pool}).Resolve(ctx, areas.Selection{Kind: "administrative", SourceID: f.source, LogicalKey: "country:FRA", ReleaseID: release.ID})
	if e != nil {
		t.Fatal(e)
	}
	secondArea, e := (areas.Service{Pool: f.h.pool}).Resolve(ctx, areas.Selection{Kind: "administrative", SourceID: otherSource, LogicalKey: "country:FRA", ReleaseID: otherRelease.ID})
	if e != nil || firstArea.RevisionID == secondArea.RevisionID || firstArea.GeometrySHA256 == secondArea.GeometrySHA256 {
		t.Fatal("source namespaces overwrote each other", e)
	}
	store := access.NewStore(f.h.pool)
	if e = store.AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"geography.request"}}); e != nil {
		t.Fatal(e)
	}
	request := reference.ReferenceRequestInput{RequestKey: "retained-depth-request", RegionID: firstArea.Geography.RegionID, Reason: "Verify the retained subdivision and place detail for this analytical study"}
	code, pending := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/reference-requests", request, nil)
	if code != 201 {
		t.Fatal(code, pending)
	}
	requestID := pending["id"].(string)
	code, retry := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/reference-requests", request, nil)
	if code != 201 || retry["id"] != requestID {
		t.Fatal("reference request retry was not idempotent", code, retry)
	}
	changed := request
	changed.Reason = "A changed request must have its own intentional identity"
	if code, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/reference-requests", changed, nil); code != 409 {
		t.Fatal("changed reference intent reused a request key", code)
	}
	if code, _ = scopeRequest(t, f.h, f.worker, "POST", "/api/v1/geography/reference-requests", request, nil); code != 403 {
		t.Fatal("worker requested human reference admission", code)
	}
	if code, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/reference-requests/"+requestID+"/resolve", map[string]string{"releaseId": otherRelease.ID}, nil); code != 400 {
		t.Fatal("request used another source's release", code)
	}
	if code, done := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/reference-requests/"+requestID+"/resolve", map[string]string{"releaseId": release.ID}, nil); code != 200 || done["state"] != "INSTALLED" {
		t.Fatal("retained request resolution", code, done)
	}
}
