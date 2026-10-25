// Package exerciseingest is the typed application boundary for the initial
// headless adapter. It accepts no public evidence or unscoped actor state.
package exerciseingest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Stamp struct {
	Schema            string `json:"schema"`
	RunID             string `json:"runId"`
	ScenarioVersionID string `json:"scenarioVersionId"`
	RuntimeVersion    string `json:"runtimeVersion"`
	RuntimeBuild      string `json:"runtimeBuild"`
	ProducerID        string `json:"producerId"`
	Sequence          int    `json:"sequence"`
	ElapsedMs         int64  `json:"elapsedMs"`
}
type Message struct {
	Stamp
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ObservedAt time.Time `json:"observedAt"`
	Input      any       `json:"input"`
}
type Service struct {
	pool      *pgxpool.Pool
	obs       *observations.Service
	telemetry *telemetry.Service
}

func NewService(pool *pgxpool.Pool, obs *observations.Service, tm *telemetry.Service) *Service {
	return &Service{pool, obs, tm}
}
func writer(ctx context.Context) (access.Resource, error) {
	p, r, ok := access.From(ctx)
	if !ok || r.Partition != access.ExerciseObservation || p.Kind != "service" || p.ID != "internal:scenario-runner" {
		return r, apperr.Forbidden("synthetic ingress requires the scoped runner identity")
	}
	return r, access.Require(ctx, "exercise.ingest")
}

// Numeric measurement quality is observable; arbitrary author strings and
// scenario instructions stay in the protected frozen definition.
func quality(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"confidence", "positionConfidence", "accuracyM", "horizontalAccuracyM", "verticalAccuracyM"} {
		if v, ok := in[key]; ok {
			switch v.(type) {
			case float64, int, int64, json.Number:
				out[key] = v
			}
		}
	}
	return out
}
func (s *Service) Observation(ctx context.Context, stamp Stamp, actor, contact string, in observations.CreateInput) (observations.IngestResult, error) {
	in.Payload = map[string]any{"synthetic": true, "scenarioRunId": stamp.RunID}
	in.Quality = quality(in.Quality)
	msg := Message{Stamp: stamp, ID: in.ID, Kind: "observation", ObservedAt: in.ObservedAt, Input: in}
	if err := s.prepare(ctx, msg, actor, contact); err != nil {
		return observations.IngestResult{}, err
	}
	result, err := s.obs.Ingest(ctx, in)
	finishErr := s.finish(ctx, msg.ID, err)
	if err != nil {
		return result, err
	}
	return result, finishErr
}
func (s *Service) Telemetry(ctx context.Context, stamp Stamp, actor string, in telemetry.CreateInput) (telemetry.IngestResult, error) {
	in.Payload = map[string]any{"synthetic": true, "scenarioRunId": stamp.RunID}
	msg := Message{Stamp: stamp, ID: in.ID, Kind: "telemetry", ObservedAt: in.ObservedAt, Input: in}
	if err := s.prepare(ctx, msg, actor, in.AssetID); err != nil {
		return telemetry.IngestResult{}, err
	}
	result, err := s.telemetry.Ingest(ctx, in)
	finishErr := s.finish(ctx, msg.ID, err)
	if err != nil {
		return result, err
	}
	return result, finishErr
}
func (s *Service) prepare(ctx context.Context, m Message, actor, contact string) error {
	resource, err := writer(ctx)
	if err != nil {
		return err
	}
	if m.Schema != "emad.synthetic.v1" || m.RunID != resource.RunID || m.Sequence < 1 || m.ElapsedMs < 0 || m.ElapsedMs > 2678400000 || m.ScenarioVersionID == "" || m.ProducerID != "yaml-adapter" || actor == "" || len(actor) > 4096 {
		return apperr.Validation("invalid synthetic run/time/producer envelope")
	}
	raw, err := json.Marshal(m)
	if err != nil || len(raw) > 32768 {
		return apperr.Validation("synthetic input exceeds 32 KiB")
	}
	hash := reference.Hash(raw)
	err = platformdb.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM scenario_runs WHERE id=$1 AND frozen_run->>'versionId'=$2 AND frozen_run->>'runtime'=$3 AND frozen_run->>'runtimeBuild'=$4)`, m.RunID, m.ScenarioVersionID, m.RuntimeVersion, m.RuntimeBuild).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return apperr.Forbidden("synthetic input does not match frozen run")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO exercise_messages(id,kind,sequence,elapsed_ms,version_id,input_hash,document,status) VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING') ON CONFLICT(scope_id,id) DO NOTHING`, m.ID, m.Kind, m.Sequence, m.ElapsedMs, m.ScenarioVersionID, hash, raw); err != nil {
			return err
		}
		var old string
		if err := tx.QueryRow(ctx, `SELECT input_hash FROM exercise_messages WHERE id=$1`, m.ID).Scan(&old); err != nil {
			return err
		}
		if old != hash {
			return apperr.Conflict("synthetic message identity reused with different input")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// A narrow writer is issued here only after validating the selected frozen
	// run. Generic background/projection authority still cannot select truth.
	truth := resource
	truth.Partition = access.ExerciseTruth
	principal := access.Principal{ID: "internal:simulation-mapping-writer", Kind: "service", Grants: []access.Grant{{Resource: truth, Permissions: []string{"sim.truth.read", "sim.truth.record"}}}}
	truthCtx, err := access.Bind(ctx, principal, truth, nil)
	if err != nil {
		return err
	}
	return platformdb.WithTx(truthCtx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(truthCtx, `INSERT INTO simulation_actor_mappings(message_id,actor_id,contact_ref,kind) VALUES($1,$2,$3,$4) ON CONFLICT(scope_id,message_id) DO NOTHING`, m.ID, actor, contact, m.Kind); err != nil {
			return err
		}
		var oldActor, oldContact, oldKind string
		if err := tx.QueryRow(truthCtx, `SELECT actor_id,contact_ref,kind FROM simulation_actor_mappings WHERE message_id=$1`, m.ID).Scan(&oldActor, &oldContact, &oldKind); err != nil {
			return err
		}
		if oldActor != actor || oldContact != contact || oldKind != m.Kind {
			return apperr.Conflict("synthetic message mapping differs from its original mapping")
		}
		return nil
	})
}
func (s *Service) finish(ctx context.Context, id string, cause error) error {
	status, code := "DELIVERED", ""
	if cause != nil {
		status = "REJECTED"
		code = string(apperr.CodeOf(cause))
	}
	_, err := platformdb.Scoped(s.pool).Exec(ctx, `UPDATE exercise_messages SET status=$2,processed_at=now(),error_code=$3 WHERE id=$1`, id, status, code)
	return err
}

// Receipt is safe for an ordinary exercise operator; it contains no actor map.
type Receipt struct {
	Stamp
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ObservedAt time.Time `json:"observedAt"`
	ReceivedAt time.Time `json:"receivedAt"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"errorCode,omitempty"`
}

func (s *Service) Receipts(ctx context.Context, run string) ([]Receipt, error) {
	if err := access.Require(ctx, "observations.read"); err != nil {
		return nil, err
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.ExerciseObservation || r.RunID != run {
		return nil, apperr.Forbidden("selected exercise run required")
	}
	rows, err := platformdb.Scoped(s.pool).Query(ctx, `SELECT m.document,m.status,m.received_at,m.error_code,COALESCE(o.public_id,m.id) FROM exercise_messages m LEFT JOIN observations o ON o.id=m.id AND o.scope_id=m.scope_id ORDER BY m.sequence,m.id LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Receipt{}
	for rows.Next() {
		var raw []byte
		var rec Receipt
		var m Message
		if err = rows.Scan(&raw, &rec.Status, &rec.ReceivedAt, &rec.ErrorCode, &rec.ID); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		rec.Stamp = m.Stamp
		rec.Kind = m.Kind
		rec.ObservedAt = m.ObservedAt
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Mappings is an evaluator application port, never mounted on ordinary HTTP/WS.
func (s *Service) Mappings(ctx context.Context) ([]map[string]string, error) {
	if err := access.Require(ctx, "sim.truth.read"); err != nil {
		return nil, err
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.ExerciseTruth {
		return nil, apperr.Forbidden("truth resource required")
	}
	rows, err := platformdb.Scoped(s.pool).Query(ctx, `SELECT message_id,actor_id,contact_ref,kind FROM simulation_actor_mappings ORDER BY message_id LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var m, a, c, k string
		if err = rows.Scan(&m, &a, &c, &k); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"messageId": m, "actorId": a, "contactRef": c, "kind": k})
	}
	return out, rows.Err()
}
