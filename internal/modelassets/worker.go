package modelassets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type ImportRejection struct{ Reason string }

func (e *ImportRejection) Error() string { return e.Reason }

type Validator interface {
	Validate(context.Context, []byte, string) (Validation, error)
	Build() string
}
type NodeValidator struct {
	Script      string
	Fingerprint string
}

func NewNodeValidator(script string) (*NodeValidator, error) {
	fingerprint, e := validatorFingerprint(script)
	if e != nil {
		return nil, e
	}
	return &NodeValidator{script, fingerprint}, nil
}
func validatorFingerprint(script string) (string, error) {
	values := []string{}
	for _, path := range []string{script, filepath.Join(filepath.Dir(script), "node_modules/gltf-validator/package.json"), filepath.Join(filepath.Dir(script), "node_modules/gltf-validator/index.js"), filepath.Join(filepath.Dir(script), "node_modules/gltf-validator/module.mjs"), filepath.Join(filepath.Dir(script), "node_modules/gltf-validator/gltf_validator.dart.js")} {
		b, e := os.ReadFile(path)
		if e != nil || len(b) > 4*1024*1024 {
			return "", fmt.Errorf("trusted validator dependency missing or too large; run pnpm install")
		}
		values = append(values, artifacts.Hash(b))
	}
	raw, _ := json.Marshal(values)
	return artifacts.Hash(raw), nil
}

func (v *NodeValidator) Build() string { return v.Fingerprint }

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("validator output budget")
	}
	return b.Buffer.Write(p)
}
func (v *NodeValidator) Validate(ctx context.Context, raw []byte, media string) (Validation, error) {
	var out Validation
	fingerprint, e := validatorFingerprint(v.Script)
	if e != nil || fingerprint != v.Fingerprint {
		return out, fmt.Errorf("validator_build_changed")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--max-old-space-size=256", v.Script, media)
	cmd.Stdin = bytes.NewReader(raw)
	stdout := &cappedBuffer{limit: 262144}
	stderr := &cappedBuffer{limit: 4096}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if e = cmd.Run(); e != nil {
		if ctx.Err() != nil {
			return out, fmt.Errorf("validation_timeout")
		}
		var rejection struct {
			AssetRejected bool   `json:"assetRejected"`
			Reason        string `json:"reason"`
		}
		if json.Unmarshal(stderr.Bytes(), &rejection) == nil && rejection.AssetRejected {
			return out, &ImportRejection{Reason: rejection.Reason}
		}
		return out, fmt.Errorf("validator_process_unavailable")
	}
	if json.Unmarshal(stdout.Bytes(), &out) != nil || out.Schema != "emad.model-validation.v1" || out.Importer != Importer || out.Fidelity != "visual_only" || out.Units != "metres" || out.ValidatorVersion == "" {
		return out, fmt.Errorf("invalid_validation_report")
	}
	if e := validateTextures(ctx, raw, media); e != nil {
		return out, &ImportRejection{Reason: e.Error()}
	}
	return out, nil
}
func worker(ctx context.Context) error {
	if e := access.Require(ctx, "sim.assets.import"); e != nil {
		return e
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "service" || r.Partition != access.ExerciseInput || r.RunID != "" {
		return apperr.Forbidden("asset validation requires a service identity bound to EXERCISE_INPUT")
	}
	return nil
}
func (s *Service) WorkOne(ctx context.Context, v Validator, build string) (bool, error) {
	if e := worker(ctx); e != nil {
		return false, e
	}
	if !hashPattern.MatchString(build) || !hashPattern.MatchString(v.Build()) {
		return false, apperr.Validation("pinned worker and validator build hashes required")
	}
	work, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	fresh, e := access.RefreshWithContext(ctx, work)
	if e != nil {
		return false, e
	}
	if e = worker(fresh); e != nil {
		return false, e
	}
	var a Upload
	lease := ids.New("mai")
	started := time.Now().UTC()
	claimed := false
	e = platformdb.WithTx(fresh, s.Pool, func(tx pgx.Tx) error {
		var id string
		var oldLease *string
		e := tx.QueryRow(fresh, `SELECT id,lease_id FROM model_asset_uploads WHERE kind='MODEL' AND (state='STAGED' OR state='RUNNING' AND lease_until<now()) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &oldLease)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		a, e = scanUpload(tx.QueryRow(fresh, uploadSQL+`WHERE id=$1`, id))
		if e != nil {
			return e
		}
		if oldLease != nil {
			raw, _ := json.Marshal(map[string]any{"id": *oldLease, "uploadId": id, "status": "INTERRUPTED", "reason": "lease_expired", "recordedAt": started})
			if _, e = tx.Exec(fresh, `INSERT INTO model_asset_attempts(id,upload_id,document)VALUES($1,$2,$3)ON CONFLICT DO NOTHING`, *oldLease, id, raw); e != nil {
				return e
			}
		}
		if a.Attempts >= 3 {
			_, e = tx.Exec(fresh, `UPDATE model_asset_uploads SET state='FAILED',lease_id=NULL,lease_until=NULL,error_code='attempt_budget' WHERE id=$1`, id)
			return e
		}
		_, e = tx.Exec(fresh, `UPDATE model_asset_uploads SET state='RUNNING',attempts=attempts+1,lease_id=$2,lease_until=now()+interval '60 seconds',error_code='' WHERE id=$1`, id, lease)
		claimed = e == nil
		a.Attempts++
		return e
	})
	if e != nil || !claimed {
		return claimed, e
	}
	raw, e := s.Store.Get(work, a.ObjectKey, a.Bytes)
	status, reason := "READY", ""
	var report Validation
	var model ModelAsset
	var transient bool
	if e != nil || artifacts.Hash(raw) != a.SHA256 {
		status, reason, transient = "STAGED", "source_unavailable_or_corrupt", true
	} else {
		report, e = v.Validate(work, raw, a.MediaType)
		if e != nil {
			var rejection *ImportRejection
			if errors.As(e, &rejection) {
				status = "REJECTED"
			} else {
				status = "STAGED"
				transient = true
			}
			reason = e.Error()
			if len(reason) > 1000 {
				reason = reason[:1000]
			}
		}
	}
	if status == "READY" {
		manifest, _ := json.Marshal(report)
		sha := artifacts.Hash(manifest)
		model = ModelAsset{Upload: a, Validation: report, WorkerBuild: build, ValidatorBuild: v.Build(), ManifestBytes: int64(len(manifest)), ManifestSHA256: sha, CreatedAt: time.Now().UTC()}
		model.Upload.State = "READY"
		identity, _ := json.Marshal([]any{a.ID, a.SHA256, Importer, build, v.Build(), sha})
		model.ID = "mav_" + artifacts.Hash(identity)
		model.ManifestKey = a.ObjectKey[:len(a.ObjectKey)-64] + "validation/" + sha
		if e = s.Store.Put(work, model.ManifestKey, manifest, "application/json"); e != nil {
			status, reason, transient = "STAGED", "manifest_storage_unavailable", true
		}
	}
	fresh, e = access.RefreshWithContext(ctx, work)
	if e != nil {
		return true, e
	}
	if e = worker(fresh); e != nil {
		return true, e
	}
	if transient && a.Attempts >= 3 {
		status = "FAILED"
	}
	e = platformdb.WithTx(fresh, s.Pool, func(tx pgx.Tx) error {
		var live bool
		if e := tx.QueryRow(fresh, `SELECT EXISTS(SELECT 1 FROM model_asset_uploads WHERE id=$1 AND state='RUNNING' AND lease_id=$2 AND lease_until>now())`, a.ID, lease).Scan(&live); e != nil {
			return e
		}
		if !live {
			return apperr.Conflict("asset import lease changed")
		}
		if status == "READY" {
			raw, _ := json.Marshal(model)
			if _, e := tx.Exec(fresh, `INSERT INTO model_asset_versions(id,upload_id,document,manifest_key)VALUES($1,$2,$3,$4)ON CONFLICT DO NOTHING`, model.ID, a.ID, raw, model.ManifestKey); e != nil {
				return e
			}
		}
		attempt, _ := json.Marshal(map[string]any{"id": lease, "uploadId": a.ID, "status": status, "reason": reason, "startedAt": started, "finishedAt": time.Now().UTC(), "workerBuild": build, "validatorBuild": v.Build(), "attempt": a.Attempts})
		if _, e := tx.Exec(fresh, `INSERT INTO model_asset_attempts(id,upload_id,document)VALUES($1,$2,$3)`, lease, a.ID, attempt); e != nil {
			return e
		}
		_, e := tx.Exec(fresh, `UPDATE model_asset_uploads SET state=$2,error_code=$3,lease_id=NULL,lease_until=NULL WHERE id=$1 AND lease_id=$4`, a.ID, status, reason, lease)
		if e != nil {
			return e
		}
		return recordAudit(fresh, tx, "simulation.asset.import.finished", a.ID, map[string]any{"attemptId": lease, "status": status, "workerBuild": build, "validatorBuild": v.Build()})
	})
	return true, e
}
