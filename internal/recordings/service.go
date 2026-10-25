package recordings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool  *pgxpool.Pool
	Store artifacts.Store
}

var ViewPermissions = []string{"sources.read", "assets.read", "telemetry.read", "observations.read", "tracks.read", "classifications.read", "geofences.read", "alerts.read", "incidents.read", "missions.read", "commands.read", "scenarios.read"}

func (s *Service) authorize(ctx context.Context, id, permission string) (context.Context, error) {
	fresh, e := access.Refresh(ctx)
	if e != nil {
		return nil, e
	}
	_, r, _ := access.From(fresh)
	if r.RunID != id || (r.Partition != access.ExerciseObservation && r.Partition != access.ExerciseTruth) {
		access.Deny(fresh, permission, "explicit recording resource mismatch")
		return nil, apperr.Forbidden("explicit recording run resource required")
	}
	if e = access.Require(fresh, permission); e != nil {
		return nil, e
	}
	if r.Partition == access.ExerciseTruth {
		if e = access.Require(fresh, "sim.truth.read"); e != nil {
			return nil, e
		}
	} else {
		if len(access.SourceIDs(fresh)) > 0 {
			access.Deny(fresh, permission, "recording requires unrestricted source grant")
			return nil, apperr.Forbidden("recording requires an unrestricted source grant for this run")
		}
		for _, p := range ViewPermissions {
			if e = access.Require(fresh, p); e != nil {
				return nil, e
			}
		}
	}
	return fresh, nil
}
func private(ctx context.Context, r access.Resource) context.Context {
	r.Partition = access.ExerciseTruth
	c, _ := access.Bind(ctx, access.Principal{ID: "internal:recording-writer", Kind: "service", Grants: []access.Grant{{Resource: r, Permissions: []string{"sim.truth.read", "sim.truth.record"}}}}, r, nil)
	return c
}

// CaptureTx is called only after lease, scene and batch validation, inside the
// heartbeat transaction with an explicitly bound truth writer. No token persists.
func (s *Service) CaptureTx(ctx context.Context, tx pgx.Tx, run simruntime.Run, scene simscene.Scene, in simruntime.Beat) error {
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.ExerciseTruth || r.RunID != run.ID {
		return apperr.Forbidden("sealed recording writer required")
	}
	appendEvent := func(kind, key string, v any) error {
		raw, e := json.Marshal(v)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `SELECT emad_record_event($1,$2,$3,$4,'INSERT',$5)`, r.ID(), in.Snapshot.ElapsedMs, kind, key, raw)
		return e
	}
	var exists bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM simulation_recording_events WHERE kind='configuration')`).Scan(&exists); e != nil {
		return e
	}
	if !exists {
		if e := appendEvent("configuration", "frozen", map[string]any{"configuration": run.Configuration, "scene": scene, "recordingSchema": Schema, "checkpointCompatibility": "recorded-output-only; matching schema and content hashes; no process takeover"}); e != nil {
			return e
		}
	}
	if e := appendEvent("truth", strconv.Itoa(in.Snapshot.Sequence), in.Snapshot); e != nil {
		return e
	}
	if len(in.Batch.Readings)+len(in.Batch.Outcomes) > 0 {
		return appendEvent("perception", strconv.Itoa(in.Snapshot.Sequence), in.Batch)
	}
	return nil
}
func (s *Service) Request(ctx context.Context, id string) (Index, error) {
	c, e := s.authorize(ctx, id, "sim.recordings.manage")
	if e != nil {
		return Index{}, e
	}
	p, r, _ := access.From(c)
	if p.Kind != "human" || r.Partition != access.ExerciseObservation {
		return Index{}, apperr.Forbidden("human exercise recording request required")
	}
	e = db.WithTx(c, s.Pool, func(tx pgx.Tx) error {
		var status string
		var started bool
		if e := tx.QueryRow(c, `SELECT status FROM native_exercises FOR UPDATE`).Scan(&status); e != nil {
			return e
		}
		if status != "STOPPED" && status != "COMPLETED" && status != "FAILED" {
			return apperr.Conflict("close the native run before archiving")
		}
		if e := tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM simulation_recording_events WHERE kind='native_exercises' AND operation='INSERT')`).Scan(&started); e != nil {
			return e
		}
		if !started {
			return apperr.Conflict("complete recording unavailable: start a new run after migration 37")
		}
		if _, e := tx.Exec(c, `INSERT INTO simulation_recordings(status,requested_by)VALUES('QUEUED',$1)ON CONFLICT DO NOTHING`, p.ID); e != nil {
			return e
		}
		return simscene.Audit(c, tx, "simulation.recording.requested", id)
	})
	if e != nil {
		return Index{}, e
	}
	return s.index(c)
}
func (s *Service) index(ctx context.Context) (Index, error) {
	out := Index{MaxAttempts: MaxAttempts}
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT status,error_code,attempts,CASE WHEN status='FAILED' AND attempts<$1 THEN lease_until END,manifest FROM simulation_recordings WHERE retain_until>clock_timestamp()`, MaxAttempts).Scan(&out.Status, &out.ErrorCode, &out.Attempts, &out.RetryAt, &raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("recording", "unavailable")
	}
	if e == nil && len(raw) > 0 {
		out.Manifest = &Manifest{}
		e = json.Unmarshal(raw, out.Manifest)
	}
	return out, e
}
func (s *Service) Index(ctx context.Context, id string) (Index, error) {
	c, e := s.authorize(ctx, id, "sim.recordings.read")
	if e != nil {
		return Index{}, e
	}
	out, e := s.index(c)
	if e != nil {
		return out, e
	}
	e = s.audit(c, "simulation.recording.read", id)
	return out, e
}
func (s *Service) audit(ctx context.Context, action, id string) error {
	// Blob reads can outlast an authorization or retention change. Recheck before
	// returning any recorded output, including privileged checkpoint/evaluation
	// content; the entry check alone cannot authorize a delayed response.
	expected, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	permission := "sim.recordings.read"
	if action == "simulation.evaluation.read" {
		permission = "sim.evaluation.read"
	}
	fresh, e := s.authorize(ctx, id, permission)
	if e != nil {
		return e
	}
	current, _ := access.ResourceFrom(fresh)
	if current != expected {
		return apperr.Forbidden("recording resource changed during read")
	}
	if _, e = s.index(fresh); e != nil {
		return e
	}
	return db.WithTx(fresh, s.Pool, func(tx pgx.Tx) error { return simscene.Audit(fresh, tx, action, id) })
}
func (s *Service) blob(ctx context.Context, ref Blob) ([]byte, error) {
	r, _ := access.ResourceFrom(ctx)
	key := "recordings/" + artifacts.Hash([]byte(r.ID())) + "/" + ref.SHA256
	raw, e := s.Store.Get(ctx, key, int64(ref.Bytes))
	if e != nil {
		return nil, apperr.Conflict("recording artifact unavailable or failed integrity")
	}
	if artifacts.Hash(raw) != ref.SHA256 {
		return nil, apperr.Conflict("recording integrity check failed")
	}
	return raw, nil
}
func (s *Service) readEvents(ctx context.Context, m Manifest) ([]Event, error) {
	if m.Schema != Schema || len(m.Chunks) > 128 || m.Events > 100000 {
		return nil, apperr.Conflict("incompatible recording schema or budget")
	}
	out := []Event{}
	size := 0
	for _, chunk := range m.Chunks {
		raw, e := s.blob(ctx, chunk)
		if e != nil {
			return nil, e
		}
		size += len(raw)
		if size > MaxArchiveBytes {
			return nil, apperr.Conflict("recording byte budget exceeded")
		}
		var events []Event
		if e = json.Unmarshal(raw, &events); e != nil {
			return nil, e
		}
		for _, v := range events {
			if v.Sequence != len(out)+1 {
				return nil, apperr.Conflict("recording sequence integrity failed")
			}
			out = append(out, v)
		}
	}
	if len(out) != m.Events {
		return nil, apperr.Conflict("recording event count mismatch")
	}
	raw, _ := json.Marshal(out)
	if artifacts.Hash(raw) != m.ContentSHA256 {
		return nil, apperr.Conflict("recording content identity mismatch")
	}
	return out, nil
}
func (s *Service) Events(ctx context.Context, id string, after int) ([]Event, error) {
	c, e := s.authorize(ctx, id, "sim.recordings.read")
	if e != nil {
		return nil, e
	}
	if after < 0 || after > 100000 {
		return nil, apperr.Validation("invalid recording cursor")
	}
	idx, e := s.index(c)
	if e != nil {
		return nil, e
	}
	if idx.Status != "READY" || idx.Manifest == nil {
		return nil, apperr.Conflict("recording is not ready")
	}
	out := []Event{}
	for _, chunk := range idx.Manifest.Chunks {
		if chunk.Last <= after {
			continue
		}
		raw, e := s.blob(c, chunk)
		if e != nil {
			return nil, e
		}
		var events []Event
		if e = json.Unmarshal(raw, &events); e != nil {
			return nil, e
		}
		for _, v := range events {
			if v.Sequence > after {
				out = append(out, v)
				if len(out) == 100 {
					break
				}
			}
		}
		if len(out) == 100 {
			break
		}
	}
	if e = s.audit(c, "simulation.recording.events.read", id); e != nil {
		return nil, e
	}
	return out, nil
}
func (s *Service) Frame(ctx context.Context, id string, ms int64, sequence int) (Frame, error) {
	c, e := s.authorize(ctx, id, "sim.recordings.read")
	if e != nil {
		return Frame{}, e
	}
	idx, e := s.index(c)
	if e != nil {
		return Frame{}, e
	}
	if idx.Status != "READY" || idx.Manifest == nil {
		return Frame{}, apperr.Conflict("recording is not ready")
	}
	m := *idx.Manifest
	if ms < 0 || ms > m.ElapsedMs || sequence < 0 || sequence > m.Events {
		return Frame{}, apperr.Validation("replay position outside recording")
	}
	out := Frame{Schema: Schema, ElapsedMs: ms, State: map[string]map[string]json.RawMessage{}, Counts: map[string]int{}}
	events, e := s.readEvents(c, m)
	if e != nil {
		return out, e
	}
	for _, v := range events {
		if v.ElapsedMs > ms || sequence > 0 && v.Sequence > sequence {
			continue
		}
		apply(out.State, v)
		out.Sequence = v.Sequence
	}
	// Histories remain paged through Events. A frame bounds each displayed layer.
	for kind, rows := range out.State {
		out.Counts[kind] = len(rows)
		if len(rows) > 200 {
			out.Truncated = true
			keys := sortedKeys(rows)
			for _, key := range keys[:len(keys)-200] {
				delete(rows, key)
			}
		}
	}
	raw, _ := json.Marshal(out)
	if len(raw) > 8*1024*1024 {
		return Frame{}, apperr.Conflict("frame exceeds budget; use paged events")
	}
	if e = s.audit(c, "simulation.recording.replay.read", id); e != nil {
		return Frame{}, e
	}
	return out, nil
}
func terminal(s string) bool { return s == "STOPPED" || s == "COMPLETED" || s == "FAILED" }
func parseInt(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	n, e := strconv.Atoi(v)
	if e != nil {
		return 0, apperr.Validation(fmt.Sprintf("invalid integer %q", v))
	}
	return n, nil
}
