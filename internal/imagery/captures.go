package imagery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/caselink"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/intents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"time"
)

type CaptureInput struct {
	CaseRevision *caselink.Revision  `json:"caseRevision,omitempty"`
	Scope        *analystscope.Input `json:"scope,omitempty"`
	RequestKey   string              `json:"requestKey"`
	SceneID      string              `json:"sceneId"`
	AOI          publicingest.Bounds `json:"aoi"`
}
type CaptureFrozen struct {
	CaseRevision *caselink.Revision     `json:"caseRevision,omitempty"`
	Scope        *analystscope.Snapshot `json:"scope,omitempty"`
	Scene        Scene                  `json:"scene"`
	AOI          publicingest.Bounds    `json:"aoi"`
	Method       string                 `json:"method"`
}
type CaptureResult struct {
	PreparationID     string   `json:"preparationId"`
	SubsetArtifactID  string   `json:"subsetArtifactId"`
	RGBArtifactID     string   `json:"rgbArtifactId"`
	QualityArtifactID string   `json:"qualityArtifactId"`
	ReceiptArtifactID string   `json:"receiptArtifactId"`
	WindowArtifactIDs []string `json:"windowArtifactIds"`
}
type Capture struct {
	ID        string         `json:"id"`
	SourceID  string         `json:"sourceId"`
	Input     CaptureFrozen  `json:"input"`
	Status    string         `json:"status"`
	Attempt   int            `json:"attempt"`
	Progress  string         `json:"progress"`
	ErrorCode string         `json:"errorCode"`
	Result    *CaptureResult `json:"result,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	Lease     string         `json:"-"`
}

const captureSelect = `SELECT id,source_id,input,status,attempt,progress,error_code,result,created_at,COALESCE(lease_id,'') FROM imagery_capture_jobs `

func scanCapture(row pgx.Row) (Capture, error) {
	var v Capture
	var input, result []byte
	e := row.Scan(&v.ID, &v.SourceID, &input, &v.Status, &v.Attempt, &v.Progress, &v.ErrorCode, &result, &v.CreatedAt, &v.Lease)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, apperr.NotFound("imagery capture", "unavailable")
	}
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(input, &v.Input); e != nil {
		return v, e
	}
	if len(result) > 0 {
		e = json.Unmarshal(result, &v.Result)
	}
	return v, e
}
func (s *Service) Capture(ctx context.Context, id string) (Capture, error) {
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return Capture{}, e
	}
	return scanCapture(db.Scoped(s.Pool).QueryRow(ctx, captureSelect+`WHERE id=$1`, id))
}
func (s *Service) Captures(ctx context.Context, regions ...string) ([]Capture, error) {
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return nil, e
	}
	region := ""
	if len(regions) > 0 {
		region = regions[0]
	}
	if len(region) > 256 {
		return nil, apperr.Validation("invalid region")
	}
	geometry := ""
	scopeGeometry, from, to, unsupported := queryscope.Catalog(ctx, "captures")
	if unsupported {
		return []Capture{}, nil
	}
	if len(regions) > 1 {
		geometry = regions[1]
	}
	if geometry != "" {
		if e := publicevents.ValidateStudyGeometry(geometry, false); e != nil {
			return nil, e
		}
	}
	if scopeGeometry != "" {
		geometry = scopeGeometry
	}
	rows, e := db.Scoped(s.Pool).Query(ctx, captureSelect+`WHERE emad_content_visible('capture',id) AND ($1::text='' OR EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$1 AND ST_Intersects(g.geometry,ST_MakeEnvelope((input#>>'{aoi,west}')::float8,(input#>>'{aoi,south}')::float8,(input#>>'{aoi,east}')::float8,(input#>>'{aoi,north}')::float8,4326)))) AND ($2::text='' OR ST_Intersects(ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($2,'')),4326),ST_MakeEnvelope((input#>>'{aoi,west}')::float8,(input#>>'{aoi,south}')::float8,(input#>>'{aoi,east}')::float8,(input#>>'{aoi,north}')::float8,4326))) AND ($3::timestamptz IS NULL OR (input#>>'{scene,acquiredAt}')::timestamptz>=$3) AND ($4::timestamptz IS NULL OR (input#>>'{scene,acquiredAt}')::timestamptz<$4) ORDER BY created_at DESC,id LIMIT 101`, region, geometry, from, to)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Capture{}
	for rows.Next() {
		v, e := scanCapture(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) SubmitCapture(ctx context.Context, in CaptureInput) (Capture, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Capture{}, e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" || p.HumanID == "" {
		return Capture{}, apperr.Forbidden("registered human capture requester required")
	}
	id, e := intents.Imagery(ctx, s.Pool, "capture", in.RequestKey, in, func(bound context.Context, key string) (string, error) {
		in.RequestKey = key
		v, e := s.submitCapture(bound, in)
		return v.ID, e
	})
	if e != nil {
		return Capture{}, e
	}
	return s.Capture(ctx, id)
}
func (s *Service) submitCapture(ctx context.Context, in CaptureInput) (Capture, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Capture{}, e
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "human" {
		return Capture{}, apperr.Forbidden("human capture request required")
	}
	if len(in.RequestKey) < 1 || len(in.RequestKey) > 128 || in.AOI.Validate() != nil || in.AOI.East-in.AOI.West > .1 || in.AOI.North-in.AOI.South > .1 {
		return Capture{}, apperr.Validation("capture requires a bounded AOI up to 0.1 degrees per side")
	}
	scene, e := s.Scene(ctx, in.SceneID)
	if e != nil {
		return Capture{}, e
	}
	if scene.Collection != publicingest.EarthSearchDataset {
		return Capture{}, apperr.Validation("automatic capture uses the admitted Sentinel-2 Collection 1 source")
	}
	a, e := s.Artifacts.Policy(ctx, scene.SourceID, scene.AdmissionID)
	if e != nil {
		return Capture{}, e
	}
	if !a.Rights.RetainRaw || !a.Rights.AllowAnalysis || !scene.Rights.RetainRaw || !scene.Rights.AllowAnalysis {
		return Capture{}, apperr.Forbidden("capture rights unavailable")
	}
	var covers bool
	e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_Covers(geometry,ST_MakeEnvelope($2,$3,$4,$5,4326)) FROM imagery_scenes WHERE id=$1`, scene.ID, in.AOI.West, in.AOI.South, in.AOI.East, in.AOI.North).Scan(&covers)
	if e != nil {
		return Capture{}, e
	}
	if !covers {
		return Capture{}, apperr.Validation("capture AOI is outside scene coverage")
	}
	frozen := CaptureFrozen{Scene: scene, AOI: in.AOI, Method: "emad.c1-window.v1"}
	raw, _ := json.Marshal(frozen)
	id := "cap_" + artifacts.Hash([]byte(r.ID()+"|"+in.RequestKey))
	e = db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var linkErr error
		frozen.CaseRevision, linkErr = caselink.Resolve(ctx, tx, in.CaseRevision)
		if linkErr != nil {
			return linkErr
		}
		raw, _ = json.Marshal(frozen)
		if in.Scope != nil {
			v, e := (analystscope.Service{Pool: s.Pool}).Freeze(ctx, tx, *in.Scope)
			if e != nil {
				return e
			}
			if v.Capabilities["captures"].State != "supported" || !analystscope.ContainsTime(v, scene.AcquiredAt) {
				return apperr.Validation("scene acquisition is outside the supported selected period")
			}
			if e = analystscope.ValidateBounds(db.UseTx(ctx, tx), s.Pool, v, in.AOI); e != nil {
				return e
			}
			frozen.Scope = &v
			raw, _ = json.Marshal(frozen)
		}
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":capture-quota"); e != nil {
			return e
		}
		var old []byte
		err := tx.QueryRow(ctx, `SELECT input FROM imagery_capture_jobs WHERE id=$1`, id).Scan(&old)
		if err == nil {
			var previous CaptureFrozen
			if json.Unmarshal(old, &previous) != nil {
				return apperr.Conflict("capture input unavailable")
			}
			a, _ := json.Marshal(previous)
			if string(a) != string(raw) {
				return apperr.Conflict("capture request key reused with different input")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var available bool
		if err = tx.QueryRow(ctx, `SELECT emad_imagery_queue_available('capture',$1)`, scene.SourceID).Scan(&available); err != nil {
			return err
		}
		if !available {
			return apperr.Conflict("capture queue budget exhausted")
		}
		_, err = tx.Exec(ctx, `INSERT INTO imagery_capture_jobs(id,source_id,input,created_by)VALUES($1,$2,$3,$4)`, id, scene.SourceID, raw, p.ID)
		return err
	})
	if e != nil {
		return Capture{}, e
	}
	return s.Capture(ctx, id)
}
func (s *Service) ClaimCapture(ctx context.Context) (Capture, error) {
	if e := artifacts.Public(ctx, "imagery.capture.execute"); e != nil {
		return Capture{}, e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "service" {
		return Capture{}, apperr.Forbidden("capture service identity required")
	}
	var out Capture
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE imagery_capture_jobs SET status='FAILED',error_code='lease_expired',progress='Worker stopped after maximum attempts',finished_at=now() WHERE status='RUNNING' AND lease_until<now() AND attempt>=3`)
		if e != nil {
			return e
		}
		v, e := scanCapture(tx.QueryRow(ctx, captureSelect+`WHERE (status='QUEUED' OR status='RUNNING' AND lease_until<now()) AND attempt<3 ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`))
		if e != nil {
			return e
		}
		v.Lease = ids.New("caplease")
		if e = capacity.Reserve(ctx, s.Pool, "capture:"+v.ID+":"+v.Lease, v.SourceID, "raster-capture", 64<<20, time.Now().Add(45*time.Second)); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE imagery_capture_jobs SET status='RUNNING',attempt=attempt+1,worker_id=$2,lease_id=$3,lease_until=now()+interval '45 seconds',progress='Capturing retained source pixels',error_code='' WHERE id=$1`, v.ID, p.ID, v.Lease)
		v.Status = "RUNNING"
		v.Attempt++
		out = v
		return e
	})
	return out, e
}
func (s *Service) UpdateCapture(ctx context.Context, id, lease, progress string, result *CaptureResult, failed bool) error {
	if e := artifacts.Public(ctx, "imagery.capture.execute"); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "service" {
		return apperr.Forbidden("capture service identity required")
	}
	if len(progress) > 200 {
		return apperr.Validation("capture progress exceeds budget")
	}
	if result != nil {
		job, e := s.Capture(ctx, id)
		if e != nil {
			return e
		}
		j, e := s.Job(ctx, result.PreparationID)
		if e != nil {
			return e
		}
		if j.Status != "SUCCEEDED" || j.Input.Kind != "PREPARE" || j.SourceID != job.SourceID || len(j.Input.Scenes) != 1 || j.Input.Scenes[0].ID != job.Input.Scene.ID || len(j.Input.Artifacts) != 1 || j.Input.Artifacts[0].ID != result.SubsetArtifactID {
			return apperr.Validation("capture preparation does not match frozen scene")
		}
		if job.Input.Scope != nil && (j.Input.Scope == nil || j.Input.Scope.ScopeHash != job.Input.Scope.ScopeHash) {
			return apperr.Validation("capture preparation must retain the exact area and period scope")
		}
		if len(result.WindowArtifactIDs) > 8 {
			return apperr.Validation("capture window budget exceeded")
		}
		if len(result.WindowArtifactIDs) < 3 {
			return apperr.Validation("capture must retain original analysis band windows")
		}
		for _, artifactID := range append([]string{result.SubsetArtifactID, result.RGBArtifactID, result.QualityArtifactID, result.ReceiptArtifactID}, result.WindowArtifactIDs...) {
			a, e := s.Artifacts.Ready(ctx, artifactID)
			if e != nil {
				return e
			}
			if a.SourceID != job.SourceID {
				return apperr.Forbidden("capture artifact source mismatch")
			}
			expectedKind, expectedMedia := "imagery_window", "image/tiff"
			switch artifactID {
			case result.SubsetArtifactID:
				expectedKind, expectedMedia = "imagery_subset", "application/json"
			case result.RGBArtifactID, result.QualityArtifactID:
				expectedKind, expectedMedia = "imagery_preview", "image/png"
			case result.ReceiptArtifactID:
				expectedKind, expectedMedia = "imagery_capture", "application/json"
			}
			if a.Kind != expectedKind || a.MediaType != expectedMedia {
				return apperr.Validation("capture artifact type does not match its role")
			}
		}
	}
	state, code := "RUNNING", ""
	if failed {
		state, code = "FAILED", "capture_failed"
	}
	if result != nil {
		state = "SUCCEEDED"
	}
	raw, _ := json.Marshal(result)
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if result != nil {
			job, e := s.Capture(db.UseTx(ctx, tx), id)
			if e != nil {
				return e
			}
			policy, e := publicingest.LockPolicy(ctx, tx, job.SourceID, job.Input.Scene.AdmissionID)
			if e != nil {
				return e
			}
			if !policy.Rights.AllowAnalysis || !policy.Rights.RetainRaw {
				return apperr.Forbidden("capture profile rights no longer available")
			}
			if e = publicingest.RequireProfileProcessing(db.UseTx(ctx, tx), s.Pool, policy); e != nil {
				return e
			}
		}
		tag, e := tx.Exec(ctx, `UPDATE imagery_capture_jobs SET status=$5,progress=$4,error_code=$6,result=CASE WHEN $5='SUCCEEDED' THEN $7::jsonb ELSE result END,lease_until=now()+interval '45 seconds',finished_at=CASE WHEN $5 IN('SUCCEEDED','FAILED') THEN now() ELSE NULL END WHERE id=$1 AND worker_id=$2 AND lease_id=$3 AND status='RUNNING' AND lease_until>now()`, id, p.ID, lease, progress, state, code, raw)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("capture lease expired, cancelled or replaced")
		}
		return nil
	})
	if e != nil {
		return e
	}
	if failed || result != nil {
		return capacity.Settle(ctx, s.Pool, "capture:"+id+":"+lease)
	}
	return capacity.Renew(ctx, s.Pool, "capture:"+id+":"+lease, time.Now().Add(45*time.Second))
}
func (s *Service) ControlCapture(ctx context.Context, id, action string) (Capture, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Capture{}, e
	}
	principal, _, _ := access.From(ctx)
	if principal.Kind != "human" {
		return Capture{}, apperr.Forbidden("human capture control required")
	}
	if action == "retry" {
		_, e := db.Scoped(s.Pool).Exec(ctx, `UPDATE imagery_capture_jobs SET status='QUEUED',error_code='',progress='Awaiting retry',worker_id=NULL,lease_id=NULL,lease_until=NULL,finished_at=NULL WHERE id=$1 AND status='FAILED' AND attempt<3`, id)
		if e != nil {
			return Capture{}, e
		}
	} else if action == "cancel" {
		_, e := db.Scoped(s.Pool).Exec(ctx, `UPDATE imagery_capture_jobs SET status='CANCELLED',progress='Cancelled by analyst',finished_at=now() WHERE id=$1 AND status IN('QUEUED','RUNNING')`, id)
		if e != nil {
			return Capture{}, e
		}
	} else {
		return Capture{}, apperr.Validation("unknown capture action")
	}
	return s.Capture(ctx, id)
}
