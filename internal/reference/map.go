package reference

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/go-chi/chi/v5"
)

type MapCollection struct {
	RecordIDs    []string            `json:"-"`
	ScopeHash    string              `json:"scopeHash,omitempty"`
	EvaluatedAt  *time.Time          `json:"evaluatedAt,omitempty"`
	Support      *queryscope.Support `json:"support,omitempty"`
	Type         string              `json:"type"`
	Features     []Feature           `json:"features"`
	Total        int                 `json:"total"`
	Truncated    bool                `json:"truncated"`
	State        string              `json:"state"`
	Scope        string              `json:"scope"`
	CountMeaning string              `json:"countMeaning"`
	Unlocated    int                 `json:"unlocated"`
}

// An unavailable query has an unknown count, serialized as null, never zero.
func (c MapCollection) MarshalJSON() ([]byte, error) {
	type alias MapCollection
	total := &c.Total
	if c.State == "unavailable" || c.State == "historical_unsupported" {
		total = nil
	}
	return json.Marshal(struct {
		alias
		Total *int `json:"total"`
	}{alias(c), total})
}

func mapFilters(ctx context.Context, q url.Values) (publicevents.Filters, error) {
	values := url.Values{}
	for _, key := range []string{"geometry", "view", "as_of", "observed_from", "observed_to", "source_id", "quality", "region_id", "bbox"} {
		if q.Get(key) != "" {
			values.Set(key, q.Get(key))
		}
	}
	return publicevents.ScopeFilters(ctx, values)
}

func (s *Service) MapFeatures(ctx context.Context, family string, q url.Values) (MapCollection, error) {
	return s.mapFeatures(ctx, family, q, false)
}
func (s *Service) MapFeaturesRead(ctx context.Context, family string, q url.Values) (MapCollection, error) {
	return s.mapFeatures(ctx, family, q, true)
}
func (s *Service) mapFeatures(ctx context.Context, family string, q url.Values, retainMembers bool) (MapCollection, error) {
	out := MapCollection{Type: "FeatureCollection", Features: []Feature{}, State: "latest_retained", Scope: q.Get("region_id"), CountMeaning: "Matching latest retained records; not independently verified incidents"}
	allowed := map[string]bool{"family": true, "zoom": true, "geometry": true, "region_id": true, "bbox": true, "view": true, "as_of": true, "observed_from": true, "observed_to": true, "source_id": true, "quality": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return out, apperr.Validation("invalid or ambiguous map query")
		}
	}
	if q.Get("zoom") != "" {
		zoom, e := strconv.Atoi(q.Get("zoom"))
		if e != nil || zoom < 0 || zoom > 24 {
			return out, apperr.Validation("invalid map zoom")
		}
	}
	permission := "sources.read"
	if family == "earthquakes" || family == "thermal" || family == "events" {
		permission = "observations.read"
	}
	if family == "aircraft" || family == "vessels" {
		permission = "tracks.read"
	}
	if family == "findings" {
		permission = "assessments.read"
	}
	if e := require(ctx, permission); e != nil {
		return out, e
	}
	temporal, scoped := queryscope.From(ctx)
	if scoped {
		out.EvaluatedAt = &temporal.EvaluatedAt
		support := queryscope.Capability(family, temporal)
		out.Support = &support
		if support.State == "unsupported" {
			out.State = "historical_unsupported"
			return out, nil
		}
	}
	f, e := mapFilters(ctx, q)
	if e != nil {
		return out, e
	}
	if f.RegionID != "" {
		var exists bool
		if e = db.Scoped(s.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM geographic_regions WHERE id=$1)`, f.RegionID).Scan(&exists); e != nil {
			return out, e
		}
		if !exists {
			return out, apperr.NotFound("geographic scope", "unavailable")
		}
	}
	if family == "earthquakes" || family == "thermal" || family == "events" {
		if family == "earthquakes" {
			f.Phenomenon = "event.earthquake"
		}
		if family == "thermal" {
			f.Phenomenon = "thermal.detection"
		}
		f.Limit = 2000
		f.HideWithdrawn = true
		zoom, _ := strconv.Atoi(q.Get("zoom"))
		raw, err := publicevents.MapFrame(ctx, s.pool, f, zoom, retainMembers)
		if err != nil {
			return out, err
		}
		var records struct {
			Total   int               `json:"total"`
			Members []string          `json:"members"`
			Items   []json.RawMessage `json:"items"`
		}
		if err = json.Unmarshal(raw, &records); err != nil {
			return out, err
		}
		if retainMembers && records.Total > 100000 {
			return out, apperr.Conflict("individual membership exceeds 100000 reports; narrow the area, period or source")
		}
		out.Total = records.Total
		out.State = f.View
		out.RecordIDs = records.Members
		if records.Total > 2000 {
			err = json.Unmarshal(raw, &out)
			out.RecordIDs = records.Members
			out.State = "aggregated"
			out.Scope = f.RegionID
			out.CountMeaning = "Complete scoped report count; aggregate centres are display summaries, not reported locations"
			for i := range out.Features {
				out.Features[i].Properties["kind"] = family
				out.Features[i].Properties["recordId"] = fmt.Sprintf("aggregate:%s:%d", family, i)
			}
			return out, err
		}

		for _, raw := range records.Items {
			var v publicevents.Event
			if json.Unmarshal(raw, &v) != nil {
				continue
			}
			if v.Reported.Withdrawn {
				continue
			}
			if v.Reported.Position == nil && v.Reported.Area == nil {
				out.Unlocated++
			}
			props := map[string]any{"recordId": v.EvidenceID, "evidenceId": v.EvidenceID, "revisionId": v.EvidenceID, "sourceId": v.SourceID, "kind": family, "title": v.Reported.Title, "date": v.Reported.ObservedAt, "geometryRole": v.Reported.GeometryRole, "count": 1}
			if v.Reported.Position != nil {
				g, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{v.Reported.Position.Lng, v.Reported.Position.Lat}})
				out.Features = append(out.Features, Feature{Type: "Feature", ID: v.EvidenceID, Geometry: g, Properties: props})
			}
			if v.CurrentValidity != nil {
				props["currentValidity"] = v.CurrentValidity
				props["validityStatus"] = v.CurrentValidity.State
			}
			if v.Reported.Area != nil {
				ring := [][]float64{}
				for _, p := range v.Reported.Area.Polygon {
					ring = append(ring, []float64{p.Lng, p.Lat})
				}
				if len(ring) > 2 {
					ring = append(ring, ring[0])
					g, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][]float64{ring}})
					out.Features = append(out.Features, Feature{Type: "Feature", ID: v.EvidenceID + ":area", Geometry: g, Properties: props})
				}
			}
		}
		return out, nil
	}
	if f.View != "latest" && !(family == "bulletins" && scoped && f.View == "known_at") ||
		(family == "weather" || family == "aircraft" || family == "vessels") && f.ObservedTo != nil && f.ObservedTo.Before(time.Now().Add(-5*time.Minute)) {
		out.State = "historical_unsupported"
		return out, nil
	}
	// The RLS-filtered source query chooses current revisions before spatial and
	// time filtering. Every count is computed before the display limit.
	var base string
	switch family {
	case "findings":
		if e := require(ctx, "sources.read"); e != nil {
			return out, e
		}
		base = `SELECT j.id,CASE WHEN j.result->'findingGeometry' IS NOT NULL AND j.result->'findingGeometry'<>'null'::jsonb THEN ST_SetSRID(ST_GeomFromGeoJSON((j.result->'findingGeometry')::text),4326) END geom,(j.input#>>'{scenes,1,acquiredAt}')::timestamptz stamp,jsonb_build_object('recordId',j.id,'jobId',j.id,'sourceId',j.source_id,'kind','findings','title','Derived NDVI change · inspect review','date',j.input#>>'{scenes,1,acquiredAt}','beforeDate',j.input#>>'{scenes,0,acquiredAt}','computedAt',j.created_at,'synthetic',j.result->'synthetic','reviewStatus',COALESCE(rv.status,'NO_PROPOSAL'),'assessmentId',rv.assessment_id,'meaning','Algorithmic geometry. Human workflow status does not establish a verified real-world fact.','count',1) props FROM imagery_jobs j LEFT JOIN LATERAL(SELECT r.status,r.assessment_id FROM assessment_review_heads h JOIN assessment_revisions r ON r.id=h.latest_id WHERE emad_content_visible('assessment',r.assessment_id) AND r.document#>>'{evidence,id}'=j.id ORDER BY r.document->>'recordedAt' DESC,r.id LIMIT 1)rv ON true WHERE emad_content_visible('job',j.id) AND j.status='SUCCEEDED' AND j.input->>'kind'='NDVI_CHANGE'`
	case "weather", "bulletins":
		kind := "WEATHER"
		selection := ""
		if family == "bulletins" {
			kind = "REPORT"
			if scoped && (temporal.Mode == "historical" || temporal.RevisionView == "known_at" || temporal.Interval.To.Before(temporal.EvaluatedAt.Add(-5*time.Minute))) ||
				!scoped && f.ObservedTo != nil && f.ObservedTo.Before(time.Now().Add(-5*time.Minute)) {
				selection = " AND EXISTS (SELECT 1 FROM public_evidence p WHERE (p.id,p.scope_id)=(public_context_records.evidence_id,public_context_records.scope_id) AND p.document#>>'{context,extractionMethod}'='nasa-eonet-floods-v1')"
				out.CountMeaning = "Retained historical flood reports from eligible sources; gaps do not establish absence."
			}
			if scoped && temporal.AsOf != nil {
				selection += " AND EXISTS (SELECT 1 FROM public_evidence_catalog c WHERE (c.evidence_id,c.scope_id)=(public_context_records.evidence_id,public_context_records.scope_id) AND c.published_at<=TIMESTAMPTZ '" + temporal.AsOf.Format(time.RFC3339Nano) + "')"
			}
		}
		base = `WITH latest AS(SELECT DISTINCT ON(source_id,record_key) * FROM public_context_records WHERE kind='` + kind + `'` + selection + ` ORDER BY source_id,record_key,revision_at DESC,known_at DESC,evidence_id DESC)
  SELECT r.evidence_id id,r.position geom,r.valid_from stamp,jsonb_build_object('recordId',r.evidence_id,'evidenceId',r.evidence_id,'sourceId',r.source_id,'kind','` + family + `','title',e.document#>>'{context,title}','revisionId',r.evidence_id,'date',r.valid_from,'validUntil',r.valid_to,'source',e.document->>'upstreamProvider','providerRevision',e.document->>'providerRevision','reportHistory',e.document#>'{context,report,eventHistory}','lineageKeys',e.document#>'{context,report,lineageKeys}','temperature',e.document#>'{context,weather,temperatureC}','wind',e.document#>'{context,weather,windSpeedMps}','uncertainty',e.document#>>'{context,uncertainty}','ageSeconds',GREATEST(0,extract(epoch FROM now()-r.valid_to)),'status',CASE WHEN r.valid_to<now()-COALESCE((ad.document#>>'{policy,freshForSeconds}')::int,3600)*interval '1 second' THEN 'stale' ELSE 'current' END,'count',1) props FROM latest r JOIN public_evidence e ON e.id=r.evidence_id JOIN retrieval_profile_revisions original_profile ON original_profile.admission_id=e.admission_id JOIN retrieval_profile_state cs ON cs.source_id=r.source_id AND cs.profile_key=original_profile.profile_key JOIN source_admissions ad ON ad.id=cs.admission_id
  WHERE COALESCE((ad.document->>'enabled')::boolean,false) AND (ad.document#>>'{rights,expiresAt}' IS NULL OR (ad.document#>>'{rights,expiresAt}')::timestamptz>clock_timestamp())`
	case "aircraft", "vessels":
		kind := "aircraft"
		if family == "vessels" {
			kind = "vessel"
		}
		base = `SELECT s.track_id id,st.position::geometry geom,s.last_seen_at stamp,jsonb_build_object('recordId',s.track_id,'sourceId',s.source_id,'kind','` + family + `','title',CASE WHEN COALESCE(NULLIF(btrim(s.document->'claims'->>'callsign'),''),'')<>'' THEN btrim(s.document->'claims'->>'callsign')||' · '||s.identity_claim ELSE s.identity_claim END,'identityClaim',s.identity_claim,'callsign',NULLIF(btrim(s.document->'claims'->>'callsign'),''),'originCountry',NULLIF(btrim(s.document->'claims'->>'registrationCountryInferredByProvider'),''),'onGround',s.document->'claims'->'onGround','date',s.last_seen_at,'course',s.document->'course','speedMps',s.document->'speedMps','altitude',COALESCE(s.document->'claims'->'barometricAltitudeMetres',s.document->'height'),'count',1) props FROM external_track_segments s JOIN track_state st ON st.track_id=s.track_id WHERE s.kind='` + kind + `'`
	case "imagery":
		base = `WITH captures AS(SELECT DISTINCT ON(input#>>'{scene,id}',input->'aoi') * FROM imagery_capture_jobs WHERE emad_content_visible('capture',id) AND status='SUCCEEDED' ORDER BY input#>>'{scene,id}',input->'aoi',created_at DESC)
  SELECT id,ST_MakeEnvelope((input#>>'{aoi,west}')::float8,(input#>>'{aoi,south}')::float8,(input#>>'{aoi,east}')::float8,(input#>>'{aoi,north}')::float8,4326) geom,(input#>>'{scene,acquiredAt}')::timestamptz stamp,jsonb_build_object('recordId',id,'kind','imagery','title',(input#>>'{scene,platform}')||' · '||(input#>>'{scene,tile}'),'revisionId',id,'date',input#>>'{scene,acquiredAt}','sourceId',source_id,'source',input#>>'{scene,originalProducer}','rgbArtifactId',result->>'rgbArtifactId','qualityArtifactId',result->>'qualityArtifactId','preparationId',result->>'preparationId','aoi',input->'aoi','count',1) props FROM captures`
	case "coverage":
		base = `SELECT CASE WHEN c.profile_key='default' THEN src.id ELSE src.id||':'||c.profile_key END AS id,ST_MakeEnvelope((a.document#>>'{policy,aoi,west}')::float8,(a.document#>>'{policy,aoi,south}')::float8,(a.document#>>'{policy,aoi,east}')::float8,(a.document#>>'{policy,aoi,north}')::float8,4326) geom,NULL::timestamptz stamp,jsonb_build_object('recordId',src.id,'sourceId',src.id,'kind','coverage','title',src.name||' · '||c.profile_key,'profileKey',c.profile_key,'meaning','Admitted query area; not complete observation coverage','count',1) props FROM sources src JOIN retrieval_profile_state c ON c.source_id=src.id AND c.scope_id=src.scope_id JOIN source_admissions a ON a.id=c.admission_id`
	default:
		return out, apperr.Validation("unsupported map family")
	}
	if f.Geometry != "" && !scoped {
		var valid bool
		if e = db.Scoped(s.pool).QueryRow(ctx, `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON($1),4326))`, f.Geometry).Scan(&valid); e != nil {
			return out, e
		}
		if !valid {
			return out, apperr.Validation("invalid study polygon")
		}
	}
	var bounds any = nil
	if f.Bounds != nil {
		v := f.Bounds
		bounds = fmt.Sprintf("POLYGON((%f %f,%f %f,%f %f,%f %f,%f %f))", v.West, v.South, v.East, v.South, v.East, v.North, v.West, v.North, v.West, v.South)
	}
	from, to := f.ObservedFrom, f.ObservedTo
	// Static capture footprints and coverage deliberately use their own dates.
	if family == "coverage" || family == "imagery" && (!scoped || temporal.BackgroundImagery) {
		from = nil
		to = nil
		out.CountMeaning = "Retained context with separately stated acquisition/coverage dates"
	}
	if scoped {
		base = strings.ReplaceAll(base, "now()", "TIMESTAMPTZ '"+temporal.EvaluatedAt.Format(time.RFC3339Nano)+"'")
	}
	query := `WITH records AS(` + base + `),filtered AS(SELECT * FROM records r WHERE geom IS NOT NULL AND ($1::text='' OR EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$1 AND ST_Intersects(g.geometry,r.geom))) AND ($2::text IS NULL OR ST_Intersects(r.geom,ST_GeomFromText($2,4326))) AND ($3::timestamptz IS NULL OR r.stamp>=$3) AND ($4::timestamptz IS NULL OR CASE WHEN $7::boolean THEN r.stamp<$4 ELSE r.stamp<=$4 END) AND ($5::text='' OR r.props->>'sourceId'=$5) AND ($6::text='' OR ST_Intersects(r.geom,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($6,'')),4326)))) SELECT id,ST_AsGeoJSON(geom),props,count(*)OVER() FROM filtered ORDER BY stamp DESC NULLS LAST,id LIMIT 2000`
	rows, e := db.Scoped(s.pool).Query(ctx, query, f.RegionID, bounds, from, to, f.SourceID, f.Geometry, scoped)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		v := Feature{Type: "Feature"}
		var g, p []byte
		if e = rows.Scan(&v.ID, &g, &p, &out.Total); e != nil {
			return out, e
		}
		v.Geometry = g
		if e = json.Unmarshal(p, &v.Properties); e != nil {
			return out, e
		}
		if family == "imagery" && scoped && temporal.BackgroundImagery {
			v.Properties["backgroundContext"] = true
			stamp, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(v.Properties["date"]))
			v.Properties["outsideMonitoringPeriod"] = stamp.Before(temporal.Interval.From) || !stamp.Before(temporal.Interval.To)
			out.CountMeaning = "Retained background acquisitions with explicit dates; excluded from current-observation counts"
		}
		out.Features = append(out.Features, v)
	}
	out.Truncated = out.Total > len(out.Features)
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	if out.Truncated {
		prefix := query[:strings.Index(query, " SELECT id,ST_AsGeoJSON")]
		aggregate := prefix + `,span AS(SELECT GREATEST(.001,(max(ST_X(ST_PointOnSurface(geom)))-min(ST_X(ST_PointOnSurface(geom))))/40,(max(ST_Y(ST_PointOnSurface(geom)))-min(ST_Y(ST_PointOnSurface(geom))))/40) size FROM filtered),cells AS(SELECT floor(ST_X(ST_PointOnSurface(geom))/size) x,floor(ST_Y(ST_PointOnSurface(geom))/size) y,count(*) n,avg(ST_X(ST_PointOnSurface(geom))) lng,avg(ST_Y(ST_PointOnSurface(geom))) lat FROM filtered,span GROUP BY 1,2) SELECT jsonb_build_object('type','FeatureCollection','total',(SELECT count(*) FROM filtered),'features',COALESCE(jsonb_agg(jsonb_build_object('type','Feature','geometry',jsonb_build_object('type','Point','coordinates',jsonb_build_array(lng,lat)),'properties',jsonb_build_object('aggregate',true,'count',n,'recordId','aggregate:'||x||':'||y,'kind',$8::text,'title',n||' retained records'))),'[]'::jsonb)) FROM cells`
		var raw []byte
		if e = db.Scoped(s.pool).QueryRow(ctx, aggregate, f.RegionID, bounds, from, to, f.SourceID, f.Geometry, scoped, family).Scan(&raw); e != nil {
			return out, e
		}
		if e = json.Unmarshal(raw, &out); e != nil {
			return out, e
		}
		out.State = "aggregated"
		out.Truncated = false
		out.Scope = f.RegionID
		out.CountMeaning = "Complete scoped count; display aggregates are not provider locations"
	}
	return out, nil
}

func (h *Handler) mountMap(r chi.Router) {
	r.Get("/map/features", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.s.MapFeatures(r.Context(), r.URL.Query().Get("family"), r.URL.Query())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/map/reference/{z}/{x}/{y}.pbf", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		z, e1 := strconv.Atoi(chi.URLParam(r, "z"))
		x, e2 := strconv.Atoi(chi.URLParam(r, "x"))
		y, e3 := strconv.Atoi(chi.URLParam(r, "y"))
		ids := strings.Split(r.URL.Query().Get("datasets"), ",")
		if e1 != nil || e2 != nil || e3 != nil || z < 0 || z > 20 || x < 0 || y < 0 || float64(x) >= math.Pow(2, float64(z)) || float64(y) >= math.Pow(2, float64(z)) || len(ids) > 32 || len(ids) < 1 {
			httpx.Error(w, apperr.Validation("invalid reference tile"))
			return
		}
		var data []byte
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `WITH tile AS(SELECT ST_TileEnvelope($1,$2,$3) b), features AS(SELECT f.feature_id, f.dataset_id,f.properties,ST_AsMVTGeom(ST_Transform(f.geometry,3857),t.b,4096,64,true) geom FROM reference_features f,tile t WHERE f.dataset_id=ANY($4::text[]) AND f.geometry&&ST_Transform(ST_Expand(t.b,(ST_XMax(t.b)-ST_XMin(t.b))*.02),4326)) SELECT ST_AsMVT(features,'reference',4096,'geom') FROM features`, z, x, y, ids).Scan(&data)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
}
