package nativeexercise

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/missions"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Tasks(ctx context.Context, run simruntime.Run) ([]simulation.RuntimeTask, error) {
	c := public(ctx, run.ID)
	out := []simulation.RuntimeTask{}
	if (run.Status != "RUNNING" && run.Status != "PAUSED") || run.Desired == "STOPPED" {
		return out, nil
	}
	rows, e := platformdb.Scoped(s.Pool).Query(c, `SELECT c.id,c.asset_id,c.type,d.issued_ms,d.deadline_ms FROM native_task_dispatch d JOIN commands c ON c.scope_id=d.scope_id AND c.id=d.command_id WHERE NOT d.finished AND c.state IN('SENT','ACKNOWLEDGED') ORDER BY c.created_at,c.id LIMIT 128`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var t simulation.RuntimeTask
		if e = rows.Scan(&t.CommandID, &t.AssetID, &t.Type, &t.IssuedMs, &t.DeadlineMs); e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Service) outcome(ctx context.Context, out simulation.TaskOutcome) error {
	var taskID string
	var deadline int64
	var finished bool
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT COALESCE(task_id,''),deadline_ms,finished FROM native_task_dispatch WHERE command_id=$1`, out.CommandID).Scan(&taskID, &deadline, &finished)
	if errors.Is(e, pgx.ErrNoRows) {
		return apperr.Forbidden("command does not belong to runtime run")
	}
	if e != nil {
		return e
	}
	if finished {
		return nil
	}
	cmd, e := s.Commands.Get(ctx, out.CommandID)
	if e != nil {
		return e
	}
	if commands.IsTerminal(cmd.State) {
		return s.finishTask(ctx, cmd, taskID)
	}
	state := commands.State(out.State)
	if out.ElapsedMs > deadline {
		state = commands.StateTimedOut
	}
	if cmd.State != state {
		if _, e = s.Commands.Transition(ctx, cmd.ID, state, out.Code, "internal:native-runtime"); e != nil {
			return e
		}
	}
	if taskID != "" {
		status := missions.TaskActive
		if state == commands.StateCompleted {
			status = missions.TaskCompleted
		} else if commands.IsTerminal(state) {
			status = missions.TaskFailed
		}
		if _, e = s.Missions.UpdateTaskStatus(ctx, cmd.MissionID, taskID, status, "internal:native-runtime"); e != nil {
			return e
		}
	}
	if commands.IsTerminal(state) {
		cmd.State = state
		return s.finishTask(ctx, cmd, taskID)
	}
	return nil
}
func (s *Service) finishTask(ctx context.Context, cmd commands.Command, taskID string) error {
	// Reapply the mission task update after a crash between the command and task
	// commits. The same command outcome never executes the actor a second time.
	if taskID != "" {
		status := missions.TaskFailed
		if cmd.State == commands.StateCompleted {
			status = missions.TaskCompleted
		}
		if cmd.State == commands.StateCancelled {
			status = missions.TaskCancelled
		}
		if _, e := s.Missions.UpdateTaskStatus(ctx, cmd.MissionID, taskID, status, "internal:native-runtime"); e != nil {
			return e
		}
	}
	_, e := platformdb.Scoped(s.Pool).Exec(ctx, `UPDATE native_task_dispatch SET finished=true WHERE command_id=$1`, cmd.ID)
	return e
}
func (s *Service) expireTasks(ctx context.Context, elapsed int64) error {
	var status, desired string
	var leaseValid bool
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT status,desired,COALESCE(lease_until>clock_timestamp(),false)FROM native_exercises`).Scan(&status, &desired, &leaseValid)
	if e != nil {
		return e
	}
	closed := desired == "STOPPED" || status == "STOPPED" || status == "FAILED" || status == "COMPLETED" || status == "STOPPING" || !leaseValid
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT command_id,COALESCE(task_id,'')FROM native_task_dispatch WHERE NOT finished AND($1 OR deadline_ms<$2)ORDER BY issued_ms LIMIT 128`, closed, elapsed)
	if e != nil {
		return e
	}
	type item struct{ id, task string }
	items := []item{}
	for rows.Next() {
		var i item
		if e = rows.Scan(&i.id, &i.task); e != nil {
			rows.Close()
			return e
		}
		items = append(items, i)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	for _, i := range items {
		cmd, e := s.Commands.Get(ctx, i.id)
		if e != nil {
			return e
		}
		if !commands.IsTerminal(cmd.State) {
			to := commands.StateTimedOut
			if closed {
				to = commands.StateCancelled
			}
			cmd, e = s.Commands.Transition(ctx, cmd.ID, to, "NATIVE_RUN_CLOSED_OR_DEADLINE", "internal:native-runtime")
			if e != nil {
				return e
			}
		}
		if e = s.finishTask(ctx, cmd, i.task); e != nil {
			return e
		}
	}
	return nil
}
