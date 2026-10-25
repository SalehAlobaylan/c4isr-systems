package simruntime

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
)

// ExerciseProjection reuses the domain ingestion/operations services outside
// the clock loop. Its durable truth outbox is committed with each heartbeat.
type ExerciseProjection interface {
	Prepare(context.Context, string, Start, simscene.Scene) error
	ValidateTx(context.Context, pgx.Tx, string, simulation.PerceptionBatch) error
	Reconcile(context.Context, string) error
	StateTx(context.Context, pgx.Tx, string, string, string, int64, bool) error
	Bindings(context.Context, string) ([]simulation.ControlledBinding, error)
	Project(context.Context, Run, simscene.Scene) error
	Tasks(context.Context, Run) ([]simulation.RuntimeTask, error)
}
