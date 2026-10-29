package imagery

import (
	"context"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"time"
)

func ScopeArgs(ctx context.Context) (string, *time.Time, *time.Time, bool) {
	return queryscope.Catalog(ctx, "imagery")
}

// JobScopePredicate is composed only with static SQL aliases and parameter
// positions from the service. It tests exact preparation grids, not scene boxes.
func JobScopePredicate(alias string, first int) string {
	return fmt.Sprintf(`($%d::text='' OR EXISTS(SELECT 1 FROM imagery_jobs p WHERE (p.id=%s.id OR p.id IN(SELECT jsonb_array_elements_text(%s.input->'preparationIds'))) AND p.result#>'{tile,grid}' IS NOT NULL AND ST_Intersects(ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($%d,'')),4326),ST_MakeEnvelope((p.result#>>'{tile,grid,west}')::float8,(p.result#>>'{tile,grid,north}')::float8-(p.result#>>'{tile,grid,height}')::float8*(p.result#>>'{tile,grid,stepLat}')::float8,(p.result#>>'{tile,grid,west}')::float8+(p.result#>>'{tile,grid,width}')::float8*(p.result#>>'{tile,grid,stepLng}')::float8,(p.result#>>'{tile,grid,north}')::float8,4326)))) AND ($%d::timestamptz IS NULL OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(%s.input->'scenes') scene WHERE (scene->>'acquiredAt')::timestamptz<$%d)) AND ($%d::timestamptz IS NULL OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(%s.input->'scenes') scene WHERE (scene->>'acquiredAt')::timestamptz>=$%d))`, first, alias, alias, first, first+1, alias, first+1, first+2, alias, first+2)
}
