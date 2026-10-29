package publicingest

import (
	"context"
	"encoding/json"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"time"
)

type Receipt struct {
	RecordedUnpublishedRecords int               `json:"recordedUnpublishedRecords"`
	PublicationReconciled      bool              `json:"publicationReconciled"`
	RunID                      string            `json:"runId"`
	SourceID                   string            `json:"sourceId"`
	ProfileRevisionID          string            `json:"profileRevisionId"`
	AdmissionID                string            `json:"admissionId"`
	RequestID                  string            `json:"requestId,omitempty"`
	Adapter                    AdapterCapability `json:"adapter"`
	AOI                        *Bounds           `json:"aoi"`
	From                       *time.Time        `json:"from"`
	To                         *time.Time        `json:"to"`
	CompletedAt                time.Time         `json:"completedAt"`
	Status                     string            `json:"status"`
	FailureCode                string            `json:"failureCode,omitempty"`
	Pages                      int               `json:"pages"`
	RejectedRecords            int               `json:"rejectedRecords"`
	ProviderTruncated          bool              `json:"providerTruncated"`
	RetainedRecords            int               `json:"retainedRecords"`
	UnpublishedRecords         int               `json:"unpublishedRecords"`
	QueryComplete              bool              `json:"queryComplete"`
	ObservationCompleteness    string            `json:"observationCompleteness"`
}

func (r *Repository) RecordReceipt(ctx context.Context, run Run, adapter string) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		out := Receipt{RunID: run.ID, SourceID: run.SourceID, ProfileRevisionID: run.ProfileRevisionID, AdmissionID: run.AdmissionID, RequestID: run.AcquisitionRequestID, Adapter: Capability(adapter), AOI: run.QueryAOI, From: run.QueryFrom, To: run.QueryTo, CompletedAt: time.Now().UTC(), Status: run.Status, FailureCode: run.FailureCode, ObservationCompleteness: "unknown; provider retrieval does not establish factual absence"}
		// An unfinished cursor resumes the same frozen window. Include preceding
		// committed pages and their gaps/publication failures in the final receipt.
		e := tx.QueryRow(ctx, `WITH chain AS(SELECT id,document FROM connector_runs WHERE source_id=$1 AND admission_id=$2 AND (document->>'queryFrom')::timestamptz=$3::timestamptz AND (document->>'queryTo')::timestamptz=$4::timestamptz AND document->'queryAoi'=$5::jsonb AND COALESCE(document->>'acquisitionRequestId','')=$6), evidence AS(SELECT DISTINCT o.evidence_id FROM retrieval_occurrences o JOIN chain c ON c.id=o.run_id) SELECT COALESCE((SELECT sum(COALESCE((document->>'pages')::int,0)) FROM chain),0),COALESCE((SELECT sum(COALESCE((document->>'rejectedRecords')::int,0)) FROM chain),0),COALESCE((SELECT bool_or(COALESCE((document->>'providerTruncated')::boolean,false)) FROM chain),false),(SELECT count(*) FROM evidence),(SELECT count(*) FROM evidence e JOIN public_evidence_outbox o ON o.evidence_id=e.evidence_id WHERE o.delivered_at IS NULL)`, run.SourceID, run.AdmissionID, run.QueryFrom, run.QueryTo, mustJSON(run.QueryAOI), run.AcquisitionRequestID).Scan(&out.Pages, &out.RejectedRecords, &out.ProviderTruncated, &out.RetainedRecords, &out.UnpublishedRecords)
		if e != nil {
			return e
		}
		out.QueryComplete = run.Status == "SUCCEEDED" && out.Pages > 0 && out.RejectedRecords == 0 && !out.ProviderTruncated && out.UnpublishedRecords == 0
		raw, _ := json.Marshal(out)
		_, e = tx.Exec(ctx, `INSERT INTO retrieval_receipts(run_id,source_id,profile_revision_id,request_id,document)VALUES($1,$2,$3,NULLIF($4,''),$5)ON CONFLICT DO NOTHING`, run.ID, run.SourceID, run.ProfileRevisionID, run.AcquisitionRequestID, raw)
		return e
	})
}
func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }

func (s *Service) releaseUnsentCost(ctx context.Context, id string) error {
	tx, e := s.repo.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var group string
	var cost int64
	var created time.Time
	e = tx.QueryRow(ctx, `UPDATE provider_cost_reservations SET state='UNSENT',lease_until=clock_timestamp() WHERE id=$1 AND state='UNCERTAIN' RETURNING group_key,cost,created_at`, id).Scan(&group, &cost, &created)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE provider_budget_state SET spent=GREATEST(0,spent-$2) WHERE group_key=$1 AND day=$3::date`, group, cost, created.UTC().Format("2006-01-02"))
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
