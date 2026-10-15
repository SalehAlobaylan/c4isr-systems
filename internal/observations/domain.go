// Package observations implements the evidence layer: append-oriented
// statements made by sources about the world at particular times. Observations
// are preserved independently of any track, classification, or assessment
// derived from them.
package observations

import (
	"encoding/json"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// maxClockSkew bounds how far in the future an observation may claim to have
// been made. Sensors with clock drift beyond this are rejected so that
// operational state cannot be polluted by bogus future timestamps.
const maxClockSkew = 5 * time.Minute

// AreaKind classifies the phenomenon an area observation describes. It is a
// closed vocabulary so alert rules can select areas without parsing free-form
// payload keys, which would drift per producer.
type AreaKind string

const (
	// AreaHazard covers threats with immediate operational consequence:
	// wildfire, flood, chemical spill, structural collapse.
	AreaHazard AreaKind = "hazard"
	// AreaWeather covers atmospheric conditions: wind, precipitation, storm.
	AreaWeather AreaKind = "weather"
	// AreaEnvironmental covers background conditions that shape operations
	// without themselves demanding a response.
	AreaEnvironmental AreaKind = "environmental"
	// AreaInfrastructure covers damage or disruption to built assets.
	AreaInfrastructure AreaKind = "infrastructure"
	// AreaCrowd covers concentrations of people.
	AreaCrowd AreaKind = "crowd"
)

func validAreaKind(kind AreaKind) bool {
	switch kind {
	case AreaHazard, AreaWeather, AreaEnvironmental, AreaInfrastructure, AreaCrowd:
		return true
	}
	return false
}

// Area is the footprint of an event observation.
//
// An area is not a track. It describes something that is *there* at an instant
// rather than something that *moved* to get there, so it is never correlated,
// never given a velocity, and never aged out the way a track is. Modelling it
// on the track pipeline would produce a "track" for a wildfire that jumps
// between hotspots and a speed the platform invented.
type Area struct {
	// Kind classifies the phenomenon for rule selection.
	Kind AreaKind
	// Polygon is the footprint ring in WGS84. Three points is the minimum;
	// the first point is not repeated at the end (GeoJSON closes it).
	Polygon []geo.Point
}

// GeoJSON renders the area as a GeoJSON Polygon with a closed ring and
// coordinates in [lng, lat] order, matching the geofence encoding so the
// browser renders both through one path.
func (a Area) GeoJSON() string {
	ring := make([][]float64, 0, len(a.Polygon)+1)
	for _, point := range a.Polygon {
		ring = append(ring, []float64{point.Lng, point.Lat})
	}
	if len(a.Polygon) > 0 {
		first := a.Polygon[0]
		ring = append(ring, []float64{first.Lng, first.Lat})
	}
	raw, err := json.Marshal(map[string]any{
		"type":        "Polygon",
		"coordinates": [][][]float64{ring},
	})
	if err != nil {
		return `{"type":"Polygon","coordinates":[[]]}`
	}
	return string(raw)
}

// Centroid returns the arithmetic centroid of the footprint. It is a display
// and anchoring convenience only: the server never decides containment from a
// centroid, because a centroid is routinely outside the shape it represents.
func (a Area) Centroid() geo.Point {
	return geo.Centroid(a.Polygon)
}

// AreaMeters2 approximates the footprint area. Used for operator-facing size and
// for rejecting degenerate polygons.
func (a Area) AreaMeters2() float64 {
	return geo.PolygonAreaMeters2(a.Polygon)
}

// Validate checks the area against domain rules.
func (a Area) Validate() error {
	if !validAreaKind(a.Kind) {
		return apperr.Validation("area kind must be one of hazard, weather, environmental, infrastructure, crowd")
	}
	if len(a.Polygon) < 3 {
		return apperr.Validation("area polygon must have at least 3 points")
	}
	for _, point := range a.Polygon {
		if !point.Valid() {
			return apperr.Validation("area polygon contains an invalid coordinate")
		}
	}
	if a.AreaMeters2() <= 0 {
		return apperr.Validation("area polygon must have a positive area")
	}
	return nil
}

// Observation is a timestamped statement from a source.
type Observation struct {
	ID          string
	SourceID    string
	Type        string
	ObservedAt  time.Time
	ReceivedAt  time.Time
	ProcessedAt *time.Time
	Position    *geo.Point
	// Area is set for event observations that describe a footprint rather than
	// a moving object. Area and TrackHint are mutually exclusive: an area is
	// not correlated into a track, so attaching one would be contradictory.
	Area      *Area
	Payload   map[string]any
	Quality   map[string]any
	TrackHint string
	CreatedAt time.Time
}

// IsArea reports whether this observation describes a footprint rather than a
// point.
func (o Observation) IsArea() bool { return o.Area != nil }

// CreateInput is the application input for observation ingestion.
type CreateInput struct {
	// ID is an optional client-supplied idempotency key. Re-submitting the
	// same id returns the stored observation instead of creating a new one.
	ID         string
	SourceID   string
	Type       string
	ObservedAt time.Time
	ReceivedAt time.Time
	Position   *geo.Point
	Area       *Area
	Payload    map[string]any
	Quality    map[string]any
	TrackHint  string
}

// Normalize trims identifiers and applies defaults.
func (in *CreateInput) Normalize() {
	in.SourceID = trimmed(in.SourceID)
	in.Type = trimmed(in.Type)
	in.TrackHint = trimmed(in.TrackHint)
	if in.ID == "" {
		in.ID = ids.New("obs")
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = time.Now().UTC()
	}
	if !in.ObservedAt.IsZero() {
		in.ObservedAt = in.ObservedAt.UTC()
	}
	if in.Area != nil {
		kind := AreaKind(trimmed(string(in.Area.Kind)))
		in.Area.Kind = kind
	}
}

// Validate checks the observation against domain rules at time now.
func (in CreateInput) Validate(now time.Time) error {
	if in.SourceID == "" {
		return apperr.Validation("sourceId is required")
	}
	if in.Type == "" {
		return apperr.Validation("observation type is required")
	}
	if in.ObservedAt.IsZero() {
		return apperr.Validation("observedAt is required")
	}
	if in.ObservedAt.After(now.Add(maxClockSkew)) {
		return apperr.Validation("observedAt is too far in the future")
	}
	if in.Position != nil && !in.Position.Valid() {
		return apperr.Validation("position must be a valid WGS84 coordinate")
	}
	if in.Area != nil {
		// An area is never correlated into a track, so a track hint on the same
		// observation is a contradiction rather than a hint. Accepting it would
		// silently drop one of the two intents.
		if in.TrackHint != "" {
			return apperr.Validation("an area observation cannot carry a trackHint")
		}
		if err := in.Area.Validate(); err != nil {
			return err
		}
		// A footprint has extent, so a point inside it adds nothing an operator
		// could act on and creates two sources of truth about where it is.
		if in.Position != nil {
			return apperr.Validation("an area observation carries an area, not a separate position")
		}
	}
	return nil
}

func trimmed(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
