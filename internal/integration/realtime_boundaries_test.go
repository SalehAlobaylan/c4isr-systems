package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/presentation"
	"github.com/SalehAlobaylan/c4isr-systems/internal/realtime"
	"github.com/coder/websocket"
)

func readEnvelope(t *testing.T, ctx context.Context, conn *websocket.Conn) realtime.Envelope {
	t.Helper()
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var env realtime.Envelope
	if err = json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env
}
func dialScope(t *testing.T, h *harness, token, query string) (*websocket.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, strings.Replace(h.server.URL, "http", "ws", 1)+"/api/v1/realtime"+query, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn, ctx
}

func TestRealtimeThreeResourcesAndOpaqueExerciseEvidence(t *testing.T) {
	h := newHarness(t)
	resources := []access.Resource{{WorkspaceID: "exercise", EnvironmentID: "lab", Partition: access.ExerciseObservation, RunID: "one"}, {WorkspaceID: "exercise", EnvironmentID: "lab", Partition: access.ExerciseObservation, RunID: "two"}, {WorkspaceID: "public", EnvironmentID: "world", Partition: access.PublicEvidence}}
	tokens, sources := []string{}, []string{}
	connections := []*websocket.Conn{}
	contexts := []context.Context{}
	for i, r := range resources {
		token := scopeToken(t, h, "subscriber-"+r.RunID+string(r.Partition), r, []string{"operational.read", "sources.read", "sources.manage", "observations.read", "observations.ingest", "tracks.read", "audit.read", "assessments.create", "assessments.read", "scenarios.read"}, nil)
		tokens = append(tokens, token)
		status, source := scopeRequest(t, h, token, "POST", "/api/v1/sources", map[string]any{"id": "sensor", "name": "PRIVATE_ACTOR_SENTINEL", "type": "sensor", "metadata": map[string]any{"actorId": "PRIVATE_ACTOR_SENTINEL"}}, nil)
		if status != 201 {
			t.Fatal(source)
		}
		sources = append(sources, source["id"].(string))
		if i < 2 && strings.Contains(string(mustJSON(t, source)), "PRIVATE_ACTOR_SENTINEL") {
			t.Fatal("source label disclosed private input")
		}
		conn, ctx := dialScope(t, h, token, "?topics=observation.received")
		ready := readEnvelope(t, ctx, conn)
		if ready.Type != "realtime.ready" || ready.Version != 2 || ready.Resource != r {
			t.Fatalf("invalid scoped handshake: %+v", ready)
		}
		connections = append(connections, conn)
		contexts = append(contexts, ctx)
	}
	firstAlias := ""
	for i, r := range resources {
		input := map[string]any{"id": "event__PRIVATE_ACTOR_SENTINEL", "sourceId": sources[i], "type": "track.observation", "trackHint": "PRIVATE_ACTOR_SENTINEL", "observedAt": time.Now().UTC().Format(time.RFC3339Nano), "position": map[string]any{"lat": 24.7, "lng": 46.6}, "payload": map[string]any{"innocent": map[string]any{"actor": "PRIVATE_ACTOR_SENTINEL"}}, "quality": map[string]any{"accuracyM": 12, "actorId": "PRIVATE_ACTOR_SENTINEL"}}
		status, obs := scopeRequest(t, h, tokens[i], "POST", "/api/v1/observations", input, nil)
		if status != 201 {
			t.Fatalf("ingest %d %v", status, obs)
		}
		if i == 0 {
			firstAlias = obs["id"].(string)
		}
		if i < 2 {
			if strings.Contains(string(mustJSON(t, obs)), "PRIVATE_ACTOR_SENTINEL") || !presentation.IsObservationAlias(obs["id"].(string)) {
				t.Fatal("exercise response disclosed private identity")
			}
			if status, detail := scopeRequest(t, h, tokens[i], "GET", "/api/v1/observations/"+obs["id"].(string), nil, nil); status != 200 || detail["id"] != obs["id"] {
				t.Fatalf("opaque evidence detail: %d %v", status, detail)
			}
			if status, body := scopeRequest(t, h, tokens[i], "POST", "/api/v1/assessments", map[string]any{"subjectType": "observation", "subjectId": obs["id"], "type": "review", "conclusion": "Observed evidence", "evidence": []any{map[string]any{"type": "observation", "id": obs["id"]}}}, nil); status != 201 {
				t.Fatalf("opaque evidence reference: %d %v", status, body)
			}
			if status, tracks := scopeRequest(t, h, tokens[i], "GET", "/api/v1/tracks", nil, nil); status != 200 || strings.Contains(string(mustJSON(t, tracks)), "PRIVATE_ACTOR_SENTINEL") {
				t.Fatalf("track metadata: %d %v", status, tracks)
			}
			if status, audit := scopeRequest(t, h, tokens[i], "GET", "/api/v1/audit", nil, nil); status != 200 || strings.Contains(string(mustJSON(t, audit)), "PRIVATE_ACTOR_SENTINEL") {
				t.Fatalf("audit disclosure: %d %v", status, audit)
			}
		} else if !strings.Contains(string(mustJSON(t, obs)), "PRIVATE_ACTOR_SENTINEL") {
			t.Fatal("raw public evidence rewritten")
		}
		env := readEnvelope(t, contexts[i], connections[i])
		if env.Type != "observation.received" || env.Resource != r || env.Data["sourceId"] != sources[i] {
			t.Fatalf("cross-resource event: %+v", env)
		}
		if i < 2 && strings.Contains(string(mustJSON(t, env)), "PRIVATE_ACTOR_SENTINEL") {
			t.Fatal("serialized exercise frame disclosed identity")
		}
	}
	// A copied alias cannot resolve in another run, and inspection is an author capability.
	alias := firstAlias
	if status, _ := scopeRequest(t, h, tokens[1], "GET", "/api/v1/observations/"+alias, nil, nil); status != 404 {
		t.Fatalf("foreign alias resolved: %d", status)
	}
	if status, _ := scopeRequest(t, h, tokens[0], "GET", "/api/v1/scenarios/runs/one/events", nil, nil); status != 403 {
		t.Fatalf("ordinary operator inspected scheduled input: %d", status)
	}
	// The original records and nested evidence remain intact in storage.
	var retained int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM observations WHERE payload::text LIKE '%PRIVATE_ACTOR_SENTINEL%'`).Scan(&retained); err != nil || retained != 3 {
		t.Fatalf("stored evidence changed: %d %v", retained, err)
	}
	owned := scopeContext(t, h, resources[0])
	var internalID, publicID string
	if err := platformdb.Scoped(h.pool).QueryRow(owned, `SELECT id,public_id FROM observations`).Scan(&internalID, &publicID); err != nil || publicID != firstAlias || !strings.Contains(internalID, "PRIVATE_ACTOR_SENTINEL") {
		t.Fatalf("stored identity/reference separation failed: %v", err)
	}
	if _, err := platformdb.Scoped(h.pool).Exec(owned, `UPDATE observations SET public_id='view_obs_00000000000000000000000000000000'`); !apperr.Is(err, apperr.CodeForbidden) {
		t.Fatalf("published reference was mutable: %v", err)
	}
}

func TestRealtimeDenialIdleRevocationAndReconnect(t *testing.T) {
	h := newHarness(t)
	r := access.Resource{WorkspaceID: "stream", EnvironmentID: "world", Partition: access.PublicEvidence}
	token := scopeToken(t, h, "reader", r, []string{"operational.read", "sources.read"}, nil)
	for _, query := range []string{"?topics=observation.received", "?topics=simulation.truth.snapshot", "?topics=exercise.input.changed", "?workspaceId=foreign&topics=source.created"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, resp, err := websocket.Dial(ctx, strings.Replace(h.server.URL, "http", "ws", 1)+"/api/v1/realtime"+query, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + token}}})
		cancel()
		if conn != nil {
			conn.CloseNow()
		}
		if err == nil || resp == nil || resp.StatusCode != 403 {
			t.Fatalf("unauthorized subscription %s accepted: %v", query, err)
		}
	}
	conn, ctx := dialScope(t, h, token, "?topics=source.created")
	readEnvelope(t, ctx, conn)
	conn.CloseNow()
	conn, ctx = dialScope(t, h, token, "?topics=source.created")
	ready := readEnvelope(t, ctx, conn)
	if ready.Data["resyncRequired"] != true || ready.Resource != r {
		t.Fatal("reconnect did not require an authorized snapshot")
	}
	store := access.NewStore(h.pool)
	session, err := store.Authenticate(context.Background(), token, func(string) []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeCredential(access.Bootstrap(context.Background()), session.Principal.CredentialID); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.Read(ctx)
	if err == nil || ctx.Err() != nil {
		t.Fatalf("idle revoked subscriber survived: %v", err)
	}
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("revocation did not send policy close: %v", err)
	}
	if status, _ := scopeRequest(t, h, token, "GET", "/api/v1/sources", nil, nil); status != 401 {
		t.Fatal("revoked reconnect credential accepted by snapshot")
	}
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
