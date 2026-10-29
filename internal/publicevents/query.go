package publicevents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Filters struct {
	EndExclusive     bool                 `json:"endExclusive,omitempty"`
	ResolvedGeometry bool                 `json:"-"`
	Geometry         string               `json:"geometry,omitempty"`
	HideWithdrawn    bool                 `json:"-"`
	RegionID         string               `json:"regionId,omitempty"`
	View             string               `json:"view"`
	AsOf             *time.Time           `json:"asOf,omitempty"`
	Revisions        string               `json:"revisions"`
	SourceID         string               `json:"sourceId,omitempty"`
	Phenomenon       string               `json:"phenomenon,omitempty"`
	Quality          string               `json:"quality,omitempty"`
	ObservedFrom     *time.Time           `json:"observedFrom,omitempty"`
	ObservedTo       *time.Time           `json:"observedTo,omitempty"`
	Bounds           *publicingest.Bounds `json:"bounds,omitempty"`
	Limit            int                  `json:"limit"`
	Offset           int                  `json:"offset"`
}
type ViewMeaning struct {
	Mode           string     `json:"mode"`
	AsOf           *time.Time `json:"asOf,omitempty"`
	Meaning        string     `json:"meaning"`
	KnownTimeBasis string     `json:"knownTimeBasis"`
	QualityMeaning string     `json:"qualityMeaning"`
}

func (f Filters) Meaning() ViewMeaning {
	meaning := map[string]string{"latest": "Latest published retained provider revision per record; withdrawals remain visible.", "known_at": "Latest provider revision among records published by EMAD at the selected time. Later arrivals and publications are excluded.", "effective_at": "Retrospective provider-effective view using evidence retained now. Late arrivals may appear; this is not what EMAD knew then."}[f.View]
	return ViewMeaning{f.View, f.AsOf, meaning, "Catalogue publication time recorded in its transaction, not a physical commit timestamp; separate from provider revision/acquisition and receipt time.", "Provider VIIRS confidence class or USGS review status; not an EMAD assessment confidence."}
}
func ParseFilters(q url.Values, now time.Time) (Filters, error) {
	return parseFilters(q, now, false)
}

// ParseScopeFilters accepts geometry already validated by the area resolver.
// Legacy filter semantics and geometry budgets remain unchanged.
func ParseScopeFilters(q url.Values, now time.Time) (Filters, error) {
	return parseFilters(q, now, true)
}
func parseFilters(q url.Values, now time.Time, resolved bool) (Filters, error) {
	f := Filters{View: q.Get("view"), Revisions: q.Get("revisions"), SourceID: q.Get("source_id"), Phenomenon: q.Get("phenomenon"), Quality: q.Get("quality"), Limit: 100}
	f.EndExclusive = resolved
	f.ResolvedGeometry = resolved
	bad := func() (Filters, error) {
		return f, apperr.Validation("invalid public evidence filter or historical time selection")
	}
	allowed := map[string]bool{"geometry": true, "region_id": true, "view": true, "as_of": true, "revisions": true, "source_id": true, "phenomenon": true, "quality": true, "observed_from": true, "observed_to": true, "bbox": true, "limit": true, "offset": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return bad()
		}
	}
	if f.View == "" {
		f.View = "latest"
	}
	if f.View != "latest" && f.View != "known_at" && f.View != "effective_at" {
		return bad()
	}
	if f.Revisions == "" {
		f.Revisions = "latest"
	}
	if f.Revisions != "latest" && f.Revisions != "all" {
		return bad()
	}
	parseTime := func(key string) (*time.Time, error) {
		if q.Get(key) == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339Nano, q.Get(key))
		if err != nil || t.IsZero() || t.After(now.Add(5*time.Minute)) {
			return nil, fmt.Errorf("invalid timestamp")
		}
		t = t.UTC()
		return &t, nil
	}
	f.Geometry = q.Get("geometry")
	if f.Geometry != "" && !resolved {
		if e := ValidateStudyGeometry(f.Geometry, false); e != nil {
			return f, e
		}
	}
	f.RegionID = q.Get("region_id")
	if len(f.RegionID) > 256 {
		return bad()
	}
	var err error
	if f.AsOf, err = parseTime("as_of"); err != nil {
		return bad()
	}
	if (f.View == "latest") != (f.AsOf == nil) {
		return bad()
	}
	if f.ObservedFrom, err = parseTime("observed_from"); err != nil {
		return bad()
	}
	if f.ObservedTo, err = parseTime("observed_to"); err != nil {
		return bad()
	}
	maxInterval := 366 * 24 * time.Hour
	if resolved {
		maxInterval = queryscope.MaxInterval
	}
	if f.ObservedFrom != nil && f.ObservedTo != nil && (f.ObservedTo.Before(*f.ObservedFrom) || f.ObservedTo.Sub(*f.ObservedFrom) > maxInterval) {
		return bad()
	}
	if len(f.SourceID) > 512 || len(f.Phenomenon) > 100 || (f.Phenomenon != "" && !regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`).MatchString(f.Phenomenon)) {
		return bad()
	}
	if f.Quality != "" && f.Quality != "low" && f.Quality != "nominal" && f.Quality != "high" && f.Quality != "reviewed" && f.Quality != "automatic" && f.Quality != "unknown" {
		return bad()
	}
	if value := q.Get("limit"); value != "" {
		f.Limit, err = strconv.Atoi(value)
		if err != nil || f.Limit < 1 || f.Limit > 200 {
			return bad()
		}
	}
	if value := q.Get("offset"); value != "" {
		f.Offset, err = strconv.Atoi(value)
		if err != nil || f.Offset < 0 || f.Offset > 10000 {
			return bad()
		}
	}
	if value := q.Get("bbox"); value != "" {
		parts := strings.Split(value, ",")
		if len(parts) != 4 {
			return bad()
		}
		v := make([]float64, 4)
		for i, p := range parts {
			v[i], err = strconv.ParseFloat(p, 64)
			if err != nil {
				return bad()
			}
		}
		b := publicingest.Bounds{West: v[0], South: v[1], East: v[2], North: v[3]}
		if b.Validate() != nil {
			return bad()
		}
		f.Bounds = &b
	}
	return f, nil
}

type QueryResult struct {
	Items     []json.RawMessage `json:"items"`
	Total     int               `json:"total"`
	Truncated bool              `json:"truncated"`
	Filters   Filters           `json:"filters"`
	View      ViewMeaning       `json:"view"`
}

// Revision selection precedes spatial, phenomenon, observed-time and quality
// filters. A correction outside the filter must not resurrect its old version.
const historyQuery = `WITH eligible AS (
 SELECT r.*,c.published_at AS known_at,
 COALESCE((e.document->>'providerUpdatedAt')::timestamptz,NULLIF(NULLIF(r.document#>>'{reported,observedAt}',''),'0001-01-01T00:00:00Z')::timestamptz,r.revision_at) AS effective_at,
 CASE WHEN e.document->>'providerUpdatedAt' IS NOT NULL THEN 'provider_revision' WHEN NULLIF(r.document#>>'{reported,observedAt}','0001-01-01T00:00:00Z') IS NOT NULL THEN 'reported_observation_no_revision_time' ELSE 'receipt_fallback' END AS effective_basis,
 CASE WHEN r.document#>>'{reported,quality,providerConfidence}' IN ('low','nominal','high') THEN r.document#>>'{reported,quality,providerConfidence}' WHEN r.document#>>'{reported,quality,reviewStatus}' IN ('reviewed','automatic') THEN r.document#>>'{reported,quality,reviewStatus}' ELSE 'unknown' END AS quality_class
 FROM public_event_revisions r JOIN public_evidence_catalog c ON c.evidence_id=r.evidence_id AND c.scope_id=r.scope_id JOIN public_evidence e ON e.id=r.evidence_id AND e.scope_id=r.scope_id
 WHERE ($1::text='' OR r.source_id=$1)
),visible AS(
 SELECT * FROM eligible WHERE ($2::text='latest' OR ($2='known_at' AND known_at<=$3::timestamptz) OR ($2='effective_at' AND effective_at<=$3::timestamptz))
),ranked AS(
 SELECT v.*,row_number() OVER(PARTITION BY v.source_id,v.provider_record_id ORDER BY v.revision_at DESC,v.evidence_id DESC) AS ordinal FROM visible v
 WHERE ($16::text='' OR (NOT $17::boolean AND v.evidence_id=$16) OR ($17 AND EXISTS(SELECT 1 FROM visible anchor WHERE anchor.evidence_id=$16 AND anchor.source_id=v.source_id AND anchor.provider_record_id=v.provider_record_id)))
),selected AS(
 SELECT * FROM ranked WHERE $4::text='all' OR ordinal=1
),filtered AS(
 SELECT COALESCE(o.position::geometry,ST_PointOnSurface(o.area::geometry)) AS display_position,s.evidence_id,s.known_at,s.revision_at,s.document||jsonb_build_object('displayPosition',ST_AsGeoJSON(COALESCE(o.position::geometry,ST_PointOnSurface(o.area::geometry)))::jsonb,'knownAt',s.known_at,'effectiveAt',s.effective_at,'effectiveBasis',s.effective_basis,'qualityClass',s.quality_class,'currentValidity',jsonb_build_object('evidenceId',s.evidence_id,'currentEvidenceId',head.evidence_id,'state',CASE WHEN COALESCE((head.document#>>'{reported,withdrawn}')::boolean,false)THEN 'withdrawn_by_provider' WHEN head.evidence_id IS NULL THEN 'unavailable' WHEN head.evidence_id=s.evidence_id THEN 'current_provider_report' ELSE 'superseded_provider_report' END,'evaluatedAt',clock_timestamp(),'meaning','Current retained provider status, separate from historical interpretation; not independent verification.')) AS document
 FROM selected s LEFT JOIN observations o ON o.id=s.observation_id AND o.scope_id=s.scope_id LEFT JOIN public_event_current current_head ON current_head.source_id=s.source_id AND current_head.scope_id=s.scope_id AND current_head.provider_record_id=s.provider_record_id LEFT JOIN public_event_revisions head ON head.evidence_id=current_head.evidence_id AND head.scope_id=s.scope_id
 WHERE ($5::text='' OR s.document#>>'{reported,type}'=$5) AND ($6::text='' OR s.quality_class=$6)
 AND ($7::timestamptz IS NULL OR o.observed_at>=$7) AND ($8::timestamptz IS NULL OR CASE WHEN $21::boolean THEN o.observed_at<$8 ELSE o.observed_at<=$8 END)
 AND (NOT $9::boolean OR ST_Intersects(o.position::geometry,ST_MakeEnvelope($10,$11,$12,$13,4326)) OR ST_Intersects(o.area::geometry,ST_MakeEnvelope($10,$11,$12,$13,4326)))
 AND (NOT $20::bool OR NOT COALESCE((s.document#>>'{reported,withdrawn}')::bool,false))
 AND ($19::text='' OR ST_Intersects(o.position::geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($19,'')),4326)) OR ST_Intersects(o.area::geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($19,'')),4326)))
 AND ($18::text='' OR EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$18 AND (ST_Covers(g.geometry,o.position::geometry) OR ST_Intersects(g.geometry,o.area::geometry))))
)
SELECT (SELECT count(*) FROM filtered),COALESCE((SELECT jsonb_agg(p.document ORDER BY p.revision_at DESC,p.evidence_id DESC) FROM(SELECT * FROM filtered ORDER BY revision_at DESC,evidence_id DESC LIMIT $14 OFFSET $15)p),'[]'::jsonb)`

func Query(ctx context.Context, pool *pgxpool.Pool, f Filters, selectedID string, history bool) (QueryResult, error) {
	result := QueryResult{Items: []json.RawMessage{}, Filters: f, View: f.Meaning()}
	if err := access.Require(ctx, "observations.read"); err != nil {
		return result, err
	}
	resource, err := access.ResourceFrom(ctx)
	if err != nil {
		return result, err
	}
	if resource.Partition != access.PublicEvidence {
		return result, apperr.Forbidden("public history requires PUBLIC_EVIDENCE")
	}
	if f.SourceID != "" {
		if err = access.RequireSource(ctx, f.SourceID); err != nil {
			return result, err
		}
	}
	if f.Geometry != "" {
		var valid bool
		if e := validateQueryGeometry(f); e != nil {
			return result, e
		}
		if e := platformdb.Scoped(pool).QueryRow(ctx, `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON($1),4326))`, f.Geometry).Scan(&valid); e != nil {
			return result, e
		}
		if !valid {
			return result, apperr.Validation("invalid study polygon")
		}
	}
	bounds := publicingest.Bounds{}
	if f.Bounds != nil {
		bounds = *f.Bounds
	}
	var raw []byte
	err = platformdb.Scoped(pool).QueryRow(ctx, historyQuery, f.SourceID, f.View, f.AsOf, f.Revisions, f.Phenomenon, f.Quality, f.ObservedFrom, f.ObservedTo, f.Bounds != nil, bounds.West, bounds.South, bounds.East, bounds.North, f.Limit, f.Offset, selectedID, history, f.RegionID, f.Geometry, f.HideWithdrawn, f.EndExclusive).Scan(&result.Total, &raw)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result.Items)
	result.Truncated = result.Total > f.Offset+len(result.Items)
	return result, err
}

func validateQueryGeometry(f Filters) error {
	if f.ResolvedGeometry {
		return nil
	}
	return ValidateStudyGeometry(f.Geometry, false)
}

// ScopeFilters bridges a resolved scope without reinterpreting legacy URLs.
func ScopeFilters(ctx context.Context, q url.Values) (Filters, error) {
	if t, ok := queryscope.From(ctx); ok {
		return ParseScopeFilters(q, t.EvaluatedAt)
	}
	return ParseFilters(q, time.Now().UTC())
}
