// Package reads shares bounded materialized-read storage and opaque page
// continuations across map and catalog consumers. No transaction spans pages.
package reads

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

const MaxBytes = 8 << 20
const MaxRecords = 50000

func Save(ctx context.Context, tx pgx.Tx, id, kind string, raw []byte, expiry time.Time) error {
	if len(raw) > MaxBytes {
		return apperr.Conflict("read exceeds 8 MiB; select a smaller area, interval or source")
	}
	var slot int
	var used, storedBytes int64
	e := tx.QueryRow(ctx, `SELECT COALESCE((SELECT n FROM generate_series(1,16)n WHERE NOT EXISTS(SELECT 1 FROM analyst_read_snapshots s WHERE s.budget_slot=n) ORDER BY n LIMIT 1),0),(SELECT COALESCE(sum(octet_length(document::text)),0) FROM analyst_read_snapshots),octet_length($1::jsonb::text)`, raw).Scan(&slot, &used, &storedBytes)
	if e != nil {
		return e
	}
	if storedBytes > MaxBytes {
		return apperr.Conflict("read exceeds 8 MiB stored representation; select a smaller area, interval or source")
	}
	if slot == 0 || used+storedBytes > 64<<20 {
		return apperr.Conflict("active read capacity exhausted; close a view or wait for expiry before refreshing")
	}
	_, e = tx.Exec(ctx, `INSERT INTO analyst_read_snapshots(id,authorization_revision,expires_at,document,budget_slot,snapshot_kind)VALUES($1,$2,$3,$4,$5,$6)`, id, access.AccessRevision(ctx), expiry, raw, slot, kind)
	return Normalize(e)
}
func Normalize(e error) error {
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		if pe.Code == "23505" || pe.Code == "40001" {
			return apperr.Conflict("concurrent read capacity claim; restart this view")
		}
		if pe.Code == "57014" {
			return apperr.Conflict("read deadline exceeded; preserve this scope and select a smaller area, period or source")
		}
	}
	return e
}
func Cursor(ctx context.Context, tx pgx.Tx, snapshot, key string, offset, size int) (string, error) {
	var token string
	e := tx.QueryRow(ctx, `INSERT INTO analyst_read_cursors(id,snapshot_id,query_key,row_offset,page_size)VALUES($1,$2,$3,$4,$5) ON CONFLICT(snapshot_id,query_key,row_offset,page_size)DO NOTHING RETURNING id`, ids.New("cursor"), snapshot, key, offset, size).Scan(&token)
	if errors.Is(e, pgx.ErrNoRows) {
		e = tx.QueryRow(ctx, `SELECT id FROM analyst_read_cursors WHERE snapshot_id=$1 AND query_key=$2 AND row_offset=$3 AND page_size=$4`, snapshot, key, offset, size).Scan(&token)
	}
	return token, e
}
func Position(ctx context.Context, tx pgx.Tx, token, snapshot, key string, size int) (int, error) {
	var offset int
	e := tx.QueryRow(ctx, `SELECT row_offset FROM analyst_read_cursors WHERE id=$1 AND snapshot_id=$2 AND query_key=$3 AND page_size=$4`, token, snapshot, key, size).Scan(&offset)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, apperr.Conflict("cursor is unavailable or belongs to another query; restart this view")
	}
	return offset, e
}
func Load(ctx context.Context, tx pgx.Tx, id, kind string, out any) error {
	var raw []byte
	var revision string
	var valid bool
	e := tx.QueryRow(ctx, `SELECT document,authorization_revision,expires_at>clock_timestamp() FROM analyst_read_snapshots WHERE id=$1 AND snapshot_kind=$2`, id, kind).Scan(&raw, &revision, &valid)
	if errors.Is(e, pgx.ErrNoRows) {
		return apperr.Conflict("read unavailable or expired; explicitly refresh this view")
	}
	if e != nil {
		return e
	}
	if !valid || revision != access.AccessRevision(ctx) {
		return apperr.Conflict("read expired or access changed; explicitly refresh this view")
	}
	return json.Unmarshal(raw, out)
}
