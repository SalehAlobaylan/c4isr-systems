// Package scenarios implements the deterministic scenario engine. From the
// core platform's perspective the runner is an external information source:
// it registers sources/assets/geofences through the same application services
// used by future real integrations, then emits normalized observations and
// telemetry. It never writes to the database directly.
package scenarios

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assessments"
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

const (
	EventPending   = "pending"
	EventRunning   = "running"
	EventCompleted = "completed"
	EventFailed    = "failed"
	EventSkipped   = "skipped"
)

// Action names supported in scenario event specs.
const (
	ActionObserve      = "observe"
	ActionMove         = "move"
	ActionClassify     = "classify"
	ActionAssessment   = "assessment"
	ActionConnection   = "connection"
	ActionAssetStatus  = "asset_status"
	ActionIncident     = "incident"
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
	Status     string         `yaml:"status" json:"status"`
	Type       string         `yaml:"type" json:"type"`
	Payload    map[string]any `yaml:"payload" json:"payload"`
	Quality    map[string]any `yaml:"quality" json:"quality"`
	Position   *Position      `yaml:"position" json:"position"`
	JitterM    float64        `yaml:"jitter_m" json:"jitterM"`
	Delay      string         `yaml:"delay" json:"delay"`
	Duplicate  bool           `yaml:"duplicate" json:"duplicate"`
	StaleFor   string         `yaml:"stale_for" json:"staleFor"`
	Fault      string         `yaml:"fault" json:"fault"`

	// Assessment fields support deterministic analytical history. Each action
	// appends a new assessment, so a later conclusion is a revision rather than
	// an in-place mutation of prior evidence.
	SubjectType    string         `yaml:"subject_type" json:"subjectType"`
	SubjectID      string         `yaml:"subject_id" json:"subjectId"`
	AssessmentType string         `yaml:"assessment_type" json:"assessmentType"`
	Conclusion     string         `yaml:"conclusion" json:"conclusion"`
	Method         string         `yaml:"method" json:"method"`
	Evidence       []EvidenceSpec `yaml:"evidence" json:"evidence"`

	// Incident fields support a synthetic escalation path. IncidentRef is a
	// scenario-local stable name; the generated database id is resolved by the
	// runner and can be referenced by later actions.
	IncidentRef         string `yaml:"incident_ref" json:"incidentRef"`
	IncidentTitle       string `yaml:"incident_title" json:"incidentTitle"`
	IncidentDescription string `yaml:"incident_description" json:"incidentDescription"`
	IncidentPriority    string `yaml:"incident_priority" json:"incidentPriority"`
	IncidentStatus      string `yaml:"incident_status" json:"incidentStatus"`
}

// EvidenceSpec is a scenario-friendly assessment evidence reference.
type EvidenceSpec struct {
	Type string `yaml:"type" json:"type"`
	ID   string `yaml:"id" json:"id"`
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
	ID                string
	ResourceNamespace string
	ScenarioName      string
	Seed              int64
	Status            string
	PlaybackSpeed     float64
	VirtualTimeMs     int64
	LastAction        string
	LastActionAt      int64
	ActionError       string
	EventsRun         int
	EventsTotal       int
	StartedAt         time.Time
	EndedAt           *time.Time
	Error             string
}

// EventInspection is a deterministic runner event in virtual-time order.
// It is intentionally a control-plane view; it does not replace audit facts
// emitted by the application services.
type EventInspection struct {
	Sequence int    `json:"sequence"`
	AtMs     int64  `json:"atMs"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
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

// validateLogicalID protects the run-qualified namespace from collapsing
// distinct logical identifiers onto the same physical resource. RunScope.ID
// intentionally trims identifiers for stable generated IDs, so accepting
// surrounding whitespace here would make values such as "asset-1" and
// " asset-1 " collide during setup.
func validateLogicalID(kind, id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return apperr.Validation("every " + kind + " requires an id")
	}
	if trimmed != id {
		return apperr.Validation(kind + " id must not have surrounding whitespace: " + id)
	}
	return nil
}

// Validate checks scenario structure and cross references.
func (s *Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return apperr.Validation("scenario name is required")
	}
	if s.PlaybackSpeed < 0 || math.IsNaN(s.PlaybackSpeed) || math.IsInf(s.PlaybackSpeed, 0) {
		return apperr.Validation("playback_speed must be positive")
	}
	if s.CommandSimulation != nil {
		switch s.CommandSimulation.Behavior {
		case "", "normal", "reject", "fail", "timeout":
		default:
			return apperr.Validation("command_simulation.behavior must be normal, reject, fail, or timeout")
		}
		var acknowledgeAfter, completeAfter int64
		for _, item := range []struct {
			field string
			raw   string
		}{
			{field: "acknowledge_after", raw: s.CommandSimulation.AcknowledgeAfter},
			{field: "complete_after", raw: s.CommandSimulation.CompleteAfter},
		} {
			parsed, err := ParseDurationMs(item.raw)
			if err != nil {
				return apperr.Validation("command_simulation." + item.field + ": " + err.Error())
			}
			if item.field == "acknowledge_after" {
				acknowledgeAfter = parsed
			} else {
				completeAfter = parsed
			}
		}
		if s.CommandSimulation.Behavior == "" || s.CommandSimulation.Behavior == "normal" {
			if acknowledgeAfter == 0 {
				acknowledgeAfter = defaultAckDelayMs
			}
			if completeAfter == 0 {
				completeAfter = defaultCompleteDelay
			}
			if completeAfter < acknowledgeAfter {
				return apperr.Validation("command_simulation.complete_after must not be before acknowledge_after")
			}
		}
	}

	sourceIDs := map[string]bool{}
	for _, src := range s.Sources {
		if err := validateLogicalID("source", src.ID); err != nil {
			return err
		}
		if sourceIDs[src.ID] {
			return apperr.Validation("duplicate source id: " + src.ID)
		}
		sourceIDs[src.ID] = true
	}

	assetIDs := map[string]bool{}
	for _, a := range s.Assets {
		if err := validateLogicalID("asset", a.ID); err != nil {
			return err
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
		if a.SpeedMPS != nil && (math.IsNaN(*a.SpeedMPS) || math.IsInf(*a.SpeedMPS, 0)) {
			return apperr.Validation("asset " + a.ID + " speed_mps must be finite")
		}
	}

	trackIDs := map[string]bool{}
	for _, t := range s.Tracks {
		if err := validateLogicalID("track", t.ID); err != nil {
			return err
		}
		if trackIDs[t.ID] {
			return apperr.Validation("duplicate track id: " + t.ID)
		}
		trackIDs[t.ID] = true
		if t.Start != nil && !t.Start.Point().Valid() {
			return apperr.Validation("track " + t.ID + " has an invalid start position")
		}
	}

	geofenceIDs := map[string]bool{}
	for _, gf := range s.Geofences {
		if err := validateLogicalID("geofence", gf.ID); err != nil {
			return err
		}
		if geofenceIDs[gf.ID] {
			return apperr.Validation("duplicate geofence id: " + gf.ID)
		}
		geofenceIDs[gf.ID] = true
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
		switch ev.Fault {
		case "", "missing_position", "invalid_coordinate", "duplicate", "stale", "conflict":
		default:
			return apperr.Validation(fmt.Sprintf("event %d has unknown fault %q", i, ev.Fault))
		}
		if ev.Position != nil && ev.Fault != "invalid_coordinate" && !ev.Position.Point().Valid() {
			return apperr.Validation(fmt.Sprintf("event %d has an invalid position", i))
		}
		for waypointIndex, waypoint := range ev.Waypoints {
			if !waypoint.Point().Valid() {
				return apperr.Validation(fmt.Sprintf("event %d waypoint %d has an invalid position", i, waypointIndex))
			}
		}
		if ev.Confidence != nil && (math.IsNaN(*ev.Confidence) || math.IsInf(*ev.Confidence, 0) || *ev.Confidence < 0 || *ev.Confidence > 1) {
			return apperr.Validation(fmt.Sprintf("event %d confidence must be between 0 and 1", i))
		}
		if math.IsNaN(ev.JitterM) || math.IsInf(ev.JitterM, 0) || ev.JitterM < 0 {
			return apperr.Validation(fmt.Sprintf("event %d jitter_m must be finite and non-negative", i))
		}
		afterMs, _ := ParseDurationMs(ev.After)
		untilMs, _ := ParseDurationMs(ev.Until)
		intervalMs, _ := ParseDurationMs(ev.Interval)
		delayMs, _ := ParseDurationMs(ev.Delay)
		if ev.Until != "" && untilMs < afterMs {
			return apperr.Validation(fmt.Sprintf("event %d until must not be before after", i))
		}
		if _, ok := addMilliseconds(afterMs, delayMs); !ok {
			return apperr.Validation(fmt.Sprintf("event %d timeline is too large", i))
		}
		if intervalMs > 0 && ev.Until == "" {
			if _, ok := addMilliseconds(afterMs, defaultObserveUntilMs); !ok {
				return apperr.Validation(fmt.Sprintf("event %d observe timeline is too large", i))
			}
		}

		switch ev.Action {
		case ActionObserve:
			if !sourceIDs[ev.Source] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown source %q", i, ev.Source))
			}
			if !trackIDs[ev.Track] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown track %q", i, ev.Track))
			}
			if ev.Duplicate || ev.Fault == "duplicate" {
				observationAt, _ := addMilliseconds(afterMs, delayMs)
				if _, ok := addMilliseconds(observationAt, 500); !ok {
					return apperr.Validation(fmt.Sprintf("event %d duplicate timeline is too large", i))
				}
			}
		case ActionMove:
			if ev.Asset == "" && ev.Track == "" {
				return apperr.Validation(fmt.Sprintf("event %d move requires asset or track", i))
			}
			if ev.Asset != "" && ev.Track != "" {
				return apperr.Validation(fmt.Sprintf("event %d move must target exactly one asset or track", i))
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
			if ev.SpeedMPS != nil && (*ev.SpeedMPS <= 0 || math.IsNaN(*ev.SpeedMPS) || math.IsInf(*ev.SpeedMPS, 0)) {
				return apperr.Validation(fmt.Sprintf("event %d speed_mps must be positive", i))
			}
		case ActionClassify:
			if !trackIDs[ev.Track] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown track %q", i, ev.Track))
			}
			if strings.TrimSpace(ev.Label) == "" {
				return apperr.Validation(fmt.Sprintf("event %d classify requires a label", i))
			}
		case ActionAssessment:
			if strings.TrimSpace(ev.SubjectType) == "" || strings.TrimSpace(ev.SubjectID) == "" {
				return apperr.Validation(fmt.Sprintf("event %d assessment requires subject_type and subject_id", i))
			}
			switch ev.SubjectType {
			case "track", "asset", "incident", "observation", "source":
			default:
				return apperr.Validation(fmt.Sprintf("event %d assessment subject_type is invalid", i))
			}
			if strings.TrimSpace(ev.AssessmentType) == "" || strings.TrimSpace(ev.Conclusion) == "" {
				return apperr.Validation(fmt.Sprintf("event %d assessment requires assessment_type and conclusion", i))
			}
			if ev.Method != "" {
				switch assessments.Method(ev.Method) {
				case assessments.MethodOperator, assessments.MethodRule, assessments.MethodAlgorithm, assessments.MethodAI:
				default:
					return apperr.Validation(fmt.Sprintf("event %d assessment method is invalid", i))
				}
			}
			for evidenceIndex, evidence := range ev.Evidence {
				if err := assessments.EvidenceType(evidence.Type).Validate(); err != nil {
					return apperr.Validation(fmt.Sprintf("event %d evidence %d: %v", i, evidenceIndex, err))
				}
				if strings.TrimSpace(evidence.ID) == "" {
					return apperr.Validation(fmt.Sprintf("event %d evidence %d requires an id", i, evidenceIndex))
				}
			}
		case ActionAssetStatus:
			if !assetIDs[ev.Asset] {
				return apperr.Validation(fmt.Sprintf("event %d references unknown asset %q", i, ev.Asset))
			}
			switch ev.Status {
			case "available", "assigned", "unavailable", "offline", "maintenance":
			default:
				return apperr.Validation(fmt.Sprintf("event %d asset status must be available, assigned, unavailable, offline, or maintenance", i))
			}
		case ActionIncident:
			incidentRefPresent := strings.TrimSpace(ev.IncidentRef) != ""
			incidentTitlePresent := strings.TrimSpace(ev.IncidentTitle) != ""
			incidentStatusPresent := strings.TrimSpace(ev.IncidentStatus) != ""
			if !incidentRefPresent && !incidentTitlePresent && !incidentStatusPresent {
				return apperr.Validation(fmt.Sprintf("event %d incident requires incident_ref, incident_title, or incident_status", i))
			}
			if incidentRefPresent {
				if err := validateLogicalID("incident_ref", ev.IncidentRef); err != nil {
					return apperr.Validation(fmt.Sprintf("event %d: %v", i, err))
				}
			}
			if incidentStatusPresent && !incidentRefPresent {
				return apperr.Validation(fmt.Sprintf("event %d incident_status requires incident_ref", i))
			}
			if ev.IncidentPriority != "" {
				switch ev.IncidentPriority {
				case "low", "medium", "high", "critical":
				default:
					return apperr.Validation(fmt.Sprintf("event %d incident priority is invalid", i))
				}
			}
			if ev.IncidentStatus != "" {
				switch ev.IncidentStatus {
				case "OPEN", "ACKNOWLEDGED", "INVESTIGATING", "RESPONDING", "RESOLVED", "CLOSED":
				default:
					return apperr.Validation(fmt.Sprintf("event %d incident status is invalid", i))
				}
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
			if strings.TrimSpace(ev.Type) == "" {
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
