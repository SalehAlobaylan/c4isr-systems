package nativeexercise

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
	"time"
)

// Validate recipients before committing the private outbox. Invalid batches
// cannot poison a run with an outcome or telemetry sample from another scope.
func (s *Service) ValidateTx(ctx context.Context, tx pgx.Tx, id string, b simulation.PerceptionBatch) error {
	c := public(ctx, id)
	if e := platformdb.BindTx(c, tx); e != nil {
		return e
	}
	states := map[string]commands.State{}
	for _, o := range b.Outcomes {
		var valid bool
		if e := tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM native_task_dispatch WHERE command_id=$1 AND issued_ms<=$2)`, o.CommandID, o.ElapsedMs).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return apperr.Forbidden("task outcome does not belong to run")
		}
		var state commands.State
		var deadline int64
		if e := tx.QueryRow(c, `SELECT c.state,d.deadline_ms FROM commands c JOIN native_task_dispatch d ON d.scope_id=c.scope_id AND d.command_id=c.id WHERE c.id=$1`, o.CommandID).Scan(&state, &deadline); e != nil {
			return e
		}
		if prior, ok := states[o.CommandID]; ok {
			state = prior
		}
		to := commands.State(o.State)
		if o.ElapsedMs > deadline {
			to = commands.StateTimedOut
		}
		if !commands.IsTerminal(state) && state != to && !commands.CanTransition(state, to) {
			return apperr.Conflict("outcome conflicts with command lifecycle")
		}
		states[o.CommandID] = to

	}
	c = truth(ctx, id)
	if e := platformdb.BindTx(c, tx); e != nil {
		return e
	}
	for _, r := range b.Readings {
		if r.Kind == "telemetry" {
			var valid bool
			if e := tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM simulation_actor_mappings WHERE message_id=$1 AND contact_ref=$2)`, "controlled-actor:"+r.ActorID, r.AssetID).Scan(&valid); e != nil {
				return e
			}
			if !valid {
				return apperr.Forbidden("controlled telemetry does not belong to run")
			}
		}
	}
	return platformdb.BindTx(ctx, tx)
}

func (s *Service) Reconcile(ctx context.Context, id string) error {
	c := public(ctx, id)
	conn, e := s.Pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	r, _ := access.ResourceFrom(ctx)
	key := r.WorkspaceID + "/" + r.EnvironmentID + "/" + id
	var locked bool
	if e = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&locked); e != nil {
		return e
	}
	if !locked {
		return nil
	}
	defer func() {
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, _ = conn.Exec(work, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key)
	}()
	var exists bool
	if e := platformdb.Scoped(s.Pool).QueryRow(c, `SELECT EXISTS(SELECT 1 FROM native_exercises)`).Scan(&exists); e != nil {
		return e
	}
	if !exists {
		return nil
	}
	var elapsed int64
	if e := platformdb.Scoped(s.Pool).QueryRow(c, `SELECT elapsed_ms FROM native_exercises`).Scan(&elapsed); e != nil {
		return e
	}
	return s.expireTasks(c, elapsed)
}
