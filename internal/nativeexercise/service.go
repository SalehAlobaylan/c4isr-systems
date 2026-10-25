// Package nativeexercise projects authenticated native runtime output through
// the existing evidence and controlled-operations services. It owns no physics.
package nativeexercise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/missions"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/scenarios"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool         *pgxpool.Pool
	Sources      *sources.Service
	Assets       *assets.Service
	Observations *observations.Service
	Telemetry    *telemetry.Service
	Commands     *commands.Service
	Missions     *missions.Service
}

func resource(ctx context.Context, id string, partition access.Partition) access.Resource {
	r, _ := access.ResourceFrom(ctx)
	r.RunID = id
	r.Partition = partition
	return r
}
func public(ctx context.Context, id string) context.Context {
	return access.Internal(ctx, resource(ctx, id, access.ExerciseObservation), "native-exercise-projection")
}
func truth(ctx context.Context, id string) context.Context {
	r := resource(ctx, id, access.ExerciseTruth)
	c, _ := access.Bind(ctx, access.Principal{ID: "internal:native-private-writer", Kind: "service", Grants: []access.Grant{{Resource: r, Permissions: []string{"sim.truth.read", "sim.truth.record"}}}}, r, nil)
	return c
}
func (s *Service) Prepare(ctx context.Context, id string, in simruntime.Start, scene simscene.Scene) error {
	if e := access.NewStore(s.Pool).EnsureResource(ctx, resource(ctx, id, access.ExerciseObservation)); e != nil {
		return e
	}
	var initialStatus, initialDesired string
	var initialElapsed int64
	if e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT status,desired,elapsed_ms FROM simulation_runtime_runs WHERE id=$1`, id).Scan(&initialStatus, &initialDesired, &initialElapsed); e != nil {
		return e
	}
	c := public(ctx, id)
	_, e := platformdb.Scoped(s.Pool).Exec(c, `INSERT INTO native_exercises(epoch,status,desired,runtime_version,scale,elapsed_ms)VALUES($1,$2,$3,$4,$5,$6)ON CONFLICT(scope_id)DO NOTHING`, in.Epoch, initialStatus, initialDesired, in.RuntimeVersion, in.Scale, initialElapsed)
	if e != nil {
		return e
	}
	if view := simscene.PublicEnvironment(scene, id); view != nil {
		raw, err := json.Marshal(view)
		if err != nil {
			return err
		}
		_, err = platformdb.Scoped(s.Pool).Exec(c, `INSERT INTO native_environment_views(scope_id,document)VALUES($1,$2)ON CONFLICT DO NOTHING`, resource(ctx, id, access.ExerciseObservation).ID(), raw)
		if err != nil {
			return err
		}
	}
	for i := range scene.Frozen.Input.Sensors {
		if _, _, e = s.Sources.Ensure(c, sources.CreateInput{ID: fmt.Sprintf("native-sensor-%d", i), Name: "Synthetic exercise observer", Type: sources.TypeSynthetic}); e != nil {
			return e
		}
	}
	if len(scene.Frozen.Input.Controlled) > 0 {
		if _, _, e = s.Sources.Ensure(c, sources.CreateInput{ID: "native-self-report", Name: "Synthetic controlled asset telemetry", Type: sources.TypeSynthetic}); e != nil {
			return e
		}
	}
	for _, declared := range scene.Frozen.Input.Controlled {
		// Random public asset IDs are retained only in the sealed actor map.
		var assetID string
		e = platformdb.Scoped(s.Pool).QueryRow(truth(ctx, id), `SELECT contact_ref FROM simulation_actor_mappings WHERE message_id=$1`, "controlled-actor:"+declared.ActorID).Scan(&assetID)
		if errors.Is(e, pgx.ErrNoRows) {
			candidate := access.ResourceID(c, "asset", ids.New("ast"))
			_, e = platformdb.Scoped(s.Pool).Exec(truth(ctx, id), `INSERT INTO simulation_actor_mappings(message_id,actor_id,contact_ref,kind)VALUES($1,$2,$3,'telemetry')ON CONFLICT DO NOTHING`, "controlled-actor:"+declared.ActorID, declared.ActorID, candidate)
			if e == nil {
				e = platformdb.Scoped(s.Pool).QueryRow(truth(ctx, id), `SELECT contact_ref FROM simulation_actor_mappings WHERE message_id=$1`, "controlled-actor:"+declared.ActorID).Scan(&assetID)
			}
		}
		if e != nil {
			return e
		}
		if _, _, e = s.Assets.Ensure(c, assets.CreateInput{ID: assetID, Name: declared.Callsign, Type: "simulated", Capabilities: []string{"HOLD", "RESUME_ROUTE"}, Metadata: map[string]any{"synthetic": true}}); e != nil {
			return e
		}
		_, e = platformdb.Scoped(s.Pool).Exec(c, `INSERT INTO native_controlled_assets(asset_id)VALUES($1)ON CONFLICT DO NOTHING`, assetID)
		if e != nil {
			return e
		}
	}
	return nil
}

// StateTx makes run closure immediately visible to the command-admission
// trigger, in the same transaction as the private broker lifecycle update.
func (s *Service) StateTx(ctx context.Context, tx pgx.Tx, id, status, desired string, elapsed int64, heartbeat bool) error {
	c := public(ctx, id)
	if e := platformdb.BindTx(c, tx); e != nil {
		return e
	}
	_, e := tx.Exec(c, `UPDATE native_exercises SET status=$1,desired=$2,elapsed_ms=CASE WHEN $3::bigint>=0 THEN $3 ELSE elapsed_ms END,lease_until=CASE WHEN $4 THEN clock_timestamp()+interval '6 seconds' ELSE lease_until END`, status, desired, elapsed, heartbeat)
	if e != nil {
		return e
	}
	return platformdb.BindTx(ctx, tx)
}
func (s *Service) Bindings(ctx context.Context, id string) ([]simulation.ControlledBinding, error) {
	rows, e := platformdb.Scoped(s.Pool).Query(truth(ctx, id), `SELECT actor_id,contact_ref FROM simulation_actor_mappings WHERE message_id LIKE 'controlled-actor:%' ORDER BY message_id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []simulation.ControlledBinding{}
	for rows.Next() {
		var b simulation.ControlledBinding
		if e = rows.Scan(&b.ActorID, &b.AssetID); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Service) Clock(ctx context.Context, wall time.Time) time.Time {
	r, e := access.ResourceFrom(ctx)
	if e != nil || r.Partition != access.ExerciseObservation {
		return wall
	}
	var epoch time.Time
	var elapsed int64
	if e = platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT epoch,elapsed_ms FROM native_exercises`).Scan(&epoch, &elapsed); e != nil {
		return wall
	}
	return epoch.Add(time.Duration(elapsed) * time.Millisecond)
}
func (s *Service) GetRun(ctx context.Context, id string) (scenarios.Run, error) {
	if e := access.Require(ctx, "scenarios.read"); e != nil {
		return scenarios.Run{}, e
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.ExerciseObservation || r.RunID != id {
		return scenarios.Run{}, apperr.NotFound("native run", "unavailable")
	}
	var run scenarios.Run
	var epoch time.Time
	var version, desired string
	var valid bool
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT status,desired,epoch,elapsed_ms,runtime_version,scale,started_at,COALESCE(lease_until>clock_timestamp(),false) FROM native_exercises`).Scan(&run.Status, &desired, &epoch, &run.VirtualTimeMs, &version, &run.PlaybackSpeed, &run.StartedAt, &valid)
	if errors.Is(e, pgx.ErrNoRows) {
		return run, apperr.NotFound("native run", "unavailable")
	}
	if e != nil {
		return run, e
	}
	if !valid && (run.Status == "RUNNING" || run.Status == "PAUSED" || run.Status == "STOPPING") {
		run.Status = "FAILED"
	}
	if e = s.Reconcile(ctx, id); e != nil {
		return run, e
	}
	run.ID = id
	run.ScenarioName = "Native exercise"
	run.Frozen = scenarios.FrozenRun{Runtime: version, Clock: scenarios.ClockConfig{Epoch: epoch, ElapsedUnit: "milliseconds", Order: "virtual_time_then_sequence", SourceTimeMapping: "preserve_source_time"}}
	return run, nil
}
func (s *Service) Project(ctx context.Context, run simruntime.Run, scene simscene.Scene) error {
	fresh, e := access.Refresh(ctx)
	if e != nil {
		return e
	}
	ctx = fresh
	if e = access.Require(ctx, "sim.runtime.execute"); e != nil {
		return e
	}
	// Non-blocking advisory ownership serializes projection and retry without
	// holding a row transaction while the domain services use their own commits.
	conn, e := s.Pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	r, _ := access.ResourceFrom(ctx)
	key := r.WorkspaceID + "/" + r.EnvironmentID + "/" + run.ID
	var locked bool
	if e = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&locked); e != nil {
		return e
	}
	if !locked {
		return apperr.Conflict("native projection is already active; retry identical heartbeat")
	}
	defer func() {
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, _ = conn.Exec(work, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key)
	}()
	private := truth(ctx, run.ID)
	c := public(ctx, run.ID)
	c = freshness.WithClock(c, func(context.Context, time.Time) time.Time {
		return run.Configuration.Epoch.Add(time.Duration(run.ElapsedMs) * time.Millisecond)
	})
	rows, e := platformdb.Scoped(s.Pool).Query(private, `SELECT sequence,document FROM native_projection_batches WHERE NOT projected AND sequence<=$1 ORDER BY sequence LIMIT 16`, run.Sequence)
	if e != nil {
		return e
	}
	type pending struct {
		seq   int
		batch simulation.PerceptionBatch
	}
	batches := []pending{}
	for rows.Next() {
		var p pending
		var raw []byte
		if e = rows.Scan(&p.seq, &raw); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal(raw, &p.batch); e != nil {
			rows.Close()
			return e
		}
		batches = append(batches, p)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	for _, p := range batches {
		// Failure/stop cancels unpublished evidence. COMPLETED flushes its final
		// already admitted samples, preserving sample time separately from receipt.
		var status, desired string
		if e = platformdb.Scoped(s.Pool).QueryRow(c, `SELECT status,desired FROM native_exercises`).Scan(&status, &desired); e != nil {
			return e
		}
		if desired != "STOPPED" && status != "FAILED" && status != "STOPPED" && status != "STOPPING" {
			for _, reading := range p.batch.Readings {
				if e = s.publish(c, private, run, scene, reading); e != nil {
					return e
				}
			}
			for _, outcome := range p.batch.Outcomes {
				if e = s.outcome(c, outcome); e != nil {
					return e
				}
			}
		}
		if _, e = platformdb.Scoped(s.Pool).Exec(private, `UPDATE native_projection_batches SET projected=true WHERE sequence=$1`, p.seq); e != nil {
			return e
		}
	}
	return s.expireTasks(c, run.ElapsedMs)
}
func (s *Service) publish(ctx, private context.Context, run simruntime.Run, scene simscene.Scene, r simulation.Reading) error {
	if r.Kind == "telemetry" {
		var asset string
		if e := platformdb.Scoped(s.Pool).QueryRow(private, `SELECT contact_ref FROM simulation_actor_mappings WHERE message_id=$1`, "controlled-actor:"+r.ActorID).Scan(&asset); e != nil {
			return e
		}
		if asset != r.AssetID {
			return apperr.Forbidden("controlled telemetry recipient mismatch")
		}
	}
	mapping := "contact:" + r.SensorID + ":" + r.ActorID
	if r.Kind == "telemetry" {
		mapping = "controlled-actor:" + r.ActorID
	}
	_, e := platformdb.Scoped(s.Pool).Exec(private, `INSERT INTO simulation_actor_mappings(message_id,actor_id,contact_ref,kind)VALUES($1,$2,$3,$4)ON CONFLICT DO NOTHING`, mapping, r.ActorID, r.Contact, r.Kind)
	if e != nil {
		return e
	}
	var old string
	if e = platformdb.Scoped(s.Pool).QueryRow(private, `SELECT contact_ref FROM simulation_actor_mappings WHERE message_id=$1`, mapping).Scan(&old); e != nil {
		return e
	}
	if old != r.Contact {
		return apperr.Conflict("synthetic contact identity changed")
	}
	position, e := scene.Frozen.Environment.Frame.FromLocal(r.Local)
	if e != nil {
		return e
	}
	point := &geo.Point{Lat: position.Latitude, Lng: position.Longitude}
	observed := run.Configuration.Epoch.Add(time.Duration(r.SampleMs) * time.Millisecond)
	payload := map[string]any{"synthetic": true, "scenarioRunId": run.ID}
	if r.Kind == "observation" {
		slot := -1
		for i, sensor := range scene.Frozen.Input.Sensors {
			if sensor.ID == r.SensorID {
				slot = i
			}
		}
		source := access.ResourceID(ctx, "source", fmt.Sprintf("native-sensor-%d", slot))
		_, e = s.Observations.Ingest(ctx, observations.CreateInput{ID: r.ID, SourceID: source, Type: "synthetic.observation", SemanticKind: observations.Moving, ObservedAt: observed, Position: point, TrackHint: r.Contact, Payload: payload, Quality: map[string]any{"confidence": r.Confidence, "horizontalAccuracyM": r.AccuracyM}})
	} else {
		source := access.ResourceID(ctx, "source", "native-self-report")
		heading := r.Heading
		_, e = s.Telemetry.Ingest(ctx, telemetry.CreateInput{ID: r.ID, MessageID: r.ID, AssetID: r.AssetID, SourceID: source, ObservedAt: observed, Position: point, Heading: &heading, ConnectionState: "connected", Health: "nominal", Payload: payload})
	}
	return e
}
