package publicevents

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Validity struct {
	EvidenceID        string    `json:"evidenceId"`
	CurrentEvidenceID string    `json:"currentEvidenceId,omitempty"`
	State             string    `json:"state"`
	EvaluatedAt       time.Time `json:"evaluatedAt"`
	Meaning           string    `json:"meaning"`
}

func CurrentValidity(ctx context.Context, pool *pgxpool.Pool, id string) (Validity, error) {
	out := Validity{EvidenceID: id, State: "unavailable", EvaluatedAt: time.Now().UTC(), Meaning: "Current retained provider status, separate from the saved historical interpretation; this is not independent verification."}
	if !access.Allows(ctx, "observations.read") {
		return out, nil
	}
	var withdrawn bool
	e := db.Scoped(pool).QueryRow(ctx, `SELECT c.evidence_id,COALESCE((head.document#>>'{reported,withdrawn}')::boolean,false) FROM public_event_revisions anchor JOIN public_event_current c ON c.source_id=anchor.source_id AND c.provider_record_id=anchor.provider_record_id AND c.scope_id=anchor.scope_id JOIN public_event_revisions head ON head.evidence_id=c.evidence_id WHERE anchor.evidence_id=$1`, id).Scan(&out.CurrentEvidenceID, &withdrawn)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	out.State = "current_provider_report"
	if id != out.CurrentEvidenceID {
		out.State = "superseded_provider_report"
	}
	if withdrawn {
		out.State = "withdrawn_by_provider"
	}
	return out, nil
}
