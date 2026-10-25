package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/coder/websocket"
)

func TestExplicitHumanGrantChangesAndTruthSeparation(t *testing.T) {
	h := newHarness(t)
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	r := access.Resource{WorkspaceID: "analyst", EnvironmentID: "world", Partition: access.PublicEvidence}
	g := access.Grant{Resource: r, Permissions: []string{"operational.read", "sources.read", "assessments.read"}}
	if err := store.CreateHuman(admin, "public-analyst", "operator-01", g); err != nil {
		t.Fatal(err)
	}
	_, token, err := store.IssueCredential(admin, "public-analyst", r, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	status, me := scopeRequest(t, h, token, "GET", "/api/v1/auth/me", nil, nil)
	if status != 200 || me["principal"].(map[string]any)["kind"] != "human" || me["resource"].(map[string]any)["partition"] != string(access.PublicEvidence) {
		t.Fatalf("human identity contract: %d %v", status, me)
	}
	if status, _ = scopeRequest(t, h, token, "POST", "/api/v1/commands", map[string]any{"assetId": "public-track", "type": "move_to"}, nil); status != 403 {
		t.Fatalf("analyst inherited exercise authority: %d", status)
	}
	if err = store.AddGrant(admin, "public-analyst", access.Grant{Resource: r, Permissions: []string{"sources.manage"}}); err != nil {
		t.Fatal(err)
	}
	if status, _ = scopeRequest(t, h, token, "POST", "/api/v1/sources", map[string]any{"name": "Analyst source", "type": "external"}, nil); status != 201 {
		t.Fatalf("new grant not effective: %d", status)
	}
	if _, err = h.pool.Exec(context.Background(), `DELETE FROM principal_grants WHERE principal_id='public-analyst' AND permissions=ARRAY['sources.manage']`); err != nil {
		t.Fatal(err)
	}
	if status, _ = scopeRequest(t, h, token, "POST", "/api/v1/sources", map[string]any{"name": "Denied source", "type": "external"}, nil); status != 403 {
		t.Fatalf("removed grant remained active: %d", status)
	}
	truth := r
	truth.Partition, truth.RunID = access.ExerciseTruth, "secret"
	if err = store.AddGrant(admin, "public-analyst", access.Grant{Resource: truth, Permissions: []string{"sim.truth.read"}}); err == nil {
		t.Fatal("ordinary administrator delegated truth")
	}
	var changes int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM authorization_changes WHERE subject_id='public-analyst'`).Scan(&changes); err != nil || changes != 2 {
		t.Fatalf("missing provisioning audit: %d (%v)", changes, err)
	}
}

func TestRealtimePinsResourceAndRechecksRevokedCredential(t *testing.T) {
	h := newHarness(t)
	store := access.NewStore(h.pool)
	a := access.Resource{WorkspaceID: "stream", EnvironmentID: "world", Partition: access.PublicEvidence}
	b := a
	b.WorkspaceID = "foreign"
	tokenA := scopeToken(t, h, "stream-a", a, []string{"operational.read", "sources.read", "sources.manage"}, nil)
	tokenB := scopeToken(t, h, "stream-b", b, []string{"sources.manage"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, strings.Replace(h.server.URL, "http", "ws", 1)+"/api/v1/realtime", &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + tokenA}, "X-EMAD-Workspace-ID": {a.WorkspaceID}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if ready := readEnvelope(t, ctx, conn); ready.Type != "realtime.ready" {
		t.Fatal("scoped handshake missing")
	}
	create := func(token, name string) {
		t.Helper()
		if status, body := scopeRequest(t, h, token, "POST", "/api/v1/sources", map[string]any{"name": name, "type": "external"}, nil); status != 201 {
			t.Fatalf("source creation: %d %v", status, body)
		}
	}
	create(tokenB, "foreign event")
	create(tokenA, "selected event")
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err = json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["type"] != "source.created" || strings.Contains(string(raw), "foreign event") || !strings.Contains(string(raw), "selected event") {
		t.Fatalf("wrong resource delivered: %s", raw)
	}
	session, err := store.Authenticate(ctx, tokenA, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeCredential(access.Bootstrap(ctx), session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	create(tokenB, "trigger authorization refresh")
	if _, _, err = conn.Read(ctx); err == nil || ctx.Err() != nil {
		t.Fatalf("revoked client remained subscribed: %v", err)
	}
}
