package artifacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Input struct {
	RequestKey  string    `json:"requestKey"`
	SourceID    string    `json:"sourceId"`
	SHA256      string    `json:"sha256"`
	Bytes       int64     `json:"bytes"`
	MediaType   string    `json:"mediaType"`
	Kind        string    `json:"kind"`
	RetainUntil time.Time `json:"retainUntil"`
	Provenance  string    `json:"provenance"`
}
type Artifact struct {
	Input
	ID          string              `json:"id"`
	AdmissionID string              `json:"admissionId"`
	Rights      publicingest.Rights `json:"rights"`
	CreatedBy   string              `json:"createdBy"`
	CreatedAt   time.Time           `json:"createdAt"`
	State       string              `json:"state"`
	ObjectKey   string              `json:"-"`
}
type Service struct {
	Pool  *pgxpool.Pool
	Store Store
}

func NewService(p *pgxpool.Pool, s Store) *Service {
	if s == nil {
		s = Unavailable{}
	}
	return &Service{p, s}
}
func Public(ctx context.Context, permission string) error {
	if e := access.Require(ctx, permission); e != nil {
		return e
	}
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("public imagery workspace required")
	}
	return nil
}
func Write(ctx context.Context) error {
	p, _, _ := access.From(ctx)
	if p.Kind == "service" && access.Allows(ctx, "imagery.jobs.execute") {
		return Public(ctx, "imagery.jobs.execute")
	}
	return Public(ctx, "sources.manage")
}
func (s *Service) Admission(ctx context.Context, source string) (publicingest.Admission, error) {
	var a publicingest.Admission
	var raw []byte
	if err := access.RequireSource(ctx, source); err != nil {
		return a, err
	}
	err := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT a.document FROM source_connector_state c JOIN source_admissions a ON a.id=c.admission_id AND a.scope_id=c.scope_id JOIN sources s ON s.id=c.source_id AND s.scope_id=c.scope_id WHERE c.source_id=$1 AND s.status='active'`, source).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apperr.NotFound("active source admission", "unavailable")
	}
	if err != nil {
		return a, err
	}
	if err = json.Unmarshal(raw, &a); err != nil {
		return a, err
	}
	if !a.Enabled || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now())) {
		return a, apperr.Conflict("source rights disabled or expired")
	}
	return a, nil
}

var hashText = regexp.MustCompile(`^[0-9a-f]{64}$`)
var requestKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

func (s *Service) Reserve(ctx context.Context, in Input) (Artifact, error) {
	var a Artifact
	if e := Write(ctx); e != nil {
		return a, e
	}
	if !hashText.MatchString(in.SHA256) || !requestKey.MatchString(in.RequestKey) || in.Bytes < 1 || in.Bytes > MaxBytes || len(in.Provenance) < 1 || len(in.Provenance) > 2000 || !in.RetainUntil.After(time.Now()) || in.RetainUntil.After(time.Now().Add(365*24*time.Hour)) {
		return a, apperr.Validation("invalid artifact hash, size, request key, provenance or retention")
	}
	switch in.MediaType {
	case "application/json", "image/tiff", "image/jp2", "image/png":
	default:
		return a, apperr.Validation("unsupported retained artifact type")
	}
	switch in.Kind {
	case "imagery_subset", "analysis_result", "source_raster", "imagery_preview", "imagery_capture", "imagery_window":
	default:
		return a, apperr.Validation("unsupported artifact kind")
	}
	admission, e := s.Admission(ctx, in.SourceID)
	if e != nil {
		return a, e
	}
	if !admission.Rights.RetainRaw || (admission.Rights.ExpiresAt != nil && in.RetainUntil.After(*admission.Rights.ExpiresAt)) {
		return a, apperr.Forbidden("artifact retention is not permitted by admitted rights")
	}
	in.RetainUntil = in.RetainUntil.UTC().Truncate(time.Microsecond)
	p, r, _ := access.From(ctx)
	a = Artifact{Input: in, ID: "art_" + Hash([]byte(r.ID()+"|"+in.SourceID+"|"+in.RequestKey)), AdmissionID: admission.ID, Rights: admission.Rights, CreatedBy: p.ID, CreatedAt: time.Now().UTC(), State: "PENDING"}
	a.ObjectKey = fmt.Sprintf("emad/v1/%s/%s/%s/shared/%s/%s/%s", Hash([]byte(r.WorkspaceID))[:32], Hash([]byte(r.EnvironmentID))[:32], r.Partition, Hash([]byte(in.SourceID)), a.ID, a.SHA256)
	raw, _ := json.Marshal(a)
	e = platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":"+in.SourceID+":artifact-quota"); e != nil {
			return e
		}
		var count int
		var total int64
		if e := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(size_bytes),0) FROM artifacts WHERE source_id=$1 AND state<>'DELETED'`, in.SourceID).Scan(&count, &total); e != nil {
			return e
		}
		var prior []byte
		e := tx.QueryRow(ctx, `SELECT document FROM artifacts WHERE id=$1`, a.ID).Scan(&prior)
		if e == nil {
			var old Artifact
			_ = json.Unmarshal(prior, &old)
			b, _ := json.Marshal(old.Input)
			c, _ := json.Marshal(in)
			if string(b) != string(c) {
				return apperr.Conflict("artifact request key reused with different metadata")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if count >= 1000 || total+in.Bytes > 512*1024*1024 {
			return apperr.Conflict("source artifact quota exhausted")
		}
		_, e = tx.Exec(ctx, `INSERT INTO artifacts(id,source_id,admission_id,sha256,size_bytes,media_type,kind,object_key,document,state,retain_until)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'PENDING',$10)`, a.ID, in.SourceID, a.AdmissionID, a.SHA256, a.Bytes, a.MediaType, a.Kind, a.ObjectKey, raw, a.RetainUntil)
		return e
	})
	if e != nil {
		return Artifact{}, e
	}
	return s.Get(ctx, a.ID)
}
func (s *Service) Get(ctx context.Context, id string) (Artifact, error) {
	var a Artifact
	if e := Public(ctx, "sources.read"); e != nil {
		return a, e
	}
	var raw []byte
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT document,state,object_key FROM artifacts WHERE id=$1`, id).Scan(&raw, &a.State, &a.ObjectKey)
	if errors.Is(e, pgx.ErrNoRows) {
		return a, apperr.NotFound("artifact", "unavailable")
	}
	if e != nil {
		return a, e
	}
	state, key := a.State, a.ObjectKey
	e = json.Unmarshal(raw, &a)
	a.State = state
	a.ObjectKey = key
	return a, e
}
func (s *Service) usable(ctx context.Context, a Artifact) error {
	if a.State != "READY" || !a.RetainUntil.After(time.Now()) || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now())) {
		return apperr.Conflict("artifact is incomplete, expired or deleted")
	}
	_, e := s.Admission(ctx, a.SourceID)
	return e
}
func (s *Service) Upload(ctx context.Context, id string, b []byte) (Artifact, error) {
	if e := Write(ctx); e != nil {
		return Artifact{}, e
	}
	a, e := s.Get(ctx, id)
	if e != nil {
		return a, e
	}
	if a.State != "PENDING" && a.State != "READY" {
		return a, apperr.Conflict("artifact cannot be uploaded")
	}
	if !a.RetainUntil.After(time.Now()) || int64(len(b)) != a.Bytes || Hash(b) != a.SHA256 {
		return a, apperr.Validation("artifact byte count/hash/retention mismatch")
	}
	if _, e = s.Admission(ctx, a.SourceID); e != nil {
		return a, e
	}
	if e = s.Store.Put(ctx, a.ObjectKey, b, a.MediaType); e != nil {
		return a, apperr.Conflict("artifact storage unavailable; immutable upload may be retried")
	}
	// Recheck rights after external I/O; a source suspension cannot complete upload.
	if _, e = s.Admission(ctx, a.SourceID); e != nil {
		return a, e
	}
	if !a.RetainUntil.After(time.Now()) || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now())) {
		return a, apperr.Conflict("artifact retention expired during upload")
	}
	tag, e := platformdb.Scoped(s.Pool).Exec(ctx, `UPDATE artifacts SET state='READY' WHERE id=$1 AND state IN ('PENDING','READY')`, id)
	if e != nil {
		return a, e
	}
	if tag.RowsAffected() != 1 {
		return a, apperr.Conflict("artifact state changed during upload")
	}
	return s.Get(ctx, id)
}
func (s *Service) Read(ctx context.Context, id string) (Artifact, []byte, error) {
	a, e := s.Get(ctx, id)
	if e != nil {
		return a, nil, e
	}
	if e = s.usable(ctx, a); e != nil {
		return a, nil, e
	}
	b, e := s.Store.Get(ctx, a.ObjectKey, a.Bytes)
	if e != nil || Hash(b) != a.SHA256 {
		return a, nil, apperr.Conflict("retained object is unavailable or corrupt")
	}
	if e = s.usable(ctx, a); e != nil {
		return a, nil, e
	}
	return a, b, nil
}
func (s *Service) Delete(ctx context.Context, id string) error {
	if e := Public(ctx, "sources.manage"); e != nil {
		return e
	}
	a, e := s.Get(ctx, id)
	if e != nil {
		return e
	}
	if a.RetainUntil.After(time.Now()) {
		return apperr.Conflict("artifact is within its immutable retention period")
	}
	if a.State == "DELETED" {
		return nil
	}
	if _, e = platformdb.Scoped(s.Pool).Exec(ctx, `UPDATE artifacts SET state='DELETING' WHERE id=$1 AND state IN ('PENDING','READY')`, id); e != nil {
		return e
	}
	if e = s.Store.Delete(ctx, a.ObjectKey); e != nil {
		return apperr.Conflict("object deletion incomplete; retry permitted")
	}
	_, e = platformdb.Scoped(s.Pool).Exec(ctx, `UPDATE artifacts SET state='DELETED' WHERE id=$1 AND state='DELETING'`, id)
	return e
}

func (s *Service) Ready(ctx context.Context, id string) (Artifact, error) {
	a, e := s.Get(ctx, id)
	if e != nil {
		return a, e
	}
	return a, s.usable(ctx, a)
}
