package imagery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"time"
)

type SubmitInput struct {
	RequestKey     string                     `json:"requestKey"`
	Kind           string                     `json:"kind"`
	SceneID        string                     `json:"sceneId,omitempty"`
	ArtifactID     string                     `json:"artifactId,omitempty"`
	PreparationIDs []string                   `json:"preparationIds,omitempty"`
	Parameters     imageryanalysis.Parameters `json:"parameters"`
}
type JobInput struct {
	Schema         string                     `json:"schema"`
	Kind           string                     `json:"kind"`
	Algorithm      string                     `json:"algorithm"`
	Scenes         []Scene                    `json:"scenes"`
	Artifacts      []artifacts.Artifact       `json:"artifacts"`
	PreparationIDs []string                   `json:"preparationIds"`
	Parameters     imageryanalysis.Parameters `json:"parameters"`
}
type JobResult struct {
	ArtifactID      string          `json:"artifactId,omitempty"`
	SHA256          string          `json:"sha256,omitempty"`
	Tile            *TileInfo       `json:"tile,omitempty"`
	ValidCells      int             `json:"validCells,omitempty"`
	DeclineCells    int             `json:"declineCells,omitempty"`
	Limitations     []string        `json:"limitations"`
	Synthetic       bool            `json:"synthetic"`
	FindingGeometry json.RawMessage `json:"findingGeometry,omitempty"`
}
type TileInfo struct {
	Grid              imageryanalysis.Grid `json:"grid"`
	SourceHashes      map[string]string    `json:"sourceHashes"`
	PreparationMethod string               `json:"preparationMethod"`
}
type Job struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"sourceId"`
	Status      string     `json:"status"`
	Input       JobInput   `json:"input"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	Attempt     int        `json:"attempt"`
	WorkerBuild string     `json:"workerBuild"`
	ErrorCode   string     `json:"errorCode"`
	Result      *JobResult `json:"result"`
	FinishedAt  *time.Time `json:"finishedAt"`
	Lease       string     `json:"-"`
}

const jobColumns = `id,source_id,status,input,created_by,created_at,attempt,COALESCE(worker_build,''),error_code,result,finished_at,COALESCE(lease_id,'')`

func scanJob(row pgx.Row) (Job, error) {
	var j Job
	var input, result []byte
	e := row.Scan(&j.ID, &j.SourceID, &j.Status, &input, &j.CreatedBy, &j.CreatedAt, &j.Attempt, &j.WorkerBuild, &j.ErrorCode, &result, &j.FinishedAt, &j.Lease)
	if errors.Is(e, pgx.ErrNoRows) {
		return j, apperr.NotFound("imagery job", "unavailable")
	}
	if e != nil {
		return j, e
	}
	if e = json.Unmarshal(input, &j.Input); e != nil {
		return j, e
	}
	if len(result) > 0 {
		e = json.Unmarshal(result, &j.Result)
	}
	return j, e
}
func (s *Service) Job(ctx context.Context, id string) (Job, error) {
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return Job{}, e
	}
	return scanJob(platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT `+jobColumns+` FROM imagery_jobs WHERE id=$1`, id))
}
func (s *Service) Jobs(ctx context.Context) ([]Job, error) {
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT `+jobColumns+` FROM imagery_jobs ORDER BY created_at DESC,id LIMIT 101`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Service) Submit(ctx context.Context, in SubmitInput) (Job, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Job{}, e
	}
	if len(in.RequestKey) < 1 || len(in.RequestKey) > 128 {
		return Job{}, apperr.Validation("bounded request key required")
	}
	input := JobInput{Schema: "emad.imagery-job.v1", Kind: in.Kind, Algorithm: imageryanalysis.Version, Scenes: []Scene{}, Artifacts: []artifacts.Artifact{}, PreparationIDs: []string{}, Parameters: in.Parameters}
	if in.Kind == "PREPARE" {
		scene, e := s.Scene(ctx, in.SceneID)
		if e != nil {
			return Job{}, e
		}
		artifact, e := s.Artifacts.Ready(ctx, in.ArtifactID)
		if e != nil {
			return Job{}, e
		}
		if artifact.SourceID != scene.SourceID || artifact.Kind != "imagery_subset" || artifact.MediaType != "application/json" {
			return Job{}, apperr.Validation("preparation requires a retained subset from the same admitted source")
		}
		input.Scenes = append(input.Scenes, scene)
		input.Artifacts = append(input.Artifacts, artifact)
	} else if in.Kind == "NDVI_CHANGE" {
		if len(in.PreparationIDs) != 2 || in.PreparationIDs[0] == in.PreparationIDs[1] || in.Parameters.Validate() != nil {
			return Job{}, apperr.Validation("two different prepared scene versions and valid parameters required")
		}
		for _, id := range in.PreparationIDs {
			j, e := s.Job(ctx, id)
			if e != nil {
				return Job{}, e
			}
			if j.Status != "SUCCEEDED" || j.Input.Kind != "PREPARE" || j.Result == nil || len(j.Input.Scenes) != 1 || len(j.Input.Artifacts) != 1 {
				return Job{}, apperr.Conflict("both scene preparations must be complete")
			}
			a, e := s.Artifacts.Ready(ctx, j.Input.Artifacts[0].ID)
			if e != nil {
				return Job{}, e
			}
			input.Scenes = append(input.Scenes, j.Input.Scenes[0])
			input.Artifacts = append(input.Artifacts, a)
			input.PreparationIDs = append(input.PreparationIDs, id)
		}
		b, a := input.Scenes[0], input.Scenes[1]
		if b.SourceID != a.SourceID || b.ID == a.ID || !a.AcquiredAt.After(b.AcquiredAt) || a.AcquiredAt.Sub(b.AcquiredAt) > 90*24*time.Hour || b.Platform != a.Platform || b.Tile != a.Tile || b.Orbit != a.Orbit || b.ProcessingVersion != a.ProcessingVersion {
			return Job{}, apperr.Validation("scene time, source, platform, orbit, tile or processing baseline are incomparable")
		}
	} else {
		return Job{}, apperr.Validation("unsupported imagery job kind")
	}
	for _, scene := range input.Scenes {
		admission, e := s.Artifacts.Admission(ctx, scene.SourceID)
		if e != nil {
			return Job{}, e
		}
		if !scene.Rights.AllowAnalysis || !admission.Rights.AllowAnalysis || !scene.Rights.RetainRaw || (scene.Rights.ExpiresAt != nil && !scene.Rights.ExpiresAt.After(time.Now())) {
			return Job{}, apperr.Forbidden("scene analysis is not permitted by rights")
		}
	}
	p, r, _ := access.From(ctx)
	source := input.Scenes[0].SourceID
	raw, _ := json.Marshal(input)
	if len(raw) > 65536 {
		return Job{}, apperr.Validation("frozen imagery input manifest exceeds 64 KiB")
	}
	hash := artifacts.Hash(raw)
	id := "ijob_" + artifacts.Hash([]byte(r.ID()+"|"+source+"|"+in.RequestKey))
	e := platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":"+source+":imagery-queue"); e != nil {
			return e
		}
		var count int
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM imagery_jobs WHERE source_id=$1 AND status IN ('QUEUED','RUNNING','CANCEL_REQUESTED')`, source).Scan(&count); e != nil {
			return e
		}
		var old string
		e := tx.QueryRow(ctx, `SELECT request_hash FROM imagery_jobs WHERE id=$1`, id).Scan(&old)
		if e == nil {
			if old != hash {
				return apperr.Conflict("job request key reused with different frozen inputs")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if count >= 32 {
			return apperr.Conflict("source imagery queue budget exhausted")
		}
		_, e = tx.Exec(ctx, `INSERT INTO imagery_jobs(id,source_id,kind,status,request_key,request_hash,input,created_by)VALUES($1,$2,$3,'QUEUED',$4,$5,$6,$7)`, id, source, in.Kind, in.RequestKey, hash, raw, p.ID)
		return e
	})
	if e != nil {
		return Job{}, e
	}
	return s.Job(ctx, id)
}
func (s *Service) Cancel(ctx context.Context, id string) (Job, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Job{}, e
	}
	j, e := s.Job(ctx, id)
	if e != nil {
		return j, e
	}
	_, e = platformdb.Scoped(s.Pool).Exec(ctx, `UPDATE imagery_jobs SET status=CASE WHEN status='QUEUED' THEN 'CANCELED' ELSE 'CANCEL_REQUESTED' END,finished_at=CASE WHEN status='QUEUED' THEN now() ELSE NULL END WHERE id=$1 AND status IN ('QUEUED','RUNNING')`, id)
	if e != nil {
		return j, e
	}
	return s.Job(ctx, id)
}
func (s *Service) Retry(ctx context.Context, id string) (Job, error) {
	if e := artifacts.Public(ctx, "imagery.jobs.submit"); e != nil {
		return Job{}, e
	}
	j, e := s.Job(ctx, id)
	if e != nil {
		return j, e
	}
	_, resource, _ := access.From(ctx)
	e = platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":"+j.SourceID+":imagery-queue"); err != nil {
			return err
		}
		current, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM imagery_jobs WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if current.Status != "FAILED" || current.Attempt >= 3 {
			return apperr.Conflict("only failed jobs within the three-attempt budget may be retried")
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM imagery_jobs WHERE source_id=$1 AND status IN ('QUEUED','RUNNING','CANCEL_REQUESTED')`, j.SourceID).Scan(&count); err != nil {
			return err
		}
		if count >= 32 {
			return apperr.Conflict("source imagery queue budget exhausted")
		}
		_, err = tx.Exec(ctx, `UPDATE imagery_jobs SET status='QUEUED',lease_id=NULL,lease_until=NULL,error_code='',finished_at=NULL WHERE id=$1`, id)
		return err
	})
	if e != nil {
		return j, e
	}
	return s.Job(ctx, id)
}
func (s *Service) Attempts(ctx context.Context, id string) ([]json.RawMessage, error) {
	if _, e := s.Job(ctx, id); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT document FROM imagery_job_attempts WHERE job_id=$1 ORDER BY attempt`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Service) claim(ctx context.Context, build string) (Job, error) {
	var j Job
	e := platformdb.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		// Recover abandoned attempts without changing their frozen inputs.
		rows, e := tx.Query(ctx, `SELECT `+jobColumns+` FROM imagery_jobs WHERE status IN ('RUNNING','CANCEL_REQUESTED') AND lease_until<now() FOR UPDATE SKIP LOCKED`)
		if e != nil {
			return e
		}
		var abandoned []Job
		for rows.Next() {
			old, e := scanJob(rows)
			if e != nil {
				rows.Close()
				return e
			}
			abandoned = append(abandoned, old)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()
		for _, old := range abandoned {
			state := "QUEUED"
			if old.Attempt >= 3 {
				state = "FAILED"
			}
			if old.Status == "CANCEL_REQUESTED" {
				state = "CANCELED"
			}
			doc, _ := json.Marshal(map[string]any{"attempt": old.Attempt, "workerBuild": old.WorkerBuild, "status": "INTERRUPTED", "errorCode": "lease_expired", "finishedAt": time.Now().UTC()})
			if _, e = tx.Exec(ctx, `INSERT INTO imagery_job_attempts(job_id,source_id,attempt,document)VALUES($1,$2,$3,$4)ON CONFLICT DO NOTHING`, old.ID, old.SourceID, old.Attempt, doc); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE imagery_jobs SET status=$2,lease_id=NULL,lease_until=NULL,error_code='lease_expired',finished_at=CASE WHEN $2::text IN ('FAILED','CANCELED') THEN now() ELSE NULL END WHERE id=$1`, old.ID, state); e != nil {
				return e
			}
		}
		j, e = scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM imagery_jobs WHERE status='QUEUED' AND attempt<3 ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`))
		if e != nil {
			return e
		}
		j.Lease = ids.New("ilease")
		j.Attempt++
		j.WorkerBuild = build
		j.Status = "RUNNING"
		_, e = tx.Exec(ctx, `UPDATE imagery_jobs SET status='RUNNING',attempt=$2,lease_id=$3,lease_until=now()+interval '150 seconds',worker_build=$4,error_code='',result=NULL,finished_at=NULL WHERE id=$1`, j.ID, j.Attempt, j.Lease, build)
		return e
	})
	return j, e
}
