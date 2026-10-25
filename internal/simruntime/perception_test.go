package simruntime

import (
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"math"
	"strings"
	"testing"
)

func perceptionScene(t *testing.T) simscene.Frozen {
	t.Helper()
	h := 0.0
	frame := simulation.DefaultFrame(simulation.Position{Longitude: 39.2, Latitude: 21.5, Height: &h, VerticalDatum: "WGS84_ELLIPSOID"})
	p, e := frame.FromLocal(simulation.Vector{X: 1000})
	if e != nil {
		t.Fatal(e)
	}
	return simscene.Frozen{Runtime: simscene.PerceptionRuntime, Environment: modelassets.EnvironmentAsset{Frame: frame}, Input: simscene.Input{Actors: []simscene.Placement{{ID: "hidden-a", Position: frame.Origin, Waypoints: []simulation.Position{p}, SpeedMPS: 10, DespawnMs: 60000}}, Sensors: []simscene.Sensor{{ID: "hidden-sensor", PeriodMs: 1000, DropEvery: 2, DelayMs: 1000, NoiseM: 25, Confidence: .6}}, Controlled: []simscene.Controlled{{ActorID: "hidden-a", Callsign: "TRAINER", TelemetryMs: 1000}}}}
}
func TestPerceptionMissingDelayNoiseAndPrivateIdentity(t *testing.T) {
	f := perceptionScene(t)
	w, e := NewWorld(f)
	if e != nil {
		t.Fatal(e)
	}
	bindings := []simulation.ControlledBinding{{ActorID: "hidden-a", AssetID: "public-own-asset"}}
	sampler := NewSampler(f, w, 51, "lease-secret", bindings)
	first, e := sampler.Advance(0)
	if e != nil || len(first) != 1 || first[0].Kind != "telemetry" {
		t.Fatalf("delayed sensor appeared at zero: %+v %v", first, e)
	}
	second, e := sampler.Advance(1000)
	if e != nil || len(second) != 2 {
		t.Fatalf("sample delivery: %+v %v", second, e)
	}
	var observation simulation.Reading
	for _, r := range second {
		if r.Kind == "observation" {
			observation = r
		}
	}
	if observation.SampleMs != 0 || observation.DeliverMs != 1000 || observation.Confidence != .6 || observation.AccuracyM != math.Sqrt2*25 {
		t.Fatal(observation)
	}
	if strings.Contains(observation.Contact, "hidden") || observation.Contact == "public-own-asset" {
		t.Fatal("identity not separated")
	}
	if math.Abs(observation.Local.X) > 25 || math.Abs(observation.Local.Y) > 25 || observation.Local.X == 0 && observation.Local.Y == 0 {
		t.Fatal("noise invalid", observation)
	}
	missed, e := sampler.Advance(2000)
	if e != nil || len(missed) != 1 || missed[0].Kind != "telemetry" {
		t.Fatal("scheduled missing sample published", missed, e)
	}
	b := simulation.PerceptionBatch{Readings: second}
	if e = ValidateBatch(f, b, 1000); e != nil {
		t.Fatal(e)
	}
	if e = ValidateBatchIdentity(b, "lease-secret"); e != nil {
		t.Fatal(e)
	}
	if e = ValidateBatchIdentity(b, "another-run-lease"); e == nil {
		t.Fatal("cross-lease identity accepted")
	}
	observation.SampleMs = 1000
	observation.DeliverMs = 2000
	if e = ValidateBatch(f, simulation.PerceptionBatch{Readings: []simulation.Reading{observation}}, 2000); e == nil {
		t.Fatal("frozen miss schedule ignored")
	}
	other := NewSampler(f, w, 51, "different-lease", bindings)
	_, _ = other.Advance(0)
	same, _ := other.Advance(1000)
	for _, r := range same {
		if r.Kind == "observation" && (r.Local != b.Readings[0].Local && r.Local != b.Readings[1].Local) {
			t.Fatal("seeded noise changed between runs")
		}
	}
	sameSampler := NewSampler(f, w, 51, "lease-secret", bindings)
	_, _ = sameSampler.Advance(0)
	again, _ := sameSampler.Advance(1000)
	x, _ := json.Marshal(second)
	y, _ := json.Marshal(again)
	if string(x) != string(y) {
		t.Fatal("repeatable samples changed")
	}
}
func TestTaskRunnerDelayedAckDeduplicationFailureAndHold(t *testing.T) {
	f := perceptionScene(t)
	w, e := NewWorld(f)
	if e != nil {
		t.Fatal(e)
	}
	r := NewTaskRunner(w, []simulation.ControlledBinding{{ActorID: "hidden-a", AssetID: "own"}})
	task := simulation.RuntimeTask{CommandID: "cmd-hold", AssetID: "own", Type: "HOLD", DeadlineMs: 10000}
	if len(r.Advance([]simulation.RuntimeTask{task}, 0)) != 0 {
		t.Fatal("immediate acknowledgement")
	}
	if len(r.Advance([]simulation.RuntimeTask{task}, 499)) != 0 {
		t.Fatal("early acknowledgement")
	}
	if out := r.Advance(nil, 500); len(out) != 1 || out[0].State != "ACKNOWLEDGED" {
		t.Fatal(out)
	}
	if out := r.Advance(nil, 1000); len(out) != 1 || out[0].State != "COMPLETED" {
		t.Fatal(out)
	}
	a, _ := w.At(1000)
	b, _ := w.At(5000)
	if math.Abs(a[0].Local.X-b[0].Local.X) > 1e-5 {
		t.Fatal("hold failed")
	}
	if len(r.Advance([]simulation.RuntimeTask{task}, 6000)) != 0 {
		t.Fatal("duplicate task re-executed")
	}
	task.CommandID = "cmd-resume"
	task.Type = "RESUME_ROUTE"
	task.IssuedMs = 6000
	_ = r.Advance([]simulation.RuntimeTask{task}, 6000)
	_ = r.Advance(nil, 6500)
	_ = r.Advance(nil, 7000)
	b, _ = w.At(8000)
	if math.Abs(b[0].Local.X-a[0].Local.X-10) > 1e-5 {
		t.Fatal("resume failed", a, b)
	}
	task.CommandID = "cmd-reject"
	task.Type = "UNKNOWN"
	_ = r.Advance([]simulation.RuntimeTask{task}, 8000)
	if out := r.Advance(nil, 8500); len(out) != 1 || out[0].State != "REJECTED" {
		t.Fatal(out)
	}
	task.CommandID = "cmd-fail"
	task.Type = "HOLD"
	task.AssetID = "unbound"
	_ = r.Advance([]simulation.RuntimeTask{task}, 8500)
	_ = r.Advance(nil, 9000)
	if out := r.Advance(nil, 9500); len(out) != 1 || out[0].State != "FAILED" {
		t.Fatal(out)
	}
	task.CommandID = "cmd-expired"
	task.DeadlineMs = 9500
	_ = r.Advance([]simulation.RuntimeTask{task}, 10000)
	if len(r.Advance(nil, 11000)) != 0 {
		t.Fatal("expired task executed")
	}
}

func TestTaskCancellationAfterReconnectDoesNotExecute(t *testing.T) {
	f := perceptionScene(t)
	w, e := NewWorld(f)
	if e != nil {
		t.Fatal(e)
	}
	r := NewTaskRunner(w, []simulation.ControlledBinding{{ActorID: "hidden-a", AssetID: "own"}})
	task := simulation.RuntimeTask{CommandID: "cancelled-command", AssetID: "own", Type: "HOLD", DeadlineMs: 10000}
	r.Sync([]simulation.RuntimeTask{task}, 0)
	if out := r.Advance(nil, 500); len(out) != 1 || out[0].State != "ACKNOWLEDGED" {
		t.Fatal(out)
	}
	r.Sync(nil, 600)
	if out := r.Advance(nil, 1000); len(out) != 0 {
		t.Fatal("cancelled task completed", out)
	}
	a, _ := w.At(1000)
	b, _ := w.At(2000)
	if math.Abs(b[0].Local.X-a[0].Local.X-10) > 1e-5 {
		t.Fatal("cancelled hold executed")
	}
	if out := r.Sync([]simulation.RuntimeTask{task}, 3000); len(out) != 0 {
		t.Fatal("cancelled identity re-executed", out)
	}
}

func TestDispatchRosterIsCompleteButExecutionIsBounded(t *testing.T) {
	f := perceptionScene(t)
	w, e := NewWorld(f)
	if e != nil {
		t.Fatal(e)
	}
	r := NewTaskRunner(w, []simulation.ControlledBinding{{ActorID: "hidden-a", AssetID: "own"}})
	tasks := make([]simulation.RuntimeTask, 40)
	for i := range tasks {
		tasks[i] = simulation.RuntimeTask{CommandID: fmt.Sprintf("cmd-%02d", i), AssetID: "own", Type: "HOLD", DeadlineMs: 10000}
	}
	r.Sync(tasks, 0)
	if got := len(r.Advance(nil, 500)); got != 32 {
		t.Fatalf("active ACK window: %d", got)
	}
	if got := len(r.Advance(nil, 1000)); got != 32 {
		t.Fatalf("active completion window: %d", got)
	}
	// A changed ordering of the full roster must not cancel queued work.
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}
	r.Sync(tasks, 1000)
	if got := len(r.Advance(nil, 1500)); got != 8 {
		t.Fatalf("remaining ACKs: %d", got)
	}
	if got := len(r.Advance(nil, 2000)); got != 8 {
		t.Fatalf("remaining completions: %d", got)
	}
}
