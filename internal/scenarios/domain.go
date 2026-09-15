// Package scenarios implements the deterministic scenario engine. From the
// core platform's perspective the runner is an external information source:
// it registers sources/assets/geofences through the same application services
// used by future real integrations, then emits normalized observations and
// telemetry. It never writes to the database directly.
package scenarios

import (
	"fmt"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Run statuses.
const (
	StatusRunning   = "RUNNING"
	StatusPaused    = "PAUSED"
	StatusCompleted = "COMPLETED"
	StatusStopped   = "STOPPED"
	StatusFailed    = "FAILED"
)

// Action names supported in scenario event specs.
const (
	ActionObserve      = "observe"
	ActionMove         = "move"
	ActionClassify     = "classify"
	ActionConnection   = "connection"
	ActionIssueCommand = "issue_command"
)

// Defaults for scenario timing.
const (
	defaultPlaybackSpeed  = 1.0
	defaultTelemetryMs    = 1000
	defaultObserveUntilMs = 120000
	maxScheduledActions   = 20000
	defaultAckDelayMs     = 3000
	defaultCompleteDelay  = 5000
)

// Scenario is the parsed YAML definition of a deterministic operational run.
type Scenario struct {
	Name              string          `yaml:"scenario" json:"name"`
	Description       string          `yaml:"description" json:"description"`
	Seed              int64           `yaml:"seed" json:"seed"`
	PlaybackSpeed     float64         `yaml:"playback_speed" json:"playbackSpeed"`
	Sources           []SourceSpec    `yaml:"sources" json:"sources"`
	Assets            []AssetSpec     `yaml:"assets" json:"assets"`
	Geofences         []GeofenceSpec  `yaml:"geofences" json:"geofences"`
	Tracks            []TrackSpec     `yaml:"tracks" json:"tracks"`
	Events            []EventSpec     `yaml:"events" json:"events"`
	CommandSimulation *CommandSimSpec `yaml:"command_simulation" json:"commandSimulation"`
}

// SourceSpec declares a synthetic source.
type SourceSpec struct {
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// AssetSpec declares a controlled asset and its initial placement.
type AssetSpec struct {
	ID       string    `yaml:"id" json:"id"`
	Name     string    `yaml:"name" json:"name"`
	Type     string    `yaml:"type" json:"type"`
	Start    *Position `yaml:"start" json:"start"`
	SpeedMPS *float64  `yaml:"speed_mps" json:"speedMps"`
}

// TrackSpec declares an observed entity. Tracks are created on first
// observation through the track hint.
type TrackSpec struct {
	ID    string    `yaml:"id" json:"id"`
	Start *Position `yaml:"start" json:"start"`
}

// GeofenceSpec declares an authoritative PostGIS polygon.
type GeofenceSpec struct {
	ID       string     `yaml:"id" json:"id"`
	Name     string     `yaml:"name" json:"name"`
	Type     string     `yaml:"type" json:"type"`
	Severity string     `yaml:"severity" json:"severity"`
	Polygon  []Position `yaml:"polygon" json:"polygon"`
}

// EventSpec is one scheduled scenario action.
type EventSpec struct {
	After      string         `yaml:"after" json:"after"`
	Action     string         `yaml:"action" json:"action"`
	Source     string         `yaml:"source" json:"source"`
	Track      string         `yaml:"track" json:"track"`
	Asset      string         `yaml:"asset" json:"asset"`
	Interval   string         `yaml:"interval" json:"interval"`
	Until      string         `yaml:"until" json:"until"`
	Waypoints  []Position     `yaml:"waypoints" json:"waypoints"`
	SpeedMPS   *float64       `yaml:"speed_mps" json:"speedMps"`
	Label      string         `yaml:"label" json:"label"`
	Confidence *float64       `yaml:"confidence" json:"confidence"`
	State      string         `yaml:"state" json:"state"`
	Type       string         `yaml:"type" json:"type"`
	Payload    map[string]any `yaml:"payload" json:"payload"`
	JitterM    float64        `yaml:"jitter_m" json:"jitterM"`
	Delay      string         `yaml:"delay" json:"delay"`
	Duplicate  bool           `yaml:"duplicate" json:"duplicate"`
	StaleFor   string         `yaml:"stale_for" json:"staleFor"`
}

// CommandSimSpec controls how the runner simulates command acknowledgments for
// commands issued against scenario assets.
type CommandSimSpec struct {
	Behavior         string `yaml:"behavior" json:"behavior"` // normal | reject | fail | timeout
	AcknowledgeAfter string `yaml:"acknowledge_after" json:"acknowledgeAfter"`
	CompleteAfter    string `yaml:"complete_after" json:"completeAfter"`
}

// Position is a YAML/JSON friendly WGS84 coordinate.
type Position struct {
	Lat float64 `yaml:"lat" json:"lat"`
	Lng float64 `yaml:"lng" json:"lng"`
}

// Point converts to the platform geometry type.
func (p Position) Point() geo.Point { return geo.Point{Lat: p.Lat, Lng: p.Lng} }

// Run is the persisted control-plane state of a scenario execution.
type Run struct {
	ID            string
	ScenarioName  string
	Seed          int64
	Status        string
	PlaybackSpeed float64
	VirtualTimeMs int64
	StartedAt     time.Time
	EndedAt       *time.Time
	Error         string
}

// Summary describes an available scenario file.
type Summary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Seed        int64  `json:"seed"`
	Sources     int    `json:"sources"`
	Assets      int    `json:"assets"`
	Tracks      int    `json:"tracks"`
	Geofences   int    `json:"geofences"`
	Events      int    `json:"events"`
}

// Summary derives a listing entry from a scenario.
func (s *Scenario) Summary() Summary {
	return Summary{
		Name:        s.Name,
		Description: s.Description,
		Seed:        s.Seed,
		Sources:     len(s.Sources),
		Assets:      len(s.Assets),
		Tracks:      len(s.Tracks),
		Geofences:   len(s.Geofences),
		Events:      len(s.Events),
	}
}

// ParseDurationMs parses a scenario duration string into milliseconds.
func ParseDurationMs(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, apperr.Validation("invalid duration " + raw)
	}
	if d < 0 {
		return 0, apperr.Validation("duration must not be negative: " + raw)
	}
	return d.Milliseconds(), nil
}

// Validate checks scenario structure and cross references.
func (s *Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return apperr.Validation("scenario name is required")
	}
	if s.PlaybackSpeed < 0 {
		return apperr.Validation("playback_speed must be positive")
	}
	if s.CommandSimulation != nil {
		switch s.CommandSimulation.Behavior {
		case "", "normal", "reject", "fail", "timeout":
		default:
			return apperr.Validation("command_simulation.behavior must be normal, reject, fail, or timeout")
		}
	}

	sourceIDs := map[string]bool{}
	for _, src := range s.Sources {
		if strings.TrimSpace(src.ID) == "" {
			return apperr.Validation("every source requires an id")
		}
		if sourceIDs[src.ID] {
			return apperr.Validation("duplicate source id: " + src.ID)
		}
		sourceIDs[src.ID] = true
	}

	assetIDs := map[string]bool{}
	for _, a := range s.Assets {
		if a.ID == "" {
			return apperr.Validation("every asset requires an id")
		}
		if assetIDs[a.ID] {
			return apperr.Validation("duplicate asset id: " + a.ID)
		}
		assetIDs[a.ID] = true
		if a.Start != nil && !a.Start.Point().Valid() {
			return apperr.Validation("asset " + a.ID + " has an invalid start position")
		}
		if a.SpeedMPS != nil && *a.SpeedMPS <= 0 {
			return apperr.Validation("asset " + a.ID + " speed_mps must be positive")
		}
	}

	trackIDs := map[string]bool{}
	for _, t := range s.Tracks {
		if t.ID == "" {
			return apperr.Validation("every track requires an id")
		}
		if trackIDs[t.ID] {
			return apperr.Validation("duplicate track id: " + t.ID)
		}
		trackIDs[t.ID] = true
		if t.Start != nil && !t.Start.Point().Valid() {
			return apperr.Validation("track " + t.ID + " has an invalid start position")
		}
	}

	for _, gf := range s.Geofences {
		if gf.ID == "" {
			return apperr.Validation("every geofence requires an id")
		}
		if len(gf.Polygon) < 3 {
			return apperr.Validation("geofence " + gf.ID + " requires at least 3 polygon points")
		}
		ring := make([]geo.Point, 0, len(gf.Polygon))
		for _, p := range gf.Polygon {
			if !p.Point().Valid() {
				return apperr.Validation("geofence " + gf.ID + " has an invalid polygon point")
			}
			ring = append(ring, p.Point())
		}
		if geo.PolygonAreaMeters2(ring) <= 0 {
			return apperr.Validation("geofence " + gf.ID + " polygon has zero area")
		}
	}

	for i, ev := range s.Events {
		if _, err := ParseDurationMs(ev.After); err != nil {
			return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
		}
		if _, err := ParseDurationMs(ev.Interval); err != nil {
			return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
		}
		if _, err := ParseDurationMs(ev.Until); err != nil {
			return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
		}
		if _, err := ParseDurationMs(ev.Delay); err != nil {
			return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
		}
		if _, err := ParseDurationMs(ev.StaleFor); err != nil {
			return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
		}

		switch ev.Action {
		case ActionObserve:
			if !sourceIDs[ev.Source] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown source %q", i, ev.Source))
			}
			if !trackIDs[ev.Track] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown track %q", i, ev.Track))
			}
		case ActionMove:
			if ev.Asset == "" && ev.Track == "" {
				return apperr.Validation(fmt.Sprintf("event %d move requires asset or track", i))
			}
			if ev.Asset != "" && !assetIDs[ev.Asset] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown asset %q", i, ev.Asset))
			}
			if ev.Track != "" && !trackIDs[ev.Track] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown track %q", i, ev.Track))
			}
			if len(ev.Waypoints) == 0 {
				return apperr.Validation(fmt.Sprintf("event %d move requires waypoints", i))
			}
			if ev.SpeedMPS != nil && *ev.SpeedMPS <= 0 {
				return apperr.Validation(fmt.Sprintf("event %d speed_mps must be positive", i))
			}
		case ActionClassify:
			if !trackIDs[ev.Track] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown track %q", i, ev.Track))
			}
			if ev.Label == "" {
				return apperr.Validation(fmt.Sprintf("event %d classify requires a label", i))
			}
		case ActionConnection:
			if !assetIDs[ev.Asset] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown asset %q", i, ev.Asset))
			}
			switch ev.State {
			case "connected", "degraded", "disconnected", "unknown":
			default:
				return apperr.Validation(fmt.Sprintf("event %d connection state must be connected, degraded, disconnected, or unknown", i))
			}
		case ActionIssueCommand:
			if !assetIDs[ev.Asset] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown asset %q", i, ev.Asset))
			}
			if ev.Type == "" {
				return apperr.Validation(fmt.Sprintf("event %d issue_command requires a type", i))
			}
		default:
			return apperr.Validation(fmt.Sprintf("event %d has unknown action %q", i, ev.Action))
		}
	}

	for i := 0; i < len(s.Events); i++ {
		for j := i + 1; j < len(s.Events); j++ {
			if s.Events[i].After == s.Events[j].After && s.Events[i].Action == "move" && s.Events[j].Action == "move" {
				return apperr.Validation("two move events share the same start time; order would be ambiguous")
			}
		}
	}
	return nil
}
