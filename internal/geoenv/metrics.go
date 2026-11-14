package geoenv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

const geographicOwnerBudget = 16 << 30

// environmentMetrics samples persisted queue and storage state under the caller's
// source grants. It does not expose identifiers as metric labels.
func (s Service) environmentMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, _, _ := access.From(ctx)
	permission := Manage
	if p.Kind == "service" {
		permission = Monitor
		// Existing worker-scoped samplers can continue while a dedicated
		// monitoring credential is commissioned.
		if !access.Allows(ctx, Monitor) && access.Allows(ctx, Execute) {
			permission = Execute
		}
	}
	if err := require(ctx, permission, p.Kind == "service"); err != nil {
		httpx.Error(w, err)
		return
	}
	var due, active, expired, failed, pendingBytes, retainedBytes, awaitingPurge int64
	var retries, publicationFailures, expiringAdmissions, freshWorkers int64
	var largestScratch, scratchSamples int64
	var oldest, ownerBytes, lastDuration float64
	err := db.Scoped(s.Pool).QueryRow(ctx, `
		WITH jobs AS (
			SELECT count(*) FILTER (WHERE state='QUEUED' AND next_attempt_at<=now()) AS due,
				count(*) FILTER (WHERE state='RUNNING' AND lease_until>now()) AS active,
				count(*) FILTER (WHERE state='RUNNING' AND lease_until<=now()) AS expired,
				count(*) FILTER (WHERE state='FAILED') AS failed,
				COALESCE(max(EXTRACT(epoch FROM now()-next_attempt_at))
					FILTER (WHERE state='QUEUED' AND next_attempt_at<=now()),0)::float8 AS oldest
			FROM geographic_environment_builds
		), assets AS (
			SELECT COALESCE(sum(size_bytes) FILTER (WHERE state='PENDING' AND purged_at IS NULL),0)::bigint AS pending,
				COALESCE(sum(size_bytes) FILTER (WHERE state='READY' AND purged_at IS NULL),0)::bigint AS retained,
				count(*) FILTER (WHERE purged_at IS NULL AND (state='EXPIRED' OR retain_until<=now())) AS awaiting_purge
			FROM geographic_assets
		), ownership AS (
			SELECT COALESCE(max(bytes),0)::float8 AS largest FROM (
			 SELECT sum(size_bytes) AS bytes FROM geographic_assets WHERE purged_at IS NULL GROUP BY owner_id
			) owners
		), attempts AS (
			SELECT count(*) FILTER(WHERE state='RUNNING' AND attempt>1) AS retries,
			 count(*) FILTER(WHERE state='FAILED' AND code IN ('invalid_output','publication_policy_unavailable') AND occurred_at>now()-interval '15 minutes') AS publications
			FROM geographic_environment_build_events
		), finished AS (
			SELECT e.build_id,e.attempt,e.occurred_at FROM geographic_environment_build_events e WHERE e.state='SUCCEEDED'
			ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1
		), duration AS (
			SELECT COALESCE(EXTRACT(epoch FROM (f.occurred_at-min(b.occurred_at))),0)::float8 AS seconds
			FROM finished f JOIN geographic_environment_build_events b ON b.build_id=f.build_id AND b.attempt=f.attempt AND b.state='RUNNING'
			GROUP BY f.occurred_at
		), expiring AS (
			SELECT count(*) AS admissions FROM (
			 SELECT DISTINCT a.admission_id,h.admission_id current_id FROM geographic_assets a
			 JOIN source_admissions old ON (old.id,old.scope_id)=(a.admission_id,a.scope_id)
			 JOIN retrieval_profile_revisions r ON (r.admission_id,r.scope_id)=(old.id,old.scope_id)
			 JOIN retrieval_profiles p ON (p.source_id,p.scope_id,p.profile_key)=(r.source_id,r.scope_id,r.profile_key)
			 JOIN retrieval_profile_revisions h ON (h.id,h.scope_id)=(p.head_revision_id,p.scope_id)
			 JOIN source_admissions live ON (live.id,live.scope_id)=(h.admission_id,h.scope_id)
			 JOIN sources s ON (s.id,s.scope_id)=(a.source_id,a.scope_id)
			 WHERE a.state='READY' AND a.retain_until>now() AND old.document->>'enabled'='true' AND live.document->>'enabled'='true'
			 AND s.status='active'
			 AND LEAST((old.document#>>'{rights,expiresAt}')::timestamptz,(live.document#>>'{rights,expiresAt}')::timestamptz)
			 BETWEEN now() AND now()+interval '7 days'
			) admissions
		), workers AS (
			SELECT count(*) AS fresh,count(scratch_bytes) AS sampled,COALESCE(max(scratch_bytes),0) AS scratch
			FROM platform_worker_status WHERE feature='environment-library' AND last_seen_at>now()-interval '15 seconds'
		)
		SELECT due,active,expired,failed,oldest,pending,retained,awaiting_purge,largest,retries,publications,
		 COALESCE((SELECT seconds FROM duration),0),admissions,fresh,sampled,scratch
		FROM jobs CROSS JOIN assets CROSS JOIN ownership CROSS JOIN attempts CROSS JOIN expiring CROSS JOIN workers
	`).Scan(&due, &active, &expired, &failed, &oldest, &pendingBytes, &retainedBytes, &awaitingPurge,
		&ownerBytes, &retries, &publicationFailures, &lastDuration, &expiringAdmissions, &freshWorkers, &scratchSamples, &largestScratch)
	if err != nil {
		httpx.Error(w, apperr.Internal(err))
		return
	}
	storeUp, storeSampled := s.sampleEnvironmentObject(ctx)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	for _, metric := range []struct {
		name  string
		help  string
		value any
	}{
		{"due_jobs", "Queued geographic jobs whose scheduled claim time has arrived.", due},
		{"active_leases", "Geographic jobs with an active lease.", active},
		{"expired_leases", "Geographic jobs with an expired lease awaiting recovery.", expired},
		{"failed_jobs", "Geographic jobs currently in the failed state.", failed},
		{"oldest_due_job_seconds", "Time since the oldest due geographic job became eligible.", oldest},
		{"pending_asset_bytes", "Logical bytes reserved for pending geographic assets.", pendingBytes},
		{"retained_asset_bytes", "Logical bytes recorded for ready geographic assets.", retainedBytes},
		{"assets_awaiting_purge", "Expired geographic assets awaiting deletion.", awaitingPurge},
		{"largest_visible_owner_reservation_ratio", "Largest visible owner reservation as a fraction of the configured 16 GiB limit.", ownerBytes / geographicOwnerBudget},
		{"retained_retry_attempts", "Persisted attempts after the first attempt, within retained history.", retries},
		{"publication_failures_15m", "Invalid or policy-blocked publications recorded in the last 15 minutes.", publicationFailures},
		{"last_successful_attempt_seconds", "Processing duration of the latest visible successful attempt, or zero if unavailable.", lastDuration},
		{"source_admissions_expiring_7d", "Visible geographic admission pairs expiring within seven days.", expiringAdmissions},
		{"fresh_worker_identities", "Scope-wide worker identities seen in the last 15 seconds.", freshWorkers},
		{"fresh_worker_scratch_samples", "Fresh worker identities that reported scratch usage.", scratchSamples},
		{"largest_worker_scratch_bytes", "Largest logical scratch usage reported by a fresh worker identity, or zero when unavailable.", largestScratch},
		{"object_read_sampled", "Whether an eligible retained object was available for the read probe.", storeSampled},
		{"object_read_ok", "Whether the scoped single-byte retained-object probe completed.", storeUp},
	} {
		name := "c4isr_environment_" + metric.name
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, metric.help, name, name, metric.value)
	}
}

func (s Service) sampleEnvironmentObject(ctx context.Context) (ok, sampled int) {
	var key string
	var size int64
	// Prefer small retained prepared packages to input originals. Keep the
	// probe scoped and return no key or identifying metadata in the metrics.
	err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT object_key,size_bytes FROM geographic_assets
	 WHERE state='READY' AND retain_until>now() AND document#>>'{representation,format}'='release-zip'
	 ORDER BY id LIMIT 1`).Scan(&key, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0
	}
	if err != nil || s.Store == nil || size < 1 {
		return 0, 1
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	body, err := s.Store.OpenGeographic(probe, key, size, &artifacts.ByteRange{Start: 0, End: 0})
	if err != nil {
		return 0, 1
	}
	defer body.Close()
	if n, err := io.Copy(io.Discard, io.LimitReader(body, 2)); err != nil || n != 1 {
		return 0, 1
	}
	return 1, 1
}
