package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

func TestDesertMapsReleaseAndCivilianPlaceHandoff(t *testing.T) {
	f := newPublicFixture(t)
	exporter := scopeToken(t, f.h, "place-exporter", f.resource, []string{"sources.read", "context.export"}, []string{f.source})
	allow := true
	rights := f.admission.Rights
	rights.OutputPermissions = &publicingest.OutputPermissions{GeographicContextExport: &allow}
	expires := time.Now().UTC().Add(time.Hour)
	rights.ExpiresAt = &expires
	stage := func(name, version string) string {
		t.Helper()
		raw := []byte(fmt.Sprintf(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[46.5,24.5]},"properties":{"key":"owned:wadi","kind":"place","name":%q}}]}`, name))
		in := reference.GeographyInput{SourceID: f.source, Label: "Owned place fixture", Version: version,
			Staged: true, Rights: rights, SHA256: reference.Hash(raw), Artifact: raw,
			Receipt: json.RawMessage(`{"schema":"emad.desert-maps-normalized.v1","sourceSha256":"` + strings.Repeat("1", 64) + `"}`)}
		code, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/editions", in, nil)
		if code != 201 {
			t.Fatalf("stage: %d %v", code, body)
		}
		return body["id"].(string)
	}
	activate := func(version, edition, expected string, want int) string {
		t.Helper()
		in := reference.ReleaseInput{SourceID: f.source, Key: "owned-places", Version: version,
			EditionIDs: []string{edition}, Activate: true, ExpectedHead: expected}
		code, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/geography/releases", in, nil)
		if code != want {
			t.Fatalf("activate: %d %v", code, body)
		}
		id, _ := body["id"].(string)
		return id
	}
	before := stage("وادي المثال", "v1")
	release := activate("v1", before, "", 201)
	after := stage("وادي المثل", "v2")
	code, visible := scopeRequest(t, f.h, exporter, "GET", "/api/v1/geography/regions?kind=place", nil, nil)
	if code != 200 {
		t.Fatalf("staged discovery: %d %v", code, visible)
	}
	items, _ := visible["features"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != before+":owned:wadi" {
		t.Fatalf("partial staging changed discovery: %v", visible)
	}
	activate("v2", after, "stale-head", 409)
	current := activate("v2", after, release, 201)
	path := "/api/v1/geography/place-handoffs"
	input := map[string]any{"featureId": before + ":owned:wadi", "releaseId": release, "expiresAt": expires.Add(-time.Minute)}
	if code, body := scopeRequest(t, f.h, f.manager, "POST", path, input, nil); code != 403 {
		t.Fatalf("missing export grant: %d %v", code, body)
	}
	code, body := scopeRequest(t, f.h, exporter, "POST", path, input, nil)
	if code != 200 || body["editionId"] != before {
		t.Fatalf("issue earlier place: %d %v", code, body)
	}
	bytes, err := base64.StdEncoding.DecodeString(body["member"].(string))
	if err != nil || reference.Hash(bytes) != body["memberSha256"] || !strings.Contains(string(bytes), "وادي المثال") {
		t.Fatalf("pinned member: %s %v", bytes, err)
	}
	input["releaseId"] = current
	if code, body = scopeRequest(t, f.h, exporter, "POST", path, input, nil); code != 404 {
		t.Fatalf("accepted crossed edition: %d %v", code, body)
	}
	if rolledBack := activate("v1", before, current, 201); rolledBack != release {
		t.Fatal("rollback did not restore the pinned release")
	}

	vector := []byte(`{"type":"FeatureCollection","features":[{"type":"Feature","id":"owned-outline","geometry":{"type":"LineString","coordinates":[[46.2,24.2],[46.3,24.3]]},"properties":{"layer":"desert-maps","family":"boundary-drawing","attribution":"Owned fixture"}}]}`)
	vectorInput := reference.Input{SourceID: f.source, DatasetKey: "desert-maps-owned-1", Version: "owned",
		Label: "Owned unnamed drawing", Kind: "vector", OriginalProducer: "Owned fixture", CRS: "EPSG:4326",
		VerticalDatum: "not_applicable", Resolution: reference.Resolution{Unit: "unknown"},
		AOI: publicingest.Bounds{West: 46, South: 24, East: 47, North: 25}, Validity: "Edition fixture",
		Quality: "Accuracy unknown", Transformation: "Source WGS84", Rights: rights, RetrievedAt: time.Now().UTC(),
		RetrievalReference: "https://provider.example.invalid/owned", Artifact: vector, SHA256: reference.Hash(vector)}
	code, imported := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", vectorInput, nil)
	if code != 201 {
		t.Fatalf("admit unnamed drawing: %d %v", code, imported)
	}
	code, retained := scopeRequest(t, f.h, exporter, "GET", "/api/v1/reference-datasets/"+imported["id"].(string)+"/features", nil, nil)
	encoded, err := json.Marshal(retained)
	if code != 200 || err != nil || !strings.Contains(string(encoded), "LineString") || !strings.Contains(string(encoded), "owned-outline") {
		t.Fatalf("read unnamed drawing: %d %v", code, retained)
	}
}
