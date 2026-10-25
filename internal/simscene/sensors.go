package simscene

import (
	"fmt"
	"strings"
)

// Sensor is an abstract sampler, not a model of a physical radar or camera.
// Identity/coverage configuration is protected authoring input.
type Sensor struct {
	PresetID   string  `json:"presetId,omitempty"`
	ID         string  `json:"id"`
	PeriodMs   int64   `json:"periodMs"`
	DropEvery  int     `json:"dropEvery"`
	DelayMs    int64   `json:"delayMs"`
	NoiseM     float64 `json:"noiseM"`
	Confidence float64 `json:"confidence"`
}
type Controlled struct {
	ActorID     string `json:"actorId"`
	Callsign    string `json:"callsign"`
	TelemetryMs int64  `json:"telemetryMs"`
}

const PerceptionRuntime = "emad-kinematic-v2"

func CompatibleRuntime(v string) bool { return v == RuntimeVersion || v == PerceptionRuntime }
func (in Input) Runtime() string {
	if len(in.Sensors)+len(in.Controlled) > 0 {
		return PerceptionRuntime
	}
	return RuntimeVersion
}
func (in Input) ValidateSensors() error {
	if len(in.Sensors) > 2 || len(in.Controlled) > 16 {
		return fmt.Errorf("sensor/controlled-actor budget exceeded")
	}
	seen := map[string]bool{}
	actors := map[string]bool{}
	for _, a := range in.Actors {
		actors[a.ID] = true
	}
	for _, s := range in.Sensors {
		if !actorID.MatchString(s.ID) || seen[s.ID] || s.PeriodMs < 1000 || s.PeriodMs > 60000 || s.DropEvery < 0 || s.DropEvery > 1000 || s.DelayMs < 0 || s.DelayMs > 10000 || !finite(s.NoiseM) || s.NoiseM < 0 || s.NoiseM > 200 || !finite(s.Confidence) || s.Confidence < 0 || s.Confidence > 1 {
			return fmt.Errorf("invalid abstract sensor contract")
		}
		seen[s.ID] = true
	}
	seen = map[string]bool{}
	for _, c := range in.Controlled {
		if !actors[c.ActorID] || seen[c.ActorID] || len(strings.TrimSpace(c.Callsign)) < 1 || len(c.Callsign) > 80 || c.TelemetryMs < 1000 || c.TelemetryMs > 60000 {
			return fmt.Errorf("invalid controlled actor declaration")
		}
		seen[c.ActorID] = true
	}
	return nil
}
func (in Input) SampleBudget(duration int64) int64 {
	var n int64
	for _, s := range in.Sensors {
		n += (duration/s.PeriodMs + 1) * int64(len(in.Actors))
	}
	for _, c := range in.Controlled {
		n += duration/c.TelemetryMs + 1
	}
	return n
}
