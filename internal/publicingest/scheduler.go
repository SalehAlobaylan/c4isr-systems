package publicingest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"math/rand/v2"
	"time"
)

type AcquisitionPart struct {
	ID                string     `json:"id"`
	RequestID         string     `json:"requestId"`
	SourceID          string     `json:"sourceId"`
	ProfileKey        string     `json:"profileKey"`
	ProfileRevisionID string     `json:"profileRevisionId"`
	AOI               Bounds     `json:"aoi"`
	From              time.Time  `json:"from"`
	To                time.Time  `json:"to"`
	Status            string     `json:"status"`
	Attempts          int        `json:"attempts"`
	NextAttemptAt     *time.Time `json:"nextAttemptAt,omitempty"`
	RunID             *string    `json:"runId,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	RecoveryOwner     string     `json:"recoveryOwner"`
	lease             string
}
type acquisitionContext struct{}

func acquisitionWork(ctx context.Context) *AcquisitionPart {
	v, _ := ctx.Value(acquisitionContext{}).(*AcquisitionPart)
	return v
}
func preserveWork(auth, work context.Context) context.Context {
	if v := acquisitionWork(auth); v != nil {
		return context.WithValue(work, acquisitionContext{}, v)
	}
	return work
}

// RunScheduled rotates authorized profiles by their last scheduling time. One
// bounded queued part shares each profile's turn with current polling, so a
// backfill cannot hold the worker indefinitely or replace another cursor.
func (s *Service) RunScheduled(ctx context.Context, source string) (Run, error) {
	if e := worker(ctx, source); e != nil {
		return Run{}, e
	}
	p, _, _ := access.From(ctx)
	var key string
	e := db.Scoped(s.repo.pool).QueryRow(ctx, `SELECT p.profile_key FROM retrieval_profiles p JOIN retrieval_profile_revisions r ON r.id=p.head_revision_id JOIN retrieval_execution_grants g ON g.profile_revision_id=r.id AND g.principal_id=$2 AND g.enabled JOIN retrieval_profile_state st ON st.source_id=p.source_id AND st.scope_id=p.scope_id AND st.profile_key=p.profile_key JOIN source_admissions a ON a.id=r.admission_id WHERE p.source_id=$1 AND (a.document->>'enabled')::boolean AND st.next_attempt_at<=clock_timestamp() AND (st.lease_until IS NULL OR st.lease_until<=clock_timestamp()) ORDER BY st.scheduled_at,p.profile_key LIMIT 1`, source, p.ID).Scan(&key)
	if errors.Is(e, pgx.ErrNoRows) {
		return Run{}, apperr.Conflict("no due authorized profile; inspect coverage and next-attempt status")
	}
	if e != nil {
		return Run{}, e
	}
	ctx = access.WithRetrievalProfile(ctx, key)
	var part *AcquisitionPart
	e = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var raw []byte
		var attempt int
		var currentTurn bool
		if e := tx.QueryRow(ctx, `SELECT query_request_id IS NULL AND (last_work_kind='ACQUISITION' OR query_from IS NOT NULL) FROM retrieval_profile_state WHERE source_id=$1 AND profile_key=$2 FOR UPDATE`, source, key).Scan(&currentTurn); e != nil {
			return e
		}
		if currentTurn {
			return nil
		}
		// Only one ordered part of an intent is outstanding. Stale claims can be
		// recovered, but the frozen query owner in Begin still fences every cursor.
		e := tx.QueryRow(ctx, `SELECT q.document,q.attempts FROM acquisition_parts q JOIN acquisition_requests r ON r.id=q.request_id WHERE q.source_id=$1 AND r.profile_key=$2 AND r.status IN('QUEUED','RUNNING','WAITING') AND q.status IN('QUEUED','WAITING','RUNNING') AND q.next_attempt_at<=clock_timestamp() AND (q.lease_until IS NULL OR q.lease_until<=clock_timestamp()) AND NOT EXISTS(SELECT 1 FROM acquisition_parts before WHERE before.request_id=q.request_id AND before.ordinal<q.ordinal AND before.status<>'SUCCEEDED') ORDER BY q.next_attempt_at,r.created_at,q.ordinal FOR UPDATE OF q SKIP LOCKED LIMIT 1`, source, key).Scan(&raw, &attempt)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		part = &AcquisitionPart{}
		if e = json.Unmarshal(raw, part); e != nil {
			return e
		}
		part.Attempts = attempt + 1
		part.lease = ids.New("acl")
		_, e = tx.Exec(ctx, `UPDATE acquisition_parts SET status='RUNNING',attempts=attempts+1,lease_id=$2,lease_until=clock_timestamp()+interval '50 seconds' WHERE id=$1`, part.ID, part.lease)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE acquisition_requests SET status='RUNNING',updated_at=clock_timestamp() WHERE id=$1`, part.RequestID)
		return e
	})
	if e != nil {
		return Run{}, e
	}
	_, e = db.Scoped(s.repo.pool).Exec(ctx, `UPDATE retrieval_profile_state SET scheduled_at=clock_timestamp(),last_work_kind=$3 WHERE source_id=$1 AND profile_key=$2`, source, key, func() string {
		if part != nil {
			return "ACQUISITION"
		}
		return "CURRENT"
	}())
	if e != nil {
		return Run{}, e
	}
	if part == nil {
		return s.Execute(ctx, source)
	}
	work := context.WithValue(ctx, acquisitionContext{}, part)
	run, executeErr := s.ExecuteWindow(work, source, &Window{From: part.From, To: part.To})
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	finishErr := s.finishPart(cleanup, *part, run, executeErr)
	if executeErr != nil {
		return run, executeErr
	}
	return run, finishErr
}
func (s *Service) finishPart(ctx context.Context, part AcquisitionPart, run Run, executeErr error) error {
	status, reason := "WAITING", run.FailureCode
	if run.Status == "SUCCEEDED" {
		status = "SUCCEEDED"
	}
	if executeErr != nil {
		var app *apperr.Error
		if errors.As(executeErr, &app) {
			reason = app.Message
		} else {
			reason = "execution unavailable; connector operator must inspect server logs"
		}
	}
	if part.Attempts >= 8 && status != "SUCCEEDED" {
		status = "FAILED"
	}
	if run.ProfileRevisionID != "" && run.ProfileRevisionID != part.ProfileRevisionID || reason == "invalid_record" || reason == "invalid_page" || reason == "revision_conflict" || reason == "source_storage_budget" {
		status = "FAILED"
	}
	delay := time.Duration(min(21600, 30*(1<<min(part.Attempts, 9))))*time.Second + time.Duration(rand.IntN(1000))*time.Millisecond
	return db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT source_id FROM retrieval_profile_state WHERE source_id=$1 AND profile_key=$2 FOR UPDATE`, part.SourceID, part.ProfileKey); e != nil {
			return e
		}
		tag, e := tx.Exec(ctx, `UPDATE acquisition_parts SET status=$3,document=document||jsonb_build_object('reason',$4::text),next_attempt_at=clock_timestamp()+$5::interval,run_id=(SELECT id FROM connector_runs WHERE id=$6),lease_id=NULL,lease_until=NULL WHERE id=$1 AND lease_id=$2 AND lease_until>clock_timestamp() AND status='RUNNING'`, part.ID, part.lease, status, reason, delay.String(), run.ID)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("acquisition claim expired or was cancelled; stale completion rejected")
		}
		if status == "FAILED" {
			if _, e = tx.Exec(ctx, `UPDATE retrieval_profile_state SET query_request_id=NULL,query_aoi=NULL,query_from=NULL,query_to=NULL,checkpoint='',lease_id=NULL,lease_until=NULL WHERE source_id=$1 AND profile_key=$2 AND query_request_id=$3`, part.SourceID, part.ProfileKey, part.RequestID); e != nil {
				return e
			}
		}
		_, e = tx.Exec(ctx, `UPDATE acquisition_requests r SET status=CASE WHEN EXISTS(SELECT 1 FROM acquisition_parts p WHERE p.request_id=r.id AND p.status='FAILED')THEN 'FAILED' WHEN NOT EXISTS(SELECT 1 FROM acquisition_parts p WHERE p.request_id=r.id AND p.status<>'SUCCEEDED')THEN 'SUCCEEDED' ELSE 'WAITING' END,updated_at=clock_timestamp() WHERE r.id=$1 AND r.status<>'CANCELLED'`, part.RequestID)
		return e
	})
}

// The request and claim are rechecked under the same transaction as every
// provider reservation and page/finish commit, including after external I/O.
func checkAcquisition(ctx context.Context, tx pgx.Tx, run Run) error {
	if run.AcquisitionRequestID == "" {
		return nil
	}
	work := acquisitionWork(ctx)
	if work == nil {
		return apperr.Forbidden("missing server acquisition claim")
	}
	var active bool
	e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM acquisition_parts p JOIN acquisition_requests r ON r.id=p.request_id WHERE p.id=$1 AND p.request_id=$2 AND p.lease_id=$3 AND p.status='RUNNING' AND p.lease_until>clock_timestamp() AND r.status='RUNNING' AND r.profile_revision_id=$4)`, work.ID, work.RequestID, work.lease, run.ProfileRevisionID).Scan(&active)
	if e != nil {
		return e
	}
	if !active {
		return apperr.Conflict("acquisition cancelled, replaced or expired")
	}
	return nil
}
