package recordings

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"testing"
)

func TestSpatialRetainedTimeAndExactSample(t *testing.T) {
	event := func(kind string, ms int64, v any) Event {
		raw, _ := json.Marshal(v)
		return Event{Kind: kind, ElapsedMs: ms, Document: raw}
	}
	events := []Event{event("truth", 0, simruntime.Snapshot{ElapsedMs: 0, Actors: []simruntime.ActorState{{ID: "hidden", Active: true, Local: simulation.Vector{X: 10}}}}), event("perception", 1000, simulation.PerceptionBatch{Readings: []simulation.Reading{{ActorID: "hidden", Contact: "perceived", SampleMs: 0, DeliverMs: 1000, Local: simulation.Vector{X: 13, Y: 4}, Confidence: .7}}}), event("truth", 2000, simruntime.Snapshot{ElapsedMs: 2000, Actors: []simruntime.ActorState{{ID: "hidden", Active: true, Local: simulation.Vector{X: 20}}}})}
	before, err := spatialFrame(events, 999)
	if err != nil || len(before.Contacts) != 0 {
		t.Fatal("future delivery leaked", before, err)
	}
	at, err := spatialFrame(events, 1500)
	if err != nil || at.TruthSnapshot.ElapsedMs != 0 || len(at.Contacts) != 1 || *at.Contacts[0].ErrorM != 5 {
		t.Fatal("exact sample comparison failed", at, err)
	}
	events = append(events, event("perception", 2500, simulation.PerceptionBatch{Readings: []simulation.Reading{{ActorID: "hidden", Contact: "perceived", SampleMs: 1500, DeliverMs: 2500}}}))
	later, err := spatialFrame(events, 2500)
	if err != nil || later.TruthSnapshot.ElapsedMs != 2000 || later.Contacts[0].ErrorM != nil || later.Contacts[0].TruthAtSample != nil {
		t.Fatal("missing sample truth was inferred", later, err)
	}
}
