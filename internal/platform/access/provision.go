package access

import (
	"context"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func delegable(permission string) bool {
	if !KnownPermission(permission) {
		return false
	}
	switch permission {
	case "exercise.ingest", "sim.truth.record", "tracks.project", "observations.process", "alerts.project", "geofences.evaluate", "audit.append", "platform.maintenance":
		return false
	}
	return true
}

func validateGrant(ctx context.Context, g Grant) error {
	if err := Require(ctx, "admin.manage"); err != nil {
		return err
	}
	r := g.Resource
	if r.RunID == "*" {
		r.RunID = "grant-validation"
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if r.Partition == ExerciseTruth {
		if err := Require(ctx, "sim.truth.grant"); err != nil {
			return err
		}
	}
	for _, permission := range g.Permissions {
		if !delegable(permission) {
			return apperr.Validation("permission cannot be delegated")
		}
		if strings.HasPrefix(permission, "sim.truth.") {
			if err := Require(ctx, "sim.truth.grant"); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddGrant provisions a human or service grant. Wildcard runs are explicit
// environment-wide authority, never a default chosen from a request header.
func (s *Store) AddGrant(ctx context.Context, principalID string, g Grant) error {
	if err := validateGrant(ctx, g); err != nil {
		return err
	}
	if g.SourceIDs == nil {
		g.SourceIDs = []string{}
	}
	if g.Resource.RunID != "*" {
		if err := s.EnsureResource(ctx, g.Resource); err != nil {
			return err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions,source_ids) VALUES($1,$2,$3,$4,$5,$6,$7)`, principalID, g.Resource.WorkspaceID, g.Resource.EnvironmentID, string(g.Resource.Partition), g.Resource.RunID, g.Permissions, g.SourceIDs); err != nil {
		return err
	}
	actor, _, _ := From(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action) VALUES($1,$2,'grant.created')`, actor.ID, principalID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateHuman(ctx context.Context, id, operatorID string, g Grant) error {
	if err := validateGrant(ctx, g); err != nil {
		return err
	}
	if !identifier.MatchString(id) || strings.HasPrefix(id, "human:") || strings.HasPrefix(id, "internal:") {
		return apperr.Validation("invalid principal ID")
	}
	if g.Resource.RunID == "*" {
		return apperr.Validation("initial resource must be concrete")
	}
	if g.SourceIDs == nil {
		g.SourceIDs = []string{}
	}
	if err := s.EnsureResource(ctx, g.Resource); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	tag, err := tx.Exec(ctx, `INSERT INTO principals(id,kind,operator_id,name) SELECT $1,'human',id,name FROM operators WHERE id=$2`, id, operatorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return apperr.Validation("operator is not registered")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions,source_ids) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, g.Resource.WorkspaceID, g.Resource.EnvironmentID, string(g.Resource.Partition), g.Resource.RunID, g.Permissions, g.SourceIDs); err != nil {
		return err
	}
	actor, _, _ := From(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action) VALUES($1,$2,'principal.created')`, actor.ID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
