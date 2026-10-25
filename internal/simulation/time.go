package simulation

import (
	"fmt"
	"time"
)

// TimeMap preserves source evidence time separately from its scenario placement.
// An offset imports historical context without rewriting the original timestamp.
type TimeMap struct {
	Mode          string    `json:"mode"`
	SourceEpoch   time.Time `json:"sourceEpoch"`
	ScenarioEpoch time.Time `json:"scenarioEpoch"`
}

func (m TimeMap) Map(source time.Time) (time.Time, error) {
	if source.IsZero() {
		return time.Time{}, fmt.Errorf("source time is unresolved")
	}
	switch m.Mode {
	case "preserve":
		return source, nil
	case "offset":
		if m.SourceEpoch.IsZero() || m.ScenarioEpoch.IsZero() {
			return time.Time{}, fmt.Errorf("time mapping requires both anchors")
		}
		delta := source.Sub(m.SourceEpoch)
		if !m.SourceEpoch.Add(delta).Equal(source) {
			return time.Time{}, fmt.Errorf("time mapping duration overflows")
		}
		return m.ScenarioEpoch.Add(delta), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported source-to-scenario time mapping")
	}
}
