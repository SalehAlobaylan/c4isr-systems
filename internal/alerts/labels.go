package alerts

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
)

// ClassifierLabeler supplies the latest classification for a track.
// Implemented by the classifications service.
type ClassifierLabeler interface {
	LatestForTrack(ctx context.Context, trackID string) (*classifications.Classification, error)
}

// defaultConfidence is the point at which a classification is treated as
// settled enough to state plainly in an alert headline.
//
// Below it the label is still shown, but hedged: "Suspected vehicle" and
// "Vehicle" are different claims, and collapsing them would overstate what the
// system knows at the moment it demands an operator's attention.
const defaultConfidence = 0.8

// Subject describes the entity an alert concerns, in operator language.
type Subject struct {
	// Label is the human name, e.g. "Vehicle" or "Unidentified contact".
	Label string
	// Confirmed reports whether the name came from a classification with
	// sufficient confidence, which the wording reflects.
	Confirmed bool
}

// DescribeTrack produces the operator-facing subject for a track.
//
// Precedence is deliberate: the newest classification is the system's current
// best hypothesis about what the track is, so it wins. When nothing is known
// the subject is stated as unidentified rather than invented, because a
// plausible-sounding wrong name is worse than an honest blank - an operator who
// trusts a name will act on it.
//
// A lookup failure is deliberately not fatal and not silently upgraded to
// "unidentified": an unreadable classification is a degraded read, and saying
// "unidentified" would quietly contradict a label that exists.
func DescribeTrack(ctx context.Context, trackID string, classifier ClassifierLabeler) Subject {
	if classifier == nil {
		return Subject{Label: "Unidentified contact"}
	}
	latest, err := classifier.LatestForTrack(ctx, trackID)
	if err != nil || latest == nil {
		return Subject{Label: "Unidentified contact"}
	}
	label := strings.TrimSpace(latest.Label)
	if label == "" {
		return Subject{Label: "Unidentified contact"}
	}
	confirmed := latest.Confidence != nil && *latest.Confidence >= defaultConfidence
	return Subject{Label: label, Confirmed: confirmed}
}

// BreachTitle renders the alert headline.
//
// The track's name is used, never its identifier. A ULID in an alert title
// carries no meaning for the person deciding whether to respond, and forces
// them to open a second view to learn what they are looking at.
func (s Subject) BreachTitle(geofenceName string) string {
	geofence := strings.TrimSpace(geofenceName)
	if geofence == "" {
		geofence = "a restricted zone"
	}
	if s.Confirmed {
		return fmt.Sprintf("%s entered %s", capitalize(s.Label), geofence)
	}
	return fmt.Sprintf("Suspected %s entered %s", strings.ToLower(s.Label), geofence)
}

// BreachMessage renders the supporting sentence beneath the headline. It names
// the geofence type that fired so the operator can judge the claim without
// opening the track.
func (s Subject) BreachMessage(geofenceType, geofenceName string) string {
	kind := strings.TrimSpace(geofenceType)
	if kind == "" {
		kind = "restricted"
	}
	name := strings.TrimSpace(geofenceName)
	if name == "" {
		name = "a restricted zone"
	}
	if s.Confirmed {
		return fmt.Sprintf("%s entered the %s geofence %s", capitalize(s.Label), kind, name)
	}
	return fmt.Sprintf("Suspected %s entered the %s geofence %s", strings.ToLower(s.Label), kind, name)
}

// areaSubject names the phenomenon an area observation describes.
//
// An area has no classification, because there is nothing to classify: it is a
// footprint of something, not a hypothesis about what something is. The kind is
// therefore the whole of what the system knows, and the wording says exactly
// that much rather than dressing a hazard up as a confirmed threat.
func areaSubject(kind string) Subject {
	switch AreaKind(kind) {
	case "hazard":
		return Subject{Label: "Hazard area", Confirmed: true}
	case "weather":
		return Subject{Label: "Weather cell", Confirmed: true}
	case "environmental":
		return Subject{Label: "Environmental condition", Confirmed: true}
	case "infrastructure":
		return Subject{Label: "Infrastructure disruption", Confirmed: true}
	case "crowd":
		return Subject{Label: "Crowd area", Confirmed: true}
	default:
		// An unrecognised kind is still an event that reached a zone. Naming it
		// as unidentified keeps the alert actionable without asserting a
		// classification the platform cannot support.
		return Subject{Label: "Unidentified area event", Confirmed: true}
	}
}

// areaKindNoun is the phrase used in the supporting sentence.
func areaKindNoun(kind string) string {
	switch AreaKind(kind) {
	case "hazard":
		return "hazard"
	case "weather":
		return "weather cell"
	case "environmental":
		return "environmental condition"
	case "infrastructure":
		return "infrastructure disruption"
	case "crowd":
		return "crowd concentration"
	default:
		return "area event"
	}
}

// AreaKind mirrors the observation area vocabulary so this module can name an
// area without importing the observations package.
type AreaKind = string

// OverlapTitle renders the headline for an area overlapping a zone.
func (s Subject) OverlapTitle(geofenceName string) string {
	geofence := strings.TrimSpace(geofenceName)
	if geofence == "" {
		geofence = "a restricted zone"
	}
	return fmt.Sprintf("%s overlaps %s", s.Label, geofence)
}

// OverlapMessage renders the supporting sentence for an area overlap. It names
// the phenomenon and the zone so the operator can judge the claim without
// opening the observation.
func (s Subject) OverlapMessage(areaKind, geofenceName string) string {
	name := strings.TrimSpace(geofenceName)
	if name == "" {
		name = "a restricted zone"
	}
	return fmt.Sprintf("An observed %s overlaps %s.", areaKindNoun(areaKind), name)
}

// capitalize upper-cases the first letter of a label.
//
// Classification labels are stored in the vocabulary's own casing - "vehicle",
// "small UAV" - which is correct for a data field. An alert headline is prose,
// and prose that begins mid-sentence reads as a formatting bug, so the case is
// adjusted for display only. Multi-word labels keep their internal casing.
func capitalize(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	runes := []rune(trimmed)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
