package simruntime

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"testing"
)

func TestPresetAttributionPreservesSamplingBaseline(t *testing.T) {
	original := perceptionScene(t)
	pinned := perceptionScene(t)
	// Preset provenance has no physical or probabilistic effect. Exact numeric
	// settings, actor/observer identity and seed still define this abstract model.
	pinned.Input.Sensors[0].PresetID = "sensor_preset_fixture"
	a, e := NewWorld(original)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewWorld(pinned)
	if e != nil {
		t.Fatal(e)
	}
	bindings := []simulation.ControlledBinding{{ActorID: "hidden-a", AssetID: "public-own-asset"}}
	sa := NewSampler(original, a, 51, "same-lease", bindings)
	sb := NewSampler(pinned, b, 51, "same-lease", bindings)
	for at := int64(0); at <= 5000; at += 500 {
		first, e := sa.Advance(at)
		if e != nil {
			t.Fatal(e)
		}
		second, e := sb.Advance(at)
		if e != nil {
			t.Fatal(e)
		}
		x, _ := json.Marshal(first)
		y, _ := json.Marshal(second)
		if string(x) != string(y) {
			t.Fatalf("preset changed behavior at %d", at)
		}
	}
}
