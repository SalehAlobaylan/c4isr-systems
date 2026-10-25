package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// SelectableResources returns only ordinary COP resources covered by the live
// principal grants. It never offers input packages or ground truth as a COP.
func (s *Store) SelectableResources(ctx context.Context) ([]Resource, error) {
	p, _, ok := From(ctx)
	result := []Resource{}
	if !ok {
		return result, nil
	}
	seen := map[string]bool{}
	for _, g := range p.Grants {
		r := g.Resource
		if r.Partition != PublicEvidence && r.Partition != ExerciseObservation {
			continue
		}
		rows, err := s.pool.Query(ctx, `SELECT workspace_id,environment_id,partition,run_id FROM data_scopes
			WHERE workspace_id=$1 AND environment_id=$2 AND partition=$3 AND (run_id=$4 OR $4='*') ORDER BY id`, r.WorkspaceID, r.EnvironmentID, r.Partition, r.RunID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var candidate Resource
			if err = rows.Scan(&candidate.WorkspaceID, &candidate.EnvironmentID, &candidate.Partition, &candidate.RunID); err != nil {
				rows.Close()
				return nil, err
			}
			bound, bindErr := Bind(ctx, p, candidate, nil)
			if bindErr == nil && Allows(bound, "operational.read") && !seen[candidate.ID()] {
				seen[candidate.ID()] = true
				result = append(result, candidate)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID() < result[j].ID() })
	return result, nil
}

// ResourceDetail is navigation metadata for already selectable COP resources.
// It reads only ordinary exercise projections, never authoring scenes or truth.
type ResourceDetail struct {
	Resource  Resource   `json:"resource"`
	Label     string     `json:"label"`
	Kind      string     `json:"kind"`
	Status    string     `json:"status,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

func (s *Store) ResourceDetails(ctx context.Context, resources []Resource) ([]ResourceDetail, error) {
	p, _, _ := From(ctx)
	result := make([]ResourceDetail, 0, len(resources))
	readable := []string{}
	byScope := map[string]int{}
	for _, r := range resources {
		detail := ResourceDetail{Resource: r, Label: "Exercise", Kind: "exercise"}
		if r.Partition == PublicEvidence {
			detail.Label, detail.Kind = "Public Context", "public"
		}
		if r.RunID == "legacy" {
			detail.Label, detail.Kind = "Legacy exercise", "legacy"
		}
		byScope[r.ID()] = len(result)
		result = append(result, detail)
		bound, err := Bind(ctx, p, r, nil)
		if err == nil && r.Partition == ExerciseObservation && Allows(bound, "scenarios.read") {
			readable = append(readable, r.ID())
		}
	}
	if len(readable) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `
	 SELECT scope_id,scenario_name,status,started_at FROM scenario_runs
	 WHERE scope_id=ANY($1::text[]) AND id=split_part(scope_id,'/',4)
	 UNION ALL
	 SELECT scope_id,'Simulation exercise',
	 CASE WHEN status IN ('RUNNING','PAUSED','STOPPING') AND NOT COALESCE(lease_until>clock_timestamp(),false) THEN 'FAILED' ELSE status END,
	 started_at FROM native_exercises WHERE scope_id=ANY($1::text[])`, readable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var scope, label, status string
		var started time.Time
		if err := rows.Scan(&scope, &label, &status, &started); err != nil {
			return nil, err
		}
		if i, ok := byScope[scope]; ok {
			result[i].Label = label
			result[i].Status = status
			result[i].StartedAt = &started
		}
	}
	return result, rows.Err()
}

// AccessRevision lets clients discard cached data after permission or source
// restrictions change without publishing grant details or credential material.
func AccessRevision(ctx context.Context) string {
	p, r, _ := From(ctx)
	raw, _ := json.Marshal(struct {
		ID       string
		Resource Resource
		Grants   []Grant
	}{p.ID, r, p.Grants})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
