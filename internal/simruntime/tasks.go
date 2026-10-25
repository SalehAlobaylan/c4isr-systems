package simruntime

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"sort"
)

type taskState struct {
	task     simulation.RuntimeTask
	received int64
	ack      bool
	done     bool
}
type TaskRunner struct {
	world  *World
	actors map[string]string
	seen   map[string]*taskState
}

func NewTaskRunner(w *World, bindings []simulation.ControlledBinding) *TaskRunner {
	r := &TaskRunner{world: w, actors: map[string]string{}, seen: map[string]*taskState{}}
	for _, b := range bindings {
		r.actors[b.AssetID] = b.ActorID
	}
	return r
}

// Sync consumes a successful active gateway reply, which is authoritative for
// this run's complete bounded dispatch roster. Missing unfinished commands were
// cancelled or closed; tick calls to Advance(nil, ...) are not such a reply.
func (r *TaskRunner) Sync(tasks []simulation.RuntimeTask, elapsed int64) []simulation.TaskOutcome {
	active := map[string]bool{}
	for _, t := range tasks {
		active[t.CommandID] = true
	}
	for id, s := range r.seen {
		if !s.done && !active[id] {
			s.done = true
		}
	}
	return r.Advance(tasks, elapsed)
}

// Deliberately simple exercise behavior: acknowledge after 500 virtual ms,
// then hold/resume after another 500 ms. Retries retain the command identity.
func (r *TaskRunner) Advance(tasks []simulation.RuntimeTask, elapsed int64) []simulation.TaskOutcome {
	active := 0
	for _, s := range r.seen {
		if !s.done {
			active++
		}
	}
	for _, t := range tasks {
		if _, ok := r.seen[t.CommandID]; !ok && len(r.seen) < 128 && active < 32 {
			r.seen[t.CommandID] = &taskState{task: t, received: elapsed}
			active++
		}
	}
	out := []simulation.TaskOutcome{}
	ids := make([]string, 0, len(r.seen))
	for id := range r.seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := r.seen[ids[i]].task, r.seen[ids[j]].task
		if a.IssuedMs != b.IssuedMs {
			return a.IssuedMs < b.IssuedMs
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		s := r.seen[id]
		if s.done || elapsed > s.task.DeadlineMs {
			continue
		}
		if !s.ack && elapsed-s.received >= 500 {
			s.ack = true
			if s.task.Type != "HOLD" && s.task.Type != "RESUME_ROUTE" {
				out = append(out, simulation.TaskOutcome{CommandID: id, State: "REJECTED", ElapsedMs: elapsed, Code: "UNSUPPORTED_ACTION"})
				s.done = true
				continue
			}
			out = append(out, simulation.TaskOutcome{CommandID: id, State: "ACKNOWLEDGED", ElapsedMs: elapsed})
			continue
		}
		if s.ack && elapsed-s.received >= 1000 {
			state, code := "COMPLETED", ""
			if actor := r.actors[s.task.AssetID]; actor == "" || !r.world.Control(actor, s.task.Type, elapsed) {
				state, code = "FAILED", "ACTOR_UNAVAILABLE"
			}
			out = append(out, simulation.TaskOutcome{CommandID: id, State: state, ElapsedMs: elapsed, Code: code})
			s.done = true
		}
	}
	return out
}
