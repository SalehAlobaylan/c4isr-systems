// Package geospatial turns location into operational meaning: geofences,
// containment state transitions, and spatial asset queries. PostGIS is
// authoritative for every spatial decision; this package orchestrates
// persistence and derives breach/exit events from persisted transitions.
package geospatial

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// Type categorizes the operational meaning of a geofence.
type Type string

const (
	TypeRestricted   Type = "restricted"
	TypePatrol       Type = "patrol"
	TypeSurveillance Type = "surveillance"
	TypeExclusion    Type = "exclusion"
	TypeProtected    Type = "protected"
)

// Severity ranks how urgently a transition demands attention.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Geofence is a named polygon with an operational meaning.
type Geofence struct {
	ID        string
	Name      string
	Type      Type
	Severity  Severity
	Active    bool
	GeoJSON   string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateInput is the application input for registering a geofence.
type CreateInput struct {
	ID       string
	Name     string
	Type     Type
	Severity Severity
	Polygon  []geo.Point
	Active   *bool
	Metadata map[string]any
}

// AssetDistance is one row of a spatial asset query.
type AssetDistance struct {
	AssetID         string
	Name            string
	Type            string
	Status          string
	ConnectionState string
	Position        *geo.Point
	DistanceM       float64
	LastSeenAt      *time.Time
}

// Normalize trims fields and applies defaults, generating an id when absent.
func (in *CreateInput) Normalize() {
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = ids.New("geo")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Severity == "" {
		in.Severity = SeverityMedium
	}
	if in.Active == nil {
		active := true
		in.Active = &active
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Name == "" {
		return apperr.Validation("geofence name is required")
	}
	switch in.Type {
	case TypeRestricted, TypePatrol, TypeSurveillance, TypeExclusion, TypeProtected:
	default:
		return apperr.Validation("geofence type must be one of restricted, patrol, surveillance, exclusion, protected")
	}
	switch in.Severity {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
	default:
		return apperr.Validation("geofence severity must be one of low, medium, high, critical")
	}
	if len(in.Polygon) < 3 {
		return apperr.Validation("geofence polygon must have at least 3 points")
	}
	for _, point := range in.Polygon {
		if !point.Valid() {
			return apperr.Validation("geofence polygon contains an invalid coordinate")
		}
	}
	if geo.PolygonAreaMeters2(in.Polygon) <= 0 {
		return apperr.Validation("geofence polygon must have a positive area")
	}
	return nil
}

// GeoJSON renders the polygon as a GeoJSON Polygon with a closed ring and
// coordinates in [lng, lat] order.
func (in CreateInput) GeoJSON() string {
	ring := make([][]float64, 0, len(in.Polygon)+1)
	for _, point := range in.Polygon {
		ring = append(ring, []float64{point.Lng, point.Lat})
	}
	if len(in.Polygon) > 0 {
		first := in.Polygon[0]
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
