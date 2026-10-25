package publicevents

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"strings"
)

// Aggregates reuses the exact historical revision-selection/filter query. Its
// counted locations are display aggregates, never new provider observations.
func Aggregates(ctx context.Context, pool *pgxpool.Pool, f Filters, zoom int) (json.RawMessage, error) {
	if e := access.Require(ctx, "observations.read"); e != nil {
		return nil, e
	}
	resource, e := access.ResourceFrom(ctx)
	if e != nil {
		return nil, e
	}
	if resource.Partition != access.PublicEvidence {
		return nil, apperr.Forbidden("public aggregates require Public Context")
	}
	if f.SourceID != "" {
		if e = access.RequireSource(ctx, f.SourceID); e != nil {
			return nil, e
		}
	}
	b := publicingest.Bounds{}
	if f.Bounds != nil {
		b = *f.Bounds
	}
	size := 360 / math.Pow(2, float64(min(max(zoom, 0), 10)+2))
	cte := historyQuery[:strings.Index(historyQuery, "\nSELECT (SELECT count(*)")]
	query := cte + `,span AS(SELECT GREATEST($21::float8,(max(ST_X(display_position))-min(ST_X(display_position)))/40,(max(ST_Y(display_position))-min(ST_Y(display_position)))/40) size FROM filtered), cells AS(SELECT floor((ST_X(display_position)+180)/size) x,floor((ST_Y(display_position)+90)/size) y,count(*) n,avg(ST_X(display_position)) lng,avg(ST_Y(display_position)) lat FROM filtered,span WHERE display_position IS NOT NULL AND $14::int>0 AND $15::int>=0 GROUP BY 1,2)
 SELECT jsonb_build_object('type','FeatureCollection','features',COALESCE((SELECT jsonb_agg(jsonb_build_object('type','Feature','geometry',jsonb_build_object('type','Point','coordinates',jsonb_build_array(lng,lat)),'properties',jsonb_build_object('aggregate',true,'count',n,'title',n||' retained reports','kind','aggregate'))) FROM cells),'[]'::jsonb),'total',(SELECT count(*) FROM filtered),'unlocated',(SELECT count(*) FROM filtered WHERE display_position IS NULL),'truncated',false)`
	var raw []byte
	e = db.Scoped(pool).QueryRow(ctx, query, f.SourceID, f.View, f.AsOf, f.Revisions, f.Phenomenon, f.Quality, f.ObservedFrom, f.ObservedTo, f.Bounds != nil, b.West, b.South, b.East, b.North, f.Limit, f.Offset, "", false, f.RegionID, f.Geometry, f.HideWithdrawn, size).Scan(&raw)
	return raw, e
}
