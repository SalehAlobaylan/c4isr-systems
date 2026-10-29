package reference

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type GeographyAvailability struct {
	RegionID   string `json:"regionId"`
	SourceID   string `json:"sourceId"`
	ReleaseID  string `json:"releaseId"`
	LogicalKey string `json:"logicalKey"`
	Regions    int    `json:"regions"`
	Places     int    `json:"places"`
	State      string `json:"state"`
	Meaning    string `json:"meaning"`
	NextAction string `json:"nextAction"`
}

func (s *Service) GeographyAvailability(ctx context.Context, id, release string) (GeographyAvailability, error) {
	out := GeographyAvailability{RegionID: id, ReleaseID: release, State: "legacy_unqualified", Meaning: "Navigation detail only; place points are not city boundaries and source scale is not survey accuracy.", NextAction: "request_reference_import"}
	if e := require(ctx, "sources.read"); e != nil {
		return out, e
	}
	e := db.Scoped(s.pool).QueryRow(ctx, `SELECT source_id,region_key FROM geographic_regions WHERE id=$1`, id).Scan(&out.SourceID, &out.LogicalKey)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("geographic area", "unavailable")
	}
	if e != nil {
		return out, e
	}
	if release == "" {
		return out, nil
	}
	var member bool
	if e = db.Scoped(s.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM geographic_regions g JOIN geographic_release_editions re ON re.edition_id=g.edition_id WHERE g.id=$1 AND re.release_id=$2)`, id, release).Scan(&member); e != nil {
		return out, e
	}
	if !member {
		return out, apperr.Validation("selected feature is not in this release")
	}
	e = db.Scoped(s.pool).QueryRow(ctx, `WITH RECURSIVE members AS(SELECT g.* FROM geographic_regions g JOIN geographic_release_editions re ON re.edition_id=g.edition_id WHERE re.release_id=$1 AND g.source_id=$2), children AS(SELECT g.region_key,g.kind FROM members g WHERE g.region_key=$3 UNION SELECT c.region_key,c.kind FROM members c JOIN children p ON c.parent_key=p.region_key) SELECT count(*)FILTER(WHERE kind IN('region','governorate')),count(*)FILTER(WHERE kind='place') FROM children`, release, out.SourceID, out.LogicalKey).Scan(&out.Regions, &out.Places)
	if e != nil {
		return out, e
	}
	out.State = "partial_reference_depth"
	if out.Regions > 0 && out.Places > 0 {
		out.State = "reference_depth_retained"
		out.NextAction = "inspect_retained_reference"
	}
	return out, nil
}

type ReferenceRequestInput struct {
	RequestKey string `json:"requestKey"`
	RegionID   string `json:"regionId"`
	Reason     string `json:"reason"`
}

func (h *Handler) mountAvailability(r chi.Router) {
	r.Get("/geography/availability", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.s.GeographyAvailability(r.Context(), r.URL.Query().Get("region_id"), r.URL.Query().Get("release_id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/geography/reference-requests", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "geography.request"); e != nil {
			httpx.Error(w, e)
			return
		}
		var in ReferenceRequestInput
		if !areas.Decode(w, r, &in) {
			return
		}
		p, resource, _ := access.From(r.Context())
		if p.Kind != "human" || !geographicKey.MatchString(in.RequestKey) || len(strings.TrimSpace(in.Reason)) < 10 || len(in.Reason) > 2000 {
			httpx.Error(w, apperr.Validation("provide a request key, exact selected area and analytical reason"))
			return
		}
		v, e := h.s.GeographyAvailability(r.Context(), in.RegionID, "")
		if e != nil {
			httpx.Error(w, e)
			return
		}
		spec, _ := json.Marshal(in)
		id := "georeq_" + Hash([]byte(resource.ID()+"/"+p.ID+"/"+in.RequestKey))
		raw, _ := json.Marshal(map[string]any{"id": id, "input": in, "requestedBy": p.ID, "recordedAt": time.Now().UTC(), "state": "REQUESTED", "sourceId": v.SourceID, "logicalKey": v.LogicalKey, "meaning": "Reference depth request; no automatic download or execution authority."})
		e = db.WithTx(r.Context(), h.s.pool, func(tx pgx.Tx) error {
			if _, e := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id); e != nil {
				return e
			}
			var old []byte
			e := tx.QueryRow(r.Context(), `SELECT document FROM geography_reference_requests WHERE id=$1`, id).Scan(&old)
			if e == nil {
				var prior struct {
					Input ReferenceRequestInput `json:"input"`
				}
				_ = json.Unmarshal(old, &prior)
				compare, _ := json.Marshal(prior.Input)
				if string(compare) != string(spec) {
					return apperr.Conflict("reference request key reused with different input")
				}
				raw = old
				return nil
			}
			if !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			_, e = tx.Exec(r.Context(), `INSERT INTO geography_reference_requests(id,source_id,request_key,requested_by,region_id,document)VALUES($1,$2,$3,$4,$5,$6)`, id, v.SourceID, in.RequestKey, p.ID, in.RegionID, raw)
			return e
		})
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, json.RawMessage(raw))
	})
	r.Get("/geography/reference-requests", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.manage"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(h.s.pool).Query(r.Context(), `SELECT document||jsonb_build_object('resolution',resolution) FROM geography_reference_requests ORDER BY id LIMIT 100`)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, raw)
		}
		if e = rows.Err(); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "limit": 100})
	})
	r.Post("/geography/reference-requests/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.manage"); e != nil {
			httpx.Error(w, e)
			return
		}
		var in struct {
			ReleaseID string `json:"releaseId"`
		}
		if !areas.Decode(w, r, &in) {
			return
		}
		p, _, _ := access.From(r.Context())
		if p.Kind != "human" {
			httpx.Error(w, apperr.Forbidden("human source administrator required"))
			return
		}
		var source, key string
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `SELECT source_id,document->>'logicalKey' FROM geography_reference_requests WHERE id=$1`, chi.URLParam(r, "id")).Scan(&source, &key)
		if e != nil {
			httpx.Error(w, apperr.NotFound("reference request", "unavailable"))
			return
		}
		var region string
		e = db.Scoped(h.s.pool).QueryRow(r.Context(), `SELECT g.id FROM geographic_regions g JOIN geographic_release_editions re ON re.edition_id=g.edition_id WHERE re.release_id=$1 AND g.source_id=$2 AND g.region_key=$3`, in.ReleaseID, source, key).Scan(&region)
		if e != nil {
			httpx.Error(w, apperr.Validation("release does not contain the requested logical area"))
			return
		}
		v, e := h.s.GeographyAvailability(r.Context(), region, in.ReleaseID)
		if e != nil || v.Regions == 0 || v.Places == 0 {
			httpx.Error(w, apperr.Conflict("requested subdivision/place depth is not retained in that release"))
			return
		}
		resolution, _ := json.Marshal(map[string]any{"releaseId": in.ReleaseID, "regionId": region, "actor": p.ID, "recordedAt": time.Now().UTC(), "state": "INSTALLED"})
		tag, e := db.Scoped(h.s.pool).Exec(r.Context(), `UPDATE geography_reference_requests SET resolution=$2 WHERE id=$1 AND resolution IS NULL`, chi.URLParam(r, "id"), resolution)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if tag.RowsAffected() != 1 {
			httpx.Error(w, apperr.Conflict("reference request already resolved"))
			return
		}
		httpx.JSON(w, 200, json.RawMessage(resolution))
	})
}
