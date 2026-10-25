package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"strings"
	"testing"
	"time"
)

func nativeGatewayRecoveryProof(t *testing.T, h *harness, input access.Resource, author, worker, sceneID, build, foreignCommand string) {
	t.Helper()
	ctx := context.Background()
	identity := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	call := func(token, method, path string, in any, status int) map[string]any {
		return studioCall(t, h, token, method, path, in, status, nil)
	}
	decode := func(v any, to any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if e := json.Unmarshal(raw, to); e != nil {
			t.Fatal(e)
		}
	}
	var publicBefore int
	if e := h.pool.QueryRow(ctx, `SELECT count(*)FROM observations WHERE split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'`).Scan(&publicBefore); e != nil {
		t.Fatal(e)
	}
	start := simruntime.Start{RequestKey: "gateway-recovery", SceneID: sceneID, RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build, Seed: 51, Epoch: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Scale: 20, DurationMs: 120000}
	id := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	cop := input
	cop.Partition = access.ExerciseObservation
	cop.RunID = id
	permissions := []string{"operational.read", "sources.read", "observations.read", "tracks.read", "assets.read", "telemetry.read", "commands.read", "commands.issue", "commands.transition", "scenarios.read", "alerts.read", "geofences.manage"}
	if e := identity.CreateHuman(admin, "native-recovery-operator", "operator-01", access.Grant{Resource: cop, Permissions: permissions}); e != nil {
		t.Fatal(e)
	}
	_, operator, e := identity.IssueCredential(admin, "native-recovery-operator", cop, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	// Another principal sharing the human operator must retain its own request keys.
	if e = identity.CreateHuman(admin, "native-recovery-peer", "operator-01", access.Grant{Resource: cop, Permissions: permissions}); e != nil {
		t.Fatal(e)
	}
	_, peer, e := identity.IssueCredential(admin, "native-recovery-peer", cop, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	var lease simruntime.Lease
	decode(call(worker, "POST", "/api/v1/simulation-runtime/runs/"+id+"/attach", simruntime.Attach{RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build}, 200), &lease)
	world, e := simruntime.NewWorld(lease.Scene.Frozen)
	if e != nil {
		t.Fatal(e)
	}
	sampler := simruntime.NewSampler(lease.Scene.Frozen, world, 51, lease.Token, lease.Bindings)
	beat := simruntime.Beat{Token: lease.Token, Status: "RUNNING", Snapshot: simruntime.Snapshot{RunID: id, SceneID: sceneID, RuntimeBuild: build, Sequence: 1}}
	send := func(elapsed int64, b simulation.PerceptionBatch, want int) {
		t.Helper()
		beat.Snapshot.ElapsedMs = elapsed
		beat.Snapshot.Actors, e = world.At(elapsed)
		if e != nil {
			t.Fatal(e)
		}
		beat.Batch = b
		call(worker, "POST", "/api/v1/simulation-runtime/runs/"+id+"/heartbeat", beat, want)
	}
	asset := lease.Bindings[0].AssetID
	samples, e := sampler.Advance(0)
	if e != nil {
		t.Fatal(e)
	}
	send(0, simulation.PerceptionBatch{Readings: samples}, 200)
	journalTruthScope := cop
	journalTruthScope.Partition = access.ExerciseTruth
	var recordedBefore int
	if e = h.pool.QueryRow(ctx, `SELECT event_count FROM simulation_recording_limits WHERE scope_id=$1`, journalTruthScope.ID()).Scan(&recordedBefore); e != nil {
		t.Fatal(e)
	}
	send(0, beat.Batch, 200)
	var recordedAfter int
	if e = h.pool.QueryRow(ctx, `SELECT event_count FROM simulation_recording_limits WHERE scope_id=$1`, journalTruthScope.ID()).Scan(&recordedAfter); e != nil || recordedBefore != recordedAfter {
		t.Fatal("duplicate heartbeat created recording output", recordedBefore, recordedAfter, e)
	}
	altered := beat
	altered.Snapshot.Sequence = 3
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+id+"/heartbeat", altered, 409)
	altered = beat
	altered.Snapshot.ElapsedMs = 1
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+id+"/heartbeat", altered, 409)
	other := scopeToken(t, h, "native-recovery-other-worker", input, []string{"operational.read", "sim.runtime.execute"}, nil)
	call(other, "POST", "/api/v1/simulation-runtime/runs/"+id+"/heartbeat", beat, 403)
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+id+"/attach", simruntime.Attach{RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build}, 409)
	beat.Snapshot.Sequence = 2
	send(1000, simulation.PerceptionBatch{Outcomes: []simulation.TaskOutcome{{CommandID: foreignCommand, State: "ACKNOWLEDGED", ElapsedMs: 1000}}}, 403)
	// A failure after private admission must be recoverable using the identical
	// heartbeat after reconnect, without duplicating evidence or advancing time.
	samples, e = sampler.Advance(1000)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.pool.Exec(ctx, `REVOKE INSERT ON observations FROM emad_data`); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = h.pool.Exec(context.Background(), `GRANT INSERT ON observations TO emad_data`) })
	send(1000, simulation.PerceptionBatch{Readings: samples}, 403)
	if _, e = h.pool.Exec(ctx, `GRANT INSERT ON observations TO emad_data`); e != nil {
		t.Fatal(e)
	}
	send(1000, beat.Batch, 200)
	send(1000, beat.Batch, 200)
	if v := call(operator, "GET", "/api/v1/observations", nil, 200); v["total"].(float64) != 3 {
		t.Fatal("reconnect duplicated samples", v)
	}
	var pending int
	truthScope := cop
	truthScope.Partition = access.ExerciseTruth
	if e = h.pool.QueryRow(ctx, `SELECT count(*)FROM native_projection_batches WHERE scope_id=$1 AND NOT projected`, truthScope.ID()).Scan(&pending); e != nil || pending != 0 {
		t.Fatal("outbox not recovered", pending, e)
	}
	issue := map[string]any{"assetId": asset, "type": "HOLD", "requestKey": "shared-operator-key"}
	oversized := map[string]any{"assetId": asset, "type": strings.Repeat("X", 81), "requestKey": "bounded-type"}
	call(operator, "POST", "/api/v1/commands", oversized, 409)
	oversized["type"] = "HOLD"
	oversized["payload"] = map[string]any{"note": strings.Repeat("X", 9000)}
	call(operator, "POST", "/api/v1/commands", oversized, 409)
	command := call(operator, "POST", "/api/v1/commands", issue, 201)["id"].(string)
	if second := call(peer, "POST", "/api/v1/commands", issue, 201)["id"].(string); second == command {
		t.Fatal("request key was shared between principals")
	}
	call(operator, "POST", "/api/v1/commands/"+command+"/transition", map[string]any{"state": "COMPLETED"}, 403)
	beat.Snapshot.Sequence = 3
	send(1500, simulation.PerceptionBatch{Outcomes: []simulation.TaskOutcome{{CommandID: command, State: "ACKNOWLEDGED", ElapsedMs: 1500}}}, 200)
	send(1500, beat.Batch, 200)
	if v := call(operator, "GET", "/api/v1/commands/"+command, nil, 200); v["state"] != "ACKNOWLEDGED" {
		t.Fatal(v)
	}
	beat.Snapshot.Sequence = 4
	send(25000, simulation.PerceptionBatch{}, 200)
	beat.Snapshot.Sequence = 5
	send(42000, simulation.PerceptionBatch{}, 200)
	if v := call(operator, "GET", "/api/v1/commands/"+command, nil, 200); v["state"] != "TIMED_OUT" {
		t.Fatal("acknowledged task missed virtual deadline", v)
	}
	issue["requestKey"] = "pending-at-stop"
	pendingCommand := call(operator, "POST", "/api/v1/commands", issue, 201)["id"].(string)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+id+"/stop", nil, 200)
	if v := call(operator, "GET", "/api/v1/commands/"+pendingCommand, nil, 200); v["state"] != "CANCELLED" {
		t.Fatal("stop did not cancel pending dispatch", v)
	}
	beat.Snapshot.Sequence = 6
	beat.Status = "STOPPED"
	send(42000, simulation.PerceptionBatch{}, 200)
	// A fresh run starts with independent contacts, assets, task queues and lease.
	start.RequestKey = "gateway-expiry"
	next := call(author, "POST", "/api/v1/simulation-runtime/runs", start, 201)["id"].(string)
	altered = beat
	altered.Snapshot.RunID = next
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+next+"/heartbeat", altered, 403)

	nextCOP := cop
	nextCOP.RunID = next
	if e = identity.AddGrant(admin, "native-recovery-operator", access.Grant{Resource: nextCOP, Permissions: permissions}); e != nil {
		t.Fatal(e)
	}
	_, nextOperator, e := identity.IssueCredential(admin, "native-recovery-operator", nextCOP, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	var nextLease simruntime.Lease
	decode(call(worker, "POST", "/api/v1/simulation-runtime/runs/"+next+"/attach", simruntime.Attach{RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build}, 200), &nextLease)
	if nextLease.Bindings[0].AssetID == asset {
		t.Fatal("new run reused controlled identity")
	}
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+next+"/heartbeat", altered, 403)
	pendingOnLoss := call(nextOperator, "POST", "/api/v1/commands", map[string]any{"assetId": nextLease.Bindings[0].AssetID, "type": "HOLD", "requestKey": "pending-on-lease-loss"}, 201)
	if _, e = h.pool.Exec(ctx, `UPDATE simulation_runtime_runs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1 AND scope_id=$2`, next, input.ID()); e != nil {
		t.Fatal(e)
	}
	if v := call(author, "GET", "/api/v1/simulation-runtime/runs/"+next, nil, 200); v["status"] != "FAILED" {
		t.Fatal("lost lease remained active", v)
	}
	if v := call(nextOperator, "GET", "/api/v1/commands/"+pendingOnLoss["id"].(string), nil, 200); v["state"] != "CANCELLED" {
		t.Fatal("lost lease retained dispatch", v)
	}
	if v := call(nextOperator, "GET", "/api/v1/observations", nil, 200); v["total"].(float64) != 0 {
		t.Fatal("old run observations contaminated fresh run", v)
	}
	call(worker, "POST", "/api/v1/simulation-runtime/runs/"+next+"/attach", simruntime.Attach{RuntimeVersion: simscene.PerceptionRuntime, RuntimeBuild: build}, 409)
	call(author, "POST", "/api/v1/simulation-runtime/runs/"+next+"/stop", nil, 200)

	var publicAfter int
	if e = h.pool.QueryRow(ctx, `SELECT count(*)FROM observations WHERE split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'`).Scan(&publicAfter); e != nil || publicBefore != publicAfter {
		t.Fatal("exercise changed public evidence", publicBefore, publicAfter, e)
	}
}
