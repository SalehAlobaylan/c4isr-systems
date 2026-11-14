package geoenv

import (
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

type storageItem struct {
	AssetID       string     `json:"assetId"`
	ReleaseID     string     `json:"releaseId,omitempty"`
	DisplayID     string     `json:"displayId,omitempty"`
	BuildID       string     `json:"buildId,omitempty"`
	State         string     `json:"state"`
	FailureCode   string     `json:"failureCode,omitempty"`
	Bytes         int64      `json:"bytes"`
	RetainUntil   time.Time  `json:"retainUntil"`
	UploadUntil   *time.Time `json:"uploadUntil,omitempty"`
	PurgeRetryAt  *time.Time `json:"purgeRetryAt,omitempty"`
	PurgeAttempts int        `json:"purgeAttempts"`
}

func (s Service) storageStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Manage, false); err != nil {
		httpx.Error(w, err)
		return
	}
	after := r.URL.Query().Get("after")
	if after != "" && !keyPattern.MatchString(after) {
		httpx.Error(w, apperr.Validation("invalid asset cursor"))
		return
	}
	rows, err := db.Scoped(s.Pool).Query(ctx, `SELECT a.id,COALESCE(d.release_id,rel.id,''),COALESCE(d.id,''),
	 COALESCE(j.id,''),COALESCE(j.failure_code,''),a.state,a.size_bytes,a.retain_until,a.upload_until,a.purge_retry_at,a.purge_attempts
	 FROM geographic_assets a LEFT JOIN geographic_environment_displays d ON (d.asset_id,d.scope_id)=(a.id,a.scope_id)
	 LEFT JOIN geographic_environment_releases rel ON (rel.bundle_id,rel.scope_id)=(a.id,a.scope_id)
	 LEFT JOIN geographic_environment_builds j ON j.scope_id=a.scope_id AND j.id=CASE WHEN a.document#>>'{representation,format}'='release-zip'
	 THEN a.document->>'requestKey' WHEN d.request_key LIKE 'display-build:%' THEN substring(d.request_key from 15) END
	 WHERE a.id>$1 AND a.purged_at IS NULL AND (a.state IN ('PENDING','EXPIRED') OR a.retain_until<=now()) ORDER BY a.id LIMIT 51`, after)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer rows.Close()
	out := page[storageItem]{Items: []storageItem{}}
	for rows.Next() {
		var item storageItem
		if err := rows.Scan(&item.AssetID, &item.ReleaseID, &item.DisplayID, &item.BuildID, &item.FailureCode,
			&item.State, &item.Bytes, &item.RetainUntil, &item.UploadUntil, &item.PurgeRetryAt, &item.PurgeAttempts); err != nil {
			httpx.Error(w, err)
			return
		}
		if len(out.Items) == 50 {
			out.Next = out.Items[49].AssetID
			break
		}
		out.Items = append(out.Items, item)
	}
	respond(w, http.StatusOK, out, rows.Err())
}
