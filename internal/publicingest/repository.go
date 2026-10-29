package publicingest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Projector interface {
	Project(context.Context, pgx.Tx, Evidence) error
}
type Repository struct {
	pool      *pgxpool.Pool
	projector Projector
}

func (r *Repository) SetProjector(p Projector) { r.projector = p }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type state struct {
	QueryRequestID           *string
	QueryAOI                 *Bounds
	Admission                Admission
	Checkpoint               string
	NextAttempt              time.Time
	LastAttempt, LastSuccess *time.Time
	LastFailure              string
	Failures                 int
	LeaseID                  *string
	LeaseUntil               *time.Time
	CoverageFrom, CoverageTo *time.Time
	QueryFrom, QueryTo       *time.Time
}

const stateSQL = `SELECT a.document,s.checkpoint,s.next_attempt_at,s.last_attempt_at,s.last_success_at,s.last_failure_code,s.failures,s.lease_id,s.lease_until,s.coverage_from,s.coverage_to,s.query_from,s.query_to,s.query_request_id,s.query_aoi
 FROM source_connector_state s JOIN source_admissions a ON a.id=s.admission_id AND a.scope_id=s.scope_id AND a.source_id=s.source_id WHERE s.source_id=$1`

func scanState(row pgx.Row) (state, error) {
	var st state
	var doc, aoi []byte
	err := row.Scan(&doc, &st.Checkpoint, &st.NextAttempt, &st.LastAttempt, &st.LastSuccess, &st.LastFailure, &st.Failures, &st.LeaseID, &st.LeaseUntil, &st.CoverageFrom, &st.CoverageTo, &st.QueryFrom, &st.QueryTo, &st.QueryRequestID, &aoi)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, apperr.NotFound("source admission", "unavailable")
	}
	if err != nil {
		return st, err
	}
	if len(aoi) > 0 {
		if err = json.Unmarshal(aoi, &st.QueryAOI); err != nil {
			return st, err
		}
	}
	err = json.Unmarshal(doc, &st.Admission)
	return st, err
}
func (r *Repository) State(ctx context.Context, source string) (state, error) {
	return scanState(platformdb.Scoped(r.pool).QueryRow(ctx, stateSQL, source))
}

func (r *Repository) Admit(ctx context.Context, a Admission, expected ...string) (Admission, error) {
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "admission:"+a.SourceID); err != nil {
			return err
		}
		old, err := scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, a.SourceID))
		if err != nil && !apperr.Is(err, apperr.CodeNotFound) {
			return err
		}
		a.Version = 1
		if err == nil {
			// A different producer/product is a new source identity. It must not
			// collide with existing provider revision keys or inherit coverage.
			if old.Admission.OriginalProducer != a.OriginalProducer || old.Admission.Dataset != a.Dataset || old.Admission.ProductVersion != a.ProductVersion {
				return apperr.Conflict("producer/dataset/product identity requires a new source")
			}
			a.Version = old.Admission.Version + 1
			if old.LeaseID != nil {
				if err = interruptRun(ctx, tx, *old.LeaseID, a.ApprovedAt, "admission_changed"); err != nil {
					return err
				}
			}
		}
		// A rights ID continues to name its original reviewed terms, including
		// when a reviewer returns to an older ID after another admission.
		rights, _ := json.Marshal(a.Rights)
		var changedRights bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_admissions WHERE source_id=$1 AND document->'rights'->>'id'=$2 AND document->'rights'<>$3::jsonb)`, a.SourceID, a.Rights.ID, rights).Scan(&changedRights); err != nil {
			return err
		}
		if changedRights {
			return apperr.Conflict("changed rights require a new rights record id")
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM source_admissions WHERE source_id=$1`, a.SourceID).Scan(&a.Version); err != nil {
			return err
		}
		var changedIdentity bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_admissions WHERE source_id=$1 AND (document->>'originalProducer'<>$2 OR document->>'dataset'<>$3 OR document->>'productVersion'<>$4))`, a.SourceID, a.OriginalProducer, a.Dataset, a.ProductVersion).Scan(&changedIdentity); err != nil {
			return err
		}
		if changedIdentity {
			return apperr.Conflict("producer/dataset/product identity requires a new source")
		}
		a.ID = ids.New("adm")
		doc, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO source_admissions(id,source_id,version,document) VALUES($1,$2,$3,$4)`, a.ID, a.SourceID, a.Version, doc); err != nil {
			return err
		}
		if err = registerProfile(ctx, tx, a, expected); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO retrieval_profile_state(source_id,admission_id,profile_key) VALUES($1,$2,$3)
 ON CONFLICT(scope_id,source_id,profile_key) DO UPDATE SET admission_id=excluded.admission_id,last_work_kind='CURRENT',checkpoint='',last_success_at=NULL,coverage_from=NULL,coverage_to=NULL,query_from=NULL,query_to=NULL,query_request_id=NULL,query_aoi=NULL,lease_id=NULL,lease_until=NULL,last_failure_code='',failures=0`, a.SourceID, a.ID, access.RetrievalProfile(ctx))
		return err
	})
	return a, err
}

func interruptRun(ctx context.Context, tx pgx.Tx, id string, now time.Time, reason string) error {
	_, err := tx.Exec(ctx, `UPDATE connector_runs SET document=document || jsonb_build_object('status','INTERRUPTED','finishedAt',$2::timestamptz,'failureCode',$3::text) WHERE id=$1 AND document->>'status'='RUNNING'`, id, now, reason)
	return err
}

func (r *Repository) Begin(ctx context.Context, source, principal, admissionID string, now time.Time, windows ...*Window) (Run, state, error) {
	var run Run
	var st state
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		st, err = scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, source))
		if err != nil {
			return err
		}
		work := acquisitionWork(ctx)
		requestID := ""
		if work != nil {
			requestID = work.RequestID
		}
		if st.QueryRequestID != nil && *st.QueryRequestID != requestID {
			return apperr.Conflict("resume the frozen acquisition query before another intent")
		}
		if work != nil && st.QueryRequestID == nil && st.QueryFrom == nil {
			st.Checkpoint = ""
		}
		if work != nil && st.QueryRequestID == nil && st.QueryFrom != nil {
			return apperr.Conflict("resume the current profile query before queued backfill")
		}
		if st.Admission.ID != admissionID || st.Admission.ConnectorPrincipalID != principal {
			return apperr.Conflict("admission changed before execution")
		}
		if !st.Admission.Enabled || (st.Admission.Rights.ExpiresAt != nil && !st.Admission.Rights.ExpiresAt.After(now)) {
			return apperr.Conflict("source admission is disabled or expired")
		}
		if st.LeaseID != nil && st.LeaseUntil != nil && st.LeaseUntil.After(now) {
			return apperr.Conflict("connector execution already active")
		}
		if st.NextAttempt.After(now) {
			return apperr.Conflict("connector is in rate limit or outage backoff; inspect source health")
		}
		if st.LeaseID != nil {
			if err = interruptRun(ctx, tx, *st.LeaseID, now, "lease_expired"); err != nil {
				return err
			}
		}
		run = Run{ProfileKey: access.RetrievalProfile(ctx), ProfileRevisionID: "rpr_" + st.Admission.ID, ID: ids.New("cnr"), SourceID: source, AdmissionID: st.Admission.ID, ConnectorID: st.Admission.ConnectorID, PrincipalID: principal, StartedAt: now, Status: "RUNNING"}
		if work != nil {
			run.AcquisitionRequestID = work.RequestID
			run.AcquisitionPartID = work.ID
			run.QueryAOI = &work.AOI
		}
		if err = checkExecution(ctx, tx, run); err != nil {
			return err
		}
		doc, _ := json.Marshal(run)
		if _, err = tx.Exec(ctx, `INSERT INTO connector_runs(id,source_id,admission_id,document) VALUES($1,$2,$3,$4)`, run.ID, source, run.AdmissionID, doc); err != nil {
			return err
		}
		if len(windows) > 0 && windows[0] != nil {
			from, to := windows[0].From.UTC().Truncate(time.Microsecond), windows[0].To.UTC().Truncate(time.Microsecond)
			if st.QueryFrom != nil && st.QueryTo != nil && (!st.QueryFrom.Equal(from) || !st.QueryTo.Equal(to)) {
				return apperr.Conflict("resume the unfinished discovery window before selecting another interval")
			}
			st.QueryFrom = &from
			st.QueryTo = &to
		}
		// Native moving feeds are live captures, without replayable cursors.
		// A failed/partial capture must never pin the next request to old time.
		if st.Admission.Dataset == OpenSkyDataset || st.Admission.Dataset == AISDataset || st.Admission.Dataset == METARDataset || st.Admission.Dataset == GDACSDataset {
			st.QueryFrom = nil
			st.QueryTo = nil
			st.Checkpoint = ""
		}
		if st.QueryFrom == nil || st.QueryTo == nil {
			// PostgreSQL timestamps retain microseconds. Use that precision on
			// the first request too, so a resumed cursor sees identical bounds.
			to := now.Truncate(time.Microsecond)
			from := to.Add(-time.Duration(st.Admission.Policy.LookbackSeconds) * time.Second)
			st.QueryFrom = &from
			st.QueryTo = &to
		}
		aoi := st.Admission.Policy.AOI
		if work != nil {
			aoi = work.AOI
			st.QueryAOI = &aoi
			st.QueryRequestID = &requestID
		}
		run.QueryAOI = &aoi
		run.QueryFrom = st.QueryFrom
		run.QueryTo = st.QueryTo
		aoiRaw, _ := json.Marshal(aoi)
		if _, err = tx.Exec(ctx, `UPDATE connector_runs SET document=document||jsonb_build_object('queryAoi',$2::jsonb,'queryFrom',$3::timestamptz,'queryTo',$4::timestamptz,'acquisitionRequestId',$5::text,'acquisitionPartId',$6::text) WHERE id=$1`, run.ID, aoiRaw, st.QueryFrom, st.QueryTo, run.AcquisitionRequestID, run.AcquisitionPartID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE source_connector_state SET lease_id=$2,lease_until=$3,last_attempt_at=$4,query_from=$5,query_to=$6,checkpoint=$7 WHERE source_id=$1`, source, run.ID, now.Add(45*time.Second), now, st.QueryFrom, st.QueryTo, st.Checkpoint)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE retrieval_profile_state SET query_request_id=NULLIF($3,''),query_aoi=$4 WHERE source_id=$1 AND profile_key=$2`, source, access.RetrievalProfile(ctx), requestID, aoiRaw)
		return err
	})
	return run, st, err
}

func checkLease(st state, run Run, now time.Time) error {
	if st.LeaseID == nil || *st.LeaseID != run.ID || st.Admission.ID != run.AdmissionID || st.LeaseUntil == nil || !st.LeaseUntil.After(now) {
		return apperr.Conflict("connector lease is no longer active")
	}
	if !st.Admission.Enabled || (st.Admission.Rights.ExpiresAt != nil && !st.Admission.Rights.ExpiresAt.After(now)) {
		return apperr.Conflict("source admission is disabled or expired")
	}
	return nil
}

func (r *Repository) ReserveRequest(ctx context.Context, run Run, now time.Time, trustedSourceCeiling int, checkOnly ...bool) error {
	return platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		st, err := scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, run.SourceID))
		if err != nil {
			return err
		}
		if err = checkLease(st, run, now); err != nil {
			return err
		}
		if err = checkExecution(ctx, tx, run); err != nil {
			return err
		}
		if st.NextAttempt.After(now) {
			return apperr.Conflict("rate_limited")
		}
		resource, _ := access.ResourceFrom(ctx)
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/source-provider-budget/"+run.SourceID); err != nil {
			return err
		}
		var combined int
		var sourceNext time.Time
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN quota_day=$2::date THEN requests_today ELSE 0 END),0),COALESCE(max(next_attempt_at),'epoch'::timestamptz) FROM retrieval_profile_state WHERE source_id=$1`, run.SourceID, now.UTC().Format("2006-01-02")).Scan(&combined, &sourceNext); err != nil {
			return err
		}
		if trustedSourceCeiling < 1 || combined >= trustedSourceCeiling {
			return apperr.Conflict("daily_quota")
		}
		if sourceNext.After(now) {
			return apperr.Conflict("rate_limited")
		}
		var used int
		if err = tx.QueryRow(ctx, `SELECT CASE WHEN quota_day=$2::date THEN requests_today ELSE 0 END FROM source_connector_state WHERE source_id=$1`, run.SourceID, now.UTC().Format("2006-01-02")).Scan(&used); err != nil {
			return err
		}
		if used >= st.Admission.Policy.MaxRequestsPerDay {
			return apperr.Conflict("daily_quota")
		}
		if len(checkOnly) > 0 && checkOnly[0] {
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE source_connector_state SET quota_day=$2::date,requests_today=$3,next_attempt_at=$4 WHERE source_id=$1`, run.SourceID, now.UTC().Format("2006-01-02"), used+1, now.Add(time.Duration(st.Admission.Policy.MinIntervalSeconds)*time.Second)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE connector_runs SET document=jsonb_set(document,'{requests}',to_jsonb((document->>'requests')::int+1)) WHERE id=$1`, run.ID)
		return err
	})
}

func (r *Repository) CommitPage(ctx context.Context, run Run, before string, page Page, evidence []Evidence, now time.Time) error {
	return platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		st, err := scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, run.SourceID))
		if err != nil {
			return err
		}
		if err = checkLease(st, run, now); err != nil {
			return err
		}
		if err = checkExecution(ctx, tx, run); err != nil {
			return err
		}
		if st.Checkpoint != before {
			return apperr.Conflict("checkpoint changed")
		}

		var storedCount, storedBytes int64
		moving := st.Admission.Dataset == OpenSkyDataset || st.Admission.Dataset == AISDataset || st.Admission.Dataset == METARDataset || st.Admission.Dataset == GDACSDataset
		if moving {
			if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(octet_length(document::text)+COALESCE(octet_length(original_record),0)),0) FROM public_evidence WHERE source_id=$1`, run.SourceID).Scan(&storedCount, &storedBytes); err != nil {
				return err
			}
		}
		var estimated int64
		for _, e := range evidence {
			doc, _ := json.Marshal(e)
			estimated += int64(len(doc) + len(e.OriginalRecord) + len(e.OriginalBytes) + 512)
		}
		if err = capacity.Check(ctx, tx, estimated, run.ID); err != nil {
			return err
		}
		added, duplicates := 0, 0
		for _, e := range evidence {
			raw := []byte(e.OriginalRecord)
			if e.OriginalMediaType == "text/csv" || e.OriginalMediaType == "application/xml" {
				raw = e.OriginalBytes
			}
			e.OriginalRecord = nil
			doc, err := json.Marshal(e)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `INSERT INTO public_evidence(id,source_id,admission_id,connector_run_id,provider_record_id,provider_revision,content_hash,document,original_record)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(scope_id,source_id,provider_record_id,provider_revision) DO NOTHING`, e.ID, e.SourceID, e.AdmissionID, e.ConnectorRunID, e.ProviderRecordID, e.ProviderRevision, e.ContentHash, doc, []byte(raw))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				var hash string
				if err = tx.QueryRow(ctx, `SELECT content_hash FROM public_evidence WHERE source_id=$1 AND provider_record_id=$2 AND provider_revision=$3`, e.SourceID, e.ProviderRecordID, e.ProviderRevision).Scan(&hash); err != nil {
					return err
				}
				if hash != e.ContentHash {
					return apperr.Conflict("provider revision reused with different content")
				}
				duplicates++
			} else {
				if moving {
					storedCount++
					storedBytes += int64(len(doc) + len(raw))
					if storedBytes > 64*1024*1024 {
						return apperr.Conflict("source_storage_budget")
					}
				}
				if _, err = tx.Exec(ctx, `INSERT INTO public_evidence_outbox(evidence_id,source_id) VALUES($1,$2)`, e.ID, e.SourceID); err != nil {
					return err
				}
				added++
			}
		}
		for _, e := range evidence {
			if _, err = tx.Exec(ctx, `INSERT INTO retrieval_occurrences(source_id,run_id,evidence_id,profile_revision_id) SELECT $1,$2,id,$3 FROM public_evidence WHERE source_id=$1 AND provider_record_id=$4 AND provider_revision=$5 ON CONFLICT DO NOTHING`, run.SourceID, run.ID, "rpr_"+run.AdmissionID, e.ProviderRecordID, e.ProviderRevision); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE source_connector_state SET checkpoint=$2 WHERE source_id=$1`, run.SourceID, page.Checkpoint); err != nil {
			return err
		}
		meta, _ := json.Marshal(run.Retrieval)
		_, err = tx.Exec(ctx, `UPDATE connector_runs SET document=document || jsonb_build_object('pages',COALESCE((document->>'pages')::int,0)+1,'rejectedRecords',COALESCE((document->>'rejectedRecords')::int,0)+$7::int,'providerTruncated',COALESCE((document->>'providerTruncated')::boolean,false) OR $8::boolean,'newEvidence',(document->>'newEvidence')::int+$2::int,'duplicates',(document->>'duplicates')::int+$3::int,'retrieval',$4::jsonb,'captureFrom',$5::timestamptz,'captureTo',$6::timestamptz) WHERE id=$1`, run.ID, added, duplicates, meta, run.CaptureFrom, run.CaptureTo, page.ExcludedRecords, page.PartialReason != "")
		return err
	})
}

func (r *Repository) Finish(ctx context.Context, run Run, status, code string, next time.Time, page *Page, now time.Time) (Run, error) {
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		st, err := scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, run.SourceID))
		if err != nil {
			return err
		}
		if err = checkLease(st, run, now); err != nil {
			return err
		}
		if err = checkExecution(ctx, tx, run); err != nil {
			return err
		}
		var from, to any
		success := status == "SUCCEEDED"
		if success && page != nil {
			from = page.CoverageFrom
			to = page.CoverageTo
		}
		_, err = tx.Exec(ctx, `UPDATE source_connector_state SET lease_id=NULL,lease_until=NULL,next_attempt_at=GREATEST(next_attempt_at,$2),
 last_failure_code=$3,failures=CASE WHEN $4 THEN 0 WHEN $5 THEN failures+1 ELSE failures END,
 last_success_at=CASE WHEN $4 THEN $6 ELSE last_success_at END,coverage_from=CASE WHEN $4 THEN $7 ELSE coverage_from END,coverage_to=CASE WHEN $4 THEN $8 ELSE coverage_to END
  ,query_from=CASE WHEN $4 THEN NULL ELSE query_from END,query_to=CASE WHEN $4 THEN NULL ELSE query_to END
 WHERE source_id=$1`, run.SourceID, next, code, success, status == "FAILED", now, from, to)
		if err != nil {
			return err
		}
		if success {
			if _, err = tx.Exec(ctx, `UPDATE retrieval_profile_state SET query_request_id=NULL,query_aoi=NULL WHERE source_id=$1 AND profile_key=$2`, run.SourceID, access.RetrievalProfile(ctx)); err != nil {
				return err
			}
		}
		var doc []byte
		meta, err := json.Marshal(run.Retrieval)
		if err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `UPDATE connector_runs SET document=document || jsonb_build_object('status',$2::text,'failureCode',$3::text,'finishedAt',$4::timestamptz) || CASE WHEN $5::jsonb='null'::jsonb THEN '{}'::jsonb ELSE jsonb_build_object('retrieval',$5::jsonb) END WHERE id=$1 RETURNING document`, run.ID, status, code, now, meta).Scan(&doc); err != nil {
			return err
		}
		return json.Unmarshal(doc, &run)
	})
	return run, err
}

// PublishOne commits catalog visibility and delivery acknowledgement together.
// A failed projection leaves the original evidence and its pending receipt.
func (r *Repository) PublishOne(ctx context.Context, source, admissionID, principalID string, now time.Time) (bool, error) {
	did := false
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		st, err := scanState(tx.QueryRow(ctx, stateSQL+` FOR UPDATE OF s`, source))
		if err != nil {
			return err
		}
		if st.Admission.ID != admissionID || st.Admission.ConnectorPrincipalID != principalID || !st.Admission.Enabled || (st.Admission.Rights.ExpiresAt != nil && !st.Admission.Rights.ExpiresAt.After(now)) {
			return apperr.Conflict("admission changed before publication")
		}
		var id string
		err = tx.QueryRow(ctx, `SELECT o.evidence_id FROM public_evidence_outbox o JOIN public_evidence e ON e.id=o.evidence_id WHERE o.source_id=$1 AND o.delivered_at IS NULL AND o.next_attempt_at<=$2 ORDER BY COALESCE((e.document->>'providerUpdatedAt')::timestamptz,(e.document->>'retrievedAt')::timestamptz),o.evidence_id FOR UPDATE OF o SKIP LOCKED LIMIT 1`, source, now).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			if m, ok := r.projector.(interface {
				Maintain(context.Context, pgx.Tx, string, time.Time) error
			}); ok {
				return m.Maintain(ctx, tx, source, now)
			}
			return nil
		}
		if err != nil {
			return err
		}
		if r.projector != nil {
			var raw []byte
			var e Evidence
			if err = tx.QueryRow(ctx, `SELECT document FROM public_evidence WHERE id=$1`, id).Scan(&raw); err != nil {
				return err
			}
			if err = json.Unmarshal(raw, &e); err != nil {
				return err
			}
			if err = r.projector.Project(ctx, tx, e); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public_evidence_catalog(evidence_id,source_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, source); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public_evidence_outbox SET delivered_at=$2,attempts=attempts+1 WHERE evidence_id=$1`, id, now)
		did = err == nil
		return err
	})
	return did, err
}

func (r *Repository) ListEvidence(ctx context.Context, source string, limit, offset int) ([]Evidence, int, error) {
	rows, err := platformdb.Scoped(r.pool).Query(ctx, `SELECT e.document FROM public_evidence_catalog c JOIN public_evidence e ON e.id=c.evidence_id AND e.source_id=c.source_id AND e.scope_id=c.scope_id WHERE c.source_id=$1 ORDER BY c.published_at DESC,c.evidence_id LIMIT $2 OFFSET $3`, source, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []Evidence{}
	for rows.Next() {
		var raw []byte
		var e Evidence
		if err = rows.Scan(&raw); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, 0, err
		}
		result = append(result, e)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	err = platformdb.Scoped(r.pool).QueryRow(ctx, `SELECT count(*) FROM public_evidence_catalog WHERE source_id=$1`, source).Scan(&total)
	return result, total, err
}

func (r *Repository) GetEvidence(ctx context.Context, source, id string) (Evidence, error) {
	var e Evidence
	var doc, raw []byte
	err := platformdb.Scoped(r.pool).QueryRow(ctx, `SELECT document,original_record FROM public_evidence WHERE source_id=$1 AND id=$2`, source, id).Scan(&doc, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, apperr.NotFound("evidence", "unavailable")
	}
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(doc, &e)
	if err != nil {
		return e, err
	}
	if len(raw) > 0 {
		policy, policyErr := ResolvePolicy(ctx, r.pool, source, e.AdmissionID)
		if policyErr != nil || !policy.Rights.RetainRaw {
			raw = nil
		}
	}
	if e.OriginalMediaType == "text/csv" || e.OriginalMediaType == "application/xml" {
		e.OriginalBytes = raw
	} else {
		e.OriginalRecord = raw
	}
	return e, err
}
func (r *Repository) Runs(ctx context.Context, source string, limit, offset int) ([]Run, int, error) {
	rows, err := platformdb.Scoped(r.pool).Query(ctx, `SELECT document FROM connector_runs WHERE source_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, source, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []Run{}
	for rows.Next() {
		var doc []byte
		var run Run
		if err = rows.Scan(&doc); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(doc, &run); err != nil {
			return nil, 0, err
		}
		result = append(result, run)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	err = platformdb.Scoped(r.pool).QueryRow(ctx, `SELECT count(*) FROM connector_runs WHERE source_id=$1`, source).Scan(&total)
	return result, total, err
}

// Summaries is a batch source-catalog port. Source status remains the existing
// registration status; ingestion coverage is a separate, derived fact.
func (r *Repository) Summaries(ctx context.Context, sourceIDs []string) (map[string]sources.IngestionSummary, error) {
	rows, err := platformdb.Scoped(r.pool).Query(ctx, `SELECT a.document,s.last_success_at,s.last_attempt_at,s.next_attempt_at,s.last_failure_code,s.failures,s.coverage_from,s.coverage_to,
 (SELECT count(*) FROM public_evidence_outbox o WHERE o.source_id=s.source_id AND o.scope_id=s.scope_id AND o.delivered_at IS NULL)
 FROM source_connector_state s JOIN source_admissions a ON a.id=s.admission_id WHERE s.source_id=ANY($1::text[])`, sourceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]sources.IngestionSummary{}
	for rows.Next() {
		var doc []byte
		var a Admission
		var info sources.IngestionSummary
		var failures int
		if err = rows.Scan(&doc, &info.LastSuccessAt, &info.LastAttemptAt, &info.NextAttemptAt, &info.FailureCode, &failures, &info.CoverageFrom, &info.CoverageTo, &info.PendingPublications); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(doc, &a); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		info.State = "unknown"
		info.AdmissionID = a.ID
		info.OriginalProducer = a.OriginalProducer
		info.UpstreamProvider = a.UpstreamProvider
		info.Dataset = a.Dataset
		info.ProductVersion = a.ProductVersion
		if !a.Enabled || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(now)) {
			info.State = "disabled"
		} else if info.LastSuccessAt != nil {
			age := now.Sub(*info.LastSuccessAt)
			if info.CoverageTo != nil && info.CoverageTo.Before(*info.LastSuccessAt) {
				age = now.Sub(*info.CoverageTo)
			}
			if age >= time.Duration(a.Policy.UnavailableAfterSeconds)*time.Second {
				info.State = "unavailable"
			} else if failures > 0 || age >= time.Duration(a.Policy.FreshForSeconds)*time.Second {
				info.State = "stale"
			} else {
				info.State = "current"
			}
		} else if failures > 0 {
			info.State = "unavailable"
		}
		info.Meaning = "Provider query coverage only; missing records are not evidence of absence."
		result[a.SourceID] = info
	}
	return result, rows.Err()
}

func (r *Repository) AdmissionVersion(ctx context.Context, source, id string) (Admission, error) {
	var a Admission
	var doc []byte
	err := platformdb.Scoped(r.pool).QueryRow(ctx, `SELECT document FROM source_admissions WHERE source_id=$1 AND id=$2`, source, id).Scan(&doc)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apperr.NotFound("source admission", "unavailable")
	}
	if err != nil {
		return a, err
	}
	err = json.Unmarshal(doc, &a)
	return a, err
}
