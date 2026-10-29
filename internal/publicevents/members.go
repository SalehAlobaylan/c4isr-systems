package publicevents

import (
	"context"
	"encoding/json"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemberPage reads immutable revisions by the original ordered membership. It
// never reruns latest-head selection. Current validity remains a live notice.
func MemberPage(ctx context.Context, pool *pgxpool.Pool, ids []string, bounds []float64, offset, limit int) (int, []json.RawMessage, error) {
	bounded := len(bounds) == 4
	if !bounded {
		bounds = []float64{0, 0, 0, 0}
	}
	query := `WITH members AS(
 SELECT s.*,m.ord,c.published_at known_at,COALESCE(o.position::geometry,ST_PointOnSurface(o.area::geometry)) point,
 head.evidence_id current_id,COALESCE((head.document#>>'{reported,withdrawn}')::bool,false) current_withdrawn,
 COALESCE((e.document->>'providerUpdatedAt')::timestamptz,NULLIF(NULLIF(s.document#>>'{reported,observedAt}',''),'0001-01-01T00:00:00Z')::timestamptz,s.revision_at) effective_at,
 CASE WHEN e.document->>'providerUpdatedAt' IS NOT NULL THEN 'provider_revision' WHEN NULLIF(s.document#>>'{reported,observedAt}','0001-01-01T00:00:00Z') IS NOT NULL THEN 'reported_observation_no_revision_time' ELSE 'receipt_fallback' END effective_basis,
 CASE WHEN s.document#>>'{reported,quality,providerConfidence}' IN('low','nominal','high') THEN s.document#>>'{reported,quality,providerConfidence}' WHEN s.document#>>'{reported,quality,reviewStatus}' IN('reviewed','automatic') THEN s.document#>>'{reported,quality,reviewStatus}' ELSE 'unknown' END quality_class
 FROM unnest($1::text[]) WITH ORDINALITY m(id,ord)
 JOIN public_event_revisions s ON s.evidence_id=m.id
 JOIN public_evidence_catalog c ON c.evidence_id=s.evidence_id AND c.scope_id=s.scope_id
 JOIN public_evidence e ON e.id=s.evidence_id AND e.scope_id=s.scope_id
 LEFT JOIN observations o ON o.id=s.observation_id AND o.scope_id=s.scope_id
 LEFT JOIN public_event_current h ON h.scope_id=s.scope_id AND h.source_id=s.source_id AND h.provider_record_id=s.provider_record_id
 LEFT JOIN public_event_revisions head ON head.evidence_id=h.evidence_id AND head.scope_id=h.scope_id
 ),selected AS(SELECT * FROM members WHERE NOT $2::bool OR (ST_X(point)>=$3 AND ST_X(point)<$5 AND ST_Y(point)>=$4 AND ST_Y(point)<$6)),page AS(SELECT * FROM selected ORDER BY ord LIMIT $7 OFFSET $8)
 SELECT (SELECT count(*) FROM selected),COALESCE((SELECT jsonb_agg(document||jsonb_build_object('knownAt',known_at,'effectiveAt',effective_at,'effectiveBasis',effective_basis,'qualityClass',quality_class,'displayPosition',ST_AsGeoJSON(point)::jsonb,'currentValidity',jsonb_build_object('evidenceId',evidence_id,'currentEvidenceId',current_id,'state',CASE WHEN current_withdrawn THEN 'withdrawn_by_provider' WHEN current_id IS NULL THEN 'unavailable' WHEN current_id=evidence_id THEN 'current_provider_report' ELSE 'superseded_provider_report' END,'evaluatedAt',clock_timestamp(),'meaning','Current retained provider status, separate from the saved interpretation; not independent verification.')) ORDER BY ord) FROM page),'[]'::jsonb)`
	total := 0
	var raw []byte
	items := []json.RawMessage{}
	e := db.Scoped(pool).QueryRow(ctx, query, ids, bounded, bounds[0], bounds[1], bounds[2], bounds[3], limit, offset).Scan(&total, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &items)
	}
	return total, items, e
}
