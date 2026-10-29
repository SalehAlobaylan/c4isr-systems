package analystscope

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/reads"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"time"
)

// RecordPage reads individual provider revisions from the very same manifest
// as its aggregates. New publication/corrections cannot alter this membership.
func (s Service) RecordPage(ctx context.Context, id, family, aggregate, cursor string, limit int) (map[string]any, error) {
	if e := access.Require(ctx, "observations.read"); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 200 {
		return nil, apperr.Validation("record page size must be 1–200")
	}
	var out map[string]any
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SET LOCAL statement_timeout='12s'`); e != nil {
			return e
		}
		var snap ReadSnapshot
		if e := reads.Load(ctx, tx, id, "map", &snap); e != nil {
			return e
		}
		values, ok := snap.RecordSets[family]
		if !ok {
			return apperr.Conflict("this family has no individual membership in the selected read")
		}
		// Validate every original member before exposing either a count or a page.
		// This catches purpose/dependency changes as well as grant revision changes.
		ids := values
		var visible int
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM public_evidence e JOIN public_evidence_catalog c ON c.evidence_id=e.id AND c.scope_id=e.scope_id WHERE e.id=ANY($1::text[])`, ids).Scan(&visible); e != nil {
			return e
		}
		if visible != len(ids) {
			return apperr.Conflict("read dependencies became unavailable; refresh before viewing counts or records")
		}
		bounds := []float64{}
		if aggregate != "" {
			layer, ok := snap.Layers[family]
			if !ok {
				return apperr.Validation("unknown read family")
			}
			for _, f := range layer.Features {
				if f.Properties["aggregateId"] == aggregate {
					raw, _ := json.Marshal(f.Properties["cellBounds"])
					_ = json.Unmarshal(raw, &bounds)
					break
				}
			}
			if len(bounds) != 4 {
				return apperr.Conflict("aggregate is not part of this read")
			}
		}
		key := snap.QueryFingerprint + "/records/" + family + "/" + aggregate
		offset := 0
		if cursor != "" {
			var e error
			offset, e = reads.Position(ctx, tx, cursor, id, key, limit)
			if e != nil {
				return e
			}
		}
		total, items, e := publicevents.MemberPage(db.UseTx(ctx, tx), s.Pool, ids, bounds, offset, limit)
		if e != nil {
			return e
		}
		if offset > total {
			return apperr.Conflict("record cursor outside frozen membership")
		}
		end := offset + len(items)
		out = map[string]any{"snapshotId": id, "scopeHash": snap.Scope.ScopeHash, "queryFingerprint": snap.QueryFingerprint, "evaluatedAt": snap.Scope.Temporal.EvaluatedAt, "expiresAt": snap.ExpiresAt, "family": family, "aggregateId": aggregate, "items": items, "total": total, "returned": len(items), "representation": "individual_provider_revisions", "hasMore": end < total, "meaning": "Exact selected retained revisions from this read; reported geometry is preserved. Current validity annotations are evaluated separately at this page read."}
		if end < total {
			token, e := reads.Cursor(ctx, tx, id, key, end, limit)
			if e != nil {
				return e
			}
			out["nextCursor"] = token
		}
		return nil
	})
	return out, e
}
func (s Service) mountRecords(r chi.Router) {
	r.Get("/analyst-scope/reads/{id}/records", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		q := r.URL.Query()
		for k, v := range q {
			if len(v) != 1 || k != "family" && k != "aggregate_id" && k != "cursor" && k != "limit" {
				httpx.Error(w, apperr.Validation("invalid record cursor query"))
				return
			}
		}
		limit := 100
		if q.Get("limit") != "" {
			v, e := strconv.Atoi(q.Get("limit"))
			if e != nil {
				httpx.Error(w, apperr.Validation("invalid record page size"))
				return
			}
			limit = v
		}
		v, e := s.RecordPage(ctx, chi.URLParam(r, "id"), q.Get("family"), q.Get("aggregate_id"), q.Get("cursor"), limit)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if hash, ok := v["scopeHash"].(string); ok {
			w.Header().Set("X-Analyst-Scope-Hash", hash)
		}
		httpx.JSON(w, 200, v)
	})
}
