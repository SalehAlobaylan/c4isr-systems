package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

func TestWorkspaceCatalogAndNegativeAuthorizationMatrix(t *testing.T) {
	h := newHarness(t)
	store := access.NewStore(h.pool)
	r := access.Resource{WorkspaceID: "catalog", EnvironmentID: "world", Partition: access.PublicEvidence}
	permissions := []string{"operational.read", "sources.read", "observations.read", "scenarios.read"}
	token := scopeToken(t, h, "catalog-reader", r, permissions, nil)
	admin := scopeContext(t, h, access.LegacyResource())
	exercise := r
	exercise.Partition, exercise.RunID = access.ExerciseObservation, "allowed-run"
	if err := store.AddGrant(admin, "catalog-reader", access.Grant{Resource: exercise, Permissions: permissions}); err != nil {
		t.Fatal(err)
	}
	for _, partition := range []access.Partition{access.ExerciseInput, access.ExerciseTruth} {
		private := r
		private.Partition = partition
		if partition == access.ExerciseTruth {
			private.RunID = "private-truth"
		}
		if err := store.AddGrant(admin, "catalog-reader", access.Grant{Resource: private, Permissions: []string{"operational.read", "sim.truth.read"}}); err != nil {
			t.Fatal(err)
		}
	}
	foreign := exercise
	foreign.RunID = "ungranted-run"
	if err := store.EnsureResource(context.Background(), foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(context.Background(), `INSERT INTO scenario_runs(id,scope_id,scenario_name,seed,status) VALUES($1,$2,'Coastal patrol',1,'STOPPED'),($3,$4,'PRIVATE_FOREIGN_SCENARIO',1,'RUNNING')`, exercise.RunID, exercise.ID(), foreign.RunID, foreign.ID()); err != nil {
		t.Fatal(err)
	}
	status, me := scopeRequest(t, h, token, "GET", "/api/v1/auth/me", nil, nil)
	raw, _ := json.Marshal(me["resources"])
	var resources []access.Resource
	if err := json.Unmarshal(raw, &resources); err != nil || status != 200 || len(resources) != 2 {
		t.Fatalf("catalog: %d %s %v", status, raw, err)
	}
	for _, item := range resources {
		if item != r && item != exercise {
			t.Fatalf("private/ungranted resource enumerated: %#v", item)
		}
	}
	detailsJSON, _ := json.Marshal(me["resourceDetails"])
	var details []access.ResourceDetail
	if err := json.Unmarshal(detailsJSON, &details); err != nil || len(details) != 2 {
		t.Fatalf("navigation details: %s %v", detailsJSON, err)
	}
	found := false
	for _, detail := range details {
		if detail.Resource == exercise {
			found = detail.Label == "Coastal patrol" && detail.Status == "STOPPED" && detail.StartedAt != nil
		}
	}
	if !found || strings.Contains(string(detailsJSON), "PRIVATE_FOREIGN_SCENARIO") {
		t.Fatalf("navigation metadata must remain scoped: %s", detailsJSON)
	}
	// Losing scenario-read permission removes metadata even while the COP stays selectable.
	if _, err := h.pool.Exec(context.Background(), `UPDATE principal_grants SET permissions=array_remove(permissions,'scenarios.read') WHERE principal_id='catalog-reader' AND partition='EXERCISE_OBSERVATION'`); err != nil {
		t.Fatal(err)
	}
	_, limited := scopeRequest(t, h, token, "GET", "/api/v1/auth/me", nil, nil)
	limitedJSON, _ := json.Marshal(limited["resourceDetails"])
	if strings.Contains(string(limitedJSON), "Coastal patrol") || strings.Contains(string(limitedJSON), "startedAt") {
		t.Fatalf("restricted run metadata leaked: %s", limitedJSON)
	}
	for _, tc := range []struct {
		name, path string
		headers    map[string]string
		want       int
	}{
		{"workspace", "/api/v1/sources", map[string]string{"X-EMAD-Workspace-ID": "foreign"}, 403},
		{"environment", "/api/v1/sources", map[string]string{"X-EMAD-Environment-ID": "foreign"}, 403},
		{"run", "/api/v1/sources", map[string]string{"X-EMAD-Partition": "EXERCISE_OBSERVATION", "X-EMAD-Run-ID": "ungranted-run"}, 403},
		{"truth transport", "/api/v1/observations", map[string]string{"X-EMAD-Partition": "EXERCISE_TRUTH", "X-EMAD-Run-ID": "private-truth"}, 403},
		{"input transport", "/api/v1/sources", map[string]string{"X-EMAD-Partition": "EXERCISE_INPUT"}, 403},
		{"public assets", "/api/v1/assets", nil, 403},
		{"public commands", "/api/v1/commands", nil, 403},
		{"unknown action", "/api/v1/unknown", nil, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, _ := scopeRequest(t, h, token, "GET", tc.path, nil, tc.headers); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
	before := me["accessRevision"]
	if _, err := h.pool.Exec(context.Background(), `UPDATE principal_grants SET source_ids=ARRAY['allowed-source'] WHERE principal_id='catalog-reader' AND partition='PUBLIC_EVIDENCE'`); err != nil {
		t.Fatal(err)
	}
	_, after := scopeRequest(t, h, token, "GET", "/api/v1/auth/me", nil, nil)
	if before == after["accessRevision"] {
		t.Fatal("source restriction change did not invalidate browser authority")
	}
}
