package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
)

func scopeRequest(t *testing.T, h *harness, token, method, path string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	if resp.StatusCode != http.StatusNoContent {
		if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode, out
}

func scopeToken(t *testing.T, h *harness, id string, r access.Resource, permissions, sources []string) string {
	t.Helper()
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	filtered := []string{}
	for _, p := range permissions {
		if p == "exercise.ingest" || p == "sim.truth.record" || p == "tracks.project" || p == "observations.process" || p == "alerts.project" || p == "geofences.evaluate" || p == "audit.append" || p == "platform.maintenance" {
			continue
		}
		if p != "admin.manage" && p != "sim.truth.read" && p != "sim.truth.grant" {
			filtered = append(filtered, p)
		}
	}
	permissions = filtered
	if err := store.CreateService(admin, id, id, r, permissions, sources); err != nil {
		t.Fatal(err)
	}
	_, token, err := store.IssueCredential(admin, id, r, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func scopeContext(t *testing.T, h *harness, r access.Resource) context.Context {
	t.Helper()
	p := access.Principal{ID: "scope-fixture", Kind: "human", Grants: []access.Grant{{Resource: r, Permissions: access.Permissions}}}
	ctx, err := access.Bind(context.Background(), p, r, access.NewStore(h.pool).Audit)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestScopedPrincipalsQueriesRelationsAndCredentialLifecycle(t *testing.T) {
	h := newHarness(t)
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	publicA := access.Resource{WorkspaceID: "alpha", EnvironmentID: "world", Partition: access.PublicEvidence}
	publicB := access.Resource{WorkspaceID: "beta", EnvironmentID: "world", Partition: access.PublicEvidence}
	permissions := []string{"sources.manage", "sources.read", "observations.ingest", "observations.read", "tracks.read", "operational.read", "assessments.create", "assessments.read"}
	tokenA := scopeToken(t, h, "owner-a", publicA, permissions, nil)
	tokenB := scopeToken(t, h, "owner-b", publicB, permissions, nil)
	createSource := func(token, id string) string {
		status, body := scopeRequest(t, h, token, "POST", "/api/v1/sources", map[string]any{"id": id, "name": id, "type": "external"}, nil)
		if status != 201 {
			t.Fatalf("create source: %d %v", status, body)
		}
		return body["id"].(string)
	}
	sourceA := createSource(tokenA, "shared-source")
	sourceB := createSource(tokenB, "shared-source")
	peerA := createSource(tokenA, "peer-source")
	if sourceA == sourceB {
		t.Fatal("caller IDs collided across scopes")
	}
	connector := scopeToken(t, h, "connector-a", publicA, []string{"sources.read", "observations.ingest", "observations.read"}, []string{sourceA})
	observation := func(source string) map[string]any {
		return map[string]any{"id": "same-event", "sourceId": source, "type": "track.observation", "trackHint": "shared-hypothesis", "observedAt": time.Now().UTC().Format(time.RFC3339Nano), "position": map[string]any{"lat": 24.7, "lng": 46.6}}
	}
	for _, item := range []struct{ token, source string }{{connector, sourceA}, {tokenB, sourceB}} {
		status, body := scopeRequest(t, h, item.token, "POST", "/api/v1/observations", observation(item.source), nil)
		if status != 201 {
			t.Fatalf("ingest: %d %v", status, body)
		}
		status, body = scopeRequest(t, h, item.token, "POST", "/api/v1/observations", observation(item.source), nil)
		if status != 200 {
			t.Fatalf("duplicate ingest: %d %v", status, body)
		}
	}
	for _, source := range []string{sourceB, peerA} {
		if status, _ := scopeRequest(t, h, connector, "POST", "/api/v1/observations", observation(source), nil); status != 403 {
			t.Fatalf("foreign source accepted: %d", status)
		}
	}
	if status, _ := scopeRequest(t, h, connector, "GET", "/api/v1/tracks", nil, nil); status != 403 {
		t.Fatal("connector inherited projection authority")
	}
	if status, _ := scopeRequest(t, h, connector, "GET", "/api/v1/sources", nil, map[string]string{"X-EMAD-Workspace-ID": "beta"}); status != 403 {
		t.Fatal("workspace header expanded grant")
	}
	if status, body := scopeRequest(t, h, connector, "GET", "/api/v1/sources", nil, nil); status != 200 || body["total"].(float64) != 1 {
		t.Fatalf("source-bound read: %d %v", status, body)
	}
	for _, token := range []string{tokenA, tokenB} {
		if status, body := scopeRequest(t, h, token, "GET", "/api/v1/tracks", nil, nil); status != 200 || body["total"].(float64) != 1 {
			t.Fatalf("track isolation: %d %v", status, body)
		}
	}
	if status, _ := scopeRequest(t, h, tokenA, "GET", "/api/v1/sources/"+sourceB, nil, nil); status != 404 {
		t.Fatalf("foreign detail: %d", status)
	}
	ctxA := scopeContext(t, h, publicA)
	if _, err := platformdb.Scoped(h.pool).Exec(ctxA, `INSERT INTO observations(id,source_id,observation_type,observed_at) VALUES('illegal-link',$1,'point',now())`, sourceB); !apperr.Is(err, apperr.CodeForbidden) {
		t.Fatalf("database accepted foreign parent: %v", err)
	}
	status, tracks := scopeRequest(t, h, tokenA, "GET", "/api/v1/tracks", nil, nil)
	if status != 200 {
		t.Fatal(tracks)
	}
	trackA := tracks["items"].([]any)[0].(map[string]any)["id"].(string)
	status, tracks = scopeRequest(t, h, tokenB, "GET", "/api/v1/tracks", nil, nil)
	if status != 200 {
		t.Fatal(tracks)
	}
	trackB := tracks["items"].([]any)[0].(map[string]any)["id"].(string)
	if status, body := scopeRequest(t, h, tokenA, "POST", "/api/v1/observations", map[string]any{"id": "peer-observation", "sourceId": peerA, "type": "point", "observedAt": time.Now().UTC().Format(time.RFC3339Nano)}, nil); status != 201 {
		t.Fatalf("peer ingestion: %d %v", status, body)
	}
	if status, body := scopeRequest(t, h, connector, "GET", "/api/v1/observations", nil, nil); status != 200 || body["total"].(float64) != 1 {
		t.Fatalf("source-bound evidence read: %d %v", status, body)
	}
	status, body := scopeRequest(t, h, tokenA, "POST", "/api/v1/assessments", map[string]any{"subjectType": "track", "subjectId": trackA, "type": "test", "conclusion": "scope fixture", "evidence": []map[string]string{{"type": "track", "id": trackB}}}, nil)
	if status != 403 {
		t.Fatalf("foreign evidence accepted: %d %v", status, body)
	}
	if status, body = scopeRequest(t, h, tokenA, "GET", "/api/v1/assessments", nil, nil); status != 200 || body["total"].(float64) != 0 {
		t.Fatalf("failed evidence write left partial assessment: %d %v", status, body)
	}
	key, newToken, err := store.IssueCredential(admin, "connector-a", publicA, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.Authenticate(context.Background(), connector, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeCredential(admin, session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	if status, _ := scopeRequest(t, h, connector, "GET", "/api/v1/sources", nil, nil); status != 401 {
		t.Fatal("revoked credential accepted")
	}
	if status, _ := scopeRequest(t, h, newToken, "GET", "/api/v1/sources", nil, nil); status != 200 {
		t.Fatal("rotated credential rejected")
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE principal_credentials SET expires_at=now()-interval '1 second' WHERE id=$1`, key); err != nil {
		t.Fatal(err)
	}
	if status, _ := scopeRequest(t, h, newToken, "GET", "/api/v1/sources", nil, nil); status != 401 {
		t.Fatal("expired credential accepted")
	}
	var denied int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM authorization_denials`).Scan(&denied); err != nil || denied < 6 {
		t.Fatalf("missing denial audit: %d (%v)", denied, err)
	}
}

func TestExerciseRunIsolationDirectServicesAndPoolReuse(t *testing.T) {
	h := newHarness(t)
	store := access.NewStore(h.pool)
	runA := access.Resource{WorkspaceID: "exercise", EnvironmentID: "lab", Partition: access.ExerciseObservation, RunID: "run-a"}
	runB := runA
	runB.RunID = "run-b"
	tokenA := scopeToken(t, h, "exercise-a", runA, access.Permissions, nil)
	tokenB := scopeToken(t, h, "exercise-b", runB, access.Permissions, nil)
	public := access.Resource{WorkspaceID: "public", EnvironmentID: "world", Partition: access.PublicEvidence}
	publicToken := scopeToken(t, h, "public-observer", public, access.Permissions, nil)
	status, source := scopeRequest(t, h, publicToken, "POST", "/api/v1/sources", map[string]any{"name": "Public fixture", "type": "external"}, nil)
	if status != 201 {
		t.Fatalf("coexisting public source: %d %v", status, source)
	}
	if status, body := scopeRequest(t, h, publicToken, "POST", "/api/v1/observations", map[string]any{"sourceId": source["id"], "type": "track.observation", "trackHint": "shared-actor", "observedAt": time.Now().UTC().Format(time.RFC3339Nano)}, nil); status != 201 {
		t.Fatalf("coexisting public observation: %d %v", status, body)
	}
	if status, body := scopeRequest(t, h, publicToken, "GET", "/api/v1/tracks", nil, nil); status != 200 || body["total"].(float64) != 1 {
		t.Fatalf("public projection: %d %v", status, body)
	}
	if status, _ := scopeRequest(t, h, publicToken, "POST", "/api/v1/commands", map[string]any{"assetId": "shared-actor", "type": "move_to"}, nil); status != 403 {
		t.Fatalf("public track accepted as controlled asset: %d", status)
	}
	otherEnvironment := runA
	otherEnvironment.EnvironmentID = "other"
	if err := store.EnsureResource(context.Background(), otherEnvironment); err != nil {
		t.Fatal(err)
	}
	if status, _ := scopeRequest(t, h, tokenA, "GET", "/api/v1/assets", nil, map[string]string{"X-EMAD-Environment-ID": "other"}); status != 403 {
		t.Fatal("foreign environment accepted")
	}
	if status, _ := scopeRequest(t, h, tokenA, "GET", "/api/v1/assets", nil, map[string]string{"X-EMAD-Run-ID": "run-b"}); status != 403 {
		t.Fatal("foreign run accepted")
	}
	assetsByRun := []string{}
	for _, token := range []string{tokenA, tokenB} {
		status, body := scopeRequest(t, h, token, "POST", "/api/v1/assets", map[string]any{"id": "shared-actor", "name": "Fixture actor", "type": "vehicle"}, nil)
		if status != 201 {
			t.Fatalf("asset creation %d %v", status, body)
		}
		assetsByRun = append(assetsByRun, body["id"].(string))
	}
	if status, _ := scopeRequest(t, h, tokenA, "GET", "/api/v1/assets/"+assetsByRun[1], nil, nil); status != 404 {
		t.Fatal("foreign asset returned")
	}
	if status, _ := scopeRequest(t, h, tokenA, "POST", "/api/v1/commands", map[string]any{"assetId": assetsByRun[1], "type": "move_to"}, nil); status < 400 {
		t.Fatal("cross-run command accepted")
	}
	if status, body := scopeRequest(t, h, tokenA, "GET", "/api/v1/picture", nil, nil); status != 200 || len(body["tracks"].([]any)) != 0 {
		t.Fatalf("scoped picture failed: %d %v", status, body)
	}
	for i, token := range []string{tokenA, tokenB} {
		input := map[string]any{"id": "same-sample", "messageId": "same-message", "assetId": assetsByRun[i], "observedAt": time.Now().UTC().Format(time.RFC3339Nano), "position": map[string]any{"lat": 24.7, "lng": 46.6}, "connectionState": "connected"}
		for _, want := range []int{201, 200} {
			if status, body := scopeRequest(t, h, token, "POST", "/api/v1/telemetry", input, nil); status != want {
				t.Fatalf("run-local telemetry idempotency: %d %v", status, body)
			}
		}
	}
	if status, body := scopeRequest(t, h, tokenA, "GET", "/api/v1/assets/"+assetsByRun[1]+"/telemetry", nil, nil); status != 200 || body["total"].(float64) != 0 {
		t.Fatalf("foreign history visible: %d %v", status, body)
	}
	for _, path := range []string{"/api/v1/geospatial/assets-within?lat=24.7&lng=46.6&radius_m=1000", "/api/v1/geospatial/nearest-assets?lat=24.7&lng=46.6"} {
		if status, body := scopeRequest(t, h, tokenA, "GET", path, nil, nil); status != 200 || body["total"].(float64) != 1 || body["items"].([]any)[0].(map[string]any)["assetId"] != assetsByRun[0] {
			t.Fatalf("foreign spatial result: %d %v", status, body)
		}
	}
	if status, body := scopeRequest(t, h, tokenA, "GET", "/api/v1/picture?scope="+assetsByRun[1]+"&at="+time.Now().UTC().Format(time.RFC3339Nano), nil, nil); status != 200 || len(body["assets"].([]any)) != 0 {
		t.Fatalf("replay scope selector widened authority: %d %v", status, body)
	}
	status, incident := scopeRequest(t, h, tokenA, "POST", "/api/v1/incidents", map[string]any{"title": "Local incident"}, nil)
	if status != 201 {
		t.Fatalf("incident creation: %d %v", status, incident)
	}
	if status, body := scopeRequest(t, h, tokenA, "POST", "/api/v1/incidents/"+incident["id"].(string)+"/relations", map[string]any{"kind": "asset", "id": assetsByRun[1]}, nil); status != 403 {
		t.Fatalf("foreign incident attachment: %d %v", status, body)
	}
	status, mission := scopeRequest(t, h, tokenA, "POST", "/api/v1/missions", map[string]any{"name": "Local mission"}, nil)
	if status != 201 {
		t.Fatalf("mission creation: %d %v", status, mission)
	}
	if status, _ := scopeRequest(t, h, tokenB, "POST", "/api/v1/missions/"+mission["id"].(string)+"/status", map[string]any{"status": "ACTIVE"}, nil); status != 404 {
		t.Fatalf("foreign mission state transition: %d", status)
	}
	if status, _ := scopeRequest(t, h, tokenA, "POST", "/api/v1/missions/"+mission["id"].(string)+"/assets", map[string]any{"assetId": assetsByRun[1]}, nil); status < 400 {
		t.Fatal("foreign mission attachment accepted")
	}
	bus := events.NewDispatcher(nil)
	assetService := assets.NewService(assets.NewPostgresRepository(h.pool), bus)
	commandService := commands.NewService(commands.NewPostgresRepository(h.pool), assetService, bus)
	if _, err := commandService.Issue(context.Background(), commands.IssueInput{AssetID: assetsByRun[0], Type: "move_to"}); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatalf("direct service bypass: %v", err)
	}
	ctxA := scopeContext(t, h, runA)
	ctxB := scopeContext(t, h, runB)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		for _, ctx := range []context.Context{ctxA, ctxB} {
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				want, _ := access.ResourceFrom(ctx)
				var role, scope string
				if err := platformdb.Scoped(h.pool).QueryRow(ctx, `SELECT current_user,current_setting('emad.scope_id')`).Scan(&role, &scope); err != nil || role != "emad_data" || scope != want.ID() {
					t.Errorf("connection context leak: %q %q (%v)", role, scope, err)
				}
			}(ctx)
		}
	}
	wg.Wait()
	var role string
	if err := h.pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&role); err != nil || role == "emad_data" {
		t.Fatalf("pooled role retained: %q (%v)", role, err)
	}
}
