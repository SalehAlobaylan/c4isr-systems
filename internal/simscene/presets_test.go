package simscene

import "testing"

func TestPresetFrozenProvenanceAndEditIsolation(t *testing.T) {
	input := PresetInput{Name: "Delayed noisy observer", PeriodMs: 1000, DropEvery: 3, DelayMs: 1500, NoiseM: 30, Confidence: .6, Limitations: "Abstract sampler; no physical validation"}
	p, e := preset(input)
	if e != nil {
		t.Fatal(e)
	}
	sensor := input.Sensor("observer")
	sensor.PresetID = p.ID
	f := Frozen{Input: Input{Sensors: []Sensor{sensor}}, SensorPresets: []SensorPreset{p}}
	if e = f.ValidatePresets(); e != nil {
		t.Fatal(e)
	}
	changed := input
	changed.DelayMs = 2000
	revision, e := preset(changed)
	if e != nil || revision.ID == p.ID {
		t.Fatal("preset revision overwrote identity")
	}
	f.Input.Sensors[0].NoiseM++
	if f.ValidatePresets() == nil {
		t.Fatal("changed values retained original preset attribution")
	}
	f.Input.Sensors[0].PresetID = ""
	f.SensorPresets = nil
	if e = f.ValidatePresets(); e != nil {
		t.Fatal("custom settings no longer supported", e)
	}
	invalid := input
	invalid.PeriodMs = 0
	if _, e = preset(invalid); e == nil {
		t.Fatal("invalid preset admitted")
	}
}
