package scenarios

import "strings"

// RunScope maps logical scenario identifiers to physical, run-owned resource
// identifiers. The run id is always part of the namespace, so restarting a
// scenario can never reuse the previous run's source or asset rows.
type RunScope struct {
	RunID             string
	ResourceNamespace string
}

func NewRunScope(runID string) RunScope {
	runID = strings.TrimSpace(runID)
	return RunScope{
		RunID:             runID,
		ResourceNamespace: runID + "__",
	}
}

func (s RunScope) ID(resourceType, logicalID string) string {
	return s.ResourceNamespace + strings.TrimSpace(resourceType) + "__" + strings.TrimSpace(logicalID)
}

func (s RunScope) Source(logicalID string) string         { return s.ID("source", logicalID) }
func (s RunScope) Asset(logicalID string) string          { return s.ID("asset", logicalID) }
func (s RunScope) Geofence(logicalID string) string       { return s.ID("geofence", logicalID) }
func (s RunScope) Track(logicalID string) string          { return s.ID("track", logicalID) }
func (s RunScope) Observation(logicalID string) string    { return s.ID("observation", logicalID) }
func (s RunScope) Telemetry(logicalID string) string      { return s.ID("telemetry", logicalID) }
func (s RunScope) Classification(logicalID string) string { return s.ID("classification", logicalID) }
func (s RunScope) Assessment(logicalID string) string     { return s.ID("assessment", logicalID) }
func (s RunScope) Command(logicalID string) string        { return s.ID("command", logicalID) }
func (s RunScope) Actor() string                          { return "scenario:" + s.RunID }
