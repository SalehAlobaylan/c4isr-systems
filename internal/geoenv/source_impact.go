package geoenv

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type sourceImpactRelease struct {
	ID             string `json:"id"`
	ManifestSHA256 string `json:"manifestSha256"`
	State          string `json:"state"`
	Displays       int64  `json:"displays"`
}

type sourceImpact struct {
	SourceID       string                    `json:"sourceId"`
	SourceStatus   string                    `json:"sourceStatus"`
	CurrentPolicy  publicingest.Admission    `json:"currentPolicy"`
	RetainedInputs int64                     `json:"retainedInputs"`
	RetainedBytes  int64                     `json:"retainedBytes"`
	PendingInputs  int64                     `json:"pendingInputs"`
	Tombstones     int64                     `json:"tombstones"`
	NextExpiry     *time.Time                `json:"nextExpiry,omitempty"`
	Releases       page[sourceImpactRelease] `json:"releases"`
}

// sourcePolicyImpact exposes administrative dependency identities under the
// existing source RLS, including releases blocked by the policy being reviewed.
func (s Service) sourcePolicyImpact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, after := chi.URLParam(r, "id"), r.URL.Query().Get("after")
	if err := require(ctx, Manage, false); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := access.Require(ctx, "sources.manage"); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := access.RequireSource(ctx, id); err != nil {
		httpx.Error(w, err)
		return
	}
	if len(id) > 512 || after != "" && !keyPattern.MatchString(after) {
		httpx.Error(w, apperr.Validation("invalid source or release cursor"))
		return
	}
	out := sourceImpact{SourceID: id, Releases: page[sourceImpactRelease]{Items: []sourceImpactRelease{}}}
	err := db.RepeatableRead(ctx, s.Pool, func(ctx context.Context, tx pgx.Tx) error {
		var raw []byte
		var revision string
		err := tx.QueryRow(ctx, `SELECT s.status,a.document,h.id FROM sources s
		 JOIN retrieval_profiles p ON (p.source_id,p.scope_id)=(s.id,s.scope_id) AND p.profile_key='geographic-retained'
		 JOIN retrieval_profile_revisions h ON (h.id,h.scope_id)=(p.head_revision_id,p.scope_id)
		 JOIN source_admissions a ON (a.id,a.scope_id)=(h.admission_id,h.scope_id) WHERE s.id=$1`, id).Scan(&out.SourceStatus, &raw, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("geographic source policy", "unavailable")
		}
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &out.CurrentPolicy); err != nil {
			return err
		}
		out.CurrentPolicy.ProfileKey = "geographic-retained"
		out.CurrentPolicy.ProfileRevisionID = revision
		if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.state='READY' AND o.purged_at IS NULL AND o.retain_until>now()),
		 COALESCE(sum(o.size_bytes) FILTER(WHERE o.state='READY' AND o.purged_at IS NULL AND o.retain_until>now()),0)::bigint,
		 count(*) FILTER(WHERE o.state='PENDING' AND o.retain_until>now()),
		 count(*) FILTER(WHERE o.state='EXPIRED'),
		 min(LEAST(o.retain_until,(a.document#>>'{rights,expiresAt}')::timestamptz,$2::timestamptz)) FILTER(WHERE o.state='READY' AND o.retain_until>now())
		 FROM geographic_assets o JOIN source_admissions a ON (a.id,a.scope_id)=(o.admission_id,o.scope_id)
		 WHERE o.source_id=$1 AND o.document#>>'{representation,format}' IN
		 ('geotiff','geojson','osm-pbf','osm-xml','geopackage','shapefile-zip','radar-capture-zip','era5-pressure-json','gebco-pair-zip')`, id, out.CurrentPolicy.Rights.ExpiresAt).
			Scan(&out.RetainedInputs, &out.RetainedBytes, &out.PendingInputs, &out.Tombstones, &out.NextExpiry); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT r.id,r.manifest_hash,
		 COALESCE((SELECT e.state FROM geographic_environment_release_events e WHERE (e.release_id,e.scope_id)=(r.id,r.scope_id) ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1),'STAGED'),
		 (SELECT count(*) FROM geographic_environment_displays d WHERE (d.release_id,d.scope_id)=(r.id,r.scope_id))
		 FROM geographic_environment_releases r WHERE r.id>$2 AND r.required_sources @> jsonb_build_array($1::text)
		 ORDER BY r.id LIMIT 21`, id, after)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item sourceImpactRelease
			if err := rows.Scan(&item.ID, &item.ManifestSHA256, &item.State, &item.Displays); err != nil {
				return err
			}
			if len(out.Releases.Items) == 20 {
				out.Releases.Next = out.Releases.Items[19].ID
				break
			}
			out.Releases.Items = append(out.Releases.Items, item)
		}
		return rows.Err()
	})
	respond(w, http.StatusOK, out, err)
}
