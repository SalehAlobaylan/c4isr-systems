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
	return mapFrame(ctx, pool, f, zoom, false, false)
}

// MapFrame selects display, exact count and ordered membership in one SQL
// snapshot evaluation. It avoids scanning a 100,000-report scope three times.
func MapFrame(ctx context.Context, pool *pgxpool.Pool, f Filters, zoom int, members bool) (json.RawMessage, error) {
	return mapFrame(ctx, pool, f, zoom, members, true)
}
func mapFrame(ctx context.Context, pool *pgxpool.Pool, f Filters, zoom int, members, individual bool) (json.RawMessage, error) {
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
	query := cte + `,span AS(SELECT GREATEST($22::float8,(max(ST_X(display_position))-min(ST_X(display_position)))/40,(max(ST_Y(display_position))-min(ST_Y(display_position)))/40) size FROM filtered), cells AS(SELECT floor((ST_X(display_position)+180)/size) x,floor((ST_Y(display_position)+90)/size) y,count(*) n,count(*)FILTER(WHERE document#>>'{currentValidity,state}'='withdrawn_by_provider') withdrawn_inputs,count(*)FILTER(WHERE document#>>'{currentValidity,state}'='superseded_provider_report') superseded_inputs,avg(ST_X(display_position)) lng,avg(ST_Y(display_position)) lat FROM filtered,span WHERE display_position IS NOT NULL AND $14::int>0 AND $15::int>=0 GROUP BY 1,2)
 SELECT jsonb_build_object('type','FeatureCollection','features',COALESCE((SELECT jsonb_agg(jsonb_build_object('type','Feature','geometry',jsonb_build_object('type','Point','coordinates',jsonb_build_array(lng,lat)),'properties',jsonb_build_object('aggregate',true,'aggregateId',x::text||':'||y::text,'cellBounds',jsonb_build_array(x*(SELECT size FROM span)-180,y*(SELECT size FROM span)-90,(x+1)*(SELECT size FROM span)-180,(y+1)*(SELECT size FROM span)-90),'count',n,'currentWithdrawnReports',withdrawn_inputs,'supersededReports',superseded_inputs,'validityMeaning','Selected retained provider revisions; current validity is a separate annotation, not independent verification.','title',n||' retained reports','kind','aggregate'))) FROM cells),'[]'::jsonb),'total',(SELECT count(*) FROM filtered),'unlocated',(SELECT count(*) FROM filtered WHERE display_position IS NULL),'truncated',false)`
	if individual {
		query = strings.TrimSuffix(query, ")") + `, 'members',CASE WHEN $23::bool THEN COALESCE((SELECT jsonb_agg(evidence_id ORDER BY revision_at DESC,evidence_id DESC) FROM(SELECT evidence_id,revision_at FROM filtered ORDER BY revision_at DESC,evidence_id DESC LIMIT 100001)m),'[]'::jsonb) ELSE '[]'::jsonb END,'items',CASE WHEN (SELECT count(*) FROM filtered)<=2000 THEN COALESCE((SELECT jsonb_agg(document ORDER BY revision_at DESC,evidence_id DESC) FROM filtered),'[]'::jsonb) ELSE '[]'::jsonb END)`
	}
	var raw []byte
	args := []any{f.SourceID, f.View, f.AsOf, f.Revisions, f.Phenomenon, f.Quality, f.ObservedFrom, f.ObservedTo, f.Bounds != nil, b.West, b.South, b.East, b.North, f.Limit, f.Offset, "", false, f.RegionID, f.Geometry, f.HideWithdrawn, f.EndExclusive, size}
	if individual {
		args = append(args, members)
	}
	e = db.Scoped(pool).QueryRow(ctx, query, args...).Scan(&raw)
	return raw, e
}
