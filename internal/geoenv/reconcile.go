package geoenv

import (
	"context"
	"errors"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
)

// ReconcileOne tombstones an expired reservation before deleting its private
// bytes. A live upload keeps its grace window even after retention has expired.
func (s Service) ReconcileOne(auth context.Context) error {
	ctx, err := access.Refresh(auth)
	if err != nil {
		return err
	}
	if err = require(ctx, Execute, true); err != nil {
		return err
	}
	var id, key string
	err = db.Scoped(s.Pool).QueryRow(ctx, `
		WITH candidate AS (
			SELECT id,scope_id FROM geographic_assets
			WHERE purged_at IS NULL AND (upload_until IS NULL OR upload_until<=clock_timestamp())
			  AND (retain_until<=clock_timestamp() OR state='EXPIRED' OR state='PENDING' AND created_at<=clock_timestamp()-interval '1 hour')
			ORDER BY retain_until,id LIMIT 1 FOR UPDATE SKIP LOCKED
		) UPDATE geographic_assets a SET state='EXPIRED' FROM candidate c
		WHERE (a.id,a.scope_id)=(c.id,c.scope_id) RETURNING a.id,a.object_key`).Scan(&id, &key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = s.Store.Delete(ctx, key); err != nil {
		return err
	}
	_, err = db.Scoped(s.Pool).Exec(ctx, `UPDATE geographic_assets SET purged_at=clock_timestamp() WHERE id=$1 AND state='EXPIRED'`, id)
	return err
}
