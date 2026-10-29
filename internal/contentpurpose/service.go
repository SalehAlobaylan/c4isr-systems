// Package contentpurpose classifies catalog use without rewriting evidence.
package contentpurpose

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Input struct {
	Kind             string `json:"kind"`
	EntityID         string `json:"entityId"`
	Purpose          string `json:"purpose"`
	Reason           string `json:"reason"`
	ReceiptReference string `json:"receiptReference"`
	ExpectedVersion  int    `json:"expectedVersion"`
}
type Entry struct {
	Kind             string     `json:"kind"`
	EntityID         string     `json:"entityId"`
	Purpose          string     `json:"purpose"`
	Version          int        `json:"version"`
	Reason           string     `json:"reason"`
	ReceiptReference string     `json:"receiptReference"`
	ActorID          string     `json:"actorId"`
	RecordedAt       *time.Time `json:"recordedAt,omitempty"`
}
type Service struct{ Pool *pgxpool.Pool }

// The inventory is evaluated under the same non-owner scope/source RLS as the
// underlying catalogs. No name-based inference or arbitrary table input occurs.
const inventory = `WITH entities AS(
 SELECT 'reference' kind,id,source_id FROM reference_datasets
 UNION ALL SELECT 'artifact',id,source_id FROM artifacts
 UNION ALL SELECT 'scene',id,source_id FROM imagery_scenes
 UNION ALL SELECT 'capture',id,source_id FROM imagery_capture_jobs
 UNION ALL SELECT 'job',id,source_id FROM imagery_jobs
 UNION ALL SELECT 'assessment',assessment_id,source_id FROM assessment_review_heads
 UNION ALL SELECT 'package',id,document#>>'{assets,0,sourceId}' FROM context_packages
 WHERE NOT EXISTS(SELECT 1 FROM jsonb_array_elements(document->'assets') a
 WHERE COALESCE(current_setting('emad.source_ids',true),'null') NOT IN ('null','[]')
 AND NOT a->>'sourceId' IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
 ) `

func CatalogView(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view := r.URL.Query().Get("content_view")
		if view == "" {
			next.ServeHTTP(w, r)
			return
		}
		if view != "ordinary" && view != "all" && view != "working" && view != "unclassified" {
			httpx.Error(w, apperr.Validation("content_view must be ordinary, working, unclassified or all"))
			return
		}
		if view != "ordinary" {
			if e := access.Require(r.Context(), "sources.manage"); e != nil {
				httpx.Error(w, e)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(access.WithContentView(r.Context(), view)))
	})
}
func (s Service) List(ctx context.Context, kind, view string, offset int) ([]Entry, int, error) {
	if e := access.Require(ctx, "sources.manage"); e != nil {
		return nil, 0, e
	}
	resource, e := access.ResourceFrom(ctx)
	if e != nil {
		return nil, 0, e
	}
	if resource.Partition != access.PublicEvidence {
		return nil, 0, apperr.Forbidden("content purpose is Public Context metadata")
	}
	if offset < 0 || offset > 100000 || view != "all" && view != "unclassified" {
		return nil, 0, apperr.Validation("choose all/unclassified and a bounded offset")
	}
	if kind != "" && !validKind(kind) {
		return nil, 0, apperr.Validation("unknown content kind")
	}
	allowed := []string{}
	for _, k := range []string{"reference", "scene", "capture", "job", "assessment", "package"} {
		if access.Allows(ctx, entityPermission(k)) {
			allowed = append(allowed, k)
		}
	}
	if kind != "" {
		if e := access.Require(ctx, entityPermission(kind)); e != nil {
			return nil, 0, e
		}
	}
	result := []Entry{}
	total := 0
	var raw []byte
	e = db.Scoped(s.Pool).QueryRow(ctx, inventory+`, matched AS(
       SELECT e.kind,e.id,COALESCE(c.purpose,'unclassified') purpose,COALESCE(c.version,0) version,
       COALESCE(c.reason,'') reason,COALESCE(c.receipt_reference,'') receipt_reference,COALESCE(c.actor_id,'') actor_id,c.recorded_at
       FROM entities e LEFT JOIN LATERAL(SELECT * FROM content_purpose_revisions WHERE entity_kind=e.kind AND entity_id=e.id ORDER BY version DESC LIMIT 1)c ON true
       WHERE e.kind=ANY($4::text[]) AND ($1='' OR e.kind=$1) AND ($2='all' OR COALESCE(c.purpose,'unclassified')='unclassified')
    ), page AS(SELECT * FROM matched ORDER BY kind,id LIMIT 100 OFFSET $3)
    SELECT (SELECT count(*) FROM matched),COALESCE((SELECT jsonb_agg(jsonb_build_object('kind',kind,'entityId',id,'purpose',purpose,'version',version,'reason',reason,'receiptReference',receipt_reference,'actorId',actor_id,'recordedAt',recorded_at) ORDER BY kind,id) FROM page),'[]'::jsonb)`, kind, view, offset, allowed).Scan(&total, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &result)
	}

	return result, total, e
}
func entityPermission(kind string) string {
	if kind == "assessment" {
		return "assessments.read"
	}
	if kind == "package" {
		return "context.export"
	}
	return "sources.read"
}
func validKind(kind string) bool {
	return kind == "artifact" || kind == "reference" || kind == "scene" || kind == "capture" || kind == "job" || kind == "assessment" || kind == "package"
}
func (s Service) Classify(ctx context.Context, in Input) (Entry, error) {
	var out Entry
	if e := access.Require(ctx, "sources.manage"); e != nil {
		return out, e
	}
	p, resource, _ := access.From(ctx)
	if resource.Partition != access.PublicEvidence || p.Kind != "human" {
		return out, apperr.Forbidden("human source administration in Public Context is required")
	}
	if !validKind(in.Kind) || len(in.EntityID) < 1 || len(in.EntityID) > 256 || len(strings.TrimSpace(in.Reason)) < 10 || len(in.Reason) > 2000 || len(in.ReceiptReference) < 1 || len(in.ReceiptReference) > 1024 || in.ExpectedVersion < 0 {
		return out, apperr.Validation("provide an exact object, reason, receipt reference and expected version")
	}
	if e := access.Require(ctx, entityPermission(in.Kind)); e != nil {
		return out, e
	}
	if in.Purpose != "working" && in.Purpose != "development" && in.Purpose != "acceptance" && in.Purpose != "unclassified" {
		return out, apperr.Validation("unknown content purpose")
	}
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var source string
		e := tx.QueryRow(ctx, inventory+`SELECT source_id FROM entities WHERE kind=$1 AND id=$2`, in.Kind, in.EntityID).Scan(&source)
		if e == pgx.ErrNoRows {
			return apperr.NotFound("catalog object", "unavailable")
		}
		if e != nil {
			return e
		}
		if e = access.RequireSource(ctx, source); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/purpose/"+in.Kind+"/"+in.EntityID); e != nil {
			return e
		}
		current := Entry{}
		e = tx.QueryRow(ctx, `SELECT purpose,version,reason,receipt_reference,actor_id,recorded_at FROM content_purpose_revisions WHERE entity_kind=$1 AND entity_id=$2 ORDER BY version DESC LIMIT 1`, in.Kind, in.EntityID).Scan(&current.Purpose, &current.Version, &current.Reason, &current.ReceiptReference, &current.ActorID, &current.RecordedAt)
		if e != nil && e != pgx.ErrNoRows {
			return e
		}
		if current.Purpose == in.Purpose && current.Reason == in.Reason && current.ReceiptReference == in.ReceiptReference {
			out = current
			out.Kind = in.Kind
			out.EntityID = in.EntityID
			return nil
		}
		if current.Version != in.ExpectedVersion {
			return apperr.Conflict("content purpose changed; reload before classifying")
		}
		if (current.Purpose == "development" || current.Purpose == "acceptance") && in.Purpose == "working" && in.Kind == "assessment" {
			return apperr.Conflict("a test publication remains a test; create a separately reviewed working successor")
		}
		now := time.Now().UTC()
		out = Entry{in.Kind, in.EntityID, in.Purpose, current.Version + 1, in.Reason, in.ReceiptReference, p.ID, &now}
		_, e = tx.Exec(ctx, `INSERT INTO content_purpose_revisions(id,entity_kind,entity_id,source_id,version,purpose,reason,receipt_reference,actor_id,recorded_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, ids.New("purpose"), in.Kind, in.EntityID, source, out.Version, in.Purpose, in.Reason, in.ReceiptReference, p.ID, now)
		return e
	})
	return out, e
}
func (s Service) Mount(r chi.Router) {
	r.Get("/content-purpose", func(w http.ResponseWriter, r *http.Request) {
		view := r.URL.Query().Get("view")
		if view == "" {
			view = "unclassified"
		}
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if value, e := strconv.Atoi(raw); e != nil {
				httpx.Error(w, apperr.Validation("invalid offset"))
				return
			} else {
				offset = value
			}
		}
		entries, total, e := s.List(r.Context(), r.URL.Query().Get("kind"), view, offset)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": entries, "total": total, "offset": offset, "hasMore": offset+len(entries) < total})
	})
	r.Post("/content-purpose", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Classify(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
}
