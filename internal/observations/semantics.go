package observations

import "strings"

const (
	Moving     = "moving"
	PointEvent = "point_event"
	AreaEvent  = "area"
)

// SemanticKind keeps geometry separate from meaning. Legacy track producers
// retain their behavior; named event/detection types cannot enter correlation.
func SemanticKind(kind, typ string, area bool) string {
	if kind != "" {
		return kind
	}
	if area {
		return AreaEvent
	}
	if typ == "earthquake" || typ == "thermal.detection" || strings.HasPrefix(typ, "event.") || strings.HasPrefix(typ, "detection.") {
		return PointEvent
	}
	return Moving
}
