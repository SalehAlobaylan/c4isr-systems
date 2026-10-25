package scenarios

import (
	"encoding/json"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

// ContextAnchor places a scripted actor at an explicitly selected retained
// reference point. It does not reinterpret observed public tracks as actors.
type ContextAnchor struct {
	Track     string `yaml:"track" json:"track"`
	PackageID string `yaml:"package_id" json:"packageId"`
	DatasetID string `yaml:"dataset_id" json:"datasetId"`
	FeatureID string `yaml:"feature_id" json:"featureId"`
}

func (e *engine) contextPositions() (map[string]geo.Point, error) {
	out := map[string]geo.Point{}
	for _, a := range e.sc.ContextAnchors {
		found := false
		for _, b := range e.contextBundles {
			if b.ID != a.PackageID {
				continue
			}
			for _, asset := range b.Manifest.Assets {
				if asset.DatasetID != a.DatasetID || asset.Kind != "vector" || asset.Retention != "retained" {
					continue
				}
				raw, ok := b.Assets[asset.SHA256]
				if !ok || reference.Hash(raw) != asset.SHA256 {
					return nil, apperr.Conflict("frozen context anchor asset is corrupt")
				}
				var collection reference.Collection
				if json.Unmarshal(raw, &collection) != nil {
					return nil, apperr.Validation("context anchor requires retained GeoJSON")
				}
				for _, f := range collection.Features {
					if f.ID != a.FeatureID {
						continue
					}
					var g struct {
						Type        string    `json:"type"`
						Coordinates []float64 `json:"coordinates"`
					}
					if json.Unmarshal(f.Geometry, &g) != nil || g.Type != "Point" || len(g.Coordinates) != 2 {
						return nil, apperr.Validation("initial context anchors require horizontal point geometry; no implicit height")
					}
					p := geo.Point{Lng: g.Coordinates[0], Lat: g.Coordinates[1]}
					bounds := b.Manifest.AOI
					if !p.Valid() || p.Lng < bounds.West || p.Lng > bounds.East || p.Lat < bounds.South || p.Lat > bounds.North {
						return nil, apperr.Validation("context anchor is outside selected AOI")
					}
					out[a.Track] = p
					found = true
				}
			}
		}
		if !found {
			return nil, apperr.Validation("frozen context anchor dependency is missing")
		}
	}
	return out, nil
}
