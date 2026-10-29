// Package acquisition owns durable analyst intent. Connector execution remains
// in publicingest and uses only trusted server-side provider configuration.
package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"time"
)

type Input struct {
	RequestKey        string             `json:"requestKey"`
	SourceID          string             `json:"sourceId"`
	ProfileKey        string             `json:"profileKey"`
	ProfileRevisionID string             `json:"profileRevisionId"`
	InvestigationID   string             `json:"investigationId,omitempty"`
	Purpose           string             `json:"purpose"`
	Scope             analystscope.Input `json:"scope"`
}
type Request struct {
	ID            string                         `json:"id"`
	Input         Input                          `json:"input"`
	Scope         analystscope.Snapshot          `json:"scope"`
	Kind          string                         `json:"kind"`
	Status        string                         `json:"status"`
	Reason        string                         `json:"reason"`
	RecoveryOwner string                         `json:"recoveryOwner"`
	RequesterID   string                         `json:"requesterId"`
	CreatedAt     time.Time                      `json:"createdAt"`
	Parts         []publicingest.AcquisitionPart `json:"parts"`
}
type Service struct {
	Pool      *pgxpool.Pool
	Ingestion *publicingest.Service
}

func require(ctx context.Context, permission string) error {
	if e := areas.Require(ctx); e != nil {
		return e
	}
	return access.Require(ctx, permission)
}
func (s Service) Create(ctx context.Context, in Input) (Request, error) {
	var out Request
	if e := require(ctx, "acquisitions.request"); e != nil {
		return out, e
	}
	p, resource, _ := access.From(ctx)
	if p.Kind != "human" || p.HumanID == "" {
		return out, apperr.Forbidden("acquisition intent requires a human analyst")
	}
	if e := access.RequireSource(ctx, in.SourceID); e != nil {
		return out, e
	}
	if len(in.RequestKey) < 8 || len(in.RequestKey) > 160 || strings.ContainsAny(in.RequestKey, "\r\n") || (in.Purpose != "discovery" && in.Purpose != "analysis") {
		return out, apperr.Validation("provide a stable request key and discovery/analysis purpose")
	}
	if in.ProfileKey == "" {
		in.ProfileKey = "default"
	}
	canonical, e := jsoncanon.Marshal(in)
	if e != nil {
		return out, e
	}
	hash := areas.Hash(canonical)
	e = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/intent/"+p.HumanID+"/"+in.RequestKey); e != nil {
			return e
		}
		// Lookup precedes rolling-time evaluation. Lost-response retry returns the
		// originally committed scope even when the map's clock has advanced.
		var prior []byte
		var priorHash string
		e := tx.QueryRow(ctx, `SELECT document,input_hash FROM acquisition_requests WHERE requester_id=$1 AND request_key=$2`, p.HumanID, in.RequestKey).Scan(&prior, &priorHash)
		if e == nil {
			if hash != priorHash {
				return apperr.Conflict("request key reused with different acquisition input")
			}
			return json.Unmarshal(prior, &out)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		policy, e := s.Ingestion.RequireAcquisitionPolicy(ctx, in.SourceID, in.ProfileKey, in.ProfileRevisionID)
		if e != nil {
			return e
		}
		frozen, e := (analystscope.Service{Pool: s.Pool}).Freeze(ctx, tx, in.Scope)
		if e != nil {
			return e
		}
		if in.InvestigationID != "" {
			var visible bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM investigations WHERE id=$1)`, in.InvestigationID).Scan(&visible)
			if e != nil {
				return e
			}
			if !visible {
				return apperr.NotFound("investigation", "unavailable")
			}
		}
		out = Request{ID: ids.New("acq"), Input: in, Scope: frozen, Kind: "ACQUISITION", Status: "QUEUED", RecoveryOwner: "connector operator", RequesterID: p.HumanID, CreatedAt: time.Now().UTC(), Parts: []publicingest.AcquisitionPart{}}
		out.Reason = "Approved profile envelope; queued work uses this frozen scope and exact revision."
		review := func(reason string) {
			out.Kind = "COVERAGE_REVIEW"
			out.Status = "NEEDS_REVIEW"
			out.Reason = reason
			out.RecoveryOwner = "source administrator"
			out.Parts = []publicingest.AcquisitionPart{}
		}
		if policy.Gate != "ready" || !policy.ExecutionGranted {
			review("Provider prerequisite, enabled rights and exact-revision execution grant require source-administrator review.")
		}
		if in.Purpose == "analysis" && !policy.Admission.Rights.AllowAnalysis {
			review("Analysis is outside this profile's approved rights.")
		}
		interval := frozen.Temporal.Interval

		if !policy.Capability.Historical {
			review("This adapter captures current context on its approved polling schedule and cannot replay a frozen acquisition window. Historical acquisition requires another approved source.")
		}
		if frozen.Temporal.RevisionView != "latest" {
			review("Acquisition cannot reconstruct the provider's known-at/effective-at history; inspect retained evidence separately.")
		}
		if len(frozen.Area.Components) == 0 {
			review("Select a bounded area before acquisition.")
		}
		a := policy.Admission.Policy
		for _, component := range frozen.Area.Components {
			b := publicingest.Bounds{West: component.Bounds[0], South: component.Bounds[1], East: component.Bounds[2], North: component.Bounds[3]}
			if b.Validate() != nil || b.West < a.AOI.West || b.South < a.AOI.South || b.East > a.AOI.East || b.North > a.AOI.North {
				review("The query footprint exceeds this profile's approved area; request coverage review or choose a smaller area.")
				break
			}
			step := time.Duration(a.LookbackSeconds) * time.Second
			for from := interval.From; from.Before(interval.To); {
				to := from.Add(step)
				if to.After(interval.To) {
					to = interval.To
				}
				if len(out.Parts) >= 64 {
					review("More than 64 bounded acquisition parts are required; choose a smaller area or period.")
					break
				}
				out.Parts = append(out.Parts, publicingest.AcquisitionPart{ID: ids.New("acp"), RequestID: out.ID, SourceID: in.SourceID, ProfileKey: in.ProfileKey, ProfileRevisionID: in.ProfileRevisionID, AOI: b, From: from, To: to, Status: "QUEUED", RecoveryOwner: "connector operator"})
				from = to
			}
			if out.Kind == "COVERAGE_REVIEW" {
				break
			}
		}
		if out.Kind == "COVERAGE_REVIEW" {
			out.Parts = []publicingest.AcquisitionPart{}
		}
		var active int
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/acquisition-active"); e != nil {
			return e
		}
		// Control-plane count includes hidden-source work but discloses no count.
		if _, e = tx.Exec(ctx, `RESET ROLE`); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `SELECT count(*) FROM acquisition_requests WHERE scope_id=$1 AND status IN('QUEUED','RUNNING','WAITING','NEEDS_REVIEW')`, resource.ID()).Scan(&active)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `SET LOCAL ROLE emad_data`); e != nil {
			return e
		}
		if active >= 32 {
			return apperr.Conflict("active request capacity exhausted; finish or cancel existing work, preserving history")
		}
		raw, _ := json.Marshal(out)
		deps, _ := json.Marshal(frozen.RequiredSources)
		_, e = tx.Exec(ctx, `INSERT INTO acquisition_requests(id,source_id,requester_id,request_key,input_hash,profile_key,profile_revision_id,scope_snapshot_id,required_sources,kind,status,document)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, out.ID, in.SourceID, p.HumanID, in.RequestKey, hash, in.ProfileKey, in.ProfileRevisionID, frozen.ID, deps, out.Kind, out.Status, raw)
		if e != nil {
			return e
		}
		for ordinal, part := range out.Parts {
			raw, _ = json.Marshal(part)
			if _, e = tx.Exec(ctx, `INSERT INTO acquisition_parts(id,source_id,request_id,ordinal,document)VALUES($1,$2,$3,$4,$5)`, part.ID, in.SourceID, out.ID, ordinal, raw); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return Request{}, e
	}
	return s.Get(ctx, out.ID)
}
func (s Service) Get(ctx context.Context, id string) (Request, error) {
	var out Request
	if e := require(ctx, "acquisitions.read"); e != nil {
		return out, e
	}
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document||jsonb_build_object('status',status) FROM acquisition_requests WHERE id=$1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("acquisition request", "unavailable")
	}
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return out, e
	}
	out.Parts = []publicingest.AcquisitionPart{}
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT document||jsonb_build_object('status',status,'attempts',attempts,'nextAttemptAt',next_attempt_at,'runId',run_id) FROM acquisition_parts WHERE request_id=$1 ORDER BY ordinal`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var part publicingest.AcquisitionPart
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if e = json.Unmarshal(raw, &part); e != nil {
			return out, e
		}
		out.Parts = append(out.Parts, part)
	}
	return out, rows.Err()
}
func (s Service) List(ctx context.Context, source string) ([]Request, error) {
	if e := require(ctx, "acquisitions.read"); e != nil {
		return nil, e
	}
	if source != "" {
		if e := access.RequireSource(ctx, source); e != nil {
			return nil, e
		}
	}
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT document||jsonb_build_object('status',status) FROM acquisition_requests WHERE ($1='' OR source_id=$1) ORDER BY created_at DESC,id LIMIT 100`, source)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		var raw []byte
		var v Request
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		v.Parts = []publicingest.AcquisitionPart{}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s Service) Cancel(ctx context.Context, id string) error {
	if e := require(ctx, "acquisitions.request"); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" || p.HumanID == "" {
		return apperr.Forbidden("human cancellation required")
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var owner, source, key, status string
		e := tx.QueryRow(ctx, `SELECT requester_id,source_id,profile_key,status FROM acquisition_requests WHERE id=$1`, id).Scan(&owner, &source, &key, &status)
		if errors.Is(e, pgx.ErrNoRows) {
			return apperr.NotFound("acquisition request", "unavailable")
		}
		if e != nil {
			return e
		}
		if owner != p.HumanID && !access.Allows(ctx, "sources.manage") {
			return apperr.Forbidden("request owner or source administrator required")
		}
		if status == "SUCCEEDED" || status == "CANCELLED" {
			return nil
		}
		// Change the profile-bound setting for the fenced state mutation. No
		// retained record, occurrence, receipt or uncertain charge is removed.
		if _, e = tx.Exec(ctx, `SELECT set_config('emad.profile_key',$1,true)`, key); e != nil {
			return e
		}
		// All execution commits lock profile state before checking intent. Use
		// the same order so cancellation serializes with the post-I/O fence.
		if _, e = tx.Exec(ctx, `SELECT source_id FROM retrieval_profile_state WHERE source_id=$1 AND profile_key=$2 FOR UPDATE`, source, key); e != nil {
			return e
		}
		if e = tx.QueryRow(ctx, `SELECT status FROM acquisition_requests WHERE id=$1 FOR UPDATE`, id).Scan(&status); e != nil {
			return e
		}
		if status == "SUCCEEDED" || status == "CANCELLED" {
			return nil
		}
		if _, e = tx.Exec(ctx, `UPDATE connector_runs SET document=document||jsonb_build_object('status','INTERRUPTED','failureCode','request_cancelled','finishedAt',clock_timestamp()) WHERE id IN(SELECT run_id FROM acquisition_parts WHERE request_id=$1) AND document->>'status'='RUNNING'`, id); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE retrieval_profile_state SET lease_id=NULL,lease_until=NULL,checkpoint='',query_from=NULL,query_to=NULL,query_request_id=NULL,query_aoi=NULL WHERE source_id=$1 AND profile_key=$2 AND query_request_id=$3`, source, key, id); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE acquisition_parts SET status='CANCELLED',lease_id=NULL,lease_until=NULL WHERE request_id=$1 AND status<>'SUCCEEDED'`, id); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE acquisition_requests SET status='CANCELLED',updated_at=clock_timestamp() WHERE id=$1`, id)
		return e
	})
}
func (s Service) Mount(r chi.Router) {
	r.Post("/acquisitions", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Create(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/acquisitions", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.List(r.Context(), r.URL.Query().Get("source_id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": v, "returned": len(v), "limit": 100, "historyMeaning": "latest 100 authorized requests; retained history is preserved"})
	})
	r.Get("/acquisitions/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/acquisitions/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if e := s.Cancel(r.Context(), chi.URLParam(r, "id")); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}
