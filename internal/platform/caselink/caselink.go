// Package caselink validates explicit immutable case revision links without
// giving a processing worker access to human case content or new source grants.
package caselink

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
)

type Revision struct {
	CaseID     string `json:"caseId"`
	RevisionID string `json:"revisionId"`
}

func Resolve(ctx context.Context, tx pgx.Tx, in *Revision) (*Revision, error) {
	if in == nil {
		return nil, nil
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" || p.HumanID == "" {
		return nil, apperr.Forbidden("human case link requires a registered investigator")
	}
	if e := access.Require(ctx, "investigations.edit"); e != nil {
		return nil, e
	}
	var head, state string
	e := tx.QueryRow(ctx, `SELECT c.head_revision_id,r.document->>'state' FROM investigations c JOIN investigation_revisions r ON r.id=c.head_revision_id AND r.scope_id=c.scope_id WHERE c.id=$1 FOR SHARE OF c`, in.CaseID).Scan(&head, &state)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperr.NotFound("investigation", "unavailable")
	}
	if e != nil {
		return nil, e
	}
	if head != in.RevisionID {
		return nil, apperr.Conflict("case revision changed; review the current case before submitting work")
	}
	if state == "ARCHIVED" {
		return nil, apperr.Conflict("reopen the investigation before submitting new work")
	}
	return in, nil
}
