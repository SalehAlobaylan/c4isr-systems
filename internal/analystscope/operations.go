package analystscope

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func ContainsTime(s Snapshot, at time.Time) bool {
	return s.Temporal.Interval != nil && !at.Before(s.Temporal.Interval.From) && at.Before(s.Temporal.Interval.To)
}
func ValidateBounds(ctx context.Context, pool *pgxpool.Pool, s Snapshot, b publicingest.Bounds) error {
	if b.Validate() != nil || s.Area.Kind == "world" || s.Area.Kind == "point" || len(s.Area.Geometry) == 0 {
		return apperr.Validation("bounded work requires a deliberate area and a valid local subset")
	}
	var covered bool
	e := db.Scoped(pool).QueryRow(ctx, `SELECT ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON($1),4326),ST_MakeEnvelope($2,$3,$4,$5,4326))`, string(s.Area.Geometry), b.West, b.South, b.East, b.North).Scan(&covered)
	if e != nil {
		return e
	}
	if !covered {
		return apperr.Validation("processing rectangle must be inside the exact selected area; bounds cannot replace holes or separate components")
	}
	return nil
}
