package recordings

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/jackc/pgx/v5"
)

func sortedKeys[V any](v map[string]V) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func (s *Service) put(ctx context.Context, raw []byte) (Blob, error) {
	if len(raw) > artifacts.MaxBytes {
		return Blob{}, apperr.Conflict("recording artifact budget exceeded")
	}
	r, _ := access.ResourceFrom(ctx)
	ref := Blob{SHA256: artifacts.Hash(raw), Bytes: len(raw)}
	e := s.Store.Put(ctx, "recordings/"+artifacts.Hash([]byte(r.ID()))+"/"+ref.SHA256, raw, "application/json")
	return ref, e
}
func (s *Service) capture(ctx context.Context) ([]Event, error) {
	var out []Event
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var truncated bool
		var watermark int
		if e := tx.QueryRow(ctx, `SELECT event_count,truncated FROM simulation_recording_limits FOR SHARE`).Scan(&watermark, &truncated); e != nil {
			return e
		}
		if truncated {
			return apperr.Conflict("recording capture truncated")
		}
		rows, e := tx.Query(ctx, `SELECT sequence,elapsed_ms,received_at,kind,entity,operation,document FROM simulation_recording_events WHERE sequence<=$1 ORDER BY sequence`, watermark)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v Event
			if e = rows.Scan(&v.Sequence, &v.ElapsedMs, &v.ReceivedAt, &v.Kind, &v.Entity, &v.Operation, &v.Document); e != nil {
				return e
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, e
}
func (s *Service) archive(ctx context.Context, events []Event, m Manifest) (Manifest, error) {
	if len(events) > 100000 {
		return m, apperr.Conflict("archive event budget exceeded")
	}
	m.Schema = Schema
	m.Chunks = []Blob{}
	m.Events = len(events)
	m.ReplayGuarantee = "Ordered recorded output at virtual milliseconds and journal sequence; no command execution or re-simulation. Checkpoints restore selected recorded state only."
	m.CreatedAt = time.Now().UTC()
	raw, _ := json.Marshal(events)
	if len(raw) > MaxArchiveBytes {
		return m, apperr.Conflict("archive byte budget exceeded")
	}
	m.ContentSHA256 = artifacts.Hash(raw)
	chunk := []Event{}
	size := 2
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		if len(m.Chunks) >= 128 {
			return apperr.Conflict("archive chunk budget exceeded")
		}
		raw, _ := json.Marshal(chunk)
		b, e := s.put(ctx, raw)
		if e != nil {
			return e
		}
		b.First = chunk[0].Sequence
		b.Last = chunk[len(chunk)-1].Sequence
		m.Chunks = append(m.Chunks, b)
		chunk = []Event{}
		size = 2
		return nil
	}
	state := map[string]map[string]json.RawMessage{}
	for _, v := range events {
		b, _ := json.Marshal(v)
		if size+len(b)+1 > ChunkBytes {
			if e := flush(); e != nil {
				return m, e
			}
		}
		chunk = append(chunk, v)
		size += len(b) + 1
		apply(state, v)
	}
	if e := flush(); e != nil {
		return m, e
	}
	// A selected-state checkpoint excludes immutable sample histories. Their exact
	// bytes remain in ordered chunks; no checkpoint is executable engine state.
	for _, kind := range []string{"observations", "asset_telemetry", "track_history", "track_observations", "perception"} {
		delete(state, kind)
	}
	if truth := state["truth"]; len(truth) > 1 {
		var latest string
		max := -1
		for k, raw := range truth {
			var v struct {
				Sequence int `json:"sequence"`
			}
			_ = json.Unmarshal(raw, &v)
			if v.Sequence > max {
				latest = k
				max = v.Sequence
			}
		}
		for k := range truth {
			if k != latest {
				delete(truth, k)
			}
		}
	}
	checkpoint := Checkpoint{Schema: Schema, Sequence: len(events), ElapsedMs: m.ElapsedMs, State: state}
	raw, _ = json.Marshal(checkpoint)
	ref, e := s.put(ctx, raw)
	if e != nil {
		return m, e
	}
	m.Checkpoint = ref
	return m, nil
}

// WorkOne is an independent archive/evaluation worker. Its environment-bound
// credential may write sealed artifacts but confers no evaluator read grant.
func (s *Service) WorkOne(ctx context.Context) (bool, error) {
	c, e := access.Refresh(ctx)
	if e != nil {
		return false, e
	}
	if e = simscene.Require(c, "sim.recordings.execute", false); e != nil {
		return false, e
	}
	p, r, _ := access.From(c)
	if p.Kind != "service" {
		return false, apperr.Forbidden("recording worker service required")
	}
	// Queue discovery is metadata only, bounded to the credential's exact environment.
	var scope string
	e = s.Pool.QueryRow(c, `SELECT scope_id FROM simulation_recordings WHERE split_part(scope_id,'/',1)=$1 AND split_part(scope_id,'/',2)=$2 AND split_part(scope_id,'/',3)='EXERCISE_OBSERVATION' AND retain_until>clock_timestamp() AND attempts<$3 AND (status='QUEUED' OR status='FAILED' AND (lease_until IS NULL OR lease_until<=clock_timestamp()) OR status='PROCESSING' AND lease_until<clock_timestamp()) ORDER BY requested_at LIMIT 1`, r.WorkspaceID, r.EnvironmentID, MaxAttempts).Scan(&scope)
	if e == pgx.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	parts := strings.Split(scope, "/")
	if len(parts) != 4 {
		return false, apperr.Conflict("invalid recording scope")
	}
	id := parts[3]
	op := access.Resource{WorkspaceID: parts[0], EnvironmentID: parts[1], Partition: access.ExerciseObservation, RunID: id}
	ordinary := access.Internal(c, op, "recording-archive")
	var attempt int
	var retention time.Time
	e = db.WithTx(ordinary, s.Pool, func(tx pgx.Tx) error {
		e := tx.QueryRow(ordinary, `UPDATE simulation_recordings SET status='PROCESSING',attempts=attempts+1,lease_until=clock_timestamp()+interval '10 minutes',error_code='' WHERE retain_until>clock_timestamp() AND attempts<$1 AND (status='QUEUED' OR status='FAILED' AND (lease_until IS NULL OR lease_until<=clock_timestamp()) OR status='PROCESSING' AND lease_until<clock_timestamp()) RETURNING attempts,retain_until`, MaxAttempts).Scan(&attempt, &retention)
		return e
	})
	if e == pgx.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	// Bound work below the lease so an unavailable store cannot hold the worker
	// indefinitely or publish after another generation becomes eligible.
	job, cancel := context.WithTimeout(c, 9*time.Minute)
	defer cancel()
	workErr := s.finish(job, access.Internal(job, op, "recording-archive"), private(job, op), id, attempt, retention)
	if workErr != nil {
		// Partial immutable blobs remain unreachable and are safe to reuse on retry.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ordinary), 5*time.Second)
		defer cancel()
		retryDelay := 20 * time.Second
		if attempt > 1 {
			retryDelay = time.Minute
		}
		_, markErr := db.Scoped(s.Pool).Exec(cleanup, `UPDATE simulation_recordings SET status='FAILED',lease_until=CASE WHEN attempts<$2 THEN clock_timestamp()+make_interval(secs=>$3) ELSE NULL END,error_code='ARCHIVE_FAILED' WHERE status='PROCESSING' AND attempts=$1`, attempt, MaxAttempts, retryDelay.Seconds())
		if markErr != nil {
			return true, markErr
		}
		return true, workErr
	}
	return true, nil
}
func (s *Service) finish(auth, ordinary, sealed context.Context, id string, attempt int, retention time.Time) error {
	var status string
	var elapsed int64
	var epoch time.Time
	if e := db.Scoped(s.Pool).QueryRow(ordinary, `SELECT status,elapsed_ms,epoch FROM native_exercises`).Scan(&status, &elapsed, &epoch); e != nil {
		return e
	}
	if !terminal(status) {
		return apperr.Conflict("run is not closed")
	}
	var unprojected int
	if e := db.Scoped(s.Pool).QueryRow(sealed, `SELECT count(*) FROM native_projection_batches WHERE NOT projected`).Scan(&unprojected); e != nil {
		return e
	}
	// The broker commits COMPLETED before the final COP projection finishes.
	// Give a healthy publisher one lease window to flush rather than racing that
	// in-flight batch into an unnecessarily partial archive. Failed runs still
	// preserve their actual published prefix without fabricating missing output.
	if status == "COMPLETED" && unprojected > 0 {
		deadline := time.NewTimer(simruntime.LeaseDuration)
		defer deadline.Stop()
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
	settle:
		for unprojected > 0 {
			select {
			case <-auth.Done():
				return auth.Err()
			case <-deadline.C:
				break settle
			case <-tick.C:
				if e := db.Scoped(s.Pool).QueryRow(sealed, `SELECT count(*) FROM native_projection_batches WHERE NOT projected`).Scan(&unprojected); e != nil {
					return e
				}
			}
		}
	}
	oe, e := s.capture(ordinary)
	if e != nil {
		return e
	}
	te, e := s.capture(sealed)
	if e != nil {
		return e
	}
	te, e = s.authorization(auth, id, elapsed, te)
	if e != nil {
		return e
	}
	if len(te) == 0 || te[0].Kind != "configuration" {
		return apperr.Conflict("sealed frozen recording identity unavailable")
	}
	base := Manifest{RunID: id, Epoch: epoch, ElapsedMs: elapsed, Status: status, ProjectionSettled: unprojected == 0, RetainUntil: retention}
	base.Partition = string(access.ExerciseObservation)
	om, e := s.archive(ordinary, oe, base)
	if e != nil {
		return e
	}
	base.Partition = string(access.ExerciseTruth)
	tm, e := s.archive(sealed, te, base)
	if e != nil {
		return e
	}
	report, e := Evaluate(oe, te, status, elapsed, om.ContentSHA256, tm.ContentSHA256)
	if e != nil {
		return e
	}
	if unprojected > 0 {
		report.Limitations = append(report.Limitations, "Gateway projection had not settled at archive time. This archive faithfully retains the published COP prefix; accepted private batches are not treated as delivered observations.")
	}
	// Revocation during artifact I/O prevents publication. A lease generation
	// prevents an abandoned worker from publishing after another attempt takes over.
	fresh, e := access.Refresh(auth)
	if e != nil {
		return e
	}
	if e = access.Require(fresh, "sim.recordings.execute"); e != nil {
		return e
	}
	return db.WithTx(ordinary, s.Pool, func(tx pgx.Tx) error {
		var generation int
		if e := tx.QueryRow(ordinary, `SELECT attempts FROM simulation_recordings WHERE status='PROCESSING' AND lease_until>clock_timestamp() FOR UPDATE`).Scan(&generation); e != nil {
			return e
		}
		if generation != attempt {
			return apperr.Conflict("archive lease lost")
		}
		ob, _ := json.Marshal(om)
		if _, e = tx.Exec(ordinary, `UPDATE simulation_recordings SET status='READY',manifest=$1,lease_until=NULL WHERE attempts=$2`, ob, attempt); e != nil {
			return e
		}
		if e = db.BindTx(sealed, tx); e != nil {
			return e
		}
		tb, _ := json.Marshal(tm)
		if _, e = tx.Exec(sealed, `INSERT INTO simulation_recordings(status,requested_by,manifest,retain_until)VALUES('READY','internal:recording-writer',$1,$2)`, tb, retention); e != nil {
			return e
		}
		rb, _ := json.Marshal(report)
		if _, e = tx.Exec(sealed, `INSERT INTO simulation_evaluations(document)VALUES($1)`, rb); e != nil {
			return e
		}
		return simscene.Audit(sealed, tx, "simulation.recording.archived", id)
	})
}
