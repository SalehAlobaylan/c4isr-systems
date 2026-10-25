package access

import (
	"context"
	"errors"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
)

// ValidateConnector is a narrow control-plane port for source reviewers. It
// verifies a service's current authority without disclosing its credentials.
func (s *Store) ValidateConnector(ctx context.Context, id string, r Resource, source string) error {
	if err := Require(ctx, "sources.manage"); err != nil {
		return err
	}
	denied := func() error { return apperr.Validation("connector service identity is not authorized for this source") }
	var p Principal
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT id,kind,disabled_at IS NULL FROM principals WHERE id=$1`, id).Scan(&p.ID, &p.Kind, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return denied()
	}
	if err != nil {
		return err
	}
	if !active || p.Kind != "service" || r.Partition != PublicEvidence {
		return denied()
	}
	rows, err := s.pool.Query(ctx, `SELECT workspace_id,environment_id,partition,run_id,permissions,source_ids FROM principal_grants WHERE principal_id=$1 ORDER BY id`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g Grant
		if err = rows.Scan(&g.Resource.WorkspaceID, &g.Resource.EnvironmentID, &g.Resource.Partition, &g.Resource.RunID, &g.Permissions, &g.SourceIDs); err != nil {
			return err
		}
		p.Grants = append(p.Grants, g)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	bound, err := Bind(ctx, p, r, nil)
	if err != nil {
		return denied()
	}
	for _, permission := range []string{"operational.read", "sources.read", "connectors.execute"} {
		if !Allows(bound, permission) {
			return denied()
		}
	}
	if err = RequireSource(bound, source); err != nil {
		return denied()
	}
	return nil
}
