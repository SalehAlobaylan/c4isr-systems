package integration

import (
	"context"
	"encoding/json"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"os"
	"testing"
)

func TestReferenceCatalogRealRegionTerrainAndIsolation(t *testing.T) {
	f := newPublicFixture(t)
	var first string
	for _, name := range []string{"places", "elevation"} {
		raw, err := os.ReadFile("../../testdata/reference/jeddah-makkah/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var in reference.Input
		if err = json.Unmarshal(raw, &in); err != nil {
			t.Fatal(err)
		}
		in.SourceID = f.source
		status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", in, nil)
		if status != 201 || body["eligibleForSimulation"] != true || body["sha256"] != reference.Hash(in.Artifact) || body["rights"].(map[string]any)["attribution"] == "" {
			t.Fatalf("reference import %s: %d %v", name, status, body)
		}
		id := body["id"].(string)
		if name == "places" {
			first = id
		}
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", in, nil); status != 200 {
			t.Fatalf("reference retry: %d", status)
		}
		_, features := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets/"+id+"/features", nil, nil)
		if name == "places" && len(features["features"].([]any)) != 2 {
			t.Fatalf("reference places missing: %v", features)
		}
		if name == "places" {
			var artifact map[string]any
			if err = json.Unmarshal(in.Artifact, &artifact); err != nil {
				t.Fatal(err)
			}
			artifact["features"].([]any)[0].(map[string]any)["geometry"].(map[string]any)["crs"] = map[string]any{"type": "name", "properties": map[string]any{"name": "EPSG:3857"}}
			bad := in
			bad.Version = "contradictory-crs"
			bad.Artifact, _ = json.Marshal(artifact)
			bad.SHA256 = reference.Hash(bad.Artifact)
			if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 400 {
				t.Fatalf("embedded CRS relabelled as WGS84: %d", status)
			}
		}
		if name == "elevation" {
			var g reference.Grid
			if json.Unmarshal(in.Artifact, &g) != nil || g.Width != 60 || g.Height != 60 || g.Values[0] == nil {
				t.Fatalf("terrain samples: %+v", g)
			}
		}
		bad := in
		bad.CRS = "EPSG:3857"
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 400 {
			t.Fatalf("incorrect CRS accepted: %d", status)
		}
		bad = in
		bad.Label = "changed metadata"
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 409 {
			t.Fatalf("mutable version: %d", status)
		}
		bad = in
		bad.SHA256 = "incorrect"
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 400 {
			t.Fatalf("checksum not verified: %d", status)
		}
		bad = in
		bad.SourceID = "unregistered-source"
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 404 {
			t.Fatalf("unregistered source: %d", status)
		}
		bad = in
		bad.Version = "rights-change"
		bad.Rights.AllowSimulationExport = false
		if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", bad, nil); status != 409 {
			t.Fatalf("rights ID changed meaning: %d", status)
		}
	}
	_, catalog := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets?eligible=true", nil, nil)
	if catalog["total"] != float64(2) {
		t.Fatalf("S0 eligible versions: %v", catalog)
	}
	if status, _ := scopeRequest(t, f.h, f.worker, "POST", "/api/v1/reference-datasets", map[string]any{}, nil); status != 403 {
		t.Fatalf("worker could review GIS import: %d", status)
	}
	foreign := f.resource
	foreign.WorkspaceID = "foreign-reference"
	var count int
	if err := platformdb.Scoped(f.h.pool).QueryRow(scopeContext(t, f.h, foreign), `SELECT count(*) FROM reference_datasets`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cross-scope catalog: %d %v", count, err)
	}
	other := scopeToken(t, f.h, "foreign-reference-reader", f.resource, []string{"operational.read", "sources.read"}, []string{"other-source"})
	if status, _ := scopeRequest(t, f.h, other, "GET", "/api/v1/reference-datasets/"+first, nil, nil); status != 404 {
		t.Fatalf("source-restricted artifact metadata disclosed: %d", status)
	}
	if _, err := f.h.pool.Exec(context.Background(), `UPDATE reference_datasets SET version='overwrite' WHERE id=$1`, first); err == nil {
		t.Fatal("reference snapshot mutable")
	}
	// Reference imports never create observations or controllable entities.
	if err := f.h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM observations)+(SELECT count(*) FROM tracks)+(SELECT count(*) FROM assets)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("reference data became operational: %d %v", count, err)
	}
}
