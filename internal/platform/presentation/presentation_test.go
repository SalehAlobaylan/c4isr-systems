package presentation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

func TestExerciseViewPreservesMeasuredEvidenceWithoutHiddenMetadata(t *testing.T) {
	r := access.LegacyResource()
	ctx := access.Internal(context.Background(), r, "fixture")
	ctx = WithViewResolver(ctx, func(_ context.Context, ids []string) (map[string]string, error) {
		out := map[string]string{}
		for _, id := range ids {
			out[id] = "view_obs_0123456789abcdef0123456789abcdef"
		}
		return out, nil
	})
	value := map[string]any{"id": "run__observation__HIDDEN_ACTOR_998__1", "sourceId": "approved-sensor", "observedAt": "2026-10-20T00:00:00Z", "receivedAt": "2026-10-20T00:00:01Z", "trackHint": "HIDDEN_ACTOR_998", "externalRef": "HIDDEN_ACTOR_998", "payload": map[string]any{"nested": map[string]any{"innocent": "HIDDEN_ACTOR_998"}}, "quality": map[string]any{"accuracyM": 12.5, "label": "HIDDEN_ACTOR_998"}, "metadata": map[string]any{"sim_actor_id": "HIDDEN_ACTOR_998", "observationCount": 2}, "position": map[string]any{"lat": 24.7, "lng": 46.6}}
	value["quality"].(map[string]any)["assetId"] = "HIDDEN_ACTOR_998"
	value["metadata"].(map[string]any)["sourceId"] = "HIDDEN_ACTOR_998"
	raw, err := Encode(ctx, "/api/v1/observations", value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "HIDDEN_ACTOR_998") {
		t.Fatalf("hidden identity escaped: %s", raw)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if !IsObservationAlias(out["id"].(string)) || out["position"] == nil || out["quality"].(map[string]any)["accuracyM"] != 12.5 {
		t.Fatalf("measured evidence lost: %s", raw)
	}
	public := access.Resource{WorkspaceID: "public", EnvironmentID: "world", Partition: access.PublicEvidence}
	publicCtx := access.Internal(context.Background(), public, "fixture")
	publicRaw, err := Encode(publicCtx, "/api/v1/observations", value)
	if err != nil || !strings.Contains(string(publicRaw), "HIDDEN_ACTOR_998") {
		t.Fatal("public evidence was silently rewritten")
	}
	if observationID("view_obs_HIDDEN_ACTOR_998", map[string]string{}) == "view_obs_HIDDEN_ACTOR_998" {
		t.Fatal("untrusted ID bypassed alias projection")
	}
	legacyID := "view_obs_48494444454e5f4143544f525f39393800"
	issued := "view_obs_0123456789abcdef0123456789abcdef"
	if observationID(legacyID, map[string]string{legacyID: issued}) != issued {
		t.Fatal("legacy primary ID bypassed the random reference registry")
	}

}

func TestTypedEvidenceAliasesResolveButRawPayloadIsUnchanged(t *testing.T) {
	alias := "view_obs_0123456789abcdef0123456789abcdef"
	ctx := WithResolver(context.Background(), func(_ context.Context, id string) (string, error) {
		if id != alias {
			t.Fatal("untyped value resolved")
		}
		return "private-id", nil
	})
	input := map[string]any{"subjectType": "observation", "subjectId": alias, "evidence": []any{map[string]any{"type": "observation", "id": alias}}, "payload": map[string]any{"subjectType": "observation", "subjectId": alias}}
	if err := ResolveInput(ctx, &input); err != nil {
		t.Fatal(err)
	}
	if input["subjectId"] != "private-id" || input["evidence"].([]any)[0].(map[string]any)["id"] != "private-id" || input["payload"].(map[string]any)["subjectId"] != alias {
		t.Fatal("typed reference resolution changed raw evidence")
	}
}

func TestOrdinaryRunStatusAndErrorExcludeScheduledInput(t *testing.T) {
	r := access.LegacyResource()
	ctx, err := access.Bind(context.Background(), access.Principal{ID: "operator", Kind: "human", Grants: []access.Grant{{Resource: r, Permissions: []string{"scenarios.read"}}}}, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(ctx, "/api/v1/scenarios/runs/legacy", map[string]any{"id": "legacy", "status": "RUNNING", "seed": 54321, "scenarioName": "HIDDEN_ACTOR_998", "lastAction": "HIDDEN_ACTOR_998", "error": "private filesystem HIDDEN_ACTOR_998"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "HIDDEN_ACTOR_998") || strings.Contains(string(raw), "54321") || !strings.Contains(string(raw), "RUNNING") {
		t.Fatalf("ordinary run status disclosed diagnostics: %s", raw)
	}
}
