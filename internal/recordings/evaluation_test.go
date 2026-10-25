package recordings

import (
	"encoding/json"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

func ev(kind, entity string, at int64, v any) Event {
	raw, _ := json.Marshal(v)
	return Event{Kind: kind, Entity: entity, ElapsedMs: at, Operation: "INSERT", Document: raw}
}
func TestEvaluationDenominatorsAssociationsAndTiming(t *testing.T) {
	scene := simscene.Scene{Frozen: simscene.Frozen{Input: simscene.Input{Sensors: []simscene.Sensor{{ID: "s", PeriodMs: 1000, DropEvery: 2}}, Actors: []simscene.Placement{{ID: "hidden-a", DespawnMs: 3000}, {ID: "hidden-b", DespawnMs: 3000}}}}}
	config := ev("configuration", "frozen", 0, map[string]any{"configuration": simruntime.Start{}, "scene": scene})
	a := simulation.Reading{ID: "a", ActorID: "hidden-a", SensorID: "s", Kind: "observation", Local: simulation.Vector{X: 1}}
	b := a
	b.ID = "b"
	b.ActorID = "hidden-b"
	sealed := []Event{config, ev("truth", "1", 0, simruntime.Snapshot{Actors: []simruntime.ActorState{{ID: "hidden-a", Active: true}, {ID: "hidden-b", Active: true}}}), ev("perception", "1", 0, simulation.PerceptionBatch{Readings: []simulation.Reading{a, b, a}, Outcomes: []simulation.TaskOutcome{{CommandID: "c", State: "ACKNOWLEDGED", ElapsedMs: 500}, {CommandID: "c", State: "COMPLETED", ElapsedMs: 1500}}})}
	operator := []Event{ev("observations", "a", 0, map[string]any{"id": "public-a"}), ev("observations", "b", 2000, map[string]any{"id": "public-b"}), ev("track_observations", "a", 0, map[string]any{"track_id": "track-one", "observation_id": "public-a"}), ev("track_observations", "b", 2000, map[string]any{"track_id": "track-one", "observation_id": "public-b"}), ev("commands", "c", 0, map[string]any{"id": "c", "state": "CREATED"}), ev("commands", "c", 1500, map[string]any{"id": "c", "state": "COMPLETED"})}
	report, e := Evaluate(operator, sealed, "COMPLETED", 2500, "operator", "truth")
	if e != nil {
		t.Fatal(e)
	}
	metrics := map[string]Metric{}
	for _, m := range report.Metrics {
		metrics[m.Name] = m
	}
	for name, want := range map[string][2]int{"missed_observations": {4, 6}, "scheduled_misses": {2, 6}, "late_observations": {1, 2}, "merged_tracks": {1, 1}, "fragmented_actors": {0, 2}, "duplicate_samples": {1, 3}, "command_completed": {1, 1}, "scenario_completed": {1, 1}} {
		got := metrics[name]
		if got.Numerator != float64(want[0]) || got.Denominator != want[1] {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	for name, want := range map[string]float64{"mean_observation_delay": 1000, "mean_horizontal_error": 1, "mean_command_ack": 500, "mean_command_completion": 1500} {
		if got := metrics[name].Value; got == nil || *got != want {
			t.Fatalf("%s: %v", name, got)
		}
		m := metrics[name]
		if m.Denominator == 0 || m.Numerator/float64(m.Denominator) != *m.Value {
			t.Fatalf("%s: reported operands do not reproduce the mean: %+v", name, m)
		}
	}
	first, _ := json.Marshal(report)
	again, e := Evaluate(operator, sealed, "COMPLETED", 2500, "operator", "truth")
	second, _ := json.Marshal(again)
	if e != nil || string(first) != string(second) {
		t.Fatal("metric result not reproducible")
	}
}
func TestNativeSampleReferenceNamespace(t *testing.T) {
	sample := "sample_0123456789abcdef0123456789abcdef"
	if sampleReference("scoped_abcdef__observation__"+sample) != sample {
		t.Fatal("native namespace did not resolve")
	}
	if sampleReference("untrusted__observation__actor-a") != "untrusted__observation__actor-a" {
		t.Fatal("arbitrary identity transformed")
	}
}
func TestUnavailableMetricAndDeletedState(t *testing.T) {
	config := ev("configuration", "frozen", 0, map[string]any{"scene": simscene.Scene{}})
	report, e := Evaluate(nil, []Event{config}, "FAILED", 0, "o", "t")
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range report.Metrics {
		if m.Denominator == 0 && m.Value != nil {
			t.Fatal("missing denominator represented as a score", m)
		}
	}
	state := map[string]map[string]json.RawMessage{}
	v := ev("commands", "c", 0, map[string]any{"state": "SENT"})
	apply(state, v)
	v.Operation = "DELETE"
	apply(state, v)
	if len(state["commands"]) != 0 {
		t.Fatal("deleted row survived replay")
	}
}
func TestCheckpointCompatibility(t *testing.T) {
	manifest := Manifest{Schema: Schema, Events: 10, ElapsedMs: 5000}
	cp := Checkpoint{Schema: Schema, Sequence: 10, ElapsedMs: 5000, State: map[string]map[string]json.RawMessage{}}
	raw, _ := json.Marshal(cp)
	if _, e := DecodeCheckpoint(raw, manifest); e != nil {
		t.Fatal(e)
	}
	cp.Schema = "future-schema"
	raw, _ = json.Marshal(cp)
	if _, e := DecodeCheckpoint(raw, manifest); e == nil {
		t.Fatal("unknown checkpoint schema admitted")
	}
	cp.Schema = Schema
	cp.Sequence = 9
	raw, _ = json.Marshal(cp)
	if _, e := DecodeCheckpoint(raw, manifest); e == nil {
		t.Fatal("checkpoint from another recording position admitted")
	}
}

func TestEvaluationFloatingPointReductionsAreReproducible(t *testing.T) {
	scene := simscene.Scene{Frozen: simscene.Frozen{Input: simscene.Input{
		Sensors: []simscene.Sensor{{ID: "s", PeriodMs: 1000}},
		Actors:  []simscene.Placement{{ID: "a", DespawnMs: 1000}, {ID: "b", DespawnMs: 1000}, {ID: "c", DespawnMs: 1000}},
	}}}
	positions := []simruntime.ActorState{{ID: "a", Active: true}, {ID: "b", Active: true}, {ID: "c", Active: true}}
	readings := []simulation.Reading{
		{ID: "a", ActorID: "a", SensorID: "s", Local: simulation.Vector{X: 1e-9}},
		{ID: "b", ActorID: "b", SensorID: "s", Local: simulation.Vector{X: 1e-9}},
		{ID: "c", ActorID: "c", SensorID: "s", Local: simulation.Vector{X: 200}},
	}
	sealed := []Event{
		ev("configuration", "frozen", 0, map[string]any{"scene": scene}),
		ev("truth", "1", 0, simruntime.Snapshot{Actors: positions}),
		ev("perception", "1", 0, simulation.PerceptionBatch{Readings: readings}),
	}
	operator := []Event{ev("observations", "a", 0, map[string]any{"id": "public-a"}), ev("observations", "b", 0, map[string]any{"id": "public-b"}), ev("observations", "c", 0, map[string]any{"id": "public-c"})}
	var baseline string
	for i := 0; i < 100; i++ {
		report, err := Evaluate(operator, sealed, "COMPLETED", 0, "operator", "truth")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			baseline = string(raw)
		} else if string(raw) != baseline {
			t.Fatal("identical journals produced different floating-point aggregates")
		}
	}
}
