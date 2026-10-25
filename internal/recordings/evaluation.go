package recordings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
)

type Metric struct {
	Name        string   `json:"name"`
	Value       *float64 `json:"value"`
	Numerator   float64  `json:"numerator"`
	Denominator int      `json:"denominator"`
	Unit        string   `json:"unit"`
	Definition  string   `json:"definition"`
}
type Evaluation struct {
	Version      string   `json:"version"`
	TimeBasis    string   `json:"timeBasis"`
	OperatorHash string   `json:"operatorHash"`
	TruthHash    string   `json:"truthHash"`
	RunStatus    string   `json:"runStatus"`
	ElapsedMs    int64    `json:"elapsedMs"`
	Metrics      []Metric `json:"metrics"`
	Limitations  []string `json:"limitations"`
}

func ratio(name string, n, d int, definition string) Metric {
	m := Metric{Name: name, Numerator: float64(n), Denominator: d, Unit: "fraction", Definition: definition}
	if d > 0 {
		v := float64(n) / float64(d)
		m.Value = &v
	}
	return m
}
func measured(name string, total float64, n int, unit, definition string) Metric {
	m := Metric{Name: name, Numerator: total, Denominator: n, Unit: unit, Definition: definition}
	if n > 0 {
		v := total / float64(n)
		m.Value = &v
	}
	return m
}

// Evaluate consumes immutable journals only. Expected sensor slots derive from
// frozen actor lifecycle, not intermittent truth snapshots or present COP state.
func Evaluate(operator, sealed []Event, status string, elapsed int64, oh, th string) (Evaluation, error) {
	out := Evaluation{Version: MetricVersion, TimeBasis: "integer virtual milliseconds; received_at is independent wall receipt time", OperatorHash: oh, TruthHash: th, RunStatus: status, ElapsedMs: elapsed, Metrics: []Metric{}, Limitations: []string{
		"Abstract scheduled observers only; no physical sensor validation or weapon effectiveness claim.",
		"Expected slots include active actors whose configured delivery time falls within the recorded interval; scheduled misses are included and reported separately.",
		"Lateness threshold is configured delivery plus 1000 virtual ms; gateway publication may add delay at accelerated clock scales.",
		"Position error is available only where an active sealed snapshot matches the exact sample millisecond; no interpolated position is inferred.",
		"Track errors count recorded merge/fragmentation relationships, not semantic classification correctness. Missing denominators are null, never zero scores.",
		"Scenario completion means runtime COMPLETED; workflow completion counts command outcomes independently. Cancelled/failed runs remain partial exercises.",
	}}
	var config struct {
		Configuration simruntime.Start `json:"configuration"`
		Scene         simscene.Scene   `json:"scene"`
	}
	if len(sealed) == 0 || sealed[0].Kind != "configuration" {
		return out, fmt.Errorf("recording configuration unavailable")
	}
	if e := json.Unmarshal(sealed[0].Document, &config); e != nil {
		return out, e
	}
	if e := config.Scene.Frozen.Input.ValidateSensors(); e != nil {
		return out, e
	}
	if elapsed < 0 || elapsed > 3600000 {
		return out, fmt.Errorf("evaluation interval outside admitted budget")
	}
	expected, scheduled := 0, 0
	for _, sensor := range config.Scene.Frozen.Input.Sensors {
		for _, actor := range config.Scene.Frozen.Input.Actors {
			for at := int64(0); at+sensor.DelayMs <= elapsed; at += sensor.PeriodMs {
				if at < actor.SpawnMs || at >= actor.DespawnMs {
					continue
				}
				expected++
				if sensor.DropEvery > 0 && (at/sensor.PeriodMs+1)%int64(sensor.DropEvery) == 0 {
					scheduled++
				}
			}
		}
	}
	readings := map[string]simulation.Reading{}
	duplicates := 0
	outcomes := map[string]map[string]int64{}
	truth := map[int64]map[string]simulation.Vector{}
	for _, event := range sealed {
		if event.Kind == "truth" {
			var snapshot simruntime.Snapshot
			if e := json.Unmarshal(event.Document, &snapshot); e != nil {
				return out, e
			}
			positions := map[string]simulation.Vector{}
			for _, a := range snapshot.Actors {
				if a.Active {
					positions[a.ID] = a.Local
				}
			}
			truth[snapshot.ElapsedMs] = positions
		}
		if event.Kind != "perception" {
			continue
		}
		var batch simulation.PerceptionBatch
		if e := json.Unmarshal(event.Document, &batch); e != nil {
			return out, e
		}
		for _, r := range batch.Readings {
			if _, ok := readings[r.ID]; ok {
				duplicates++
			}
			readings[r.ID] = r
		}
		for _, o := range batch.Outcomes {
			if outcomes[o.CommandID] == nil {
				outcomes[o.CommandID] = map[string]int64{}
			}
			if _, ok := outcomes[o.CommandID][o.State]; !ok {
				outcomes[o.CommandID][o.State] = o.ElapsedMs
			}
		}
	}
	published := map[string]Event{}
	publicActor := map[string]string{}
	trackActors := map[string]map[string]bool{}
	actorTracks := map[string]map[string]bool{}
	issued := map[string]int64{}
	states := map[string]string{}
	firstAck := map[string]int64{}
	ended := map[string]int64{}
	for _, event := range operator {
		var row map[string]any
		if e := json.Unmarshal(event.Document, &row); e != nil {
			return out, e
		}
		if event.Kind == "observations" && event.Operation == "INSERT" {
			id := sampleReference(event.Entity)
			published[id] = event
			if r, ok := readings[id]; ok {
				publicActor[fmt.Sprint(row["id"])] = r.ActorID
			}
		}
		if event.Kind == "commands" {
			id := fmt.Sprint(row["id"])
			state := fmt.Sprint(row["state"])
			if _, ok := issued[id]; !ok {
				issued[id] = event.ElapsedMs
			}
			states[id] = state
			if state == "ACKNOWLEDGED" {
				if _, ok := firstAck[id]; !ok {
					firstAck[id] = event.ElapsedMs
				}
			}
			if commandTerminal(state) {
				ended[id] = event.ElapsedMs
			}
		}
	}
	for _, event := range operator {
		if event.Kind != "track_observations" || event.Operation == "DELETE" {
			continue
		}
		var row map[string]any
		_ = json.Unmarshal(event.Document, &row)
		actor := publicActor[fmt.Sprint(row["observation_id"])]
		track := fmt.Sprint(row["track_id"])
		if actor == "" {
			continue
		}
		if trackActors[track] == nil {
			trackActors[track] = map[string]bool{}
		}
		trackActors[track][actor] = true
		if actorTracks[actor] == nil {
			actorTracks[actor] = map[string]bool{}
		}
		actorTracks[actor][track] = true
	}
	delivered, late, exactErrors := 0, 0, 0
	errorTotal, delayTotal := 0.0, 0.0
	// Keep floating-point reductions stable across map iteration orders. Sorting
	// the finished metric list does not make the preceding sums reproducible.
	for _, id := range sortedKeys(published) {
		event := published[id]
		r, ok := readings[id]
		if !ok || r.Kind != "observation" {
			continue
		}
		delivered++
		if event.ElapsedMs > r.DeliverMs+1000 {
			late++
		}
		delayTotal += float64(event.ElapsedMs - r.SampleMs)
		if p, ok := truth[r.SampleMs][r.ActorID]; ok {
			dx, dy := r.Local.X-p.X, r.Local.Y-p.Y
			errorTotal += math.Sqrt(dx*dx + dy*dy)
			exactErrors++
		}
	}
	missing := expected - delivered
	if missing < 0 {
		missing = 0
	}
	merges, fragments := 0, 0
	for _, actors := range trackActors {
		if len(actors) > 1 {
			merges++
		}
	}
	for _, tracks := range actorTracks {
		if len(tracks) > 1 {
			fragments++
		}
	}
	ackTotal, completeTotal := 0.0, 0.0
	acks, completed, terminalCount := 0, 0, 0
	for id, state := range states {
		at := issued[id]
		ack, ok := outcomes[id]["ACKNOWLEDGED"]
		if !ok {
			ack, ok = firstAck[id]
		}
		if ok {
			ackTotal += float64(ack - at)
			acks++
		}
		if commandTerminal(state) {
			terminalCount++
		}
		if state == "COMPLETED" {
			end, ok := outcomes[id]["COMPLETED"]
			if !ok {
				end = ended[id]
			}
			completeTotal += float64(end - at)
			completed++
		}
	}
	out.Metrics = []Metric{
		ratio("missed_observations", missing, expected, "Expected active actor/sensor slots due by final virtual time without a recorded published observation; includes configured scheduled misses."),
		ratio("scheduled_misses", scheduled, expected, "Frozen dropEvery slots among expected slots."),
		ratio("late_observations", late, delivered, "Published observations received by the COP more than 1000 virtual ms after configured delivery."),
		measured("mean_observation_delay", delayTotal, delivered, "virtual_ms", "Mean COP publication virtual time minus original sample time."),
		measured("mean_horizontal_error", errorTotal, exactErrors, "m", "Mean local horizontal error only where a sealed active snapshot exists at the exact sample millisecond."),
		ratio("merged_tracks", merges, len(trackActors), "Recorded tracks containing observations from more than one sealed actor."),
		ratio("fragmented_actors", fragments, len(actorTracks), "Observed actors associated with more than one recorded track."),
		ratio("duplicate_samples", duplicates, len(readings)+duplicates, "Repeated exact sample IDs in accepted sealed batches; rejected gateway retries are not new accepted events."),
		ratio("command_terminal", terminalCount, len(states), "Commands ending COMPLETED, REJECTED, FAILED, TIMED_OUT or CANCELLED."),
		ratio("command_completed", completed, len(states), "Commands with recorded final state COMPLETED."),
		measured("mean_command_ack", ackTotal, acks, "virtual_ms", "Mean first runtime acknowledgment elapsed time minus command issue time; no-ACK commands excluded."),
		measured("mean_command_completion", completeTotal, completed, "virtual_ms", "Mean runtime completion elapsed time minus command issue time for completed commands."),
	}
	completion := 0
	if status == "COMPLETED" {
		completion = 1
	}
	out.Metrics = append(out.Metrics, ratio("scenario_completed", completion, 1, "Run reached declared duration with runtime status COMPLETED."))
	// Stable result byte order, independent of map iteration or worker retries.
	sort.Slice(out.Metrics, func(i, j int) bool { return out.Metrics[i].Name < out.Metrics[j].Name })
	return out, nil
}

// Observation ingestion adds its resource namespace. The original sample ID
// remains lease-keyed opaque data; strip only the known native observation prefix.
func sampleReference(id string) string {
	const separator = "__observation__"
	if i := strings.LastIndex(id, separator); i >= 0 {
		suffix := id[i+len(separator):]
		if strings.HasPrefix(suffix, "sample_") && len(suffix) == 39 {
			return suffix
		}
	}
	return id
}
func commandTerminal(s string) bool {
	return s == "COMPLETED" || s == "REJECTED" || s == "FAILED" || s == "TIMED_OUT" || s == "CANCELLED"
}
func (s *Service) Evaluation(ctx context.Context, id string) (Evaluation, error) {
	c, e := s.authorize(ctx, id, "sim.evaluation.read")
	if e != nil {
		return Evaluation{}, e
	}
	r, _ := access.ResourceFrom(c)
	if r.Partition != access.ExerciseTruth {
		return Evaluation{}, apperr.Forbidden("privileged evaluation resource required")
	}
	idx, e := s.index(c)
	if e != nil {
		return Evaluation{}, e
	}
	if idx.Status != "READY" {
		return Evaluation{}, apperr.Conflict("evaluation is not ready")
	}
	var raw []byte
	var out Evaluation
	if e = db.Scoped(s.Pool).QueryRow(c, `SELECT document FROM simulation_evaluations`).Scan(&raw); e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return out, e
	}
	e = s.audit(c, "simulation.evaluation.read", id)
	return out, e
}

type Release struct {
	Metrics []string `json:"metrics"`
	Reason  string   `json:"reason"`
}
type ReleasedReport struct {
	Version    string    `json:"version"`
	Metrics    []Metric  `json:"metrics"`
	Reason     string    `json:"reason"`
	ReleasedBy string    `json:"releasedBy"`
	ReleasedAt time.Time `json:"releasedAt"`
}

func (s *Service) Release(ctx context.Context, id string, in Release) (ReleasedReport, error) {
	c, e := s.authorize(ctx, id, "sim.evaluation.release")
	if e != nil {
		return ReleasedReport{}, e
	}
	p, r, _ := access.From(c)
	if p.Kind != "human" || r.Partition != access.ExerciseTruth {
		return ReleasedReport{}, apperr.Forbidden("human evaluator release required")
	}
	if len(in.Metrics) < 1 || len(in.Metrics) > 13 || len(in.Reason) < 1 || len(in.Reason) > 500 {
		return ReleasedReport{}, apperr.Validation("select bounded numeric metrics and release reason")
	}
	evaluation, e := s.Evaluation(c, id)
	if e != nil {
		return ReleasedReport{}, e
	}
	report := ReleasedReport{Version: evaluation.Version, Metrics: []Metric{}, Reason: in.Reason, ReleasedBy: p.ID, ReleasedAt: time.Now().UTC()}
	seen := map[string]bool{}
	for _, name := range in.Metrics {
		if seen[name] {
			return report, apperr.Validation("duplicate released metric")
		}
		seen[name] = true
		found := false
		for _, metric := range evaluation.Metrics {
			if metric.Name == name {
				report.Metrics = append(report.Metrics, metric)
				found = true
			}
		}
		if !found {
			return report, apperr.Validation("unknown released metric")
		}
	}
	// The only declassification path copies these explicitly selected numeric
	// aggregates. No actor IDs, paths, sample mappings or frozen inputs are copied.
	r.Partition = access.ExerciseObservation
	ordinary := access.Internal(c, r, "evaluation-release")
	e = db.WithTx(c, s.Pool, func(tx pgx.Tx) error {
		if e := simscene.Audit(c, tx, "simulation.evaluation.released", id); e != nil {
			return e
		}
		if e := db.BindTx(ordinary, tx); e != nil {
			return e
		}
		raw, _ := json.Marshal(report)
		tag, e := tx.Exec(ordinary, `INSERT INTO simulation_released_reports(document)VALUES($1)ON CONFLICT DO NOTHING`, raw)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 1 {
			return nil
		}
		var oldRaw []byte
		var old ReleasedReport
		if e = tx.QueryRow(ordinary, `SELECT document FROM simulation_released_reports`).Scan(&oldRaw); e != nil {
			return e
		}
		if e = json.Unmarshal(oldRaw, &old); e != nil {
			return e
		}
		a, _ := json.Marshal(old.Metrics)
		b, _ := json.Marshal(report.Metrics)
		if old.Reason != report.Reason || old.ReleasedBy != report.ReleasedBy || string(a) != string(b) {
			return apperr.Conflict("a different immutable operator summary has already been released")
		}
		report = old
		return nil
	})
	return report, e
}
func (s *Service) Released(ctx context.Context, id string) (ReleasedReport, error) {
	c, e := s.authorize(ctx, id, "sim.recordings.read")
	if e != nil {
		return ReleasedReport{}, e
	}
	r, _ := access.ResourceFrom(c)
	if r.Partition != access.ExerciseObservation {
		return ReleasedReport{}, apperr.Forbidden("exercise report resource required")
	}
	var out ReleasedReport
	var raw []byte
	e = db.Scoped(s.Pool).QueryRow(c, `SELECT document FROM simulation_released_reports`).Scan(&raw)
	if e == pgx.ErrNoRows {
		return out, apperr.NotFound("released report", "unavailable")
	}
	if e == nil {
		e = json.Unmarshal(raw, &out)
	}
	return out, e
}
