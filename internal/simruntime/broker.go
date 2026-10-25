package simruntime

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const LeaseDuration = 6 * time.Second

type Start struct {
	RequestKey     string    `json:"requestKey"`
	SceneID        string    `json:"sceneId"`
	RuntimeVersion string    `json:"runtimeVersion"`
	RuntimeBuild   string    `json:"runtimeBuild"`
	Seed           int64     `json:"seed"`
	Epoch          time.Time `json:"epoch"`
	Scale          float64   `json:"scale"`
	DurationMs     int64     `json:"durationMs"`
}
type Run struct {
	ID            string                   `json:"id"`
	Configuration Start                    `json:"configuration"`
	Status        string                   `json:"status"`
	Desired       string                   `json:"desired"`
	Sequence      int                      `json:"sequence"`
	ElapsedMs     int64                    `json:"elapsedMs"`
	HeartbeatAt   *time.Time               `json:"heartbeatAt"`
	FailureCode   string                   `json:"failureCode"`
	Tasks         []simulation.RuntimeTask `json:"tasks,omitempty"`
}
type Attach struct {
	RuntimeVersion string `json:"runtimeVersion"`
	RuntimeBuild   string `json:"runtimeBuild"`
}
type Lease struct {
	Token       string                         `json:"token"`
	Run         Run                            `json:"run"`
	Scene       simscene.Scene                 `json:"scene"`
	HeartbeatMs int                            `json:"heartbeatMs"`
	Bindings    []simulation.ControlledBinding `json:"bindings,omitempty"`
}
type Beat struct {
	Token       string                     `json:"token"`
	Status      string                     `json:"status"`
	FailureCode string                     `json:"failureCode"`
	Snapshot    Snapshot                   `json:"snapshot"`
	Batch       simulation.PerceptionBatch `json:"batch,omitempty"`
}
type Broker struct {
	AdmittedBuilds []string
	Pool           *pgxpool.Pool
	Scenes         *simscene.Service
	Exercise       ExerciseProjection
	Recorder       interface {
		CaptureTx(context.Context, pgx.Tx, Run, simscene.Scene, Beat) error
	}
}

func isHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && s == hex.EncodeToString(b)
}
func worker(ctx context.Context) error {
	if e := simscene.Require(ctx, "sim.runtime.execute", false); e != nil {
		return e
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "service" {
		return apperr.Forbidden("runtime service identity required")
	}
	return nil
}
func scanRun(row pgx.Row) (Run, error) {
	var r Run
	var raw []byte
	e := row.Scan(&r.ID, &raw, &r.Status, &r.Desired, &r.Sequence, &r.ElapsedMs, &r.HeartbeatAt, &r.FailureCode)
	if errors.Is(e, pgx.ErrNoRows) {
		return r, apperr.NotFound("runtime run", "unavailable")
	}
	if e == nil {
		e = json.Unmarshal(raw, &r.Configuration)
	}
	return r, e
}

const runSelect = `SELECT id,configuration,status,desired,sequence,elapsed_ms,heartbeat_at,failure_code FROM simulation_runtime_runs `

func event(ctx context.Context, tx pgx.Tx, id, kind string) error {
	_, e := tx.Exec(ctx, `INSERT INTO simulation_runtime_events(run_id,kind)VALUES($1,$2)`, id, kind)
	return e
}

// Expiry is materialized on the next request. An expired worker cannot publish
// or attach again; recovery creates a fresh run from the immutable scene.
func (b *Broker) expire(ctx context.Context) error {
	return platformdb.WithTx(ctx, b.Pool, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `UPDATE simulation_runtime_runs SET status='FAILED',desired='STOPPED',failure_code='HEARTBEAT_TIMEOUT',lease_hash=NULL WHERE status IN('RUNNING','PAUSED','STOPPING') AND lease_until<now() RETURNING id`)
		if e != nil {
			return e
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return e
		}
		for _, id := range ids {
			if e = event(ctx, tx, id, "FAILED"); e != nil {
				return e
			}
			if b.Exercise != nil {
				if e = b.Exercise.StateTx(ctx, tx, id, "FAILED", "STOPPED", -1, false); e != nil {
					return e
				}
			}
			if e = simscene.Audit(ctx, tx, "simulation.runtime.timeout", id); e != nil {
				return e
			}
		}
		return nil
	})
}
func (b *Broker) Start(ctx context.Context, in Start) (Run, error) {
	if e := simscene.Require(ctx, "sim.runtime.control", true); e != nil {
		return Run{}, e
	}
	if len(b.AdmittedBuilds) > 0 {
		allowed := false
		for _, build := range b.AdmittedBuilds {
			if build == in.RuntimeBuild {
				allowed = true
			}
		}
		if !allowed {
			return Run{}, apperr.Conflict("runtime build is not admitted; select an available runtime")
		}
	}
	if len(in.RequestKey) < 1 || len(in.RequestKey) > 160 || !simscene.CompatibleRuntime(in.RuntimeVersion) || !isHash(in.RuntimeBuild) || in.DurationMs < 1000 || in.DurationMs > 3600000 || math.IsNaN(in.Scale) || math.IsInf(in.Scale, 0) || in.Scale < 0.1 || in.Scale > 20 || in.Epoch.IsZero() || in.Epoch.Year() < 1970 || in.Epoch.Year() > 2200 || in.Seed < 0 || in.Seed > 9007199254740991 {
		return Run{}, apperr.Validation("invalid runtime version, build, clock, seed or duration")
	}
	scene, e := b.Scenes.Load(ctx, in.SceneID)
	if e != nil {
		return Run{}, e
	}
	if in.RuntimeVersion != scene.Frozen.Runtime || scene.Frozen.Input.SampleBudget(in.DurationMs) > 20000 {
		return Run{}, apperr.Validation("scene runtime or synthetic sample budget mismatch")
	}
	if e := b.expire(ctx); e != nil {
		return Run{}, e
	}
	r, _ := access.ResourceFrom(ctx)
	id := "rt_" + artifacts.Hash([]byte(r.ID()+"|"+in.RequestKey))
	raw, _ := json.Marshal(in)
	// Creating a truth resource grants no reader permission. All later accesses
	// still require either a live evaluator grant or the narrowly bound writer.
	truth := r
	truth.Partition = access.ExerciseTruth
	truth.RunID = id
	e = platformdb.WithTx(ctx, b.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":runtime-quota"); e != nil {
			return e
		}
		old, e := scanRun(tx.QueryRow(ctx, runSelect+`WHERE id=$1`, id))
		if e == nil {
			a, _ := json.Marshal(old.Configuration)
			if string(a) != string(raw) {
				return apperr.Conflict("runtime request key reused with different configuration")
			}
			return nil
		}
		if !apperr.Is(e, apperr.CodeNotFound) {
			return e
		}
		var n, active int
		if e = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status IN('QUEUED','RUNNING','PAUSED','STOPPING')) FROM simulation_runtime_runs`).Scan(&n, &active); e != nil {
			return e
		}
		if n >= 1000 || active >= 4 {
			return apperr.Conflict("runtime environment budget exhausted")
		}
		if _, e = tx.Exec(ctx, `INSERT INTO simulation_runtime_runs(id,scene_id,configuration,status,desired)VALUES($1,$2,$3,'QUEUED','RUNNING')`, id, in.SceneID, raw); e != nil {
			return e
		}
		if e = event(ctx, tx, id, "START"); e != nil {
			return e
		}
		return simscene.Audit(ctx, tx, "simulation.runtime.started", id)
	})
	if e != nil {
		return Run{}, e
	}
	// Only admitted run IDs may create output resources. Rejected starts must
	// not bypass the run quota by accumulating orphaned truth scopes.
	if e = access.NewStore(b.Pool).EnsureResource(ctx, truth); e != nil {
		return Run{}, e
	}
	if b.Exercise != nil && in.RuntimeVersion == simscene.PerceptionRuntime {
		if e = b.Exercise.Prepare(ctx, id, in, scene); e != nil {
			return Run{}, e
		}
	}
	return b.get(ctx, id)
}
func (b *Broker) get(ctx context.Context, id string) (Run, error) {
	return scanRun(platformdb.Scoped(b.Pool).QueryRow(ctx, runSelect+`WHERE id=$1`, id))
}
func (b *Broker) Get(ctx context.Context, id string) (Run, error) {
	if e := simscene.Require(ctx, "sim.runtime.control", false); e != nil {
		return Run{}, e
	}
	if e := b.expire(ctx); e != nil {
		return Run{}, e
	}
	if b.Exercise != nil {
		if e := b.Exercise.Reconcile(ctx, id); e != nil {
			return Run{}, e
		}
	}
	return b.get(ctx, id)
}
func (b *Broker) List(ctx context.Context) ([]Run, error) {
	if e := simscene.Require(ctx, "sim.runtime.control", false); e != nil {
		return nil, e
	}
	if e := b.expire(ctx); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(b.Pool).Query(ctx, runSelect+`ORDER BY created_at DESC,id LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		v, e := scanRun(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (b *Broker) Control(ctx context.Context, id, action string) (Run, error) {
	if e := simscene.Require(ctx, "sim.runtime.control", true); e != nil {
		return Run{}, e
	}
	if e := b.expire(ctx); e != nil {
		return Run{}, e
	}
	desired, kind := "", ""
	switch action {
	case "pause":
		desired, kind = "PAUSED", "PAUSE"
	case "resume":
		desired, kind = "RUNNING", "RESUME"
	case "stop":
		desired, kind = "STOPPED", "STOP"
	default:
		return Run{}, apperr.Validation("unknown runtime control")
	}
	e := platformdb.WithTx(ctx, b.Pool, func(tx pgx.Tx) error {
		r, e := scanRun(tx.QueryRow(ctx, runSelect+`WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return e
		}
		if r.Status == "STOPPED" || r.Status == "COMPLETED" || r.Status == "FAILED" || r.Status == "STOPPING" {
			if action == "stop" && r.Desired == "STOPPED" {
				return nil
			}
			return apperr.Conflict("run is terminal or stopping")
		}
		if r.Desired == desired {
			return nil
		}
		var n int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM simulation_runtime_events WHERE run_id=$1`, id).Scan(&n); e != nil {
			return e
		}
		if n >= 128 && action != "stop" {
			return apperr.Conflict("runtime control event budget exhausted")
		}
		status := r.Status
		if desired == "STOPPED" {
			status = "STOPPING"
			if r.Status == "QUEUED" {
				status = "STOPPED"
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE simulation_runtime_runs SET desired=$2,status=$3 WHERE id=$1`, id, desired, status); e != nil {
			return e
		}
		if e = event(ctx, tx, id, kind); e != nil {
			return e
		}
		if b.Exercise != nil {
			if e = b.Exercise.StateTx(ctx, tx, id, status, desired, r.ElapsedMs, false); e != nil {
				return e
			}
		}
		return simscene.Audit(ctx, tx, "simulation.runtime."+action, id)
	})
	if e != nil {
		return Run{}, e
	}
	if b.Exercise != nil {
		if e := b.Exercise.Reconcile(ctx, id); e != nil {
			return Run{}, e
		}
	}
	return b.get(ctx, id)
}
func (b *Broker) Attach(ctx context.Context, id string, in Attach) (Lease, error) {
	if e := worker(ctx); e != nil {
		return Lease{}, e
	}
	if e := b.expire(ctx); e != nil {
		return Lease{}, e
	}
	r, e := b.get(ctx, id)
	if e != nil {
		return Lease{}, e
	}
	if in.RuntimeVersion != r.Configuration.RuntimeVersion || in.RuntimeBuild != r.Configuration.RuntimeBuild {
		return Lease{}, apperr.Conflict("runtime version/build negotiation failed")
	}
	scene, e := b.Scenes.Load(ctx, r.Configuration.SceneID)
	if e != nil {
		return Lease{}, e
	}
	if b.Exercise != nil && r.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
		if e = b.Exercise.Prepare(ctx, id, r.Configuration, scene); e != nil {
			return Lease{}, e
		}
	}
	// Idempotently repair a creation interrupted between run admission and
	// resource registration before any worker can receive the lease.
	truth, _ := access.ResourceFrom(ctx)
	truth.Partition = access.ExerciseTruth
	truth.RunID = id
	if e = access.NewStore(b.Pool).EnsureResource(ctx, truth); e != nil {
		return Lease{}, e
	}
	// Refresh after loading dependencies, before binding exclusive ownership.
	fresh, e := access.Refresh(ctx)
	if e != nil {
		return Lease{}, e
	}
	if e = worker(fresh); e != nil {
		return Lease{}, e
	}
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		return Lease{}, e
	}
	token := hex.EncodeToString(secret)
	p, _, _ := access.From(fresh)
	e = platformdb.WithTx(fresh, b.Pool, func(tx pgx.Tx) error {
		tag, e := tx.Exec(fresh, `UPDATE simulation_runtime_runs SET status=CASE WHEN desired='PAUSED' THEN 'PAUSED' ELSE 'RUNNING' END,worker_id=$2,lease_hash=$3,lease_until=now()+interval '6 seconds',heartbeat_at=now() WHERE id=$1 AND status='QUEUED' AND worker_id IS NULL`, id, p.ID, artifacts.Hash([]byte(token)))
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("run is already attached, terminal or stopping")
		}
		if e = event(fresh, tx, id, "ATTACH"); e != nil {
			return e
		}
		if b.Exercise != nil {
			status := "RUNNING"
			if r.Desired == "PAUSED" {
				status = "PAUSED"
			}
			if e = b.Exercise.StateTx(fresh, tx, id, status, r.Desired, 0, true); e != nil {
				return e
			}
		}
		return simscene.Audit(fresh, tx, "simulation.runtime.attached", id)
	})
	if e != nil {
		return Lease{}, e
	}
	r, e = b.get(fresh, id)
	var bindings []simulation.ControlledBinding
	if e == nil && b.Exercise != nil && r.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
		bindings, e = b.Exercise.Bindings(fresh, id)
	}
	return Lease{Token: token, Run: r, Scene: scene, HeartbeatMs: 500, Bindings: bindings}, e
}
func (b *Broker) Heartbeat(ctx context.Context, id string, in Beat) (Run, error) {
	if e := worker(ctx); e != nil {
		return Run{}, e
	}
	if !isHash(in.Token) || in.Snapshot.RunID != id || in.Snapshot.Sequence < 1 || in.Snapshot.Sequence > 8000 || len(in.Snapshot.Actors) > 16 {
		return Run{}, apperr.Validation("invalid bounded runtime heartbeat")
	}
	if in.Status != "RUNNING" && in.Status != "PAUSED" && in.Status != "STOPPED" && in.Status != "COMPLETED" && in.Status != "FAILED" {
		return Run{}, apperr.Validation("invalid runtime status")
	}
	if in.Status == "FAILED" {
		if in.FailureCode != "RUNTIME_ERROR" && in.FailureCode != "INTERRUPTED" && in.FailureCode != "CONTRACT_ERROR" {
			return Run{}, apperr.Validation("invalid failure code")
		}
	} else if in.FailureCode != "" {
		return Run{}, apperr.Validation("unexpected failure detail")
	}
	raw, _ := json.Marshal(in.Snapshot)
	hash := artifacts.Hash(append(append(raw, []byte(in.Status)...), []byte(in.FailureCode)...))
	batchRaw, _ := json.Marshal(in.Batch)
	if len(in.Batch.Readings)+len(in.Batch.Outcomes) > 0 {
		hash = artifacts.Hash(append([]byte(hash), batchRaw...))
	}
	if len(batchRaw) > 262144 || len(in.Batch.Readings) > 512 || len(in.Batch.Outcomes) > 64 {
		return Run{}, apperr.Validation("perception batch exceeds admitted budget")
	}
	p, resource, _ := access.From(ctx)
	e := platformdb.WithTx(ctx, b.Pool, func(tx pgx.Tx) error {
		r, e := scanRun(tx.QueryRow(ctx, runSelect+`WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return e
		}
		var owner, leaseHash, lastHash string
		var leaseValid bool
		var heartbeatAgeMs float64
		// Lease and publication wall budgets use one database clock. Comparing a
		// container timestamp with the API host clock can reject valid snapshots.
		if e = tx.QueryRow(ctx, `SELECT COALESCE(worker_id,''),COALESCE(lease_hash,''),last_hash,COALESCE(lease_until>clock_timestamp(),false),GREATEST(0,EXTRACT(epoch FROM(clock_timestamp()-COALESCE(heartbeat_at,clock_timestamp())))*1000)::float8 FROM simulation_runtime_runs WHERE id=$1`, id).Scan(&owner, &leaseHash, &lastHash, &leaseValid, &heartbeatAgeMs); e != nil {
			return e
		}
		if owner != p.ID || subtle.ConstantTimeCompare([]byte(leaseHash), []byte(artifacts.Hash([]byte(in.Token)))) != 1 || !leaseValid {
			return apperr.Forbidden("runtime lease unavailable or expired")
		}
		if in.Snapshot.SceneID != r.Configuration.SceneID || in.Snapshot.RuntimeBuild != r.Configuration.RuntimeBuild {
			return apperr.Conflict("snapshot frozen identity mismatch")
		}
		if in.Snapshot.Sequence == r.Sequence && hash == lastHash {
			return nil
		}
		if r.Status == "STOPPED" || r.Status == "FAILED" || r.Status == "COMPLETED" {
			return apperr.Conflict("run is terminal")
		}
		if in.Snapshot.Sequence != r.Sequence+1 || in.Snapshot.ElapsedMs < r.ElapsedMs || in.Snapshot.ElapsedMs > r.Configuration.DurationMs {
			return apperr.Conflict("snapshot sequence or virtual time mismatch")
		}
		// Publishers cannot outrun the admitted wall/virtual budget. A paused worker
		// may acknowledge an already in-flight pause at the next bounded snapshot.
		if float64(in.Snapshot.ElapsedMs-r.ElapsedMs) > heartbeatAgeMs*r.Configuration.Scale+1500*r.Configuration.Scale {
			return apperr.Conflict("runtime clock exceeded admitted scale")
		}
		if in.Status == "COMPLETED" && in.Snapshot.ElapsedMs != r.Configuration.DurationMs || in.Status == "STOPPED" && r.Desired != "STOPPED" {
			return apperr.Conflict("runtime acknowledgement conflicts with desired state")
		}
		// These are contract and budget checks only. No simulation is evaluated here.
		scene, e := b.Scenes.Load(ctx, r.Configuration.SceneID)
		if e != nil {
			return e
		}
		if e = ValidateBatchIdentity(in.Batch, in.Token); e != nil {
			return apperr.Validation(e.Error())
		}
		if b.Exercise != nil && r.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
			if e = b.Exercise.ValidateTx(ctx, tx, id, in.Batch); e != nil {
				return e
			}
		}
		if e = ValidateBatch(scene.Frozen, in.Batch, in.Snapshot.ElapsedMs); e != nil {
			return apperr.Validation(e.Error())
		}
		if len(in.Snapshot.Actors) != len(scene.Frozen.Input.Actors) {
			return apperr.Validation("snapshot actor set mismatch")
		}
		for i, a := range in.Snapshot.Actors {
			if a.ID != scene.Frozen.Input.Actors[i].ID || math.IsNaN(a.Heading) || math.IsInf(a.Heading, 0) || a.Heading < 0 || a.Heading >= 360 {
				return apperr.Validation("invalid runtime actor state")
			}
			if _, e = scene.Frozen.Environment.Frame.FromLocal(a.Local); e != nil || math.Sqrt(a.Local.X*a.Local.X+a.Local.Y*a.Local.Y+a.Local.Z*a.Local.Z) > simscene.Radius {
				return apperr.Validation("runtime position outside scene budget")
			}
		}
		status := in.Status
		if r.Desired == "STOPPED" && status != "STOPPED" && status != "FAILED" {
			status = "STOPPING"
		}
		tag, e := tx.Exec(ctx, `UPDATE simulation_runtime_runs SET status=$2,sequence=$3,last_hash=$4,elapsed_ms=$5,heartbeat_at=now(),lease_until=now()+interval '6 seconds',failure_code=$6,sample_count=sample_count+$7 WHERE id=$1 AND sample_count+$7<=20000`, id, status, in.Snapshot.Sequence, hash, in.Snapshot.ElapsedMs, in.FailureCode, len(in.Batch.Readings))
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("native sample budget exhausted")
		}
		if status == "STOPPED" || status == "COMPLETED" || status == "FAILED" {
			if e = event(ctx, tx, id, status); e != nil {
				return e
			}
			if e = simscene.Audit(ctx, tx, "simulation.runtime."+status, id); e != nil {
				return e
			}
		}
		if b.Exercise != nil {
			if e = b.Exercise.StateTx(ctx, tx, id, status, r.Desired, in.Snapshot.ElapsedMs, true); e != nil {
				return e
			}
		}
		// The sealed latest snapshot is an explicit separate payload and partition.
		// The role switch occurs in this transaction after the validated lease update.
		truth := resource
		truth.Partition = access.ExerciseTruth
		truth.RunID = id
		writer := access.Principal{ID: "internal:runtime-truth-writer", Kind: "service", Grants: []access.Grant{{Resource: truth, Permissions: []string{"sim.truth.read", "sim.truth.record"}}}}
		c, e := access.Bind(ctx, writer, truth, nil)
		if e != nil {
			return e
		}
		if e = platformdb.BindTx(c, tx); e != nil {
			return e
		}
		_, e = tx.Exec(c, `INSERT INTO simulation_runtime_truth(document)VALUES($1) ON CONFLICT(scope_id)DO UPDATE SET document=excluded.document,updated_at=now()`, raw)
		if e == nil && len(in.Batch.Readings)+len(in.Batch.Outcomes) > 0 {
			_, e = tx.Exec(c, `INSERT INTO native_projection_batches(sequence,document)VALUES($1,$2)`, in.Snapshot.Sequence, batchRaw)
		}
		if e == nil && b.Recorder != nil && r.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
			e = b.Recorder.CaptureTx(c, tx, r, scene, in)
		}
		return e
	})
	if e != nil {
		return Run{}, e
	}
	run, e := b.get(ctx, id)
	if e == nil && b.Exercise != nil && run.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
		scene, loadErr := b.Scenes.Load(ctx, run.Configuration.SceneID)
		if loadErr != nil {
			return Run{}, loadErr
		}
		e = b.Exercise.Project(ctx, run, scene)
	}
	if e == nil && b.Exercise != nil && run.Configuration.RuntimeVersion == simscene.PerceptionRuntime {
		run.Tasks, e = b.Exercise.Tasks(ctx, run)
	}
	return run, e
}
func (b *Broker) Truth(ctx context.Context, id string) (Snapshot, error) {
	if e := access.Require(ctx, "sim.truth.read"); e != nil {
		return Snapshot{}, e
	}
	_, r, _ := access.From(ctx)
	if r.Partition != access.ExerciseTruth || r.RunID != id {
		return Snapshot{}, apperr.Forbidden("explicit run truth resource required")
	}
	var out Snapshot
	e := platformdb.WithTx(ctx, b.Pool, func(tx pgx.Tx) error {
		var raw []byte
		e := tx.QueryRow(ctx, `SELECT document FROM simulation_runtime_truth`).Scan(&raw)
		if errors.Is(e, pgx.ErrNoRows) {
			return apperr.NotFound("snapshot", "unavailable")
		}
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &out); e != nil {
			return e
		}
		return simscene.Audit(ctx, tx, "simulation.runtime.truth.read", id)
	})
	return out, e
}
