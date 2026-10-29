package publicingest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var profileKey = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,100}$`)

type ProfileInput struct {
	ExpectedRevisionID string         `json:"expectedRevisionId"`
	Admission          AdmissionInput `json:"admission"`
}
type ExecutionGrant struct {
	ProfileRevisionID string `json:"profileRevisionId"`
	PrincipalID       string `json:"principalId"`
	Enabled           bool   `json:"enabled"`
}

func registerProfile(ctx context.Context, tx pgx.Tx, a Admission, expected []string) error {
	key := access.RetrievalProfile(ctx)
	var head string
	e := tx.QueryRow(ctx, `SELECT head_revision_id FROM retrieval_profiles WHERE source_id=$1 AND profile_key=$2`, a.SourceID, key).Scan(&head)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if len(expected) > 0 && head != expected[0] {
		return apperr.Conflict("retrieval profile changed; review its current revision")
	}
	if head == "" {
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM retrieval_profiles WHERE source_id=$1`, a.SourceID).Scan(&count); e != nil {
			return e
		}
		if count >= 64 {
			return apperr.Conflict("source profile limit reached; revise an existing reviewed profile")
		}
	}
	revision := "rpr_" + a.ID
	raw, _ := json.Marshal(map[string]any{"id": revision, "key": key, "sourceId": a.SourceID, "admissionId": a.ID, "version": a.Version, "approvedBy": a.ApprovedBy, "recordedAt": a.ApprovedAt, "coverageDescription": a.CoverageDescription, "analystQuestion": a.AnalystQuestion})
	if _, e = tx.Exec(ctx, `INSERT INTO retrieval_profile_revisions(id,source_id,profile_key,admission_id,document)VALUES($1,$2,$3,$4,$5)`, revision, a.SourceID, key, a.ID, raw); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO retrieval_profiles(source_id,profile_key,head_revision_id)VALUES($1,$2,$3) ON CONFLICT(scope_id,source_id,profile_key)DO UPDATE SET head_revision_id=EXCLUDED.head_revision_id`, a.SourceID, key, revision); e != nil {
		return e
	}
	// The source-only admission API explicitly approves its existing default
	// executor. New named profiles never inherit that execution envelope.
	if key == "default" {
		_, e = tx.Exec(ctx, `INSERT INTO retrieval_execution_grants(source_id,profile_revision_id,principal_id,enabled,actor_id)VALUES($1,$2,$3,true,$4)`, a.SourceID, revision, a.ConnectorPrincipalID, a.ApprovedBy)
	}
	return e
}
func checkExecution(ctx context.Context, tx pgx.Tx, run Run) error {
	if e := checkAcquisition(ctx, tx, run); e != nil {
		return e
	}
	if _, e := LockPolicy(ctx, tx, run.SourceID, run.AdmissionID); e != nil {
		return e
	}
	var allowed bool
	e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM retrieval_profile_revisions r JOIN retrieval_profiles p ON p.source_id=r.source_id AND p.scope_id=r.scope_id AND p.profile_key=r.profile_key AND p.head_revision_id=r.id JOIN retrieval_execution_grants g ON g.profile_revision_id=r.id JOIN sources s ON s.id=r.source_id AND s.scope_id=r.scope_id WHERE r.admission_id=$1 AND r.source_id=$2 AND r.profile_key=$3 AND g.principal_id=$4 AND g.enabled AND s.status='active')`, run.AdmissionID, run.SourceID, access.RetrievalProfile(ctx), run.PrincipalID).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		return apperr.Forbidden("profile execution revision is not authorized or source is suspended")
	}
	return nil
}

// LockPolicy keeps the owning profile head and source status stable through a
// processing/publication commit. The lock is shared with rights/grant updates.
func LockPolicy(ctx context.Context, tx pgx.Tx, source, original string) (Admission, error) {
	var head string
	e := tx.QueryRow(ctx, `SELECT p.head_revision_id FROM retrieval_profile_revisions r JOIN retrieval_profiles p ON p.source_id=r.source_id AND p.scope_id=r.scope_id AND p.profile_key=r.profile_key JOIN sources s ON s.id=r.source_id AND s.scope_id=r.scope_id WHERE r.admission_id=$1 AND r.source_id=$2 FOR SHARE OF p,s`, original, source).Scan(&head)
	if e != nil {
		return Admission{}, apperr.NotFound("original profile policy", "unavailable")
	}
	return ResolvePolicy(db.UseTx(ctx, tx), nil, source, original)
}

// ResolvePolicy intersects the original admission's rights with the current
// approved head of that SAME profile. It never borrows another profile's rights.
// Immutable historical admissions themselves remain independently readable.
func ResolvePolicy(ctx context.Context, pool *pgxpool.Pool, source, original string) (Admission, error) {
	var out, current Admission
	var raw, live []byte
	var key, revision string
	if e := access.RequireSource(ctx, source); e != nil {
		return out, e
	}
	e := db.Scoped(pool).QueryRow(ctx, `SELECT old.document,now.document,r.profile_key,h.id FROM source_admissions old JOIN retrieval_profile_revisions r ON r.admission_id=old.id AND r.scope_id=old.scope_id JOIN retrieval_profiles p ON p.source_id=r.source_id AND p.scope_id=r.scope_id AND p.profile_key=r.profile_key JOIN retrieval_profile_revisions h ON h.id=p.head_revision_id JOIN source_admissions now ON now.id=h.admission_id JOIN sources s ON s.id=old.source_id AND s.scope_id=old.scope_id WHERE old.source_id=$1 AND old.id=$2 AND s.status='active'`, source, original).Scan(&raw, &live, &key, &revision)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("original profile policy", "unavailable")
	}
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return out, e
	}
	if e = json.Unmarshal(live, &current); e != nil {
		return out, e
	}
	if !out.Enabled || !current.Enabled || out.Rights.ExpiresAt != nil && !out.Rights.ExpiresAt.After(time.Now()) || current.Rights.ExpiresAt != nil && !current.Rights.ExpiresAt.After(time.Now()) {
		return out, apperr.Conflict("source/profile rights disabled or expired")
	}
	out.Rights.OutputPermissions = IntersectOutputPermissions(out.Rights.OutputPermissions, current.Rights.OutputPermissions)
	out.Rights.RetainRaw = out.Rights.RetainRaw && current.Rights.RetainRaw
	out.Rights.AllowAnalysis = out.Rights.AllowAnalysis && current.Rights.AllowAnalysis
	out.Rights.AllowSimulationExport = out.Rights.AllowSimulationExport && current.Rights.AllowSimulationExport
	if current.Rights.ExpiresAt != nil && (out.Rights.ExpiresAt == nil || current.Rights.ExpiresAt.Before(*out.Rights.ExpiresAt)) {
		out.Rights.ExpiresAt = current.Rights.ExpiresAt
	}
	out.ProfileKey = key
	out.ProfileRevisionID = revision
	out.CurrentAdmissionID = current.ID
	return out, nil
}
func RequireProfileProcessing(ctx context.Context, pool *pgxpool.Pool, a Admission) error {
	p, _, _ := access.From(ctx)
	if p.Kind != "service" || a.ProfileKey == "default" {
		return nil
	}
	var allowed bool
	e := db.Scoped(pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM retrieval_execution_grants WHERE profile_revision_id=$1 AND principal_id=$2 AND enabled)`, a.ProfileRevisionID, p.ID).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		return apperr.Forbidden("explicit execution grant required for this profile revision")
	}
	return nil
}
func (s *Service) AdmitProfile(ctx context.Context, source, key string, in ProfileInput) (Admission, error) {
	if !profileKey.MatchString(key) || len(strings.TrimSpace(in.Admission.AnalystQuestion)) < 20 {
		return Admission{}, apperr.Validation("profile requires a stable key and reviewed analytical purpose")
	}
	return s.Admit(access.WithRetrievalProfile(ctx, key), source, in.Admission, in.ExpectedRevisionID)
}
func (s *Service) grantProfile(ctx context.Context, source, key string, in ExecutionGrant) error {
	if e := requirePublic(ctx, source, "sources.manage"); e != nil {
		return e
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "human" || !profileKey.MatchString(key) {
		return apperr.Forbidden("human profile approval required")
	}
	// Validate a live scoped service identity without changing its source grants.
	var eligible bool
	e := s.repo.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals p JOIN principal_grants g ON g.principal_id=p.id WHERE p.id=$1 AND p.kind='service' AND p.disabled_at IS NULL AND g.workspace_id=$2 AND g.environment_id=$3 AND g.partition='PUBLIC_EVIDENCE' AND g.run_id='' AND g.permissions&&ARRAY['connectors.execute','imagery.jobs.execute','imagery.capture.execute'] AND (cardinality(g.source_ids)=0 OR $4=ANY(g.source_ids)))`, in.PrincipalID, r.WorkspaceID, r.EnvironmentID, source).Scan(&eligible)
	if e != nil {
		return e
	}
	if !eligible {
		return apperr.Validation("profile executor is not eligible under its live resource/source grant")
	}
	return db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var head string
		if e := tx.QueryRow(ctx, `SELECT head_revision_id FROM retrieval_profiles WHERE source_id=$1 AND profile_key=$2 FOR UPDATE`, source, key).Scan(&head); e != nil {
			return apperr.NotFound("retrieval profile", "unavailable")
		}
		if head != in.ProfileRevisionID {
			return apperr.Conflict("execution approval must name the current exact profile revision")
		}
		_, e := tx.Exec(ctx, `INSERT INTO retrieval_execution_grants(source_id,profile_revision_id,principal_id,enabled,actor_id)VALUES($1,$2,$3,$4,$5)ON CONFLICT(profile_revision_id,principal_id)DO UPDATE SET enabled=EXCLUDED.enabled,actor_id=EXCLUDED.actor_id,recorded_at=clock_timestamp()`, source, head, in.PrincipalID, in.Enabled, p.ID)
		if e != nil {
			return e
		}
		raw, _ := json.Marshal(map[string]any{"profileKey": key, "grant": in, "actor": p.ID, "recordedAt": time.Now().UTC()})
		_, e = tx.Exec(ctx, `INSERT INTO retrieval_execution_events(id,source_id,document)VALUES($1,$2,$3)`, ids.New("profile_grant"), source, raw)
		return e
	})
}
func (h *Handler) mountProfiles(r chi.Router) {
	r.Get("/sources/{id}/profiles", func(w http.ResponseWriter, r *http.Request) {
		source := chi.URLParam(r, "id")
		if e := requirePublic(r.Context(), source, "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(h.service.repo.pool).Query(r.Context(), `SELECT r.document||jsonb_build_object('admission',a.document) FROM retrieval_profiles p JOIN retrieval_profile_revisions r ON r.id=p.head_revision_id JOIN source_admissions a ON a.id=r.admission_id WHERE p.source_id=$1 ORDER BY p.profile_key LIMIT 64`, source)
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
		if e = rows.Err(); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "limit": 64})
	})
	r.Post("/sources/{id}/profiles/{key}", func(w http.ResponseWriter, r *http.Request) {
		var in ProfileInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := h.service.AdmitProfile(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "key"), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Post("/sources/{id}/profiles/{key}/execution", func(w http.ResponseWriter, r *http.Request) {
		var in ExecutionGrant
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		if e := h.service.grantProfile(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "key"), in); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"profileRevisionId": in.ProfileRevisionID, "enabled": in.Enabled})
	})
	r.Post("/sources/{id}/profiles/{key}/connector-runs", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		if !profileKey.MatchString(key) {
			httpx.Error(w, apperr.Validation("invalid profile key"))
			return
		}
		var window *Window
		if r.ContentLength != 0 {
			var in Window
			if e := httpx.DecodeJSON(r, &in); e != nil {
				httpx.Error(w, e)
				return
			}
			window = &in
		}
		v, e := h.service.ExecuteWindow(access.WithRetrievalProfile(r.Context(), key), chi.URLParam(r, "id"), window)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
}

// Unknown output permission remains unknown. Legacy simulation export retains
// its original separate meaning and never grants these output purposes.
func IntersectOutputPermissions(a, b *OutputPermissions) *OutputPermissions {
	if a == nil || b == nil {
		return nil
	}
	both := func(x, y *bool) *bool {
		if x == nil || y == nil {
			return nil
		}
		v := *x && *y
		return &v
	}
	return &OutputPermissions{InternalAnalysis: both(a.InternalAnalysis, b.InternalAnalysis), Citation: both(a.Citation, b.Citation), IncludedImagery: both(a.IncludedImagery, b.IncludedImagery), RawDistribution: both(a.RawDistribution, b.RawDistribution), GeographicEnvironment: both(a.GeographicEnvironment, b.GeographicEnvironment), MetadataRetention: both(a.MetadataRetention, b.MetadataRetention)}
}
