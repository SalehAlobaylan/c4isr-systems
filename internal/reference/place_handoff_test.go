package reference

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPlaceHandoffBindsEditionAndRights(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	in := placeHandoffInput{FeatureID: "fixture-place", ReleaseID: "georel_fixture", ExpiresAt: now.Add(time.Hour)}
	release := Release{ReleaseInput: ReleaseInput{SourceID: "owned"}, ID: in.ReleaseID,
		SHA256: strings.Repeat("a", 64), Editions: []map[string]string{{"id": "one", "sha256": strings.Repeat("b", 64)}}}
	permissions := map[string]any{"geographicContextExport": true}
	feature := Feature{ID: in.FeatureID, Type: "Feature", Geometry: json.RawMessage(`{"type":"Point","coordinates":[0,0]}`),
		Properties: map[string]any{"kind": "place", "name": "وادي المثال", "sourceId": "owned", "editionId": "one", "sha256": strings.Repeat("b", 64),
			"receipt": map[string]any{"schema": "emad.desert-maps-normalized.v1", "sourceSha256": strings.Repeat("c", 64)},
			"rights": map[string]any{"id": "fixture", "attribution": "Owned fixture", "outputPermissions": permissions,
				"expiresAt": now.Add(2 * time.Hour).Format(time.RFC3339)}}}
	got, err := buildPlaceHandoff(in, release, feature, now)
	if err != nil || got.MemberSHA256 != Hash(got.Member) || got.EditionID != "one" || got.ReleaseSHA256 != release.SHA256 ||
		got.Attribution != "Owned fixture" || !strings.Contains(string(got.Member), "وادي المثال") {
		t.Fatalf("invalid handoff: %+v, %v", got, err)
	}
	wrongEdition := release
	wrongEdition.Editions = []map[string]string{{"id": "two", "sha256": strings.Repeat("d", 64)}}
	if _, err := buildPlaceHandoff(in, wrongEdition, feature, now); err == nil {
		t.Fatal("accepted a different edition")
	}
	if _, err := buildPlaceHandoff(in, release, feature, now.Add(3*time.Hour)); err == nil {
		t.Fatal("accepted an expired handoff")
	}
	permissions["geographicContextExport"] = false
	if _, err := buildPlaceHandoff(in, release, feature, now); err == nil {
		t.Fatal("accepted a source without distribution permission")
	}
}
