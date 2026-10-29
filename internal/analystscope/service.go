// Package analystscope combines exact areas and evaluated periods. Read manifests
// are ephemeral; frozen scopes are immutable and suitable for jobs and cases.
package analystscope

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Input struct {
	ReadSnapshotID string              `json:"readSnapshotId,omitempty"`
	Area           areas.Selection     `json:"area"`
	Temporal       queryscope.Temporal `json:"temporal"`
}
type Snapshot struct {
	RequiredSources       []string                      `json:"requiredSources"`
	ReadManifest          json.RawMessage               `json:"readManifest,omitempty"`
	Schema                string                        `json:"schema"`
	ID                    string                        `json:"id"`
	ScopeHash             string                        `json:"scopeHash"`
	Area                  areas.Revision                `json:"area"`
	Temporal              queryscope.Evaluated          `json:"temporal"`
	AuthorizationRevision string                        `json:"authorizationRevision"`
	Capabilities          map[string]queryscope.Support `json:"capabilities"`
}
type Service struct{ Pool *pgxpool.Pool }

var Families = []string{"earthquakes", "thermal", "weather", "bulletins", "aircraft", "vessels", "imagery", "findings", "coverage", "events", "scenes", "captures"}

func (s Service) Resolve(ctx context.Context, in Input) (Snapshot, error) {
	return s.resolve(ctx, in, time.Now().UTC())
}
func (s Service) resolve(ctx context.Context, in Input, now time.Time) (Snapshot, error) {
	var out Snapshot
	a, e := (areas.Service{Pool: s.Pool}).Resolve(ctx, in.Area)
	if e != nil {
		return out, e
	}
	t, e := queryscope.Evaluate(in.Temporal, now)
	if e != nil {
		return out, e
	}
	out = Snapshot{Schema: "emad.scope-snapshot.v1", Area: a, Temporal: t, AuthorizationRevision: access.AccessRevision(ctx), Capabilities: map[string]queryscope.Support{}}
	out.RequiredSources = append([]string{}, a.RequiredSources...)
	for _, family := range Families {
		out.Capabilities[family] = queryscope.Capability(family, t)
	}
	// The content fingerprint excludes access/cache identity and wall-clock query
	// evaluation for a fixed interval. Access is still independently rechecked.
	raw, _ := json.Marshal(struct {
		Area, Normalization, Geometry string
		Temporal                      queryscope.Temporal
	}{a.RevisionID, a.NormalizationVersion, a.GeometrySHA256, t.Temporal})
	out.ScopeHash = areas.Hash(raw)
	out.ID = "scope_" + out.ScopeHash
	return out, nil
}
func (s Service) Freeze(ctx context.Context, tx pgx.Tx, in Input) (Snapshot, error) {
	ctx = db.UseTx(ctx, tx)
	out, e := s.Resolve(ctx, in)
	if e != nil {
		return out, e
	}
	a, e := (areas.Service{Pool: s.Pool}).Persist(ctx, tx, in.Area)
	if e != nil {
		return out, e
	}
	out.Area = a
	if in.ReadSnapshotID != "" {
		var raw []byte
		var revision string
		var active bool
		e = tx.QueryRow(ctx, `SELECT document,authorization_revision,expires_at>clock_timestamp() FROM analyst_read_snapshots WHERE id=$1`, in.ReadSnapshotID).Scan(&raw, &revision, &active)
		if errors.Is(e, pgx.ErrNoRows) {
			return out, apperr.Conflict("read snapshot unavailable; restart the view before citing it")
		}
		if e != nil {
			return out, e
		}
		if revision != access.AccessRevision(ctx) || !active {
			return out, apperr.Conflict("read snapshot expired or access changed; restart the view")
		}
		var read ReadSnapshot
		if e = json.Unmarshal(raw, &read); e != nil {
			return out, e
		}
		if read.Scope.ScopeHash != out.ScopeHash {
			return out, apperr.Conflict("citation scope differs from the selected read")
		}
		out.ReadManifest = raw
		out.ID = "scope_" + areas.Hash([]byte(out.ScopeHash+"/"+read.QueryFingerprint+"/"+read.ID))
		for _, receipt := range read.CoverageReceipts {
			var v struct {
				SourceID string `json:"sourceId"`
			}
			if e = json.Unmarshal(receipt, &v); e != nil {
				return out, e
			}
			found := false
			for _, source := range a.RequiredSources {
				found = found || source == v.SourceID
			}
			if !found {
				a.RequiredSources = append(a.RequiredSources, v.SourceID)
			}
		}
	}
	out.RequiredSources = append([]string{}, a.RequiredSources...)
	raw, _ := json.Marshal(out)
	sources, _ := json.Marshal(out.RequiredSources)
	_, e = tx.Exec(ctx, `INSERT INTO analyst_scope_snapshots(id,area_revision_id,document,required_sources)VALUES($1,$2,$3,$4) ON CONFLICT(id,scope_id)DO NOTHING`, out.ID, a.RevisionID, raw, sources)
	if e != nil {
		return out, e
	}
	return s.Get(ctx, out.ID)
}
func (s Service) Get(ctx context.Context, id string) (Snapshot, error) {
	var out Snapshot
	if e := areas.Require(ctx); e != nil {
		return out, e
	}
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM analyst_scope_snapshots WHERE id=$1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("scope snapshot", "unavailable")
	}
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(raw, &out)
	out.AuthorizationRevision = access.AccessRevision(ctx)
	return out, e
}
func FromSnapshot(s Snapshot) Input {
	return Input{Area: areas.SelectionFor(s.Area), Temporal: s.Temporal.Temporal}
}
func (s Service) Mount(r chi.Router) {
	r.Post("/analyst-scope/resolve", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Resolve(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/analyst-scope/freeze", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		var v Snapshot
		e := db.WithTx(r.Context(), s.Pool, func(tx pgx.Tx) error { var e error; v, e = s.Freeze(r.Context(), tx, in); return e })
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/analyst-scope/snapshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	s.mountReads(r)
}
