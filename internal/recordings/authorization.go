package recordings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// Identity history is shared-kernel control-plane data, not a COP read path.
// The authenticated archive worker copies only this environment/run's history
// into EXERCISE_TRUTH. Effective wall time is preserved separately from the final
// virtual anchor; historical grants never authorize a present playback request.
func (s *Service) authorization(ctx context.Context, id string, elapsed int64, events []Event) ([]Event, error) {
	r, _ := access.ResourceFrom(ctx)
	rows, e := s.Pool.Query(ctx, `SELECT id,principal_id,partition,run_id,kind,occurred_at,document FROM simulation_authorization_history WHERE workspace_id=$1 AND environment_id=$2 AND (run_id IN($3,'*') OR partition='EXERCISE_INPUT' AND run_id='') ORDER BY id LIMIT 4097`, r.WorkspaceID, r.EnvironmentID, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var seq int64
		var principal, partition, run, kind string
		var at time.Time
		var raw json.RawMessage
		if e = rows.Scan(&seq, &principal, &partition, &run, &kind, &at, &raw); e != nil {
			return nil, e
		}
		var policy struct {
			CaptureTruncated bool `json:"captureTruncated"`
		}
		if json.Unmarshal(raw, &policy) != nil || policy.CaptureTruncated {
			return nil, apperr.Conflict("authorization history incomplete")
		}
		count++
		if count > 4096 || len(events) >= 100000 {
			return nil, apperr.Conflict("authorization recording budget exceeded")
		}
		doc, _ := json.Marshal(map[string]any{"principalId": principal, "partition": partition, "runId": run, "change": kind, "effectiveWallTime": at, "timeBasis": "wall clock identity change; final virtual time is an archive anchor only", "policy": raw})
		events = append(events, Event{Sequence: len(events) + 1, ElapsedMs: elapsed, ReceivedAt: at, Kind: "authorization", Entity: principal + "/" + kind + "/" + at.Format(time.RFC3339Nano), Operation: "INSERT", Document: doc})
	}
	return events, rows.Err()
}
