package modelassets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"time"
)

type Service struct {
	Pool  *pgxpool.Pool
	Store artifacts.Store
}

func New(pool *pgxpool.Pool, store artifacts.Store) *Service {
	if store == nil {
		store = artifacts.Unavailable{}
	}
	return &Service{pool, store}
}
func require(ctx context.Context, write bool) error {
	if err := access.Require(ctx, "sim.scenarios.author"); err != nil {
		return err
	}
	p, r, _ := access.From(ctx)
	if r.Partition != access.ExerciseInput || r.RunID != "" {
		return apperr.Forbidden("asset registry requires an explicitly granted EXERCISE_INPUT environment")
	}
	if write && p.Kind != "human" {
		return apperr.Forbidden("asset admission requires a human author")
	}
	return nil
}
func refresh(ctx context.Context, write bool) (context.Context, error) {
	c, e := access.Refresh(ctx)
	if e != nil {
		return nil, e
	}
	return c, require(c, write)
}

const uploadSQL = `SELECT document,state,attempts,error_code,object_key FROM model_asset_uploads `

func scanUpload(row pgx.Row) (Upload, error) {
	var a Upload
	var raw []byte
	var state, code, key string
	var attempts int
	e := row.Scan(&raw, &state, &attempts, &code, &key)
	if errors.Is(e, pgx.ErrNoRows) {
		return a, apperr.NotFound("asset upload", "unavailable")
	}
	if e != nil {
		return a, e
	}
	if e = json.Unmarshal(raw, &a); e != nil {
		return a, e
	}
	a.State = state
	a.ErrorCode = code
	a.Attempts = attempts
	a.ObjectKey = key
	return a, nil
}
func (s *Service) get(ctx context.Context, id string) (Upload, error) {
	return scanUpload(platformdb.Scoped(s.Pool).QueryRow(ctx, uploadSQL+`WHERE id=$1`, id))
}
func (s *Service) Get(ctx context.Context, id string) (Upload, error) {
	if e := require(ctx, false); e != nil {
		return Upload{}, e
	}
	return s.get(ctx, id)
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (s *Service) Reserve(ctx context.Context, in UploadInput) (Upload, error) {
	if e := require(ctx, true); e != nil {
		return Upload{}, e
	}
	if e := in.Validate(); e != nil {
		return Upload{}, e
	}
	if in.SourceFileID != "" {
		src, e := s.Get(ctx, in.SourceFileID)
		if e != nil {
			return Upload{}, e
		}
		if src.Kind != "SOURCE" || src.State != "STORED" {
			return Upload{}, apperr.Validation("source file must be a retained opaque authoring source")
		}
		a, _ := json.Marshal(src.License)
		b, _ := json.Marshal(in.License)
		if string(a) != string(b) {
			return Upload{}, apperr.Validation("model must preserve retained authoring source rights")
		}
	}
	if in.ParentVersionID != "" {
		parent, e := s.Model(ctx, in.ParentVersionID)
		if e != nil {
			return Upload{}, e
		}
		a, _ := json.Marshal(parent.Upload.License)
		b, _ := json.Marshal(in.License)
		if string(a) != string(b) {
			return Upload{}, apperr.Validation("variant must preserve its parent license")
		}
	}
	p, r, _ := access.From(ctx)
	a := Upload{UploadInput: in, ID: "mup_" + artifacts.Hash([]byte(r.ID()+"|"+in.RequestKey)), State: "PENDING", CreatedBy: p.ID, CreatedAt: time.Now().UTC()}
	a.ObjectKey = fmt.Sprintf("emad/v1/%s/%s/EXERCISE_INPUT/model-assets/%s/%s", artifacts.Hash([]byte(r.WorkspaceID))[:32], artifacts.Hash([]byte(r.EnvironmentID))[:32], a.ID, in.SHA256)
	raw, _ := json.Marshal(a)
	e := platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":model-asset-quota"); e != nil {
			return e
		}
		old, e := scanUpload(tx.QueryRow(ctx, uploadSQL+`WHERE id=$1`, a.ID))
		if e == nil {
			b, _ := json.Marshal(old.UploadInput)
			c, _ := json.Marshal(in)
			if string(b) != string(c) {
				return apperr.Conflict("asset request key reused with different content or rights")
			}
			return nil
		}
		if !apperr.Is(e, apperr.CodeNotFound) {
			return e
		}
		var count int
		var bytes int64
		if e := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(size_bytes),0) FROM model_asset_uploads`).Scan(&count, &bytes); e != nil {
			return e
		}
		if count >= 1000 || bytes+in.Bytes > 512*1024*1024 {
			return apperr.Conflict("environment asset archive budget exhausted")
		}
		_, e = tx.Exec(ctx, `INSERT INTO model_asset_uploads(id,kind,document,object_key,size_bytes,state,source_file_id,parent_version_id)VALUES($1,$2,$3,$4,$5,'PENDING',$6,$7)`, a.ID, a.Kind, raw, a.ObjectKey, a.Bytes, nullable(in.SourceFileID), nullable(in.ParentVersionID))
		if e != nil {
			return e
		}
		return recordAudit(ctx, tx, "simulation.asset.admitted", a.ID, map[string]any{"sha256": a.SHA256, "licenseId": a.License.ID, "kind": a.Kind})
	})
	if e != nil {
		return Upload{}, e
	}
	return s.Get(ctx, a.ID)
}
func (s *Service) Upload(ctx context.Context, id string, b []byte) (Upload, error) {
	if e := require(ctx, true); e != nil {
		return Upload{}, e
	}
	a, e := s.Get(ctx, id)
	if e != nil {
		return a, e
	}
	if a.State != "PENDING" && a.State != "STAGED" && a.State != "STORED" {
		return a, apperr.Conflict("upload already under validation or immutable")
	}
	if int64(len(b)) != a.Bytes || artifacts.Hash(b) != a.SHA256 {
		return a, apperr.Validation("model source hash or byte count mismatch")
	}
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if e = s.Store.Put(work, a.ObjectKey, b, a.MediaType); e != nil {
		return a, apperr.Conflict("asset storage unavailable; identical upload may be retried")
	}
	fresh, e := refresh(work, true)
	if e != nil {
		return a, e
	}
	state := "STAGED"
	if a.Kind != "MODEL" {
		state = "STORED"
	}
	tag, e := platformdb.Scoped(s.Pool).Exec(fresh, `UPDATE model_asset_uploads SET state=$2 WHERE id=$1 AND state IN('PENDING','STAGED','STORED')`, id, state)
	if e != nil {
		return a, e
	}
	if tag.RowsAffected() != 1 {
		return a, apperr.Conflict("asset state changed during upload")
	}
	return s.Get(fresh, id)
}
func (s *Service) Content(ctx context.Context, id string) (Upload, []byte, error) {
	a, e := s.Get(ctx, id)
	if e != nil {
		return a, nil, e
	}
	if a.State == "PENDING" {
		return a, nil, apperr.Conflict("asset content has not been retained")
	}
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	b, e := s.Store.Get(work, a.ObjectKey, a.Bytes)
	if e != nil || artifacts.Hash(b) != a.SHA256 {
		return a, nil, apperr.Conflict("asset content unavailable or corrupt")
	}
	if _, e = refresh(work, false); e != nil {
		return a, nil, e
	}
	return a, b, nil
}
func (s *Service) Model(ctx context.Context, id string) (ModelAsset, error) {
	var v ModelAsset
	if e := require(ctx, false); e != nil {
		return v, e
	}
	var raw []byte
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT document,manifest_key FROM model_asset_versions WHERE id=$1`, id).Scan(&raw, &v.ManifestKey)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, apperr.NotFound("model version", "unavailable")
	}
	if e == nil {
		key := v.ManifestKey
		e = json.Unmarshal(raw, &v)
		v.ManifestKey = key
	}
	return v, e
}
func (s *Service) Manifest(ctx context.Context, id string) ([]byte, error) {
	v, e := s.Model(ctx, id)
	if e != nil {
		return nil, e
	}
	b, e := s.Store.Get(ctx, v.ManifestKey, v.ManifestBytes)
	if e != nil || artifacts.Hash(b) != v.ManifestSHA256 {
		return nil, apperr.Conflict("model validation artifact unavailable or corrupt")
	}
	if _, e = refresh(ctx, false); e != nil {
		return nil, e
	}
	return b, nil
}
func (s *Service) List(ctx context.Context, family string) ([]json.RawMessage, error) {
	if e := require(ctx, false); e != nil {
		return nil, e
	}
	var sql string
	switch family {
	case "uploads":
		sql = `SELECT document || jsonb_build_object('state',state,'attempts',attempts,'errorCode',error_code) FROM model_asset_uploads ORDER BY id DESC LIMIT 101`
	case "models":
		sql = `SELECT document FROM model_asset_versions ORDER BY id DESC LIMIT 101`
	case "templates":
		sql = `SELECT document FROM actor_templates ORDER BY id DESC LIMIT 101`
	case "environments":
		sql = `SELECT document FROM environment_assets ORDER BY id DESC LIMIT 101`
	default:
		return nil, apperr.Validation("unknown asset family")
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, sql)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}
func (s *Service) Template(ctx context.Context, in ActorTemplate) (ActorTemplate, error) {
	if e := require(ctx, true); e != nil {
		return in, e
	}
	if in.ID != "" || in.CreatedBy != "" || !in.CreatedAt.IsZero() || !text(in.Name, 200) || !text(in.Limitations, 2000) || in.Fidelity != "abstract_behavior" || math.IsNaN(in.MaxSpeedMPS) || math.IsInf(in.MaxSpeedMPS, 0) || in.MaxSpeedMPS < 0 || in.MaxSpeedMPS > 400 || in.Behavior != "STATIC" && in.Behavior != "WAYPOINT_KINEMATIC" || in.Behavior == "STATIC" && in.MaxSpeedMPS != 0 || in.Behavior == "WAYPOINT_KINEMATIC" && in.MaxSpeedMPS <= 0 {
		return in, apperr.Validation("invalid abstract actor template")
	}
	if _, e := s.Model(ctx, in.ModelVersionID); e != nil {
		return in, e
	}
	raw, _ := json.Marshal(in)
	in.ID = "atp_" + artifacts.Hash(raw)
	p, _, _ := access.From(ctx)
	in.CreatedBy = p.ID
	in.CreatedAt = time.Now().UTC()
	raw, _ = json.Marshal(in)
	e := s.saveDefinition(ctx, "actor_templates", in.ID, in.ModelVersionID, raw)
	if e == nil {
		stored, err := s.Definition(ctx, "templates", in.ID)
		if err != nil {
			return in, err
		}
		e = json.Unmarshal(stored, &in)
	}
	return in, e
}
func (s *Service) Environment(ctx context.Context, in EnvironmentAsset) (EnvironmentAsset, error) {
	if e := require(ctx, true); e != nil {
		return in, e
	}
	if in.ID != "" || in.CreatedBy != "" || !in.CreatedAt.IsZero() || !text(in.Name, 200) || !text(in.Provenance, 2000) || in.Frame.Validate() != nil || in.ModelAxesToENU != "x_east_y_up_z_south" || in.Fidelity != "visual_only" {
		return in, apperr.Validation("invalid visual environment georeference")
	}
	model, e := s.Model(ctx, in.ModelVersionID)
	if e != nil {
		return in, e
	}
	if in.ReleaseToOperators && in.Prepared == nil {
		return in, apperr.Validation("operator release requires a prepared geographic environment manifest")
	}
	if m := in.Prepared; m != nil {
		a, _ := json.Marshal(m.Frame)
		b, _ := json.Marshal(in.Frame)
		if m.Schema != "emad.prepared-environment.v1" || m.AOI.Validate() != nil || m.ModelSHA256 != model.Upload.SHA256 || !hashPattern.MatchString(m.TerrainParentSHA256) || !hashPattern.MatchString(m.GeoidSHA256) || !hashPattern.MatchString(m.OSMParentSHA256) || !hashPattern.MatchString(m.ImagerySHA256) || m.Frame.Validate() != nil || string(a) != string(b) || m.InputVerticalDatum != "EGM2008" || m.OutputVerticalDatum != "WGS84_ELLIPSOID" || !text(m.Transformation, 2000) || m.TerrainVertices < 4 || m.TerrainVertices > 65536 || m.Buildings < 0 || m.Buildings > 2000 || m.RoadSegments < 0 || m.RoadSegments > 1000 || len(m.Limitations) < 1 || len(m.Limitations) > 20 {
			return in, apperr.Validation("invalid prepared environment provenance, frame or geometry budget")
		}
	}
	raw, _ := json.Marshal(in)
	in.ID = "env_" + artifacts.Hash(raw)
	p, _, _ := access.From(ctx)
	in.CreatedBy = p.ID
	in.CreatedAt = time.Now().UTC()
	raw, _ = json.Marshal(in)
	e = s.saveDefinition(ctx, "environment_assets", in.ID, in.ModelVersionID, raw)
	if e == nil {
		stored, err := s.Definition(ctx, "environments", in.ID)
		if err != nil {
			return in, err
		}
		e = json.Unmarshal(stored, &in)
	}
	return in, e
}
func (s *Service) Attempts(ctx context.Context, id string) ([]json.RawMessage, error) {
	if _, e := s.Get(ctx, id); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT document FROM model_asset_attempts WHERE upload_id=$1 ORDER BY id DESC LIMIT 101`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

// IDs for future runtime actor instances must be minted separately by the run
// owner. Registry admission deliberately creates no SimulationActor or asset.

func (s *Service) saveDefinition(ctx context.Context, table, id, model string, raw []byte) error {
	// table is selected only by the two typed internal callers.
	return platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		resource, _ := access.ResourceFrom(ctx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":"+table); e != nil {
			return e
		}
		var exists bool
		if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE id=$1)", id).Scan(&exists); e != nil {
			return e
		}
		if exists {
			return nil
		}
		var count int
		if e := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil {
			return e
		}
		if count >= 1000 {
			return apperr.Conflict("authoring definition archive budget exhausted")
		}
		_, e := tx.Exec(ctx, "INSERT INTO "+table+"(id,model_version_id,document)VALUES($1,$2,$3)", id, model, raw)
		if e != nil {
			return e
		}
		return recordAudit(ctx, tx, "simulation.definition.created", id, map[string]any{"family": table, "modelVersionId": model})
	})
}

func recordAudit(ctx context.Context, tx pgx.Tx, action, id string, data any) error {
	p, _, _ := access.From(ctx)
	kind := "SYSTEM"
	if p.Kind == "human" {
		kind = "OPERATOR"
	}
	raw, _ := json.Marshal(data)
	_, e := tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data)VALUES($1,now(),$2,$3,$4,'simulation_asset',$5,$6,$7)`, ids.New("aud"), kind, p.ID, action, id, httpx.GetRequestID(ctx), raw)
	return e
}
func (s *Service) Definition(ctx context.Context, family, id string) (json.RawMessage, error) {
	if e := require(ctx, false); e != nil {
		return nil, e
	}
	table := "actor_templates"
	if family == "environments" {
		table = "environment_assets"
	} else if family != "templates" {
		return nil, apperr.Validation("unknown authoring definition")
	}
	var raw []byte
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, "SELECT document FROM "+table+" WHERE id=$1", id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperr.NotFound("authoring definition", "unavailable")
	}
	return raw, e
}
