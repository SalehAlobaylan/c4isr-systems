package arearule

import (
	"context"

	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
)

// geospatialAdapter bridges the geospatial module to this package's structural
// types.
//
// The two shapes are kept separate on purpose: this package names what it needs
// from a spatial decision rather than importing the geospatial domain types, so
// the area rule cannot accidentally reach into geospatial internals or take a
// dependency on behaviour it does not use. The cost is one small adapter.
type geospatialAdapter struct {
	svc *geospatial.Service
}

// NewGeospatialAdapter wraps a geospatial service as an overlap querier.
func NewGeospatialAdapter(svc *geospatial.Service) OverlapQuerier {
	return geospatialAdapter{svc: svc}
}

func (a geospatialAdapter) IntersectingArea(ctx context.Context, area Overlap) ([]Geofence, error) {
	zones, err := a.svc.IntersectingArea(ctx, geospatial.AreaOverlap{GeoJSON: area.GeoJSON})
	if err != nil {
		return nil, err
	}
	out := make([]Geofence, 0, len(zones))
	for _, zone := range zones {
		out = append(out, Geofence{
			ID:       zone.ID,
			Name:     zone.Name,
			Type:     string(zone.Type),
			Severity: string(zone.Severity),
			Metadata: zone.Metadata,
		})
	}
	return out, nil
}
