package db

import (
	"context"
	"encoding/json"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionKey struct{}
type boundTransaction struct {
	tx      pgx.Tx
	binding string
}

func transactionBinding(ctx context.Context) string {
	p, r, _ := access.From(ctx)
	raw, _ := json.Marshal([]any{r.ID(), p.ID, p.HumanID, access.RetrievalProfile(ctx), access.AccessRevision(ctx), access.SourceIDs(ctx), access.ContentView(ctx)})
	return string(raw)
}

// UseTx shares only a transaction already bound by BindTx in this request.
// Domain queries may reuse it; they never commit it or retain it across pages.
func UseTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, transactionKey{}, boundTransaction{tx, transactionBinding(ctx)})
}
func transaction(ctx context.Context) pgx.Tx {
	v, ok := ctx.Value(transactionKey{}).(boundTransaction)
	if ok && v.binding == transactionBinding(ctx) {
		return v.tx
	}
	return nil
}
func RepeatableRead(ctx context.Context, pool *pgxpool.Pool, fn func(context.Context, pgx.Tx) error) error {
	tx, e := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if e != nil {
		return e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if e = BindTx(ctx, tx); e != nil {
		return e
	}
	if e = fn(UseTx(ctx, tx), tx); e != nil {
		return normalizeScopeError(ctx, e)
	}
	return tx.Commit(ctx)
}
