package intents

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Imagery registers the original, unevaluated request and created job in the
// same transaction. A lost response cannot roll its period or create more work.
func Imagery(ctx context.Context, pool *pgxpool.Pool, operation, key string, input any, create func(context.Context, string) (string, error)) (string, error) {
	p, r, ok := access.From(ctx)
	if !ok {
		return "", apperr.Forbidden("authenticated requester required")
	}
	requester := p.HumanID
	if p.Kind == "service" {
		requester = p.ID
	}
	if requester == "" {
		return "", apperr.Forbidden("registered requester required")
	}
	if key == "" || len(key) > 128 {
		return "", apperr.Validation("bounded stable request key required")
	}
	raw, e := jsoncanon.Marshal(input)
	if e != nil {
		return "", e
	}
	hash := areas.Hash(raw)
	id := ""
	e = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		bound := db.UseTx(ctx, tx)
		scopedKey := "intent_" + areas.Hash([]byte(r.ID()+"/"+operation+"/"+requester+"/"+key))
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scopedKey); e != nil {
			return e
		}
		var prior string
		e := tx.QueryRow(ctx, `SELECT input_hash,entity_id FROM imagery_request_intents WHERE operation=$1 AND requester_id=$2 AND request_key=$3`, operation, requester, key).Scan(&prior, &id)
		if e == nil {
			if prior != hash {
				return apperr.Conflict("imagery request key reused with different original input")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		id, e = create(bound, scopedKey)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO imagery_request_intents(operation,requester_id,request_key,input_hash,entity_id)VALUES($1,$2,$3,$4,$5)`, operation, requester, key, hash, id)
		return e
	})
	return id, e
}
