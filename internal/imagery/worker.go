package imagery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"time"
)

func workerAccess(ctx context.Context) error {
	if e := artifacts.Public(ctx, "imagery.jobs.execute"); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "service" {
		return apperr.Forbidden("imagery processing requires a service identity")
	}
	return nil
}

// WorkOne is used by the independent worker CLI. No HTTP route calls it.
func (s *Service) WorkOne(auth context.Context, build string) (bool, error) {
	ctx, e := access.Refresh(auth)
	if e != nil {
		return false, e
	}
	if e = workerAccess(ctx); e != nil {
		return false, e
	}
	if len(build) != 64 {
		return false, apperr.Validation("immutable worker build SHA-256 required")
	}
	j, e := s.claim(ctx, build)
	if apperr.Is(e, apperr.CodeNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	work, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				fresh, err := access.RefreshWithContext(auth, work)
				if err != nil || workerAccess(fresh) != nil {
					cancel()
					return
				}
				row, err := s.Job(fresh, j.ID)
				if err != nil || row.Status != "RUNNING" || row.Lease != j.Lease {
					cancel()
					return
				}
			}
		}
	}()
	result, processErr := s.process(work, j)
	cancel()
	<-done
	// Reauthenticate after external I/O. A revoked worker cannot publish results;
	// its claim is recovered under a new authorized worker after lease expiry.
	finalCtx, finalCancel := context.WithTimeout(context.WithoutCancel(auth), 10*time.Second)
	defer finalCancel()
	finalCtx, e = access.RefreshWithContext(auth, finalCtx)
	if e != nil {
		return true, e
	}
	if e = workerAccess(finalCtx); e != nil {
		return true, e
	}
	code := ""
	status := "SUCCEEDED"
	if processErr != nil {
		status = "FAILED"
		code = safeProcessingError(processErr)
	}
	e = platformdb.WithTx(finalCtx, s.Pool, func(tx pgx.Tx) error {
		current, e := scanJob(tx.QueryRow(finalCtx, `SELECT `+jobColumns+` FROM imagery_jobs WHERE id=$1 FOR UPDATE`, j.ID))
		if e != nil {
			return e
		}
		if current.Lease != j.Lease {
			return apperr.Conflict("worker lease no longer owns job")
		}
		if current.Status == "CANCEL_REQUESTED" {
			status = "CANCELED"
			code = "canceled"
		}
		if current.Status != "RUNNING" && current.Status != "CANCEL_REQUESTED" {
			return apperr.Conflict("job is no longer processing")
		}
		var resultRaw []byte
		if status == "SUCCEEDED" {
			for _, scene := range j.Input.Scenes {
				policy, e := publicingest.LockPolicy(finalCtx, tx, scene.SourceID, scene.AdmissionID)
				if e != nil || !policy.Rights.AllowAnalysis || !policy.Rights.RetainRaw {
					status = "FAILED"
					code = "rights_unavailable"
					break
				}
				if e = publicingest.RequireProfileProcessing(platformdb.UseTx(finalCtx, tx), s.Pool, policy); e != nil {
					status = "FAILED"
					code = "rights_unavailable"
					break
				}
			}
		}
		if status == "SUCCEEDED" {
			resultRaw, _ = json.Marshal(result)
			if j.Input.Kind == "PREPARE" {
				_, e = tx.Exec(finalCtx, `INSERT INTO imagery_preparations(job_id,source_id,scene_id,artifact_id,document)VALUES($1,$2,$3,$4,$5)`, j.ID, j.SourceID, j.Input.Scenes[0].ID, j.Input.Artifacts[0].ID, resultRaw)
				if e != nil {
					return e
				}
			}
		}
		doc, _ := json.Marshal(map[string]any{"attempt": j.Attempt, "workerBuild": build, "algorithm": j.Input.Algorithm, "status": status, "errorCode": code, "finishedAt": time.Now().UTC()})
		if _, e = tx.Exec(finalCtx, `INSERT INTO imagery_job_attempts(job_id,source_id,attempt,document)VALUES($1,$2,$3,$4)`, j.ID, j.SourceID, j.Attempt, doc); e != nil {
			return e
		}
		_, e = tx.Exec(finalCtx, `UPDATE imagery_jobs SET status=$2,error_code=$3,result=$4,finished_at=now(),lease_id=NULL,lease_until=NULL WHERE id=$1`, j.ID, status, code, resultRaw)
		return e
	})
	return true, e
}
func safeProcessingError(e error) string {
	if errors.Is(e, context.Canceled) {
		return "interrupted"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "processing_timeout"
	}
	switch e.Error() {
	case "invalid_subset", "source_hashes_missing", "invalid_quality_mask", "invalid_reflectance", "grid_alignment_mismatch", "scene_time_incomparable", "radiometric_or_geometry_incomparable", "insufficient_common_clear_pixels", "finding_geometry_exceeds_budget", "scene_subset_mismatch", "subset_outside_scene", "subset_outside_selected_area", "subset_outside_selected_period", "rights_unavailable":
		return e.Error()
	}
	return "artifact_or_processing_unavailable"
}
func (s *Service) process(ctx context.Context, j Job) (JobResult, error) {
	out := JobResult{Limitations: []string{}}
	if j.Input.Schema != "emad.imagery-job.v1" || j.Input.Algorithm != imageryanalysis.Version {
		return out, errors.New("unsupported frozen algorithm")
	}
	tiles := []imageryanalysis.Tile{}
	for i, a := range j.Input.Artifacts {
		scene := j.Input.Scenes[i]
		admission, e := s.Artifacts.Policy(ctx, scene.SourceID, scene.AdmissionID)
		if e != nil || !admission.Rights.AllowAnalysis || !scene.Rights.AllowAnalysis || (scene.Rights.ExpiresAt != nil && !scene.Rights.ExpiresAt.After(time.Now())) {
			return out, errors.New("rights_unavailable")
		}
		actual, raw, e := s.Artifacts.Read(ctx, a.ID)
		if e != nil {
			return out, e
		}
		if actual.SHA256 != a.SHA256 {
			return out, errors.New("artifact_hash_mismatch")
		}
		tile, e := imageryanalysis.Decode(raw)
		if e != nil {
			return out, e
		}
		if tile.SceneID != scene.ID || tile.SceneHash != scene.ContentHash || !tile.AcquiredAt.Equal(scene.AcquiredAt) || tile.Platform != scene.Platform || tile.Tile != scene.Tile || tile.Orbit != scene.Orbit || tile.ProcessingVersion != scene.ProcessingVersion {
			return out, errors.New("scene_subset_mismatch")
		}
		b := tile.Grid.Bounds()
		if j.Input.Scope != nil {
			if !analystscope.ContainsTime(*j.Input.Scope, tile.AcquiredAt) {
				return out, errors.New("subset_outside_selected_period")
			}
			if e = analystscope.ValidateBounds(ctx, s.Pool, *j.Input.Scope, b); e != nil {
				if apperr.Is(e, apperr.CodeValidation) {
					return out, errors.New("subset_outside_selected_area")
				}
				return out, e
			}
		}
		var covers bool
		if e = platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_Covers(geometry,ST_MakeEnvelope($2,$3,$4,$5,4326)) FROM imagery_scenes WHERE id=$1`, scene.ID, b.West, b.South, b.East, b.North).Scan(&covers); e != nil {
			return out, e
		}
		if !covers {
			return out, errors.New("subset_outside_scene")
		}
		tiles = append(tiles, tile)
	}
	if j.Input.Kind == "PREPARE" {
		if len(tiles) != 1 {
			return out, errors.New("invalid preparation")
		}
		out.Tile = &TileInfo{tiles[0].Grid, tiles[0].SourceHashes, tiles[0].PreparationMethod}
		out.ArtifactID = j.Input.Artifacts[0].ID
		out.SHA256 = j.Input.Artifacts[0].SHA256
		out.Synthetic = tiles[0].Synthetic
		out.Limitations = []string{imageryanalysis.TileSummary(tiles[0]), "Source hashes/provenance are preparer-supplied; this validates the retained subset, not a fresh upstream download."}
		return out, nil
	}
	if len(tiles) != 2 {
		return out, errors.New("invalid comparison")
	}
	compared, e := imageryanalysis.Compare(ctx, tiles[0], tiles[1], j.Input.Parameters)
	if e != nil {
		return out, e
	}
	if e = platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT postgis_full_version()`).Scan(&compared.GeometryProcessor); e != nil {
		return out, e
	}
	if string(compared.FindingGeometry) != "null" {
		var geometry []byte
		if e = platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_AsGeoJSON(ST_UnaryUnion(ST_SetSRID(ST_GeomFromGeoJSON($1),4326)),15)`, string(compared.FindingGeometry)).Scan(&geometry); e != nil {
			return out, e
		}
		compared.FindingGeometry = geometry
	}
	raw, e := json.Marshal(compared)
	if e != nil {
		return out, e
	}
	retention := j.Input.Artifacts[0].RetainUntil
	if j.Input.Artifacts[1].RetainUntil.Before(retention) {
		retention = j.Input.Artifacts[1].RetainUntil
	}
	deps := []string{}
	for _, scene := range j.Input.Scenes {
		deps = append(deps, scene.AdmissionID)
	}
	for _, artifact := range j.Input.Artifacts {
		deps = append(deps, artifact.AdmissionID)
		deps = append(deps, artifact.DependencyAdmissionIDs...)
	}
	a, e := s.Artifacts.Reserve(ctx, artifacts.Input{DependencyAdmissionIDs: deps, RequestedAdmissionID: j.Input.Scenes[0].AdmissionID, RequestKey: "result:" + j.ID + ":" + j.WorkerBuild[:12], SourceID: j.SourceID, SHA256: artifacts.Hash(raw), Bytes: int64(len(raw)), Kind: "analysis_result", MediaType: "application/json", RetainUntil: retention, Provenance: fmt.Sprintf("%s; worker %s; job %s; two frozen retained subsets", imageryanalysis.Version, j.WorkerBuild, j.ID)})
	if e != nil {
		return out, e
	}
	a, e = s.Artifacts.Upload(ctx, a.ID, raw)
	if e != nil {
		return out, e
	}
	for _, scene := range j.Input.Scenes {
		admission, e := s.Artifacts.Policy(ctx, scene.SourceID, scene.AdmissionID)
		if e != nil || !admission.Rights.AllowAnalysis {
			return out, errors.New("rights_unavailable")
		}
	}
	out = JobResult{ArtifactID: a.ID, SHA256: a.SHA256, ValidCells: compared.ValidCells, DeclineCells: compared.DeclineCells, FindingGeometry: compared.FindingGeometry, Limitations: compared.Limitations, Synthetic: compared.Synthetic}
	return out, nil
}
