// Package catalog materializes bounded, permission-bound catalog reads. Offset
// discovery is a legacy API; opaque continuation pages never requery live heads.
package catalog

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/reads"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"time"
)

type Filters struct {
	SourceID  string `json:"sourceId,omitempty"`
	Query     string `json:"query,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Kind      string `json:"kind,omitempty"`
	ReleaseID string `json:"releaseId,omitempty"`
	EditionID string `json:"editionId,omitempty"`
	Status    string `json:"status,omitempty"`
}
type Input struct {
	Kind              string              `json:"kind"`
	Scope             *analystscope.Input `json:"scope,omitempty"`
	Filters           Filters             `json:"filters"`
	Limit             int                 `json:"limit"`
	Zoom              int                 `json:"zoom"`
	SnapshotID        string              `json:"snapshotId,omitempty"`
	Cursor            string              `json:"cursor,omitempty"`
	ReplaceSnapshotID string              `json:"replaceSnapshotId,omitempty"`
}
type Snapshot struct {
	Schema          string                 `json:"schema"`
	ID              string                 `json:"id"`
	Kind            string                 `json:"kind"`
	Fingerprint     string                 `json:"fingerprint"`
	Scope           *analystscope.Snapshot `json:"scope,omitempty"`
	EvaluatedAt     time.Time              `json:"evaluatedAt"`
	ExpiresAt       time.Time              `json:"expiresAt"`
	RequiredSources []string               `json:"requiredSources"`
	IDs             []string               `json:"ids"`
	Items           []json.RawMessage      `json:"items"`
	State           string                 `json:"state"`
}
type Page struct {
	SnapshotID       string            `json:"snapshotId"`
	QueryFingerprint string            `json:"queryFingerprint"`
	ScopeHash        string            `json:"scopeHash,omitempty"`
	Kind             string            `json:"kind"`
	Items            []json.RawMessage `json:"items"`
	Total            *int              `json:"total"`
	Returned         int               `json:"returned"`
	Offset           int               `json:"offset"`
	Representation   string            `json:"representation"`
	State            string            `json:"state"`
	CountMeaning     string            `json:"countMeaning"`
	EvaluatedAt      time.Time         `json:"evaluatedAt"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	HasMore          bool              `json:"hasMore"`
	NextCursor       string            `json:"nextCursor,omitempty"`
}
type Service struct{ Pool *pgxpool.Pool }

func permission(kind string) string {
	switch kind {
	case "investigations":
		return "investigations.read"
	case "findings":
		return "assessments.read"
	case "geography", "scenes", "captures", "jobs":
		return "sources.read"
	}
	return ""
}
func (s Service) Read(ctx context.Context, in Input) (Page, error) {
	var out Page
	if e := areas.Require(ctx); e != nil {
		return out, e
	}
	perm := permission(in.Kind)
	if perm == "" {
		return out, apperr.Validation("choose a supported catalog")
	}
	if e := access.Require(ctx, perm); e != nil {
		return out, e
	}
	if in.Limit == 0 {
		in.Limit = 100
	}
	if in.Limit < 1 || in.Limit > 300 || in.Zoom < 0 || in.Zoom > 24 || len(in.Filters.Query) > 160 || len(in.Filters.Parent) > 160 || len(in.Filters.Kind) > 40 || len(in.Filters.Status) > 40 || len(in.Filters.SourceID) > 512 || len(in.Filters.ReleaseID) > 160 || len(in.Filters.EditionID) > 160 || in.Filters.ReleaseID != "" && in.Filters.EditionID != "" {
		return out, apperr.Validation("invalid bounded catalog query")
	}
	if in.Filters.SourceID != "" {
		if e := access.RequireSource(ctx, in.Filters.SourceID); e != nil {
			return out, e
		}
	}
	if in.Filters.Kind != "" && (in.Kind != "geography" || !strings.Contains("|country|region|governorate|place|study|", "|"+in.Filters.Kind+"|")) {
		return out, apperr.Validation("unsupported catalog kind filter")
	}
	if len(in.Cursor) > 128 || len(in.SnapshotID) > 128 || len(in.ReplaceSnapshotID) > 128 || in.Cursor != "" && in.ReplaceSnapshotID != "" {
		return out, apperr.Validation("invalid read continuation or replacement")
	}
	if (in.Cursor == "") != (in.SnapshotID == "") {
		return out, apperr.Validation("supply a snapshot and its opaque continuation together")
	}
	identity := in
	identity.Cursor = ""
	identity.ReplaceSnapshotID = ""
	identity.SnapshotID = ""
	raw, e := jsoncanon.Marshal(identity)
	if e != nil {
		return out, e
	}
	fingerprint := areas.Hash(raw)
	if _, e = s.Pool.Exec(ctx, `DELETE FROM analyst_read_snapshots WHERE expires_at<clock_timestamp()`); e != nil {
		return out, e
	}
	e = db.RepeatableRead(ctx, s.Pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SET LOCAL statement_timeout='12s'`); e != nil {
			return e
		}
		var snap Snapshot
		offset := 0
		if in.Cursor != "" {
			if e := reads.Load(ctx, tx, in.SnapshotID, "catalog", &snap); e != nil {
				return e
			}
			if snap.Fingerprint != fingerprint || snap.Kind != in.Kind {
				return apperr.Conflict("cursor query or scope changed; restart the catalog")
			}
			for _, source := range snap.RequiredSources {
				if e := access.RequireSource(ctx, source); e != nil {
					return e
				}
			}
			if e := s.visible(ctx, tx, snap); e != nil {
				return e
			}
			var e error
			offset, e = reads.Position(ctx, tx, in.Cursor, snap.ID, fingerprint, in.Limit)
			if e != nil {
				return e
			}
		} else {
			snap = Snapshot{Schema: "emad.catalog-read.v1", ID: ids.New("catalog"), Kind: in.Kind, Fingerprint: fingerprint, EvaluatedAt: time.Now().UTC(), RequiredSources: []string{}, IDs: []string{}, Items: []json.RawMessage{}, State: "available"}
			if in.Scope != nil {
				scope, e := (analystscope.Service{Pool: s.Pool}).Resolve(ctx, *in.Scope)
				if e != nil {
					return e
				}
				snap.Scope = &scope
				snap.EvaluatedAt = scope.Temporal.EvaluatedAt
				snap.RequiredSources = scope.RequiredSources
				family := in.Kind
				if family == "jobs" || family == "findings" {
					family = "imagery"
				}
				if support, ok := scope.Capabilities[family]; ok && support.State == "unsupported" {
					snap.State = "historical_unsupported"
				}
			}
			if snap.State == "available" {
				query, args := s.query(in, snap.Scope)
				rows, e := tx.Query(ctx, query, args...)
				if e != nil {
					return e
				}
				bytes := 0
				for rows.Next() {
					var id string
					var item []byte
					if e = rows.Scan(&id, &item); e != nil {
						rows.Close()
						return e
					}
					bytes += len(item) + len(id) + 8
					if bytes > reads.MaxBytes-32768 || len(snap.Items) >= reads.MaxRecords {
						rows.Close()
						return apperr.Conflict("catalog read budget exceeded; preserve the scope and select a smaller area, period or source")
					}
					snap.IDs = append(snap.IDs, id)
					snap.Items = append(snap.Items, json.RawMessage(item))
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					return e
				}
			}
			if e := tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '5 minutes'`).Scan(&snap.ExpiresAt); e != nil {
				return e
			}
			doc, _ := json.Marshal(snap)
			if in.ReplaceSnapshotID != "" {
				if _, e := tx.Exec(ctx, `DELETE FROM analyst_read_snapshots WHERE id=$1 AND snapshot_kind='catalog' AND document->>'fingerprint'=$2`, in.ReplaceSnapshotID, fingerprint); e != nil {
					return e
				}
			}
			if e := reads.Save(ctx, tx, snap.ID, "catalog", doc, snap.ExpiresAt); e != nil {
				return e
			}
		}
		total := len(snap.Items)
		if offset > total {
			return apperr.Conflict("cursor outside frozen membership")
		}
		end := min(offset+in.Limit, total)
		out = Page{SnapshotID: snap.ID, QueryFingerprint: snap.Fingerprint, Kind: snap.Kind, Items: snap.Items[offset:end], Total: &total, Returned: end - offset, Offset: offset, Representation: "individual_records", State: snap.State, CountMeaning: "Exact authorized membership at evaluation; pages retain the same order and revisions.", EvaluatedAt: snap.EvaluatedAt, ExpiresAt: snap.ExpiresAt, HasMore: end < total}
		if snap.Scope != nil {
			out.ScopeHash = snap.Scope.ScopeHash
		}
		if snap.State == "historical_unsupported" {
			out.Total = nil
			out.CountMeaning = "Query unsupported for this historical interpretation; count unknown."
		}
		if out.HasMore {
			var e error
			out.NextCursor, e = reads.Cursor(ctx, tx, snap.ID, fingerprint, end, in.Limit)
			return e
		}
		return nil
	})
	return out, reads.Normalize(e)
}
func (s Service) visible(ctx context.Context, tx pgx.Tx, v Snapshot) error {
	if len(v.IDs) == 0 {
		return nil
	}
	query := ""
	switch v.Kind {
	case "geography":
		query = `SELECT count(*) FROM geographic_regions WHERE id=ANY($1)`
	case "scenes":
		query = `SELECT count(*) FROM imagery_scenes WHERE id=ANY($1) AND emad_content_visible('scene',id)`
	case "captures":
		query = `SELECT count(*) FROM imagery_capture_jobs WHERE id=ANY($1) AND emad_content_visible('capture',id)`
	case "jobs":
		query = `SELECT count(*) FROM imagery_jobs WHERE id=ANY($1) AND emad_content_visible('job',id)`
	case "findings":
		query = `SELECT count(*) FROM assessment_revisions WHERE id=ANY($1) AND emad_content_visible('assessment',assessment_id)`
	case "investigations":
		query = `SELECT count(*) FROM investigations WHERE id=ANY($1)`
	}
	var count int
	if e := tx.QueryRow(ctx, query, v.IDs).Scan(&count); e != nil {
		return e
	}
	if count != len(v.IDs) {
		return apperr.Conflict("catalog dependencies or membership became unavailable; refresh before reading counts or records")
	}
	return nil
}
func (s Service) query(in Input, scope *analystscope.Snapshot) (string, []any) {
	geometry := ""
	var from, to *time.Time
	if scope != nil {
		geometry = string(scope.Area.Geometry)
		if geometry == "null" {
			geometry = ""
		}
		from = &scope.Temporal.Interval.From
		to = &scope.Temporal.Interval.To
	}
	f := in.Filters
	common := []any{geometry, from, to, f.SourceID, f.Status}
	switch in.Kind {
	case "scenes":
		return `SELECT id,document FROM imagery_scenes WHERE emad_content_visible('scene',id) AND ($1='' OR ST_Intersects(geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($1,'')),4326))) AND ($2::timestamptz IS NULL OR acquired_at>=$2) AND ($3::timestamptz IS NULL OR acquired_at<$3) AND ($4='' OR source_id=$4) AND $5::text='' ORDER BY acquired_at DESC,id LIMIT 50001`, common
	case "captures":
		return `SELECT id,jsonb_build_object('id',id,'sourceId',source_id,'input',input,'status',status,'attempt',attempt,'progress',progress,'errorCode',error_code,'result',result,'createdAt',created_at) FROM imagery_capture_jobs WHERE emad_content_visible('capture',id) AND ($1='' OR ST_Intersects(ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($1,'')),4326),ST_MakeEnvelope((input#>>'{aoi,west}')::float8,(input#>>'{aoi,south}')::float8,(input#>>'{aoi,east}')::float8,(input#>>'{aoi,north}')::float8,4326))) AND ($2::timestamptz IS NULL OR (input#>>'{scene,acquiredAt}')::timestamptz>=$2) AND ($3::timestamptz IS NULL OR (input#>>'{scene,acquiredAt}')::timestamptz<$3) AND ($4='' OR source_id=$4) AND ($5='' OR status=$5) ORDER BY created_at DESC,id LIMIT 50001`, common
	case "jobs":
		return `SELECT j.id,jsonb_build_object('id',j.id,'sourceId',j.source_id,'status',j.status,'input',j.input,'createdBy',j.created_by,'createdAt',j.created_at,'attempt',j.attempt,'workerBuild',COALESCE(j.worker_build,''),'errorCode',j.error_code,'result',j.result,'finishedAt',j.finished_at) FROM imagery_jobs j WHERE emad_content_visible('job',j.id) AND ` + imagery.JobScopePredicate("j", 1) + ` AND ($4='' OR j.source_id=$4) AND ($5='' OR j.status=$5) ORDER BY j.created_at DESC,j.id LIMIT 50001`, common
	case "findings":
		return `SELECT r.id,r.document FROM assessment_review_heads h JOIN assessment_revisions r ON r.id=h.latest_id AND r.scope_id=h.scope_id WHERE emad_content_visible('assessment',r.assessment_id) AND NOT (r.document ? 'reportEvidence') AND EXISTS(SELECT 1 FROM imagery_jobs j WHERE j.id=r.document#>>'{evidence,id}' AND ` + imagery.JobScopePredicate("j", 1) + ` AND ($4='' OR j.source_id=$4)) AND ($5='' OR r.document->>'status'=$5) ORDER BY r.document->>'recordedAt' DESC,r.id LIMIT 50001`, common
	case "investigations":
		return `SELECT c.id,jsonb_build_object('id',c.id,'ownerHumanId',c.owner_human_id,'members',c.members,'membershipVersion',c.membership_version,'revision',r.document) FROM investigations c JOIN investigation_revisions r ON r.id=c.head_revision_id AND r.scope_id=c.scope_id WHERE ($1='' OR ST_Intersects(ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($1,'')),4326),ST_SetSRID(ST_GeomFromGeoJSON(NULLIF(r.document#>'{scope,area,geometry}','null'::jsonb)),4326))) AND ($2::timestamptz IS NULL OR (r.document#>>'{scope,temporal,interval,to}')::timestamptz>$2) AND ($3::timestamptz IS NULL OR (r.document#>>'{scope,temporal,interval,from}')::timestamptz<$3) AND ($4='' OR c.required_sources @> jsonb_build_array($4::text)) AND ($5='' OR r.document->>'state'=$5) ORDER BY r.document->>'recordedAt' DESC,c.id LIMIT 50001`, common
	case "geography":
		prefix, join := reference.GeographyDiscoveryCTE, `JOIN chosen c ON c.id=g.id `
		if f.ReleaseID != "" || f.EditionID != "" {
			prefix = ""
			join = ""
		}
		return prefix + `SELECT g.id,jsonb_build_object('type','Feature','id',g.id,'geometry',ST_AsGeoJSON(ST_SimplifyPreserveTopology(g.geometry,360.0/power(2,$7::int)/1024),9)::jsonb,'properties',g.properties||jsonb_build_object('name',g.name,'kind',g.kind,'regionKey',g.region_key,'parentKey',g.parent_key,'sourceId',g.source_id,'editionId',g.edition_id,'releaseId',COALESCE(NULLIF($5,''),(SELECT h.release_id FROM geographic_release_heads h WHERE h.source_id=g.source_id AND h.scope_id=g.scope_id AND EXISTS(SELECT 1 FROM geographic_release_editions e WHERE e.release_id=h.release_id AND e.edition_id=g.edition_id))),'displayOnly',true,'meaning','Simplified navigation display; resolve the exact source feature for analysis.')) FROM geographic_regions g ` + join + `WHERE ($8::text='' OR ST_Intersects(g.geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($8,'')),4326))) AND ($1='' OR strpos(emad_geographic_search(g.name||' '||g.aliases),emad_geographic_search($1))>0) AND ($2='' OR g.parent_key=$2) AND ($3='' OR g.kind=$3) AND ($4='' OR g.source_id=$4) AND ($5='' OR EXISTS(SELECT 1 FROM geographic_release_editions e WHERE e.release_id=$5 AND e.edition_id=g.edition_id)) AND ($6='' OR g.edition_id=$6) ORDER BY g.kind,g.name,g.id LIMIT 50001`, []any{f.Query, f.Parent, f.Kind, f.SourceID, f.ReleaseID, f.EditionID, in.Zoom, geometry}
	}
	return "", nil
}
func (s Service) Mount(r chi.Router) {
	r.Post("/catalog/reads", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		v, e := s.Read(ctx, in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if v.ScopeHash != "" {
			w.Header().Set("X-Analyst-Scope-Hash", v.ScopeHash)
		}
		httpx.JSON(w, 200, v)
	})
}
