package investigations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type FindingInput struct {
	ID                 string     `json:"id,omitempty"`
	ExpectedRevisionID string     `json:"expectedRevisionId,omitempty"`
	Title              string     `json:"title"`
	Conclusion         string     `json:"conclusion"`
	Confidence         string     `json:"confidence"`
	Uncertainty        string     `json:"uncertainty"`
	Purpose            string     `json:"purpose"`
	Evidence           []Evidence `json:"evidence"`
}
type Finding struct {
	FindingInput
	RevisionID      string    `json:"revisionId"`
	PreviousID      string    `json:"previousId,omitempty"`
	ActorHumanID    string    `json:"actorHumanId"`
	ScopeHash       string    `json:"scopeHash"`
	RequiredSources []string  `json:"requiredSources"`
	RecordedAt      time.Time `json:"recordedAt"`
}
type Notice struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	State   string `json:"state"`
	Meaning string `json:"meaning"`
}
type ReviewInput struct {
	Action             string `json:"action"`
	ExpectedRevisionID string `json:"expectedRevisionId"`
	ExpectedEventID    string `json:"expectedEventId"`
	ReviewerHumanID    string `json:"reviewerHumanId,omitempty"`
	Reason             string `json:"reason"`
	OutputID           string `json:"outputId,omitempty"`
}
type ReviewEvent struct {
	OutputContentDigest string    `json:"outputContentDigest,omitempty"`
	ID                  string    `json:"id"`
	CaseID              string    `json:"caseId"`
	RevisionID          string    `json:"revisionId"`
	Action              string    `json:"action"`
	ReviewerHumanID     string    `json:"reviewerHumanId"`
	ActorHumanID        string    `json:"actorHumanId"`
	ContentDigest       string    `json:"contentDigest"`
	Contributors        []string  `json:"contributors"`
	Reason              string    `json:"reason"`
	OutputID            string    `json:"outputId,omitempty"`
	RenderSHA256        string    `json:"renderSha256,omitempty"`
	RecordedAt          time.Time `json:"recordedAt"`
}

func revisionDigest(r Revision) string { raw, _ := jsoncanon.Marshal(r); return areas.Hash(raw) }
func (s Service) Save(ctx context.Context, id string, in Input) (Case, error) {
	if in.RequestKey == "" {
		return s.save(ctx, id, in)
	} // compatibility; new UI always retains an intent
	if e := require(ctx, "investigations.read"); e != nil {
		return Case{}, e
	}
	if len(in.RequestKey) > 128 {
		return Case{}, apperr.Validation("bounded save intent required")
	}
	p, r, _ := access.From(ctx)
	raw, _ := jsoncanon.Marshal([]any{id, in})
	digest := areas.Hash(raw)
	var savedCase, savedRevision string
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+p.HumanID+in.RequestKey); e != nil {
			return e
		}
		var prior string
		e := tx.QueryRow(ctx, `SELECT input_hash,case_id,revision_id FROM investigation_save_intents WHERE human_id=$1 AND request_key=$2`, p.HumanID, in.RequestKey).Scan(&prior, &savedCase, &savedRevision)
		if e == nil {
			if prior != digest {
				return apperr.Conflict("save intent reused with different content")
			}
			allowed, e := s.liveHuman(ctx, tx, p.HumanID, r, "investigations.read", savedCase)
			if e != nil {
				return e
			}
			if !allowed {
				return apperr.Forbidden("live case/source access required to recover a save")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		c, e := s.save(ctx, id, in)
		if e != nil {
			return e
		}
		savedCase = c.ID
		savedRevision = c.Revision.ID
		_, e = tx.Exec(ctx, `INSERT INTO investigation_save_intents(human_id,request_key,input_hash,case_id,revision_id)VALUES($1,$2,$3,$4,$5)`, p.HumanID, in.RequestKey, digest, savedCase, savedRevision)
		return e
	})
	if e != nil {
		return Case{}, e
	}
	return s.Get(ctx, savedCase, savedRevision)
}
func (s Service) freezeFindings(ctx context.Context, tx pgx.Tx, caseID, revisionID string, scope analystscope.Snapshot, input []FindingInput, prior []Finding) ([]Finding, []string, error) {
	if input == nil {
		return prior, []string{}, nil
	}
	if len(input) > 100 {
		return nil, nil, apperr.Validation("at most 100 findings")
	}
	p, _, _ := access.From(ctx)
	out := []Finding{}
	deps := []string{}
	seen := map[string]bool{}
	for _, f := range input {
		if len(strings.TrimSpace(f.Title)) == 0 || len(f.Title) > 200 || len(strings.TrimSpace(f.Conclusion)) == 0 || len(f.Conclusion) > 8000 || len(f.Confidence) > 1000 || len(f.Uncertainty) > 4000 || len(f.Evidence) == 0 || len(f.Evidence) > 100 {
			return nil, nil, apperr.Validation("finding requires bounded title, conclusion, uncertainty and exact citations")
		}
		if f.Purpose == "" {
			f.Purpose = "unclassified"
		}
		if f.Purpose != "working" && f.Purpose != "development" && f.Purpose != "acceptance" && f.Purpose != "unclassified" {
			return nil, nil, apperr.Validation("invalid finding purpose")
		}
		var old *Finding
		for i := range prior {
			if prior[i].ID == f.ID {
				old = &prior[i]
			}
		}
		if f.ID == "" {
			f.ID = ids.New("finding")
		} else if old == nil || old.RevisionID != f.ExpectedRevisionID {
			return nil, nil, apperr.Conflict("finding revision changed")
		}
		if seen[f.ID] {
			return nil, nil, apperr.Validation("duplicate finding")
		}
		seen[f.ID] = true
		refs := []Evidence{}
		sources := []string{}
		for _, ref := range f.Evidence {
			actual, more, e := s.evidence(ctx, tx, ref)
			if e != nil {
				return nil, nil, e
			}
			refs = append(refs, actual)
			sources = append(sources, more...)
		}
		if f.Purpose == "working" {
			for _, ref := range refs {
				kind := ref.Kind
				if kind == "analysis" {
					kind = "job"
				}
				if kind != "observation" {
					var purpose string
					if e := tx.QueryRow(ctx, `SELECT emad_content_purpose($1,CASE WHEN $1='assessment' THEN (SELECT assessment_id FROM assessment_revisions WHERE id=$2) ELSE $2 END)`, kind, ref.ID).Scan(&purpose); e != nil {
						return nil, nil, e
					}
					if purpose != "working" {
						return nil, nil, apperr.Conflict("non-working input requires a separately reviewed working successor; reclassification is not independent review")
					}
				}
			}
		}
		f.Evidence = refs
		candidate := Finding{FindingInput: f, RevisionID: ids.New("finding_rev"), ActorHumanID: p.HumanID, ScopeHash: scope.ScopeHash, RequiredSources: sources, RecordedAt: time.Now().UTC()}
		candidate.ExpectedRevisionID = ""
		if old != nil {
			a := old.FindingInput
			a.ExpectedRevisionID = ""
			b := candidate.FindingInput
			ra, _ := jsoncanon.Marshal(a)
			rb, _ := jsoncanon.Marshal(b)
			if string(ra) == string(rb) {
				candidate = *old
			} else {
				candidate.PreviousID = old.RevisionID
			}
		}
		out = append(out, candidate)
		deps = append(deps, candidate.RequiredSources...)
	}
	return out, deps, nil
}
func (s Service) registerRevision(ctx context.Context, tx pgx.Tx, r Revision) error {
	refs := append([]Evidence{}, r.Evidence...)
	for _, f := range r.Findings {
		raw, _ := json.Marshal(f)
		if _, e := tx.Exec(ctx, `INSERT INTO investigation_finding_revisions(id,case_id,finding_id,document)VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, f.RevisionID, r.CaseID, f.ID, raw); e != nil {
			return e
		}
		refs = append(refs, f.Evidence...)
	}
	for _, ref := range refs {
		if _, e := tx.Exec(ctx, `INSERT INTO investigation_dependencies(case_id,revision_id,dependency_kind,dependency_id,source_id,digest)VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, r.CaseID, r.ID, ref.Kind, ref.ID, ref.SourceID, ref.SHA256); e != nil {
			return e
		}
	}
	previous, e := s.latestReview(ctx, r.CaseID)
	if e != nil {
		return e
	}
	if previous != nil && previous.Action == "REVIEW_REQUESTED" && previous.RevisionID != r.ID {
		next := *previous
		next.ID = ids.New("review")
		next.Action = "SUPERSEDED"
		next.Reason = "Authored content changed; request a review of the new revision."
		next.RecordedAt = time.Now().UTC()
		return s.insertReview(ctx, tx, next)
	}
	return nil
}
func (s Service) notices(ctx context.Context, r Revision) []Notice {
	out := []Notice{}
	refs := append([]Evidence{}, r.Evidence...)
	for _, f := range r.Findings {
		refs = append(refs, f.Evidence...)
		if f.Purpose != "working" {
			out = append(out, Notice{"finding", f.RevisionID, "non_working", "Finding purpose is not working content; a working successor needs its own independent review."})
		}
		if f.ScopeHash != r.Scope.ScopeHash {
			out = append(out, Notice{"finding", f.RevisionID, "different_scope", "Finding retains its original area/period; it was not relabeled by this case scope change."})
		}
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		key := ref.Kind + ref.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		e := db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error { _, _, e := s.evidence(ctx, tx, ref); return e })
		if e != nil {
			out = append(out, Notice{ref.Kind, ref.ID, "unavailable", "An exact retained input is unavailable, changed or access-restricted. Restore authorized access or adopt a new supported revision; no input was substituted."})
			continue
		}
		if ref.Kind != "observation" {
			kind := ref.Kind
			if kind == "analysis" {
				kind = "job"
			}
			var purpose string
			e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT emad_content_purpose($1,CASE WHEN $1='assessment' THEN (SELECT assessment_id FROM assessment_revisions WHERE id=$2) ELSE $2 END)`, kind, ref.ID).Scan(&purpose)
			if e != nil || purpose != "working" {
				out = append(out, Notice{ref.Kind, ref.ID, "non_working_input", "Current input purpose is unclassified or non-working; a reviewed working successor is required."})
			}
		}
		var sourceActive bool
		if e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT status='active' FROM sources WHERE id=$1`, ref.SourceID).Scan(&sourceActive); e != nil || !sourceActive {
			out = append(out, Notice{ref.Kind, ref.ID, "source_unavailable", "A required source is unavailable. Historical content is retained; current review and output eligibility need recovery."})
		}

		policyErr := db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
			policies, e := s.policies(ctx, tx, ref)
			if e != nil {
				return e
			}
			for _, p := range policies {
				if _, e := publicingest.ResolvePolicy(ctx, s.Pool, p.SourceID, p.AdmissionID); e != nil {
					return e
				}
			}
			return nil
		})
		if policyErr != nil {
			out = append(out, Notice{ref.Kind, ref.ID, "policy_unavailable", "An original or contributing source policy is disabled, expired or unavailable; current review requires reassessment."})
		}

		if ref.Kind == "artifact" {
			var state string
			var until time.Time
			e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT state,(document->>'retainUntil')::timestamptz FROM artifacts WHERE id=$1`, ref.ID).Scan(&state, &until)
			if e != nil || state != "READY" || !until.After(time.Now()) {
				out = append(out, Notice{ref.Kind, ref.ID, "retention_unavailable", "Retained bytes expired, were deleted, or are not ready. Citation does not extend retention."})
			}
		}
		if ref.Kind == "job" {
			out = append(out, Notice{ref.Kind, ref.ID, "legacy_analysis_input", "This legacy link pins job inputs only. Adopt the exact completed analysis result before review/output."})
		}
		if ref.Kind == "analysis" {
			var hash string
			_ = db.Scoped(s.Pool).QueryRow(ctx, `SELECT COALESCE(input#>>'{scope,scopeHash}','') FROM imagery_jobs WHERE id=$1`, ref.ID).Scan(&hash)
			if hash != r.Scope.ScopeHash {
				out = append(out, Notice{ref.Kind, ref.ID, "different_scope", "Analysis retains its original scope; review the mismatch instead of relabeling it."})
			}
		}
	}
	// Provider notices are computed on open and eligibility checks, so a missed
	// notification cannot conceal a withdrawal/correction of supporting evidence.
	for _, ref := range refs {
		if ref.Kind == "observation" {
			v, e := publicValidity(ctx, s, ref.ID)
			if e != nil || v != "current_provider_report" {
				out = append(out, Notice{ref.Kind, ref.ID, "support_changed", "Current provider support changed; historical content is preserved and reassessment is required."})
			}
		}
	}
	return out
}
func publicValidity(ctx context.Context, s Service, id string) (string, error) {
	v, e := publicevents.CurrentValidity(ctx, s.Pool, id)
	return v.State, e
}
func (s Service) latestReview(ctx context.Context, id string) (*ReviewEvent, error) {
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM investigation_review_events WHERE case_id=$1 ORDER BY sequence DESC LIMIT 1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var v ReviewEvent
	e = json.Unmarshal(raw, &v)
	return &v, e
}
func (s Service) insertReview(ctx context.Context, tx pgx.Tx, v ReviewEvent) error {
	raw, _ := json.Marshal(v)
	_, e := tx.Exec(ctx, `INSERT INTO investigation_review_events(id,case_id,revision_id,document)VALUES($1,$2,$3,$4)`, v.ID, v.CaseID, v.RevisionID, raw)
	return e
}
func (s Service) contributors(ctx context.Context, c Case) ([]string, error) {
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT document FROM investigation_revisions WHERE case_id=$1`, c.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	set := map[string]bool{}
	revisions := []Revision{}
	for rows.Next() {
		var raw []byte
		var r Revision
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &r); e != nil {
			return nil, e
		}
		revisions = append(revisions, r)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	for _, r := range revisions {
		refs := append([]Evidence{}, r.Evidence...)
		for _, f := range r.Findings {
			refs = append(refs, f.Evidence...)
		}
		for _, ref := range refs {
			if ref.Kind == "assessment" {
				var raw []byte
				if e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM assessment_revisions WHERE id=$1`, ref.ID).Scan(&raw); e != nil {
					return nil, e
				}
				var authors struct {
					Author string `json:"authorHumanId"`
					Actor  string `json:"actorHumanId"`
				}
				if e := json.Unmarshal(raw, &authors); e != nil {
					return nil, e
				}
				if authors.Author == "" {
					return nil, apperr.Conflict("legacy contributor identity must be reconciled before independent review")
				}
				set[authors.Author] = true
				if authors.Actor != "" {
					set[authors.Actor] = true
				}
			}
		}
		set[r.ActorHumanID] = true
		for _, f := range r.Findings {
			set[f.ActorHumanID] = true
		}

	}

	out := []string{}
	for human := range set {
		out = append(out, human)
	}
	sort.Strings(out)
	return out, rows.Err()
}
func (s Service) Review(ctx context.Context, id string, in ReviewInput) (ReviewEvent, error) {
	var out ReviewEvent
	permission := "investigations.review"
	if in.Action == "REVIEW_REQUESTED" {
		permission = "investigations.edit"
	}
	if e := require(ctx, permission); e != nil {
		return out, e
	}
	p, r, _ := access.From(ctx)
	if len(strings.TrimSpace(in.Reason)) < 10 || len(in.Reason) > 4000 {
		return out, apperr.Validation("review requires a substantive bounded reason")
	}
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		var head string
		if e := tx.QueryRow(ctx, `SELECT head_revision_id FROM investigations WHERE id=$1 FOR UPDATE`, id).Scan(&head); e != nil {
			return e
		}
		c, e := s.Get(ctx, id, "")
		if e != nil {
			return e
		}
		actorOK, e := s.liveHuman(ctx, tx, p.HumanID, r, permission, id)
		if e != nil {
			return e
		}
		if !actorOK {
			return apperr.Forbidden("live case/source grant required")
		}
		if head != in.ExpectedRevisionID {
			return apperr.Conflict("case revision changed; review the current content")
		}
		if c.Revision.State == "ARCHIVED" {
			return apperr.Conflict("reopen the case before review")
		}
		prior, e := s.latestReview(ctx, id)
		if e != nil {
			return e
		}
		if (prior == nil && in.ExpectedEventID != "") || (prior != nil && prior.ID != in.ExpectedEventID) {
			return apperr.Conflict("review workflow changed")
		}
		authors, e := s.contributors(ctx, c)
		if e != nil {
			return e
		}
		reviewer := in.ReviewerHumanID
		if in.Action == "REVIEW_REQUESTED" {
			eligible, e := s.liveHuman(ctx, tx, reviewer, r, "investigations.review", id)
			if e != nil {
				return e
			}
			if !eligible {
				return apperr.Forbidden("assigned reviewer requires a live registered review grant")
			}
			found := false
			for _, member := range c.Members {
				found = found || member == reviewer
			}
			if !found {
				return apperr.Forbidden("reviewer must already be an authorized case member")
			}
		} else {
			if prior == nil || prior.Action != "REVIEW_REQUESTED" || prior.RevisionID != head || prior.ReviewerHumanID != p.HumanID {
				return apperr.Conflict("only the assigned independent reviewer can decide this pending revision")
			}
			reviewer = prior.ReviewerHumanID
			if in.Action != "APPROVED" && in.Action != "CHANGES_REQUESTED" {
				return apperr.Validation("unsupported review action")
			}
			if revisionDigest(c.Revision) != prior.ContentDigest {
				return apperr.Conflict("reviewed content changed")
			}
		}
		for _, author := range authors {
			if author == reviewer {
				return apperr.Forbidden("reviewer must be a different canonical human from every contributor")
			}
		}
		if in.Action != "CHANGES_REQUESTED" {
			refs := append([]Evidence{}, c.Revision.Evidence...)
			for _, f := range c.Revision.Findings {
				refs = append(refs, f.Evidence...)
			}
			for _, ref := range refs {
				policies, e := s.policies(ctx, tx, ref)
				if e != nil {
					return e
				}
				for _, policy := range policies {
					if _, e := publicingest.LockPolicy(ctx, tx, policy.SourceID, policy.AdmissionID); e != nil {
						return e
					}
				}
			}
		}
		if len(c.Notices) > 0 && in.Action != "CHANGES_REQUESTED" {
			return apperr.Conflict("supporting inputs or finding purpose require reassessment before review")
		}
		out = ReviewEvent{ID: ids.New("review"), CaseID: id, RevisionID: head, Action: in.Action, ReviewerHumanID: reviewer, ActorHumanID: p.HumanID, ContentDigest: revisionDigest(c.Revision), Contributors: authors, Reason: in.Reason, OutputID: in.OutputID, RecordedAt: time.Now().UTC()}
		if prior != nil && in.Action != "REVIEW_REQUESTED" {
			out.OutputID = prior.OutputID
			out.RenderSHA256 = prior.RenderSHA256
			out.OutputContentDigest = prior.OutputContentDigest
		}
		if out.OutputID != "" {
			o, e := s.output(ctx, out.OutputID)
			if e != nil {
				return e
			}
			if o.RevisionID != head || o.State != "READY" {
				return apperr.Conflict("review requires the exact ready rendered output")
			}
			if e = s.outputEligibility(ctx, tx, o, false); e != nil {
				return e
			}
			if out.RenderSHA256 != "" && out.RenderSHA256 != o.RenderSHA256 {
				return apperr.Conflict("rendered content changed")
			}
			for _, author := range []string{o.CreatorHumanID, o.RenderHumanID} {
				if author == reviewer {
					return apperr.Forbidden("output creator and renderer contributor cannot approve their own content")
				}
				found := false
				for _, existing := range out.Contributors {
					found = found || existing == author
				}
				if author != "" && !found {
					out.Contributors = append(out.Contributors, author)
				}
			}
			sort.Strings(out.Contributors)
			out.RenderSHA256 = o.RenderSHA256
			out.OutputContentDigest = o.ContentDigest
		}
		return s.insertReview(ctx, tx, out)
	})
	return out, e
}
func (s Service) mountWorkflow(r chi.Router) {
	r.Get("/investigations/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "investigations.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(s.Pool).Query(r.Context(), `SELECT document FROM investigation_revisions WHERE case_id=$1 ORDER BY version DESC LIMIT 200`, chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, raw)
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "limit": 200, "meaning": "Immutable revisions; use an exact revision identifier for older content."})
	})
	r.Get("/investigations/{id}/reviews", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "investigations.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(s.Pool).Query(r.Context(), `SELECT document FROM investigation_review_events WHERE case_id=$1 ORDER BY sequence DESC LIMIT 200`, chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, raw)
		}
		httpx.JSON(w, 200, map[string]any{"items": items})
	})
	r.Post("/investigations/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		var in ReviewInput
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Review(r.Context(), chi.URLParam(r, "id"), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/investigations/review-inbox", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "investigations.review"); e != nil {
			httpx.Error(w, e)
			return
		}
		p, _, _ := access.From(r.Context())
		rows, e := db.Scoped(s.Pool).Query(r.Context(), `SELECT document FROM (SELECT DISTINCT ON(case_id) case_id,revision_id,document,sequence FROM investigation_review_events ORDER BY case_id,sequence DESC)e JOIN investigations c ON c.id=e.case_id AND c.head_revision_id=e.revision_id WHERE e.document->>'action'='REVIEW_REQUESTED' AND e.document->>'reviewerHumanId'=$1 ORDER BY sequence DESC LIMIT 100`, p.HumanID)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, raw)
		}
		httpx.JSON(w, 200, map[string]any{"items": items})
	})
}

func (s Service) liveHuman(ctx context.Context, tx pgx.Tx, human string, r access.Resource, permission, caseID string) (bool, error) {
	p, _, _ := access.From(ctx)
	principal := ""
	if human == p.HumanID {
		principal = p.ID
	}
	if _, e := tx.Exec(ctx, `RESET ROLE`); e != nil {
		return false, e
	}
	if human == p.HumanID && p.CredentialID != "" {
		var credential string
		e := tx.QueryRow(ctx, `SELECT id FROM principal_credentials WHERE id=$1 AND principal_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, p.CredentialID, p.ID).Scan(&credential)
		if e != nil {
			restore := db.BindTx(ctx, tx)
			if errors.Is(e, pgx.ErrNoRows) {
				return false, restore
			}
			return false, e
		}
	}
	var id string
	e := tx.QueryRow(ctx, `SELECT p.id FROM principals p JOIN principal_grants g ON g.principal_id=p.id JOIN investigations c ON c.id=$5 AND c.scope_id=$6 WHERE p.kind='human' AND p.disabled_at IS NULL AND COALESCE(p.operator_id,p.id)=$1 AND ($7='' OR p.id=$7) AND g.workspace_id=$2 AND g.environment_id=$3 AND g.partition='PUBLIC_EVIDENCE' AND g.run_id='' AND $4=ANY(g.permissions) AND 'investigations.read'=ANY(g.permissions) AND (cardinality(g.source_ids)=0 OR c.required_sources <@ to_jsonb(g.source_ids)) FOR SHARE OF p,g`, human, r.WorkspaceID, r.EnvironmentID, permission, caseID, r.ID(), principal).Scan(&id)
	restore := db.BindTx(ctx, tx)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, restore
	}
	if e != nil {
		return false, e
	}
	return true, restore
}
