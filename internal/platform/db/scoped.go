package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Scoped binds every statement, including generated queries, to a transaction-
// local resource and a restricted non-owner role. No pooled setting survives it.
func Scoped(pool *pgxpool.Pool) *ScopedDB { return &ScopedDB{pool: pool} }

type ScopedDB struct{ pool *pgxpool.Pool }

func BindTx(ctx context.Context, tx pgx.Tx) error {
	r, err := access.ResourceFrom(ctx)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "SET LOCAL ROLE emad_data"); err != nil {
		return err
	}
	ids, _ := json.Marshal(access.SourceIDs(ctx))
	p, _, _ := access.From(ctx)
	_, err = tx.Exec(ctx, `SELECT set_config('emad.scope_id',$1,true),set_config('emad.source_ids',$2,true),set_config('emad.content_view',$3,true),set_config('emad.principal_id',$4,true),set_config('emad.human_id',$5,true),set_config('emad.case_read',$6,true),set_config('emad.profile_key',$7,true),set_config('emad.profile_policy','retrieval-v1',true),set_config('emad.scheduler_policy','durable-v1',true),set_config('emad.imagery_worker',$8,true)`, r.ID(), string(ids), access.ContentView(ctx), p.ID, p.HumanID, fmt.Sprint(access.Allows(ctx, "investigations.read")), access.RetrievalProfile(ctx), fmt.Sprint(p.Kind == "service" && (access.Allows(ctx, "imagery.jobs.execute") || access.Allows(ctx, "imagery.capture.execute"))))
	return err
}

func (d *ScopedDB) begin(ctx context.Context) (pgx.Tx, error) {
	if _, err := access.ResourceFrom(ctx); err != nil {
		return nil, err
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if err = BindTx(ctx, tx); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return nil, err
	}
	return tx, nil
}

func normalizeScopeError(ctx context.Context, err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && (pe.Code == "42501" || pe.ConstraintName == "emad_scope_relation") {
		access.Deny(ctx, "resource.relate", "database scope constraint")
		return apperr.Forbidden("resource relationship is not authorized")
	}
	return err
}
func (d *ScopedDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx := transaction(ctx); tx != nil {
		return tx.Exec(ctx, sql, args...)
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return tag, normalizeScopeError(ctx, err)
	}
	return tag, tx.Commit(ctx)
}

type scopedRow struct {
	db   *ScopedDB
	ctx  context.Context
	sql  string
	args []any
}

func (r scopedRow) Scan(dest ...any) error {
	if tx := transaction(r.ctx); tx != nil {
		return tx.QueryRow(r.ctx, r.sql, r.args...).Scan(dest...)
	}
	tx, err := r.db.begin(r.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(r.ctx))
	if err = tx.QueryRow(r.ctx, r.sql, r.args...).Scan(dest...); err != nil {
		_ = tx.Rollback(context.WithoutCancel(r.ctx))
		return normalizeScopeError(r.ctx, err)
	}
	return tx.Commit(r.ctx)
}
func (d *ScopedDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return scopedRow{d, ctx, sql, args}
}

type scopedRows struct {
	pgx.Rows
	tx        pgx.Tx
	ctx       context.Context
	closed    bool
	commitErr error
}

func (r *scopedRows) Close() {
	if r.closed {
		return
	}
	r.closed = true
	r.Rows.Close()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), 5*time.Second)
	defer cancel()
	if r.Rows.Err() != nil {
		_ = r.tx.Rollback(ctx)
	} else {
		r.commitErr = r.tx.Commit(ctx)
	}
}
func (r *scopedRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.Close()
	}
	return next
}
func (r *scopedRows) Err() error {
	if err := r.Rows.Err(); err != nil {
		return normalizeScopeError(r.ctx, err)
	}
	return r.commitErr
}
func (d *ScopedDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx := transaction(ctx); tx != nil {
		return tx.Query(ctx, sql, args...)
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return nil, normalizeScopeError(ctx, err)
	}
	return &scopedRows{Rows: rows, tx: tx, ctx: ctx}, nil
}
