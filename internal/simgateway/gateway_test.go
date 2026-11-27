package simgateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

type memBlobStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newMemBlobStore() *memBlobStore {
	return &memBlobStore{data: make(map[string][]byte)}
}

func (m *memBlobStore) Put(_ context.Context, key string, b []byte, _ string) error {
	if !strings.HasSuffix(key, "/"+artifacts.Hash(b)) {
		return errors.New("key must end with content sha256")
	}
	m.mu.Lock()
	m.data[key] = append([]byte(nil), b...)
	m.mu.Unlock()
	return nil
}

func (m *memBlobStore) Get(_ context.Context, key string, _ int64) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), b...), nil
}

func (m *memBlobStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func handlerTransport(h http.Handler) http.RoundTripper {
	return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Result(), nil
	})
}

func bindPrincipal(t *testing.T, ctx context.Context, p access.Principal, r access.Resource) context.Context {
	t.Helper()
	bound, err := access.Bind(ctx, p, r, nil)
	if err != nil {
		t.Fatalf("bind principal: %v", err)
	}
	return bound
}

func TestGatewayExecutionS3PersistenceSSEAndTruthIsolation(t *testing.T) {
	const secret = "loopback-test-secret-xyz"
	var mu sync.Mutex
	pollCounts := map[string]int{}

	sampleSalvoReplay := map[string]any{
		"schema":   "emad.salvo-replay.v1",
		"scenario": map[string]any{"environmentId": "owned-fixture-v1", "threats": []any{map[string]any{"id": "dec-01", "lethal": false}}},
		"sourceDraft": map[string]any{
			"environmentId": "owned-fixture-v1",
			"raid":          map[string]any{"decoys": 2, "uavs": 4},
		},
		"comparison": map[string]any{"wtaVsGreedy": map[string]any{"scoreDelta": 12.5}},
		"runs": map[string]any{
			"combinatorial_wta": map[string]any{
				"policy":     "combinatorial_wta",
				"evaluation": map[string]any{"leakers": 0, "compositeScore": 88.2},
				"truth": map[string]any{
					"scope": "EXERCISE_TRUTH",
					"threats": []any{
						map[string]any{"id": "dec-01", "kind": "decoy", "lethal": false, "samples": []any{}},
					},
					"interceptors": []any{
						map[string]any{"id": "int-1", "batteryId": "sam-1", "batteryTier": "tier1_sam", "trackId": "trk_abc", "launchS": 4.5, "endS": 9.0, "outcome": "kill", "costKUsd": 1200, "samples": []any{}},
					},
					"shrapnelImpacts": []any{map[string]any{"enu": []any{1, 2, 3}}},
				},
				"observations": map[string]any{
					"scope":       "EXERCISE_OBSERVATION",
					"trackFrames": []any{map[string]any{"tS": 4.0, "tracks": []any{map[string]any{"trackId": "trk_abc"}}}},
				},
				"events": []any{
					map[string]any{"tS": 4.5, "kind": "interceptor_launch", "batteryId": "sam-1"},
					map[string]any{"tS": 9.0, "kind": "intercept_kill", "threatId": "dec-01"},
					map[string]any{"tS": 12.0, "kind": "leaker_impact", "threatId": "uav-02"},
				},
			},
		},
	}

	// Mock unified Python hub verifying X-EMAD-Worker-Secret on every call (socket-free)
	workerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" && r.Header.Get(workerSecretHeader) != secret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"worker authentication required"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/health":
			_, _ = w.Write([]byte(`{"status":"ok","service":"emad-sim-hub"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/salvo/runs":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			jid, _ := req["jobId"].(string)
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobId":         jid,
				"environmentId": "owned-fixture-v1",
				"title":         "Fixture Salvo",
				"status":        "running",
				"stage":         "1/3 Running Fusion…",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/salvo/runs/") && strings.HasSuffix(r.URL.Path, "/replay.json"):
			_ = json.NewEncoder(w).Encode(sampleSalvoReplay)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/salvo/runs/"):
			jid := strings.TrimPrefix(r.URL.Path, "/api/salvo/runs/")
			mu.Lock()
			pollCounts[jid]++
			n := pollCounts[jid]
			mu.Unlock()
			if n == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jobId":  jid,
					"status": "running",
					"stage":  "2/3 Comparing WTA…",
				})
				return
			}
			done := time.Now().UTC().Format(time.RFC3339)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobId":       jid,
				"status":      "completed",
				"stage":       "Completed",
				"completedAt": done,
				"policies":    []string{"greedy_earliest", "combinatorial_wta"},
				"elapsedS":    0.42,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/salvo/sessions":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sessionId":"sess_abc123","durationS":60,"policy":"combinatorial_wta","tick":0,"virtualTimeS":0,"observation":{"tracks":[],"batteries":[]}}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/salvo/sessions/") && strings.HasSuffix(r.URL.Path, "/step"):
			_, _ = w.Write([]byte(`{"tick":1,"virtualTimeS":0.25,"completed":false,"policy":"combinatorial_wta","holdFire":true,"observation":{"tracks":[{"trackId":"trk_1"}],"batteries":[]}}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/salvo/sessions/") && strings.HasSuffix(r.URL.Path, "/stop"):
			_, _ = w.Write([]byte(`{"status":"stopped"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	blobs := newMemBlobStore()
	svc := NewService("http://sim-worker.local", secret, 2, true, nil, blobs, nil)
	svc.Worker.HTTP = &http.Client{Transport: handlerTransport(workerHandler)}
	defer svc.Close()

	router := chi.NewRouter()
	svc.Mount(router)

	// 1. Submit async run and stream SSE events to completion
	recStart := httptest.NewRecorder()
	reqStart := httptest.NewRequest(http.MethodPost, "/sim-platform/salvo/runs", bytes.NewBufferString(`{"environmentId":"owned-fixture-v1"}`))
	reqStart.Header.Set("Idempotency-Key", "idemp-salvo-1")
	router.ServeHTTP(recStart, reqStart)
	if recStart.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", recStart.Code, recStart.Body.String())
	}
	var started RunRecord
	if err := json.Unmarshal(recStart.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode started run: %v", err)
	}

	// Idempotent replay returns same jobId
	recDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest(http.MethodPost, "/sim-platform/salvo/runs", bytes.NewBufferString(`{"environmentId":"owned-fixture-v1"}`))
	reqDup.Header.Set("Idempotency-Key", "idemp-salvo-1")
	router.ServeHTTP(recDup, reqDup)
	var dup RunRecord
	_ = json.Unmarshal(recDup.Body.Bytes(), &dup)
	if dup.JobID != started.JobID {
		t.Fatalf("expected idempotent jobId %s, got %s", started.JobID, dup.JobID)
	}

	// Stream SSE until completed event (socket-free via ResponseRecorder)
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sseCancel()
	recSSE := httptest.NewRecorder()
	reqSSE := httptest.NewRequest(http.MethodGet, "/sim-platform/salvo/runs/"+started.JobID+"/events", nil).WithContext(sseCtx)
	router.ServeHTTP(recSSE, reqSSE)
	if !strings.Contains(recSSE.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", recSSE.Header().Get("Content-Type"))
	}
	sawCompleted := false
	scanner := bufio.NewScanner(bytes.NewReader(recSSE.Body.Bytes()))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "event: completed") {
			sawCompleted = true
			break
		}
	}
	if !sawCompleted {
		t.Fatalf("expected SSE stream to emit completed event, got:\n%s", recSSE.Body.String())
	}

	// Verify content-addressed S3 persistence stored both observation & debrief blobs
	doneRec, ok := svc.Store.GetRun(context.Background(), started.JobID)
	if !ok || !doneRec.PersistedToS3 || doneRec.ReplayObsBlobKey == "" || doneRec.ReplayDebriefBlobKey == "" {
		t.Fatalf("expected content-addressed S3 blobs to be persisted: %+v", doneRec)
	}

	// 2. Security negative test: EXERCISE_OBSERVATION operator WITHOUT sim.truth.read
	obsRes := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseObservation, RunID: started.JobID}
	opPrincipal := access.Principal{
		ID:   "local-operator",
		Kind: "human",
		Grants: []access.Grant{
			{Resource: obsRes, Permissions: []string{"operational.read", "observations.read"}},
		},
	}
	opCtx := bindPrincipal(t, context.Background(), opPrincipal, obsRes)

	// Operator requesting debrief MUST receive 403 Forbidden
	recForbidden := httptest.NewRecorder()
	reqForbidden := httptest.NewRequest(http.MethodGet, "/sim-platform/salvo/runs/"+started.JobID+"/replay?view=debrief", nil).WithContext(opCtx)
	router.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for operator requesting debrief truth, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}

	// Operator requesting view=operator (or omitted view) receives strictly redacted EXERCISE_OBSERVATION payload
	recOp := httptest.NewRecorder()
	reqOp := httptest.NewRequest(http.MethodGet, "/sim-platform/salvo/runs/"+started.JobID+"/replay", nil).WithContext(opCtx)
	router.ServeHTTP(recOp, reqOp)
	if recOp.Code != http.StatusOK {
		t.Fatalf("expected 200 for operator view, got %d", recOp.Code)
	}
	if got := recOp.Header().Get("X-EMAD-View-Scope"); got != "EXERCISE_OBSERVATION" {
		t.Fatalf("expected X-EMAD-View-Scope=EXERCISE_OBSERVATION, got %q", got)
	}
	var opDoc map[string]any
	if err := json.Unmarshal(recOp.Body.Bytes(), &opDoc); err != nil {
		t.Fatalf("decode operator replay: %v", err)
	}
	if opDoc["truthRedacted"] != true || opDoc["comparison"] != nil {
		t.Fatalf("expected truthRedacted=true and comparison=nil, got %+v", opDoc)
	}
	if scen, ok := opDoc["scenario"].(map[string]any); ok {
		if _, leaked := scen["threats"]; leaked {
			t.Fatalf("sealed scenario.threats leaked into operator view")
		}
	}
	wtaRun := opDoc["runs"].(map[string]any)["combinatorial_wta"].(map[string]any)
	if _, leaked := wtaRun["truth"]; leaked {
		t.Fatalf("sealed truth object leaked into operator view")
	}
	if wtaRun["evaluation"] != nil {
		t.Fatalf("sealed evaluation leaked into operator view")
	}
	events := wtaRun["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["kind"] != "interceptor_launch" {
		t.Fatalf("expected only operator-visible interceptor_launch event, got %+v", events)
	}

	// 3. Evaluator WITH sim.truth.read unlocks full debrief truth
	evalPrincipal := access.Principal{
		ID:   "local-evaluator",
		Kind: "human",
		Grants: []access.Grant{
			{Resource: obsRes, Permissions: []string{"operational.read", "sim.truth.read"}},
		},
	}
	evalCtx := bindPrincipal(t, context.Background(), evalPrincipal, obsRes)
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodGet, "/sim-platform/salvo/runs/"+started.JobID+"/replay?view=debrief", nil).WithContext(evalCtx)
	router.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusOK {
		t.Fatalf("expected 200 for evaluator debrief, got %d: %s", recEval.Code, recEval.Body.String())
	}
	var evalDoc map[string]any
	_ = json.Unmarshal(recEval.Body.Bytes(), &evalDoc)
	if evalDoc["truthRedacted"] != false {
		t.Fatalf("expected truthRedacted=false for evaluator debrief")
	}
	evalWta := evalDoc["runs"].(map[string]any)["combinatorial_wta"].(map[string]any)
	if _, ok := evalWta["truth"]; !ok {
		t.Fatalf("expected full truth in evaluator debrief")
	}

	// 4. Live Interactive Session start, command enqueue, step & stop
	recSess := httptest.NewRecorder()
	reqSess := httptest.NewRequest(http.MethodPost, "/sim-platform/sessions", bytes.NewBufferString(`{"environmentId":"owned-fixture-v1","startPaused":true}`))
	router.ServeHTTP(recSess, reqSess)
	if recSess.Code != http.StatusCreated {
		t.Fatalf("expected 201 for live session, got %d: %s", recSess.Code, recSess.Body.String())
	}
	var sessSnap LiveSessionSnapshot
	_ = json.Unmarshal(recSess.Body.Bytes(), &sessSnap)
	if sessSnap.SessionID != "sess_abc123" || sessSnap.Status != "paused" {
		t.Fatalf("unexpected live session snapshot: %+v", sessSnap)
	}

	recCmd := httptest.NewRecorder()
	reqCmd := httptest.NewRequest(http.MethodPost, "/sim-platform/sessions/sess_abc123/commands", bytes.NewBufferString(`{"kind":"HOLD_FIRE_ZONE","payload":{"enabled":true}}`))
	router.ServeHTTP(recCmd, reqCmd)
	if recCmd.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for command enqueue, got %d", recCmd.Code)
	}

	recStep := httptest.NewRecorder()
	reqStep := httptest.NewRequest(http.MethodPost, "/sim-platform/sessions/sess_abc123/step", bytes.NewBufferString(`{"ticks":1}`))
	router.ServeHTTP(recStep, reqStep)
	if recStep.Code != http.StatusOK {
		t.Fatalf("expected 200 for step, got %d: %s", recStep.Code, recStep.Body.String())
	}
	var stepped LiveSessionSnapshot
	_ = json.Unmarshal(recStep.Body.Bytes(), &stepped)
	if stepped.Tick != 1 || !stepped.HoldFire {
		t.Fatalf("expected tick=1 and holdFire=true, got %+v", stepped)
	}

	recStop := httptest.NewRecorder()
	router.ServeHTTP(recStop, httptest.NewRequest(http.MethodPost, "/sim-platform/sessions/sess_abc123/stop", nil))
	if recStop.Code != http.StatusOK {
		t.Fatalf("expected 200 for session stop, got %d", recStop.Code)
	}

	// 5. Verify httpx.PolicyFor authorizes all sim-platform routes
	for _, tc := range []struct{ m, p string }{
		{http.MethodGet, "/api/v1/sim-platform/health"},
		{http.MethodGet, "/api/v1/sim-platform/salvo/environments"},
		{http.MethodPost, "/api/v1/sim-platform/salvo/runs"},
		{http.MethodGet, "/api/v1/sim-platform/salvo/runs/salvjob_1234567890ab/events"},
		{http.MethodGet, "/api/v1/sim-platform/salvo/runs/salvjob_1234567890ab/replay"},
		{http.MethodPost, "/api/v1/sim-platform/coverage/los-profile"},
		{http.MethodPost, "/api/v1/sim-platform/bench/monte-carlo"},
		{http.MethodPost, "/api/v1/sim-platform/sessions"},
		{http.MethodPost, "/api/v1/sim-platform/sessions/sess_abc123/commands"},
		{http.MethodPost, "/api/v1/sim-platform/sessions/sess_abc123/control"},
		{http.MethodPost, "/api/v1/sim-platform/sessions/sess_abc123/stop"},
		{http.MethodPost, "/api/v1/sim-platform/sim/runs/simjob_1234567890ab/cancel"},
	} {
		pol := httpx.PolicyFor(tc.m, tc.p)
		if !pol.Known || pol.Permission == "" {
			t.Fatalf("route %s %s not authorized in httpx.PolicyFor: %+v", tc.m, tc.p, pol)
		}
	}
}

func TestJobIDValidationCancelAndBoundedReplayCache(t *testing.T) {
	workerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sim/runs":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"jobId":"simjob_abcdef012345","status":"running","stage":"Running…"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/sim/runs/simjob_abcdef012345":
			_, _ = w.Write([]byte(`{"jobId":"simjob_abcdef012345","status":"running","stage":"Running…"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/sim/runs/simjob_abcdef012345/cancel":
			_, _ = w.Write([]byte(`{"jobId":"simjob_abcdef012345","status":"cancelled","stage":"Cancelled by operator"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewService("http://sim-worker.local", "", 1, true, nil, nil, nil)
	svc.Worker.HTTP = &http.Client{Transport: handlerTransport(workerHandler)}
	defer svc.Close()

	router := chi.NewRouter()
	svc.Mount(router)

	// Invalid client jobId must be rejected with 400 Bad Request
	recBad := httptest.NewRecorder()
	router.ServeHTTP(recBad, httptest.NewRequest(http.MethodPost, "/sim-platform/sim/runs", bytes.NewBufferString(`{"jobId":"../escape"}`)))
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed jobId, got %d: %s", recBad.Code, recBad.Body.String())
	}

	// Valid client jobId is preserved and can be cancelled via unified /runs/{id}/cancel
	recOk := httptest.NewRecorder()
	router.ServeHTTP(recOk, httptest.NewRequest(http.MethodPost, "/sim-platform/sim/runs", bytes.NewBufferString(`{"jobId":"simjob_abcdef012345"}`)))
	if recOk.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for valid jobId, got %d: %s", recOk.Code, recOk.Body.String())
	}
	recCancel := httptest.NewRecorder()
	router.ServeHTTP(recCancel, httptest.NewRequest(http.MethodPost, "/sim-platform/runs/simjob_abcdef012345/cancel", nil))
	if recCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancel, got %d: %s", recCancel.Code, recCancel.Body.String())
	}
	var cancelled RunRecord
	_ = json.Unmarshal(recCancel.Body.Bytes(), &cancelled)
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", cancelled.Status)
	}

	// Bounded replay memory cache caps entries cleanly at maxCachedReplayBlobs
	m := make(map[string][]byte)
	var order []string
	for i := 0; i < maxCachedReplayBlobs+6; i++ {
		key := "job_" + string(rune('a'+i))
		putBoundedBlob(m, &order, key, []byte("payload"))
	}
	if len(m) != maxCachedReplayBlobs || len(order) != maxCachedReplayBlobs {
		t.Fatalf("expected bounded cache size %d, got map=%d order=%d", maxCachedReplayBlobs, len(m), len(order))
	}
}
