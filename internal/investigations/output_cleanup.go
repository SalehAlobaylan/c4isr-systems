package investigations

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

// Record before I/O, independently of the eventual publication transaction.
// The first candidate hash binds retries to one private key; concurrent or
// failed uploads cannot fan out unaccounted objects under one reservation.
func (s Service) planUpload(ctx context.Context, o Output, key, hash string) (string, error) {
	id := ids.New("upload")
	until, _ := ctx.Deadline()
	raw, _ := json.Marshal(map[string]any{"action": "UPLOAD_PLANNED", "uploadId": id, "objectKey": key, "sha256": hash, "writeUntil": until, "recordedAt": time.Now().UTC()})
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var candidate string
		if e := tx.QueryRow(ctx, `SELECT upload_sha256 FROM composite_outputs WHERE id=$1 FOR UPDATE`, o.ID).Scan(&candidate); e != nil {
			return e
		}
		if candidate != "" && candidate != hash {
			return apperr.Conflict("the first render attempt binds its byte identity; freeze a new output for changed bytes")
		}
		if candidate == "" {
			if _, e := tx.Exec(ctx, `UPDATE composite_outputs SET upload_sha256=$2 WHERE id=$1`, o.ID, hash); e != nil {
				return e
			}
		}
		_, e := tx.Exec(ctx, `INSERT INTO composite_output_events(id,case_id,output_id,document)VALUES($1,$2,$3,$4)`, id, o.CaseID, o.ID, raw)
		return e
	})
	return id, e
}
func (s Service) uploadFinished(ctx context.Context, o Output, uploadID string) {
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	raw, _ := json.Marshal(map[string]any{"action": "UPLOAD_FINISHED", "uploadId": uploadID, "recordedAt": time.Now().UTC()})
	_ = db.WithTx(work, s.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(work, `INSERT INTO composite_output_events(id,case_id,output_id,document)VALUES($1,$2,$3,$4)`, ids.New("upload_done"), o.CaseID, o.ID, raw)
		return e
	})
}
func (s Service) expiredKeys(ctx context.Context, o Output) ([]string, error) {
	keys := []string{}
	seen := map[string]bool{}
	if o.ObjectKey != "" {
		keys = append(keys, o.ObjectKey)
		seen[o.ObjectKey] = true
	}
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT p.document->>'objectKey',(p.document->>'writeUntil')::timestamptz,EXISTS(SELECT 1 FROM composite_output_events f WHERE f.output_id=p.output_id AND f.document->>'action'='UPLOAD_FINISHED' AND f.document->>'uploadId'=p.document->>'uploadId') FROM composite_output_events p WHERE p.output_id=$1 AND p.document->>'action'='UPLOAD_PLANNED' ORDER BY p.id LIMIT 129`, o.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		var key string
		var until time.Time
		var done bool
		if e := rows.Scan(&key, &until, &done); e != nil {
			return nil, e
		}
		if !done && until.After(time.Now()) {
			return nil, apperr.Conflict("deletion pending: an uncertain bounded upload can still finish; retry after its write deadline")
		}
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	if count > 128 {
		return nil, apperr.Conflict("upload history exceeds automatic cleanup bound; administrator reconciliation required")
	}
	return keys, rows.Err()
}

// Idle workers reconcile one expired author-owned output, throttled per row.
// Tombstones never restore bytes, even if the ordinary object reader is offline.
func (s Service) ReconcileDeliveryOutput(auth context.Context) (bool, error) {
	ctx, e := access.Refresh(auth)
	if e != nil {
		return false, e
	}
	if e = require(ctx, "outputs.create"); e != nil {
		return false, e
	}
	p, _, _ := access.From(ctx)
	var id string
	e = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT id FROM composite_outputs WHERE creator_human_id=$1 AND retain_until<clock_timestamp() AND state<>'TOMBSTONED' AND next_cleanup_at<=clock_timestamp() ORDER BY next_cleanup_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, p.HumanID).Scan(&id)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE composite_outputs SET next_cleanup_at=clock_timestamp()+interval '30 seconds' WHERE id=$1`, id)
		return e
	})
	if e == pgx.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	e = s.ExpireOutput(ctx, id)
	if apperr.Is(e, apperr.CodeConflict) {
		return false, nil
	}
	return e == nil, e
}
