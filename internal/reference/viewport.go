package reference

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

type Viewport struct {
	Type      string    `json:"type"`
	Features  []Feature `json:"features"`
	Matched   int       `json:"matched"`
	Truncated bool      `json:"truncated"`
}

func (s *Service) Viewport(ctx context.Context, b publicingest.Bounds, ids []string, offset int) (Viewport, error) {
	out := Viewport{Type: "FeatureCollection", Features: []Feature{}}
	if e := require(ctx, "sources.read"); e != nil {
		return out, e
	}
	if b.Validate() != nil || len(ids) > 32 || len(ids) < 1 || offset < 0 || offset > 100000 {
		return out, apperr.Validation("viewport requires bounds, 1–32 dataset IDs and bounded offset")
	}
	for _, id := range ids {
		if _, e := s.Get(ctx, id); e != nil {
			return out, e
		}
	}
	rows, e := db.Scoped(s.pool).Query(ctx, `SELECT f.feature_id,ST_AsGeoJSON(f.geometry),f.properties,f.dataset_id,count(*)OVER() FROM reference_features f WHERE f.dataset_id=ANY($1::text[]) AND ST_Intersects(f.geometry,ST_MakeEnvelope($2,$3,$4,$5,4326)) ORDER BY f.dataset_id,f.feature_id LIMIT 2000 OFFSET $6`, ids, b.West, b.South, b.East, b.North, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		f := Feature{Type: "Feature"}
		var geometry, props []byte
		var dataset string
		if e = rows.Scan(&f.ID, &geometry, &props, &dataset, &out.Matched); e != nil {
			return out, e
		}
		f.Geometry = geometry
		if e = json.Unmarshal(props, &f.Properties); e != nil {
			return out, e
		}
		if f.Properties == nil {
			f.Properties = map[string]any{}
		}
		f.Properties["datasetId"] = dataset
		f.ID = dataset + ":" + f.ID
		out.Features = append(out.Features, f)
	}
	out.Truncated = out.Matched > offset+len(out.Features)
	return out, rows.Err()
}
