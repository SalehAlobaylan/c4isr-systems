// Package capacity enforces the resource's retained-byte and concurrent-work
// bounds. Aggregation runs in the control plane; hidden source usage is never
// returned to a caller. Retained or referenced objects are never deleted here.
package capacity

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

const StorageBytes int64 = 8 << 30
const ActiveWork = 32

// Check must run in the transaction that commits retained bytes. The lock is
// shared with reservations and artifact insertion, including hidden sources.
func Check(ctx context.Context, tx pgx.Tx, extra int64, excluding string) error {
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `RESET ROLE`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "resource-capacity:"+r.ID()); e != nil {
		return e
	}
	var used, reserved int64
	e = tx.QueryRow(ctx, `SELECT (SELECT COALESCE(sum(size_bytes),0) FROM artifacts WHERE scope_id=$1 AND state<>'DELETED')+(SELECT COALESCE(sum(octet_length(document::text)+COALESCE(octet_length(original_record),0)),0) FROM public_evidence WHERE scope_id=$1),(SELECT COALESCE(sum(estimated_bytes),0) FROM resource_capacity_reservations WHERE scope_id=$1 AND settled_at IS NULL AND lease_until>clock_timestamp() AND id<>$2)`, r.ID(), excluding).Scan(&used, &reserved)
	if e != nil {
		return e
	}
	// Compatible with an older schema during forward-upgrade checks. New
	// composite reservations share the same resource lock with every artifact.
	var composites bool
	if e = tx.QueryRow(ctx, `SELECT to_regclass('public.composite_outputs') IS NOT NULL`).Scan(&composites); e != nil {
		return e
	}
	if composites {
		var bytes int64
		if e = tx.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes+octet_length(document::text)),0) FROM composite_outputs WHERE scope_id=$1 AND state<>'TOMBSTONED'`, r.ID()).Scan(&bytes); e != nil {
			return e
		}
		used += bytes
	}
	if _, e = tx.Exec(ctx, `SET LOCAL ROLE emad_data`); e != nil {
		return e
	}
	if extra < 0 || used+reserved+extra > StorageBytes {
		return apperr.Conflict("storage_full: acquisition paused; an administrator must reconcile storage or change the approved retention policy")
	}
	return nil
}

func Reserve(ctx context.Context, pool *pgxpool.Pool, id, source, kind string, estimate int64, until time.Time) error {
	if e := access.RequireSource(ctx, source); e != nil {
		return e
	}
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if e = Check(ctx, tx, estimate, id); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `RESET ROLE`); e != nil {
		return e
	}
	var active int
	e = tx.QueryRow(ctx, `SELECT count(*) FROM resource_capacity_reservations WHERE scope_id=$1 AND id<>$2 AND settled_at IS NULL AND lease_until>clock_timestamp()`, r.ID(), id).Scan(&active)
	if e != nil {
		return e
	}
	if active >= ActiveWork {
		return apperr.Conflict("active_capacity: wait for existing work; history does not consume queue slots")
	}
	_, e = tx.Exec(ctx, `INSERT INTO resource_capacity_reservations(id,scope_id,source_id,kind,estimated_bytes,lease_until)VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id)DO UPDATE SET lease_until=excluded.lease_until WHERE resource_capacity_reservations.settled_at IS NULL`, id, r.ID(), source, kind, estimate, until)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func Settle(ctx context.Context, pool *pgxpool.Pool, id string) error {
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	_, e = pool.Exec(ctx, `UPDATE resource_capacity_reservations SET settled_at=clock_timestamp(),actual_bytes=(SELECT COALESCE(sum(octet_length(document::text)+COALESCE(octet_length(original_record),0)),0) FROM public_evidence WHERE connector_run_id=$1) WHERE id=$1 AND scope_id=$2 AND settled_at IS NULL`, id, r.ID())
	return e
}

func Renew(ctx context.Context, pool *pgxpool.Pool, id string, until time.Time) error {
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	_, e = pool.Exec(ctx, `UPDATE resource_capacity_reservations SET lease_until=$3 WHERE id=$1 AND scope_id=$2 AND settled_at IS NULL`, id, r.ID(), until)
	return e
}
