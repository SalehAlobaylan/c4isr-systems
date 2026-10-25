package publicingest

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"time"
)

// ReportedObservation is a normalized provider statement, never an assessment
// or independent evidence producer. Geometry does not determine its semantics.
type ReportedObservation struct {
	SemanticKind string         `json:"semanticKind"`
	Type         string         `json:"type"`
	Title        string         `json:"title"`
	ObservedAt   time.Time      `json:"observedAt"`
	Position     *geo.Point     `json:"position,omitempty"`
	Area         *Footprint     `json:"area,omitempty"`
	GeometryRole string         `json:"geometryRole"`
	Withdrawn    bool           `json:"withdrawn"`
	Properties   map[string]any `json:"properties"`
	Quality      map[string]any `json:"quality"`
}
type Footprint struct {
	Kind    observations.AreaKind `json:"kind"`
	Polygon []geo.Point           `json:"polygon"`
}

func (o ReportedObservation) Validate(now time.Time) error {
	encoded, err := json.Marshal(o)
	if err != nil || len(encoded) > 32768 {
		return apperr.Validation("normalized provider statement exceeds 32 KiB budget")
	}
	if o.Area != nil && len(o.Area.Polygon) > 512 {
		return apperr.Validation("reported footprint exceeds 512 vertices")
	}
	if o.SemanticKind != observations.PointEvent && o.SemanticKind != observations.AreaEvent {
		return apperr.Validation("public event normalization must declare point_event or area")
	}
	if o.Type == "" || len(o.Type) > 100 || len(o.Title) > 1000 {
		return apperr.Validation("invalid event type/title")
	}
	if o.Withdrawn {
		return nil
	}
	in := observations.CreateInput{ID: "validation", SourceID: "validation", Type: o.Type, SemanticKind: o.SemanticKind, GeometryRole: o.GeometryRole, ObservedAt: o.ObservedAt, Position: o.Position}
	if o.Area != nil {
		in.Area = &observations.Area{Kind: o.Area.Kind, Polygon: o.Area.Polygon}
	}
	in.Normalize()
	return in.Validate(now)
}
