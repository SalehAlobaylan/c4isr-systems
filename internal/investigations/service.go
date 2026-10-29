// Package investigations owns authored case revisions, never provider evidence.
package investigations

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Evidence struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	SHA256   string `json:"sha256,omitempty"`
	SourceID string `json:"sourceId,omitempty"`
}
type Input struct {
	RequestKey         string             `json:"requestKey,omitempty"`
	Findings           []FindingInput     `json:"findings,omitempty"`
	ExpectedRevisionID string             `json:"expectedRevisionId"`
	Title              string             `json:"title"`
	Question           string             `json:"question"`
	Notes              string             `json:"notes"`
	State              string             `json:"state"`
	Scope              analystscope.Input `json:"scope"`
	Evidence           []Evidence         `json:"evidence"`
}
type Revision struct {
	Findings     []Finding             `json:"findings"`
	ID           string                `json:"id"`
	CaseID       string                `json:"caseId"`
	Version      int                   `json:"version"`
	PreviousID   string                `json:"previousId,omitempty"`
	Title        string                `json:"title"`
	Question     string                `json:"question"`
	Notes        string                `json:"notes"`
	State        string                `json:"state"`
	Scope        analystscope.Snapshot `json:"scope"`
	Evidence     []Evidence            `json:"evidence"`
	ActorHumanID string                `json:"actorHumanId"`
	RecordedAt   time.Time             `json:"recordedAt"`
}
type Case struct {
	Notices           []Notice                `json:"notices"`
	Workflow          *ReviewEvent            `json:"workflow,omitempty"`
	EvidenceValidity  []publicevents.Validity `json:"evidenceValidity"`
	MembershipVersion int                     `json:"membershipVersion"`
	ID                string                  `json:"id"`
	OwnerHumanID      string                  `json:"ownerHumanId"`
	Members           []string                `json:"members"`
	Revision          Revision                `json:"revision"`
}
type Membership struct {
	ExpectedMembershipVersion int      `json:"expectedMembershipVersion"`
	ExpectedRevisionID        string   `json:"expectedRevisionId"`
	OwnerHumanID              string   `json:"ownerHumanId"`
	Members                   []string `json:"members"`
}
type Service struct {
	Pool  *pgxpool.Pool
	Store artifacts.Store
}

func require(ctx context.Context, permission string) error {
	if e := areas.Require(ctx); e != nil {
		return e
	}
	if e := access.Require(ctx, "investigations.read"); e != nil {
		return e
	}
	if e := access.Require(ctx, permission); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" || p.HumanID == "" {
		return apperr.Forbidden("registered human investigator required")
	}
	return nil
}
func (s Service) Get(ctx context.Context, id, revision string) (Case, error) {
	var c Case
	if e := require(ctx, "investigations.read"); e != nil {
		return c, e
	}
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT c.id,c.owner_human_id,c.members,c.membership_version,r.document FROM investigations c JOIN investigation_revisions r ON r.case_id=c.id AND r.scope_id=c.scope_id AND r.id=CASE WHEN $2='' THEN c.head_revision_id ELSE $2 END WHERE c.id=$1`, id, revision).Scan(&c.ID, &c.OwnerHumanID, &c.Members, &c.MembershipVersion, &raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return c, apperr.NotFound("investigation", "unavailable")
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(raw, &c.Revision)
	if e != nil {
		return c, e
	}
	c.EvidenceValidity = []publicevents.Validity{}
	for _, ref := range c.Revision.Evidence {
		if ref.Kind == "observation" {
			v, e := publicevents.CurrentValidity(ctx, s.Pool, ref.ID)
			if e != nil {
				v = publicevents.Validity{EvidenceID: ref.ID, State: "unavailable", Meaning: "Current support could not be evaluated; original case content is retained."}
			}
			c.EvidenceValidity = append(c.EvidenceValidity, v)
		}
	}
	c.Notices = s.notices(ctx, c.Revision)
	var changed bool
	if err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM investigations c CROSS JOIN LATERAL jsonb_array_elements_text(c.required_sources) dep JOIN sources s ON s.id=dep.value AND s.scope_id=c.scope_id WHERE c.id=$1 AND s.status<>'active')`, id).Scan(&changed); err != nil || changed {
		c.Notices = append(c.Notices, Notice{"case", id, "historical_source_unavailable", "A cumulative source dependency is unavailable, including dependencies retained from earlier prose and citations."})
	}

	c.Workflow, _ = s.latestReview(ctx, id)
	return c, nil
}
func (s Service) List(ctx context.Context, offset int) ([]Case, int, error) {
	out := []Case{}
	if e := require(ctx, "investigations.read"); e != nil {
		return out, 0, e
	}
	if offset < 0 || offset > 10000 {
		return out, 0, apperr.Validation("invalid case page")
	}
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT c.id,c.owner_human_id,c.members,c.membership_version,r.document,count(*) OVER() FROM investigations c JOIN investigation_revisions r ON r.id=c.head_revision_id AND r.scope_id=c.scope_id ORDER BY r.document->>'recordedAt' DESC,c.id LIMIT 50 OFFSET $1`, offset)
	if e != nil {
		return out, 0, e
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var c Case
		var raw []byte
		if e = rows.Scan(&c.ID, &c.OwnerHumanID, &c.Members, &c.MembershipVersion, &raw, &total); e != nil {
			return out, 0, e
		}
		if e = json.Unmarshal(raw, &c.Revision); e != nil {
			return out, 0, e
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Each reference is resolved under domain RLS and its original read permission.
// The digest describes the exact retained document, not a mutable catalog head.
func (s Service) evidence(ctx context.Context, tx pgx.Tx, in Evidence) (Evidence, []string, error) {
	var query, permission string
	switch in.Kind {
	case "observation":
		query = `SELECT source_id,document FROM public_evidence WHERE id=$1`
		permission = "observations.read"
	case "artifact":
		query = `SELECT source_id,document FROM artifacts WHERE id=$1 AND emad_content_visible('artifact',id)`
		permission = "sources.read"
	case "reference":
		query = `SELECT source_id,document FROM reference_datasets WHERE id=$1 AND emad_content_visible('reference',id)`
		permission = "sources.read"
	case "assessment":
		query = `SELECT source_id,document FROM assessment_revisions WHERE id=$1 AND emad_content_visible('assessment',assessment_id)`
		permission = "assessments.read"
	case "analysis":
		query = `SELECT source_id,jsonb_build_object('input',input,'result',result) FROM imagery_jobs WHERE id=$1 AND status='SUCCEEDED' AND emad_content_visible('job',id)`
		permission = "sources.read"
	case "job":
		query = `SELECT source_id,input FROM imagery_jobs WHERE id=$1 AND emad_content_visible('job',id)`
		permission = "assessments.read"
	case "capture":
		query = `SELECT source_id,input FROM imagery_capture_jobs WHERE id=$1 AND emad_content_visible('capture',id)`
		permission = "sources.read"
	default:
		return in, nil, apperr.Validation("unsupported exact case evidence kind")
	}
	if e := access.Require(ctx, permission); e != nil {
		return in, nil, e
	}
	var source string
	var raw []byte
	e := tx.QueryRow(ctx, query, in.ID).Scan(&source, &raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return in, nil, apperr.NotFound("case evidence", "unavailable")
	}
	if e != nil {
		return in, nil, e
	}
	digest := areas.Hash(raw)
	if in.SourceID != "" && in.SourceID != source || in.SHA256 != "" && in.SHA256 != digest {
		return in, nil, apperr.Conflict("case evidence identity changed")
	}
	sources := []string{source}
	policies, e := s.policies(ctx, tx, in)
	if e != nil {
		return in, nil, e
	}
	for _, policy := range policies {
		sources = append(sources, policy.SourceID)
	}
	var nested struct {
		Input struct {
			Scope *analystscope.Snapshot `json:"scope"`
		} `json:"input"`
		DependencyAdmissionIDs []string               `json:"dependencyAdmissionIds"`
		Scope                  *analystscope.Snapshot `json:"scope"`
		Evidence               struct {
			Input struct {
				Scope *analystscope.Snapshot `json:"scope"`
			} `json:"input"`
		} `json:"evidence"`
	}
	_ = json.Unmarshal(raw, &nested)
	if nested.Input.Scope != nil {
		sources = append(sources, nested.Input.Scope.RequiredSources...)
	}
	if len(nested.DependencyAdmissionIDs) > 0 {
		rows, e := tx.Query(ctx, `SELECT source_id FROM source_admissions WHERE id=ANY($1::text[])`, nested.DependencyAdmissionIDs)
		if e != nil {
			return in, nil, e
		}
		for rows.Next() {
			var source string
			if e = rows.Scan(&source); e != nil {
				rows.Close()
				return in, nil, e
			}
			sources = append(sources, source)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return in, nil, e
		}
	}
	if nested.Scope != nil {
		sources = append(sources, nested.Scope.RequiredSources...)
	}
	if nested.Evidence.Input.Scope != nil {
		sources = append(sources, nested.Evidence.Input.Scope.RequiredSources...)
	}
	return Evidence{in.Kind, in.ID, digest, source}, sources, nil
}
func (s Service) save(ctx context.Context, id string, in Input) (Case, error) {
	var out Case
	permission := "investigations.edit"
	if id == "" {
		permission = "investigations.create"
	}
	if e := require(ctx, permission); e != nil {
		return out, e
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Question = strings.TrimSpace(in.Question)
	if in.Title == "" || len(in.Title) > 200 || in.Question == "" || len(in.Question) > 4000 || len(in.Notes) > 64000 || len(in.Evidence) > 200 || in.State != "DRAFT" && in.State != "ACTIVE" && in.State != "ARCHIVED" {
		return out, apperr.Validation("case requires a title, question, bounded notes/evidence and draft, active or archived state")
	}
	p, r, _ := access.From(ctx)
	creating := id == ""
	if creating {
		id = ids.New("case")
	}
	newID := ids.New("case_rev")
	e := db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		var prior Case
		deps := []string{}
		if !creating {
			var rawDeps []byte
			e := tx.QueryRow(ctx, `SELECT required_sources FROM investigations WHERE id=$1 FOR UPDATE`, id).Scan(&rawDeps)
			if errors.Is(e, pgx.ErrNoRows) {
				return apperr.NotFound("investigation", "unavailable")
			}
			if e != nil {
				return e
			}
			_ = json.Unmarshal(rawDeps, &deps)
			prior, e = s.Get(ctx, id, "")
			if e != nil {
				return e
			}
			if prior.Revision.ID != in.ExpectedRevisionID {
				return apperr.Conflict("investigation changed; reload before saving your draft")
			}
			if (in.State == "ARCHIVED" || prior.Revision.State == "ARCHIVED") && prior.OwnerHumanID != p.HumanID {
				return apperr.Forbidden("only the owner may archive or reopen this investigation")
			}
		} else if in.ExpectedRevisionID != "" || in.State == "ARCHIVED" {
			return apperr.Validation("new investigation must start as draft or active")
		}
		scope, e := (analystscope.Service{Pool: s.Pool}).Freeze(ctx, tx, in.Scope)
		if e != nil {
			return e
		}
		deps = append(deps, scope.RequiredSources...)
		refs := []Evidence{}
		seen := map[string]bool{}
		for _, ref := range in.Evidence {
			key := ref.Kind + "/" + ref.ID
			if seen[key] {
				return apperr.Validation("duplicate case evidence")
			}
			seen[key] = true
			actual, more, e := s.evidence(ctx, tx, ref)
			if e != nil {
				return e
			}
			refs = append(refs, actual)
			deps = append(deps, more...)
		}
		findingRevisions, more, e := s.freezeFindings(ctx, tx, id, newID, scope, in.Findings, prior.Revision.Findings)
		if e != nil {
			return e
		}
		deps = append(deps, more...)
		unique := map[string]bool{}
		sources := []string{}
		for _, source := range deps {
			if e := access.RequireSource(ctx, source); e != nil {
				return e
			}
			if !unique[source] {
				unique[source] = true
				sources = append(sources, source)
			}
		}
		sort.Strings(sources)
		rawSources, _ := json.Marshal(sources)
		out = Case{MembershipVersion: 1, ID: id, OwnerHumanID: p.HumanID, Members: []string{p.HumanID}, Revision: Revision{Findings: findingRevisions, ID: newID, CaseID: id, Version: 1, Title: in.Title, Question: in.Question, Notes: in.Notes, State: in.State, Scope: scope, Evidence: refs, ActorHumanID: p.HumanID, RecordedAt: time.Now().UTC()}}
		if creating {
			if _, e = tx.Exec(ctx, `INSERT INTO investigations(id,owner_human_id,members,head_revision_id,required_sources)VALUES($1,$2,$3,$4,$5)`, id, p.HumanID, out.Members, newID, rawSources); e != nil {
				return e
			}
		} else {
			out.OwnerHumanID = prior.OwnerHumanID
			out.Members = prior.Members
			out.MembershipVersion = prior.MembershipVersion
			out.Revision.PreviousID = prior.Revision.ID
			out.Revision.Version = prior.Revision.Version + 1
			tag, e := tx.Exec(ctx, `UPDATE investigations SET head_revision_id=$2,required_sources=$3 WHERE id=$1 AND head_revision_id=$4`, id, newID, rawSources, in.ExpectedRevisionID)
			if e != nil {
				return e
			}
			if tag.RowsAffected() != 1 {
				return apperr.Conflict("investigation changed")
			}
		}
		raw, _ := json.Marshal(out.Revision)
		if _, e = tx.Exec(ctx, `INSERT INTO investigation_revisions(id,case_id,version,document)VALUES($1,$2,$3,$4)`, newID, id, out.Revision.Version, raw); e != nil {
			return e
		}
		if e := s.registerRevision(ctx, tx, out.Revision); e != nil {
			return e
		}
		permission := "investigations.edit"
		if creating {
			permission = "investigations.create"
		}
		allowed, e := s.liveHuman(ctx, tx, p.HumanID, r, permission, out.ID)
		if e != nil {
			return e
		}
		if !allowed {
			return apperr.Forbidden("live case save and all-source grant required")
		}
		return nil

	})
	return out, e
}
func (s Service) Members(ctx context.Context, id string, in Membership) (Case, error) {
	if e := require(ctx, "investigations.manage"); e != nil {
		return Case{}, e
	}
	p, r, _ := access.From(ctx)
	// Only registered, currently authorized human members are eligible. This
	// control-plane lookup grants no source access and returns no account catalog.
	if len(in.Members) < 1 || len(in.Members) > 32 {
		return Case{}, apperr.Validation("select 1–32 registered human members")
	}
	sort.Strings(in.Members)
	owner := false
	for i, human := range in.Members {
		if human == "" || i > 0 && human == in.Members[i-1] {
			return Case{}, apperr.Validation("invalid members")
		}
		owner = owner || human == in.OwnerHumanID
	}
	if !owner {
		return Case{}, apperr.Validation("an eligible owner must remain a member")
	}
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, e := tx.Exec(ctx, `SELECT id FROM investigations WHERE id=$1 FOR UPDATE`, id); e != nil {
			return e
		}
		c, e := s.Get(ctx, id, "")
		if e != nil {
			return e
		}
		if c.OwnerHumanID != p.HumanID {
			return apperr.Forbidden("only the investigation owner may change membership")
		}
		if c.Revision.ID != in.ExpectedRevisionID || c.MembershipVersion != in.ExpectedMembershipVersion {
			return apperr.Conflict("investigation changed")
		}
		var deps []byte
		if e = tx.QueryRow(ctx, `SELECT required_sources FROM investigations WHERE id=$1`, id).Scan(&deps); e != nil {
			return e
		}
		for _, human := range in.Members {
			var eligible bool
			e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals p JOIN principal_grants g ON g.principal_id=p.id WHERE p.kind='human' AND p.disabled_at IS NULL AND COALESCE(p.operator_id,p.id)=$1 AND g.workspace_id=$2 AND g.environment_id=$3 AND g.partition='PUBLIC_EVIDENCE' AND g.run_id='' AND 'investigations.read'=ANY(g.permissions) AND ($5::boolean OR ('investigations.edit'=ANY(g.permissions) AND 'investigations.manage'=ANY(g.permissions))) AND (cardinality(g.source_ids)=0 OR $4::jsonb <@ to_jsonb(g.source_ids)))`, human, r.WorkspaceID, r.EnvironmentID, deps, human != in.OwnerHumanID).Scan(&eligible)
			if e != nil {
				return e
			}
			if !eligible {
				return apperr.Validation("a selected member is not eligible for this investigation")
			}
		}
		// Ownership can be transferred while the previous owner remains a member;
		// the new owner can subsequently remove them through an audited action.
		keepActor := false
		for _, human := range in.Members {
			keepActor = keepActor || human == p.HumanID
		}
		if !keepActor {
			return apperr.Validation("transfer ownership before removing the current owner")
		}
		if _, e = tx.Exec(ctx, `UPDATE investigations SET owner_human_id=$2,members=$3,membership_version=membership_version+1 WHERE id=$1`, id, in.OwnerHumanID, in.Members); e != nil {
			return e
		}
		raw, _ := json.Marshal(map[string]any{"actorHumanId": p.HumanID, "recordedAt": time.Now().UTC(), "previousOwner": c.OwnerHumanID, "previousMembers": c.Members, "membership": in})
		_, e = tx.Exec(ctx, `INSERT INTO investigation_membership_events(id,case_id,document)VALUES($1,$2,$3)`, ids.New("case_members"), id, raw)
		return e
	})
	if e != nil {
		return Case{}, e
	}
	return s.Get(ctx, id, "")
}
func (s Service) Mount(r chi.Router) {
	s.mountWorkflow(r)
	s.mountOutputs(r)
	s.mountDeliveries(r)
	r.Get("/investigations", func(w http.ResponseWriter, r *http.Request) {
		offset := 0
		if r.URL.Query().Get("offset") != "" {
			var e error
			offset, e = strconv.Atoi(r.URL.Query().Get("offset"))
			if e != nil {
				httpx.Error(w, apperr.Validation("invalid case page"))
				return
			}
		}
		v, total, e := s.List(r.Context(), offset)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": v, "total": total, "offset": offset, "hasMore": offset+len(v) < total})
	})
	r.Get("/investigations/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("revision"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("ETag", `"`+v.Revision.ID+`"`)
		httpx.JSON(w, 200, v)
	})
	save := func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Save(r.Context(), chi.URLParam(r, "id"), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("ETag", `"`+v.Revision.ID+`"`)
		httpx.JSON(w, 201, v)
	}
	r.Post("/investigations", save)
	r.Post("/investigations/{id}/revisions", save)
	r.Post("/investigations/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		var in Membership
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Members(r.Context(), chi.URLParam(r, "id"), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}
