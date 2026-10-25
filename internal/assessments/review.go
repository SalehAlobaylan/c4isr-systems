package assessments

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publiccontext"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"strings"
	"time"
)

type Proposal struct {
	ReportEvidenceID    string   `json:"reportEvidenceId,omitempty"`
	JobID               string   `json:"jobId"`
	Conclusion          string   `json:"conclusion"`
	Method              Method   `json:"method"`
	Confidence          *float64 `json:"confidence"`
	ConfidenceRationale string   `json:"confidenceRationale"`
	ModelScore          *float64 `json:"modelScore"`
	ExpectedRevisionID  string   `json:"expectedRevisionId,omitempty"`
}
type ReviewRevision struct {
	ReportEvidence       *publicingest.Evidence `json:"reportEvidence,omitempty"`
	ID                   string                 `json:"id"`
	AssessmentID         string                 `json:"assessmentId"`
	SourceID             string                 `json:"sourceId"`
	Version              int                    `json:"version"`
	Status               string                 `json:"status"`
	PreviousID           string                 `json:"previousId,omitempty"`
	Conclusion           string                 `json:"conclusion"`
	Method               Method                 `json:"method"`
	Confidence           *float64               `json:"confidence"`
	ConfidenceRationale  string                 `json:"confidenceRationale"`
	ModelScore           *float64               `json:"modelScore"`
	AuthorID             string                 `json:"authorId"`
	ActorID              string                 `json:"actorId"`
	RecordedAt           time.Time              `json:"recordedAt"`
	ReviewRationale      string                 `json:"reviewRationale"`
	Evidence             imagery.Job            `json:"evidence,omitzero"`
	PublishedBy          string                 `json:"publishedBy,omitempty"`
	AuthorHumanID        string                 `json:"authorHumanId,omitempty"`
	ActorHumanID         string                 `json:"actorHumanId,omitempty"`
	PublisherHumanID     string                 `json:"publisherHumanId,omitempty"`
	ReviewIdentityStatus string                 `json:"reviewIdentityStatus,omitempty"`
}
type ReviewAction struct {
	ExpectedRevisionID  string   `json:"expectedRevisionId"`
	Action              string   `json:"action"`
	Rationale           string   `json:"rationale"`
	Confidence          *float64 `json:"confidence"`
	ConfidenceRationale string   `json:"confidenceRationale"`
}
type Reviews struct {
	Pool    *pgxpool.Pool
	Imagery *imagery.Service
}

func NewReviews(pool *pgxpool.Pool, images *imagery.Service) *Reviews { return &Reviews{pool, images} }
func reviewHuman(ctx context.Context, permission string) error {
	if e := artifacts.Public(ctx, permission); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" {
		return apperr.Forbidden("assessment authorship/review requires a human identity")
	}
	if p.HumanID == "" {
		return apperr.Forbidden("registered human identity required")
	}
	return nil
}
func validConfidence(v *float64) bool {
	return v == nil || (!math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 && *v <= 1)
}
func (r *Reviews) jobEvidence(ctx context.Context, id string) (imagery.Job, error) {
	j, e := r.Imagery.Job(ctx, id)
	if e != nil {
		return j, e
	}
	if j.Status != "SUCCEEDED" || j.Input.Kind != "NDVI_CHANGE" || j.Result == nil {
		return j, apperr.Conflict("proposal requires a completed reproducible imagery result")
	}
	for _, a := range j.Input.Artifacts {
		actual, e := r.Imagery.Artifacts.Ready(ctx, a.ID)
		if e != nil {
			return j, e
		}
		if actual.SHA256 != a.SHA256 {
			return j, apperr.Conflict("input artifact hash differs")
		}
	}
	output, e := r.Imagery.Artifacts.Ready(ctx, j.Result.ArtifactID)
	if e != nil {
		return j, e
	}
	if output.SHA256 != j.Result.SHA256 {
		return j, apperr.Conflict("derived result hash differs")
	}
	for _, scene := range j.Input.Scenes {
		a, e := r.Imagery.Artifacts.Admission(ctx, scene.SourceID)
		if e != nil || !a.Rights.AllowAnalysis || !scene.Rights.AllowAnalysis {
			return j, apperr.Forbidden("evidence analysis rights unavailable")
		}
	}
	return j, nil
}
func (r *Reviews) Propose(ctx context.Context, in Proposal, assessmentID string) (ReviewRevision, error) {
	var v ReviewRevision
	if e := reviewHuman(ctx, "assessments.create"); e != nil {
		return v, e
	}
	in.Conclusion = strings.TrimSpace(in.Conclusion)
	if len(in.Conclusion) < 10 || len(in.Conclusion) > 8000 || !validConfidence(in.Confidence) || !validConfidence(in.ModelScore) || len(in.ConfidenceRationale) > 4000 || len(in.ExpectedRevisionID) > 128 {
		return v, apperr.Validation("invalid proposal conclusion, confidence or revision")
	}
	if in.Method != MethodOperator && in.Method != MethodAlgorithm && in.Method != MethodAI {
		return v, apperr.Validation("proposal method must distinguish OPERATOR, ALGORITHM and AI")
	}
	if in.Method != MethodAI && in.ModelScore != nil {
		return v, apperr.Validation("model score is separate and only applies to an AI proposal")
	}
	if in.Confidence != nil && len(strings.TrimSpace(in.ConfidenceRationale)) < 10 {
		return v, apperr.Validation("proposed confidence requires a rationale")
	}
	var j imagery.Job
	var report *publicingest.Evidence
	var e error
	source, assessmentType := "", "imagery.change"
	if (in.ReportEvidenceID == "") == (in.JobID == "") {
		return v, apperr.Validation("choose exactly one imagery job or report evidence revision")
	}
	if in.ReportEvidenceID != "" {
		value, err := r.reportEvidence(ctx, in.ReportEvidenceID)
		if err != nil {
			return v, err
		}
		report = &value
		source = value.SourceID
		assessmentType = "report.context"
	} else {
		j, e = r.jobEvidence(ctx, in.JobID)
		if e != nil {
			return v, e
		}
		source = j.SourceID
	}
	p, _, _ := access.From(ctx)
	newRoot := assessmentID == ""
	if newRoot {
		assessmentID = ids.New("asm")
	}
	v = ReviewRevision{ID: ids.New("asrv"), AssessmentID: assessmentID, SourceID: source, ReportEvidence: report, Version: 1, Status: "PROPOSED", Conclusion: in.Conclusion, Method: in.Method, Confidence: in.Confidence, ConfidenceRationale: in.ConfidenceRationale, ModelScore: in.ModelScore, AuthorID: p.ID, ActorID: p.ID, RecordedAt: time.Now().UTC(), Evidence: j}
	v.AuthorHumanID, v.ActorHumanID = p.HumanID, p.HumanID
	v.ReviewIdentityStatus = "PENDING"
	e = platformdb.WithTx(ctx, r.Pool, func(tx pgx.Tx) error {
		if newRoot {
			if _, e := tx.Exec(ctx, `INSERT INTO assessments(id,subject_type,subject_id,type,conclusion,confidence,method,created_by,review_source_id)VALUES($1,'source',$2,$7,$3,$4,$5,$6,$2)`, assessmentID, source, v.Conclusion, v.Confidence, v.Method, p.ID, assessmentType); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `INSERT INTO assessment_evidence(assessment_id,evidence_type,evidence_id)VALUES($1,'source',$2)`, assessmentID, source); e != nil {
				return e
			}
		} else {
			latest, e := loadReview(ctx, tx, assessmentID, true)
			if e != nil {
				return e
			}
			if latest.ID != in.ExpectedRevisionID {
				return apperr.Conflict("assessment revision changed; reload before revising")
			}
			if latest.Status != "PUBLISHED" && latest.Status != "REJECTED" && latest.Status != "DRAFT" && latest.Status != "PROPOSED" {
				return apperr.Conflict("assessment is under review")
			}
			if latest.SourceID != source || (latest.ReportEvidence == nil) != (report == nil) {
				return apperr.Validation("a different source requires a new assessment")
			}
			v.PreviousID = latest.ID
			v.Version = latest.Version + 1
			v.Status = "DRAFT"
		}
		return saveReview(ctx, tx, v, newRoot)
	})
	return v, e
}
func loadReview(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string, lock bool) (ReviewRevision, error) {
	var v ReviewRevision
	var raw []byte
	sql := `SELECT r.document FROM assessment_review_heads h JOIN assessment_revisions r ON r.id=h.latest_id AND r.scope_id=h.scope_id WHERE h.assessment_id=$1`
	if lock {
		sql += ` FOR UPDATE OF h`
	}
	e := q.QueryRow(ctx, sql, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, apperr.NotFound("assessment revision", "unavailable")
	}
	if e == nil {
		e = json.Unmarshal(raw, &v)
	}
	if e == nil {
		e = ResolveReviewIdentity(ctx, q, &v)
	}
	return v, e
}
func saveReview(ctx context.Context, tx pgx.Tx, v ReviewRevision, newRoot bool) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO assessment_revisions(id,assessment_id,source_id,version,status,document)VALUES($1,$2,$3,$4,$5,$6)`, v.ID, v.AssessmentID, v.SourceID, v.Version, v.Status, raw); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO assessment_review_identities(revision_id,scope_id,source_id,author_human_id,actor_human_id,publisher_human_id) VALUES($1,NULLIF(current_setting('emad.scope_id',true),''),$2,$3,$4,NULLIF($5,''))`, v.ID, v.SourceID, v.AuthorHumanID, v.ActorHumanID, v.PublisherHumanID); e != nil {
		return e
	}
	if newRoot {
		_, e = tx.Exec(ctx, `INSERT INTO assessment_review_heads(assessment_id,source_id,latest_id)VALUES($1,$2,$3)`, v.AssessmentID, v.SourceID, v.ID)
	} else {
		_, e = tx.Exec(ctx, `UPDATE assessment_review_heads SET latest_id=$2,published_id=CASE WHEN $3::text='PUBLISHED' THEN $2 ELSE published_id END WHERE assessment_id=$1`, v.AssessmentID, v.ID, v.Status)
	}
	if e != nil {
		return e
	}
	data, _ := json.Marshal(map[string]any{"sourceId": v.SourceID, "revisionId": v.ID, "version": v.Version, "status": v.Status, "method": v.Method})
	_, e = tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data,created_at)VALUES($1,$2,'OPERATOR',$3,'assessment.revision.recorded','assessment',$4,$5,$6,now())`, ids.New("aud"), v.RecordedAt, v.ActorID, v.AssessmentID, httpx.GetRequestID(ctx), data)
	return e
}
func (r *Reviews) Act(ctx context.Context, id string, in ReviewAction) (ReviewRevision, error) {
	var v ReviewRevision
	permission := "assessments.review"
	if in.Action == "submit" {
		permission = "assessments.create"
	}
	if e := reviewHuman(ctx, permission); e != nil {
		return v, e
	}
	if len(strings.TrimSpace(in.Rationale)) < 10 || len(in.Rationale) > 4000 {
		return v, apperr.Validation("record an explicit review rationale")
	}
	p, _, _ := access.From(ctx)
	e := platformdb.WithTx(ctx, r.Pool, func(tx pgx.Tx) error {
		old, e := loadReview(ctx, tx, id, true)
		if e != nil {
			return e
		}
		if old.ID != in.ExpectedRevisionID {
			return apperr.Conflict("assessment revision changed; reload before reviewing")
		}
		if (in.Action == "review" || in.Action == "publish" || in.Action == "reject") && (old.AuthorHumanID == "" || p.HumanID == old.AuthorHumanID) {
			return apperr.Forbidden("an independent human must review this proposal")
		}
		next := ""
		switch in.Action {
		case "submit":
			if old.Status == "DRAFT" {
				next = "PROPOSED"
			}
		case "review":
			if old.Status == "PROPOSED" {
				next = "IN_REVIEW"
			}
		case "publish":
			if old.Status == "IN_REVIEW" {
				next = "PUBLISHED"
			}
		case "reject":
			if old.Status == "IN_REVIEW" {
				next = "REJECTED"
			}
		}
		if next == "" {
			return apperr.Conflict("review action is invalid for the current state")
		}
		v = old
		v.ID = ids.New("asrv")
		v.PreviousID = old.ID
		v.Version++
		v.Status = next
		v.ActorID = p.ID
		v.ActorHumanID = p.HumanID
		if in.Action != "submit" {
			v.ReviewIdentityStatus = "DISTINCT_REGISTERED_HUMANS"
		}
		v.RecordedAt = time.Now().UTC()
		v.ReviewRationale = in.Rationale
		if next == "PUBLISHED" {
			if !validConfidence(in.Confidence) || in.Confidence == nil || len(strings.TrimSpace(in.ConfidenceRationale)) < 10 || len(in.ConfidenceRationale) > 4000 {
				return apperr.Validation("publication requires analyst confidence and its own rationale")
			}
			// Validate the exact frozen evidence, not the latest catalogue scene.
			if old.ReportEvidence != nil {
				exact, e := r.reportEvidence(ctx, old.ReportEvidence.ID)
				if e != nil {
					return e
				}
				if exact.ContentHash != old.ReportEvidence.ContentHash || exact.RawHash != old.ReportEvidence.RawHash {
					return apperr.Conflict("report evidence hash differs")
				}
			} else if _, e := r.jobEvidence(ctx, old.Evidence.ID); e != nil {
				return e
			}
			v.Confidence = in.Confidence
			v.ConfidenceRationale = in.ConfidenceRationale
			v.PublishedBy = p.ID
			v.PublisherHumanID = p.HumanID
		}
		return saveReview(ctx, tx, v, false)
	})
	return v, e
}
func (r *Reviews) Latest(ctx context.Context, id string) (ReviewRevision, error) {
	if e := artifacts.Public(ctx, "assessments.read"); e != nil {
		return ReviewRevision{}, e
	}
	return loadReview(ctx, platformdb.Scoped(r.Pool), id, false)
}

// Revision retrieves a pinned version even after it leaves the bounded history view.
func (r *Reviews) Revision(ctx context.Context, id, revisionID string) (ReviewRevision, error) {
	var v ReviewRevision
	if e := artifacts.Public(ctx, "assessments.read"); e != nil {
		return v, e
	}
	var raw []byte
	e := platformdb.Scoped(r.Pool).QueryRow(ctx, `SELECT document FROM assessment_revisions WHERE assessment_id=$1 AND id=$2`, id, revisionID).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, apperr.NotFound("assessment revision", "unavailable")
	}
	if e == nil {
		e = json.Unmarshal(raw, &v)
	}
	if e == nil {
		e = ResolveReviewIdentity(ctx, platformdb.Scoped(r.Pool), &v)
	}
	return v, e
}
func (r *Reviews) History(ctx context.Context, id string) ([]ReviewRevision, error) {
	if e := artifacts.Public(ctx, "assessments.read"); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(r.Pool).Query(ctx, `SELECT document FROM assessment_revisions WHERE assessment_id=$1 ORDER BY version DESC LIMIT 101`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ReviewRevision{}
	for rows.Next() {
		var raw []byte
		var v ReviewRevision
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	for i := range out {
		if e = ResolveReviewIdentity(ctx, platformdb.Scoped(r.Pool), &out[i]); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (r *Reviews) List(ctx context.Context) ([]ReviewRevision, error) { return r.list(ctx, false) }
func (r *Reviews) ListReports(ctx context.Context) ([]ReviewRevision, error) {
	return r.list(ctx, true)
}
func (r *Reviews) list(ctx context.Context, reports bool) ([]ReviewRevision, error) {
	if e := artifacts.Public(ctx, "assessments.read"); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(r.Pool).Query(ctx, `SELECT r.document FROM assessment_review_heads h JOIN assessment_revisions r ON r.id=h.latest_id AND r.scope_id=h.scope_id WHERE (r.document ? 'reportEvidence')=$1 ORDER BY r.document->>'recordedAt' DESC,r.id LIMIT 101`, reports)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ReviewRevision{}
	for rows.Next() {
		var b []byte
		var v ReviewRevision
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	for i := range out {
		if e = ResolveReviewIdentity(ctx, platformdb.Scoped(r.Pool), &out[i]); e != nil {
			return nil, e
		}
	}
	return out, nil
}

// Reports freeze the exact retained source revision, including hashes, extraction
// and lineage. Current and original rights are rechecked at publication.
func (r *Reviews) reportEvidence(ctx context.Context, id string) (publicingest.Evidence, error) {
	entry, err := (publiccontext.Service{Pool: r.Pool}).Get(ctx, id)
	if err != nil {
		return publicingest.Evidence{}, err
	}
	v := entry.Evidence
	if v.Context == nil || v.Context.Kind != "REPORT" {
		return v, apperr.Validation("proposal requires a source report revision")
	}
	var original, active []byte
	var retained bool
	err = platformdb.Scoped(r.Pool).QueryRow(ctx, `SELECT a.document,curr.document,e.original_record IS NOT NULL FROM public_evidence e JOIN source_admissions a ON a.id=e.admission_id JOIN source_connector_state c ON c.source_id=e.source_id AND c.scope_id=e.scope_id JOIN source_admissions curr ON curr.id=c.admission_id WHERE e.id=$1`, id).Scan(&original, &active, &retained)
	if err != nil {
		return v, err
	}
	for _, raw := range [][]byte{original, active} {
		var a publicingest.Admission
		if err = json.Unmarshal(raw, &a); err != nil {
			return v, err
		}
		if !retained || !a.Rights.AllowAnalysis || !a.Rights.RetainRaw || a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now()) {
			return v, apperr.Forbidden("retained report analysis rights unavailable")
		}
	}
	return v, nil
}
