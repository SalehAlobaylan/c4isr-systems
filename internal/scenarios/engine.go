package scenarios

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assessments"
	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/incidents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
	"github.com/SalehAlobaylan/c4isr-systems/internal/tracks"
)

// EngineDeps are the application services the runner feeds. The runner uses
// exactly the same ingestion boundaries as future external systems.
type EngineDeps struct {
	Sources         *sources.Service
	Assets          *assets.Service
	Observations    *observations.Service
	Telemetry       *telemetry.Service
	Geofences       *geospatial.Service
	Classifications *classifications.Service
	Assessments     *assessments.Service
	Incidents       *incidents.Service
	Commands        *commands.Service
	Tracks          *tracks.Service
	Logger          *slog.Logger
}

type scheduledAction struct {
	AtMs int64
	Seq  int
	Name string
	Run  func(ctx context.Context) error
}

type dynamicAction struct {
	DelayMs int64
	Name    string
	Run     func(ctx context.Context) error
}

const maxInt64Value = int64(1<<63 - 1)

type motionSegment struct {
	FromMs int64
	ToMs   int64
	From   geo.Point
	To     geo.Point
}

// entityMotion is the deterministic piecewise-linear path of one simulated
// entity. It exists only inside the runner, mirroring what a future external
// simulator would compute on its own.
type entityMotion struct {
	start    geo.Point
	hasStart bool
	segments []motionSegment
}

func (m *entityMotion) at(ms int64) (geo.Point, bool) {
	if m == nil {
		return geo.Point{}, false
	}
	if len(m.segments) == 0 {
		if m.hasStart {
			return m.start, true
		}
		return geo.Point{}, false
	}
	first := m.segments[0]
	if ms <= first.FromMs {
		if m.hasStart {
			return m.start, true
		}
		return first.From, true
	}
	for _, seg := range m.segments {
		if ms <= seg.ToMs {
			span := float64(seg.ToMs - seg.FromMs)
			if span <= 0 {
				return seg.To, true
			}
			frac := float64(ms-seg.FromMs) / span
			return geo.Point{
				Lat: seg.From.Lat + (seg.To.Lat-seg.From.Lat)*frac,
				Lng: seg.From.Lng + (seg.To.Lng-seg.From.Lng)*frac,
			}, true
		}
	}
	last := m.segments[len(m.segments)-1]
	return last.To, true
}

func (m *entityMotion) lastAt() int64 {
	if m == nil || len(m.segments) == 0 {
		return 0
	}
	return m.segments[len(m.segments)-1].ToMs
}

// engine executes one scenario run. Virtual time advances action-by-action so
// the sequence of emitted facts is deterministic even under pause or speed
// changes; wall-clock time is used only to pace the run and timestamp inputs.
type engine struct {
	runID  string
	scope  RunScope
	sc     *Scenario
	deps   EngineDeps
	repo   Repository
	logger *slog.Logger
	rng    *rand.Rand

	assetIDs     map[string]bool
	trackRefs    map[string]bool
	incidentRefs map[string]string

	mu            sync.Mutex
	paused        bool
	stopped       bool
	speed         float64
	virtualBaseMs float64
	lastResume    time.Time
	wake          chan struct{}

	static     []scheduledAction
	nextStatic int
	pending    []scheduledAction
	finished   bool
	reserved   int

	seq               map[string]int
	nextSeq           int
	observationIDs    map[string]string
	classificationIDs map[string]string
	pendingCommands   map[string]struct{}
	lastProgress      time.Time
	onProgress        func(ms int64)
	lastAction        string
	lastActionAt      int64
	actionError       string
	eventsRun         int
	eventStates       map[int]EventInspection
}

func newEngine(runID string, scope RunScope, sc *Scenario, seed int64, speed float64, deps EngineDeps, repo Repository, onProgress func(int64)) *engine {
	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		speed = defaultPlaybackSpeed
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &engine{
		runID:             runID,
		scope:             scope,
		sc:                sc,
		deps:              deps,
		repo:              repo,
		logger:            logger.With("run_id", runID, "scenario", sc.Name),
		rng:               rand.New(rand.NewSource(seed)),
		assetIDs:          map[string]bool{},
		trackRefs:         map[string]bool{},
		incidentRefs:      map[string]string{},
		eventStates:       map[int]EventInspection{},
		observationIDs:    map[string]string{},
		classificationIDs: map[string]string{},
		pendingCommands:   map[string]struct{}{},
		speed:             speed,
		lastResume:        time.Now(),
		wake:              make(chan struct{}, 1),
		seq:               map[string]int{},
		onProgress:        onProgress,
	}
}

type engineSnapshot struct {
	LastAction   string
	LastActionAt int64
	ActionError  string
	EventsRun    int
	EventsTotal  int
}

func (e *engine) snapshot() engineSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return engineSnapshot{
		LastAction:   e.lastAction,
		LastActionAt: e.lastActionAt,
		ActionError:  e.actionError,
		EventsRun:    e.eventsRun,
		EventsTotal:  len(e.eventStates),
	}
}

func (e *engine) snapshotEvents() []EventInspection {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]EventInspection, 0, len(e.eventStates))
	for _, item := range e.eventStates {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AtMs != out[j].AtMs {
			return out[i].AtMs < out[j].AtMs
		}
		return out[i].Sequence < out[j].Sequence
	})
	return out
}

// ownsAsset reports whether the run simulates the given asset.
func (e *engine) ownsAsset(assetID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.assetIDs[assetID]
}

// build precomputes the entire deterministic timeline.
func (e *engine) build() error {
	for _, a := range e.sc.Assets {
		e.assetIDs[e.scope.Asset(a.ID)] = true
	}
	for _, t := range e.sc.Tracks {
		e.trackRefs[t.ID] = true
	}

	motions := map[string]*entityMotion{}
	motionKey := func(assetID, trackID string) string {
		if assetID != "" {
			return "asset:" + assetID
		}
		return "track:" + trackID
	}
	ensureMotion := func(key string) *entityMotion {
		m, ok := motions[key]
		if !ok {
			m = &entityMotion{}
			motions[key] = m
		}
		return m
	}
	for _, a := range e.sc.Assets {
		m := ensureMotion("asset:" + a.ID)
		if a.Start != nil {
			m.start = a.Start.Point()
			m.hasStart = true
		}
	}
	for _, t := range e.sc.Tracks {
		m := ensureMotion("track:" + t.ID)
		if t.Start != nil {
			m.start = t.Start.Point()
			m.hasStart = true
		}
	}

	assetSpeed := map[string]float64{}
	for _, a := range e.sc.Assets {
		if a.SpeedMPS != nil {
			assetSpeed[a.ID] = *a.SpeedMPS
		}
	}

	var actions []scheduledAction
	seq := 0
	add := func(atMs int64, name string, run func(context.Context) error) (int, error) {
		if len(actions) >= maxScheduledActions {
			return 0, apperr.Validation("scenario schedules too many actions")
		}
		seq++
		actions = append(actions, scheduledAction{AtMs: atMs, Seq: seq, Name: name, Run: run})
		e.eventStates[seq] = EventInspection{Sequence: seq, AtMs: atMs, Name: name, Status: EventPending}
		return seq, nil
	}

	var moveEvents []EventSpec
	for _, ev := range e.sc.Events {
		if ev.Action == ActionMove {
			moveEvents = append(moveEvents, ev)
		}
	}
	sort.SliceStable(moveEvents, func(i, j int) bool {
		a, _ := ParseDurationMs(moveEvents[i].After)
		b, _ := ParseDurationMs(moveEvents[j].After)
		return a < b
	})

	for i := range moveEvents {
		ev := moveEvents[i]
		startMs, _ := ParseDurationMs(ev.After)
		speed := 15.0
		if ev.SpeedMPS != nil {
			speed = *ev.SpeedMPS
		} else if ev.Asset != "" && assetSpeed[ev.Asset] > 0 {
			speed = assetSpeed[ev.Asset]
		}
		if speed <= 0 {
			return apperr.Validation("move event requires a positive speed")
		}

		m := ensureMotion(motionKey(ev.Asset, ev.Track))
		at := startMs
		if m.lastAt() > at {
			at = m.lastAt()
		}
		cursor, ok := m.at(at)
		if !ok {
			cursor = ev.Waypoints[0].Point()
			m.start = cursor
			m.hasStart = true
		}
		for _, wp := range ev.Waypoints {
			target := wp.Point()
			fromMs := at
			dist := geo.DistanceMeters(cursor, target)
			if speed > 0 && dist > 0 {
				durationMs := dist / speed * 1000
				if math.IsNaN(durationMs) || math.IsInf(durationMs, 0) || durationMs >= float64(maxInt64Value) {
					return apperr.Validation("move event duration is too large")
				}
				duration := int64(durationMs)
				if duration < 0 {
					return apperr.Validation("move event duration is too large")
				}
				var ok bool
				at, ok = addMilliseconds(at, duration)
				if !ok {
					return apperr.Validation("move event duration is too large")
				}
			}
			m.segments = append(m.segments, motionSegment{FromMs: fromMs, ToMs: at, From: cursor, To: target})
			cursor = target
		}
	}

	// Asset telemetry: an initial position, then one sample per second while
	// moving. The observation stream for tracks is defined by observe events.
	for _, a := range e.sc.Assets {
		m := motions["asset:"+a.ID]
		assetID := a.ID
		if a.Start != nil {
			start := a.Start.Point()
			var eventSeq int
			var err error
			eventSeq, err = add(0, "telemetry."+assetID, func(ctx context.Context) error {
				return e.emitTelemetry(ctx, assetID, &start, "", "", eventSeq)
			})
			if err != nil {
				return err
			}
		}
		if m == nil {
			continue
		}
		for at := int64(defaultTelemetryMs); at <= m.lastAt(); {
			if pos, ok := m.at(at); ok {
				p := pos
				var eventSeq int
				var err error
				eventSeq, err = add(at, "telemetry."+assetID, func(ctx context.Context) error {
					return e.emitTelemetry(ctx, assetID, &p, "", "", eventSeq)
				})
				if err != nil {
					return err
				}
			}
			if at >= m.lastAt() {
				break
			}
			next, ok := addMilliseconds(at, defaultTelemetryMs)
			if !ok {
				return apperr.Validation("telemetry timeline is too large")
			}
			at = next
		}
	}

	for i := range e.sc.Events {
		ev := e.sc.Events[i]
		switch ev.Action {
		case ActionObserve:
			startMs, _ := ParseDurationMs(ev.After)
			untilMs, _ := ParseDurationMs(ev.Until)
			intervalMs, _ := ParseDurationMs(ev.Interval)
			delayMs, _ := ParseDurationMs(ev.Delay)

			lastMs := startMs
			if intervalMs > 0 {
				if untilMs == 0 {
					var ok bool
					untilMs, ok = addMilliseconds(startMs, defaultObserveUntilMs)
					if !ok {
						return apperr.Validation("observe timeline is too large")
					}
				}
				lastMs = untilMs
			}
			motion := motions["track:"+ev.Track]
			trackRef := ev.Track
			step := max64(intervalMs, 1)
			for at := startMs; at <= lastMs; {
				observationAt, ok := addMilliseconds(at, delayMs)
				if !ok {
					return apperr.Validation("observation timeline is too large")
				}
				pos, hasPos := motion.at(at)
				if ev.Position != nil {
					pos = ev.Position.Point()
					hasPos = true
				}
				if ev.Fault == "missing_position" {
					hasPos = false
				}
				var point *geo.Point
				if hasPos {
					p := e.jitter(pos, ev.JitterM)
					if ev.Fault == "invalid_coordinate" {
						p = geo.Point{Lat: 91, Lng: 181}
					}
					point = &p
				}
				e.seq[trackRef]++
				seqNo := e.seq[trackRef]
				if _, err := add(observationAt, "observe."+trackRef, func(ctx context.Context) error {
					return e.emitObservation(ctx, ev, trackRef, point, seqNo)
				}); err != nil {
					return err
				}
				if ev.Duplicate || ev.Fault == "duplicate" {
					duplicateAt, ok := addMilliseconds(observationAt, 500)
					if !ok {
						return apperr.Validation("duplicate observation timeline is too large")
					}
					if _, err := add(duplicateAt, "observe.duplicate."+trackRef, func(ctx context.Context) error {
						return e.emitObservation(ctx, ev, trackRef, point, seqNo)
					}); err != nil {
						return err
					}
				}
				if intervalMs <= 0 {
					break
				}
				if at >= lastMs {
					break
				}
				next, ok := addMilliseconds(at, step)
				if !ok {
					return apperr.Validation("observe timeline is too large")
				}
				at = next
			}

		case ActionClassify:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			if _, err := add(at, "classify."+ev.Track, func(ctx context.Context) error {
				return e.emitClassification(ctx, evCopy)
			}); err != nil {
				return err
			}

		case ActionAssessment:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			if _, err := add(at, "assessment."+ev.SubjectID, func(ctx context.Context) error {
				return e.emitAssessment(ctx, evCopy)
			}); err != nil {
				return err
			}

		case ActionConnection:
			at, _ := ParseDurationMs(ev.After)
			motion := motions["asset:"+ev.Asset]
			pos, hasPos := motion.at(at)
			var point *geo.Point
			if hasPos {
				point = &pos
			}
			assetID, state := ev.Asset, ev.State
			var eventSeq int
			var err error
			eventSeq, err = add(at, "connection."+assetID, func(ctx context.Context) error {
				return e.emitTelemetry(ctx, assetID, point, state, "", eventSeq)
			})
			if err != nil {
				return err
			}

		case ActionAssetStatus:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			if _, err := add(at, "asset_status."+ev.Asset, func(ctx context.Context) error {
				return e.emitAssetStatus(ctx, evCopy)
			}); err != nil {
				return err
			}

		case ActionIncident:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			if _, err := add(at, "incident."+ev.IncidentRef, func(ctx context.Context) error {
				return e.emitIncident(ctx, evCopy)
			}); err != nil {
				return err
			}

		case ActionIssueCommand:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			if _, err := add(at, "command."+ev.Asset, func(ctx context.Context) error {
				return e.emitCommand(ctx, evCopy)
			}); err != nil {
				return err
			}
		}
	}

	if len(actions) > maxScheduledActions {
		return apperr.Validation("scenario schedules too many actions")
	}

	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].AtMs != actions[j].AtMs {
			return actions[i].AtMs < actions[j].AtMs
		}
		return actions[i].Seq < actions[j].Seq
	})
	e.static = actions
	e.nextSeq = seq
	return nil
}

func (e *engine) persistStaticEvents(ctx context.Context) error {
	if e.repo == nil {
		return nil
	}
	for _, action := range e.static {
		if err := e.withPersistence(ctx, func(persistCtx context.Context) error {
			return e.repo.CreateEvent(persistCtx, e.runID, e.eventStates[action.Seq])
		}); err != nil {
			return err
		}
	}
	return nil
}

// run executes the timeline until completion, stop, or failure.
func (e *engine) run(ctx context.Context) error {
	e.mu.Lock()
	e.lastResume = time.Now()
	e.mu.Unlock()

	e.reportProgress(true)
	for {
		next := e.peekNext()
		if next == nil {
			e.reportProgress(true)
			return nil
		}
		if !e.waitUntil(ctx, next.AtMs) {
			e.markFinished()
			reason := "scenario run stopped"
			if ctx.Err() != nil && !e.IsStopped() {
				reason = ctx.Err().Error()
			}
			e.skipPending(ctx, reason)
			if ctx.Err() != nil && !e.IsStopped() {
				return ctx.Err()
			}
			return nil
		}
		next = e.popNext(next.Seq)
		if next == nil {
			// A dynamic action may have been inserted ahead of the action we
			// were waiting for. Re-select the head so virtual-time ordering
			// remains authoritative under concurrent command injection.
			continue
		}
		e.mu.Lock()
		e.virtualBaseMs = float64(next.AtMs)
		e.lastResume = time.Now()
		e.lastAction = next.Name
		e.lastActionAt = next.AtMs
		e.actionError = ""
		if item, ok := e.eventStates[next.Seq]; ok {
			item.Status = EventRunning
			e.eventStates[next.Seq] = item
		}
		e.mu.Unlock()
		if err := e.persistEvent(ctx, next.Seq, EventRunning, ""); err != nil {
			e.logger.Error("persist scenario action start", "sequence", next.Seq, "error", err)
			e.markFinished()
			e.skipPending(ctx, err.Error())
			return err
		}

		actionErr := next.Run(ctx)
		fatalActionErr := false
		if actionErr != nil {
			e.logger.Warn("scenario action failed", "action", next.Name, "error", actionErr)
			e.mu.Lock()
			e.actionError = actionErr.Error()
			if item, ok := e.eventStates[next.Seq]; ok {
				item.Status = EventFailed
				item.Error = actionErr.Error()
				e.eventStates[next.Seq] = item
			}
			e.mu.Unlock()
			if err := e.persistEvent(ctx, next.Seq, EventFailed, actionErr.Error()); err != nil {
				e.markFinished()
				e.skipPending(ctx, err.Error())
				return err
			}
			fatalActionErr = apperr.Is(actionErr, apperr.CodeInternal)
		} else {
			e.mu.Lock()
			e.actionError = ""
			e.mu.Unlock()
		}
		e.mu.Lock()
		if item, ok := e.eventStates[next.Seq]; ok && item.Status == EventRunning {
			item.Status = EventCompleted
			e.eventStates[next.Seq] = item
		}
		e.eventsRun++
		lastAction := e.lastAction
		lastActionAt := e.lastActionAt
		actionError := e.actionError
		virtualTime := int64(e.virtualBaseMs)
		e.mu.Unlock()
		if actionErr == nil {
			if err := e.persistEvent(ctx, next.Seq, EventCompleted, ""); err != nil {
				e.markFinished()
				e.skipPending(ctx, err.Error())
				return err
			}
		}
		if err := e.persistCursor(ctx, virtualTime, lastAction, lastActionAt, actionError); err != nil {
			e.markFinished()
			e.skipPending(ctx, err.Error())
			return err
		}
		e.reportProgress(false)
		if fatalActionErr {
			e.markFinished()
			e.skipPending(ctx, actionErr.Error())
			return actionErr
		}
	}
}

func (e *engine) peekNext() *scheduledAction {
	e.mu.Lock()
	defer e.mu.Unlock()
	var candidate *scheduledAction
	if e.nextStatic < len(e.static) {
		candidate = &e.static[e.nextStatic]
	}
	if len(e.pending) > 0 {
		p := &e.pending[0]
		if candidate == nil || actionBefore(*p, *candidate) {
			candidate = p
		}
	}
	if candidate == nil {
		e.finished = true
		return nil
	}
	copy := *candidate
	return &copy
}

func actionBefore(left, right scheduledAction) bool {
	if left.AtMs != right.AtMs {
		return left.AtMs < right.AtMs
	}
	return left.Seq < right.Seq
}

func (e *engine) popNext(sequence int) *scheduledAction {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.nextStatic < len(e.static) && e.static[e.nextStatic].Seq == sequence {
		action := e.static[e.nextStatic]
		e.nextStatic++
		return &action
	}
	if len(e.pending) > 0 && e.pending[0].Seq == sequence {
		action := e.pending[0]
		e.pending = e.pending[1:]
		return &action
	}
	return nil
}

// scheduleAfter inserts a dynamic action relative to the current virtual time.
func (e *engine) scheduleAfter(ctx context.Context, delayMs int64, name string, fn func(ctx context.Context) error) error {
	return e.scheduleAfterMany(ctx, []dynamicAction{{DelayMs: delayMs, Name: name, Run: fn}})
}

func (e *engine) scheduleAfterMany(ctx context.Context, actions []dynamicAction) error {
	e.mu.Lock()
	if len(actions) == 0 {
		e.mu.Unlock()
		return nil
	}
	if e.stopped || e.finished {
		e.mu.Unlock()
		return apperr.Conflict("scenario run is no longer accepting actions")
	}
	for _, item := range actions {
		if item.DelayMs < 0 {
			e.mu.Unlock()
			return apperr.Validation("dynamic action delay must not be negative")
		}
	}
	if len(e.eventStates)+e.reserved+len(actions) > maxScheduledActions {
		e.mu.Unlock()
		return apperr.Conflict("scenario action capacity exhausted")
	}
	e.reserved += len(actions)
	base := virtualTimeMs(e.currentVirtualLocked())
	if scope, ok := runctx.ScopeFrom(ctx); ok && scope.RunID == e.scope.RunID {
		// Actions emitted synchronously by a scenario action are anchored to
		// that action's virtual timestamp, not to database/service wall time.
		// This keeps same-seed replays deterministic even when persistence
		// latency varies between runs.
		base = e.lastActionAt
	}
	reserved := make([]scheduledAction, 0, len(actions))
	newEvents := make([]EventInspection, 0, len(actions))
	for _, item := range actions {
		e.nextSeq++
		if item.DelayMs > 0 && base > maxInt64Value-item.DelayMs {
			e.reserved -= len(actions)
			e.mu.Unlock()
			return apperr.Validation("dynamic action time is too large")
		}
		at := base + item.DelayMs
		action := scheduledAction{AtMs: at, Seq: e.nextSeq, Name: item.Name, Run: item.Run}
		reserved = append(reserved, action)
		newEvents = append(newEvents, EventInspection{Sequence: action.Seq, AtMs: at, Name: action.Name, Status: EventPending})
	}
	e.mu.Unlock()
	for _, event := range newEvents {
		if e.repo != nil {
			if err := e.withPersistence(ctx, func(persistCtx context.Context) error {
				return e.repo.CreateEvent(persistCtx, e.runID, event)
			}); err != nil {
				message := fmt.Sprintf("could not persist dynamic action: %v", err)
				for _, persisted := range newEvents {
					_ = e.withPersistence(ctx, func(persistCtx context.Context) error {
						return e.repo.UpdateEvent(persistCtx, e.runID, persisted.Sequence, EventSkipped, message)
					})
				}
				e.mu.Lock()
				e.reserved -= len(newEvents)
				for _, failed := range newEvents {
					failed.Status = EventSkipped
					failed.Error = message
					e.eventStates[failed.Sequence] = failed
				}
				e.mu.Unlock()
				return err
			}
		}
	}
	e.mu.Lock()
	if e.stopped || e.finished {
		e.reserved -= len(newEvents)
		message := "scenario run stopped before dynamic action was scheduled"
		for index := range newEvents {
			newEvents[index].Status = EventSkipped
			newEvents[index].Error = message
			e.eventStates[newEvents[index].Sequence] = newEvents[index]
		}
		e.mu.Unlock()
		for _, event := range newEvents {
			if e.repo != nil {
				_ = e.withPersistence(ctx, func(persistCtx context.Context) error {
					return e.repo.UpdateEvent(persistCtx, e.runID, event.Sequence, EventSkipped, message)
				})
			}
		}
		return apperr.Conflict("scenario run is no longer accepting actions")
	}
	e.reserved -= len(newEvents)
	for index, action := range reserved {
		e.pending = append(e.pending, action)
		e.eventStates[action.Seq] = newEvents[index]
	}
	sort.SliceStable(e.pending, func(i, j int) bool { return actionBefore(e.pending[i], e.pending[j]) })
	e.mu.Unlock()
	e.signal()
	return nil
}

func (e *engine) withPersistence(ctx context.Context, fn func(context.Context) error) error {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	persistCtx, cancel := context.WithTimeout(base, 5*time.Second)
	defer cancel()
	persistCtx = runctx.WithScope(persistCtx, runctx.Scope{RunID: e.scope.RunID, ResourceNamespace: e.scope.ResourceNamespace})
	return fn(persistCtx)
}

func (e *engine) persistEvent(ctx context.Context, sequence int, status, errorMessage string) error {
	if e.repo == nil {
		return nil
	}
	return e.withPersistence(ctx, func(persistCtx context.Context) error {
		return e.repo.UpdateEvent(persistCtx, e.runID, sequence, status, errorMessage)
	})
}

func (e *engine) persistCursor(ctx context.Context, virtualTimeMs int64, lastAction string, lastActionAt int64, actionError string) error {
	if e.repo == nil {
		return nil
	}
	return e.withPersistence(ctx, func(persistCtx context.Context) error {
		return e.repo.UpdateCursor(persistCtx, e.runID, virtualTimeMs, lastAction, lastActionAt, actionError)
	})
}

func (e *engine) skipPending(ctx context.Context, errorMessage string) {
	e.mu.Lock()
	for sequence, item := range e.eventStates {
		if item.Status != EventPending && item.Status != EventRunning {
			continue
		}
		item.Status = EventSkipped
		if errorMessage != "" {
			item.Error = errorMessage
		}
		e.eventStates[sequence] = item
	}
	e.mu.Unlock()
	if e.repo != nil {
		if err := e.withPersistence(ctx, func(persistCtx context.Context) error {
			return e.repo.SkipPendingEvents(persistCtx, e.runID, errorMessage)
		}); err != nil {
			e.logger.Error("persist skipped scenario actions", "error", err)
		}
	}
	e.failPendingCommands(ctx, "scenario run ended before simulated command outcome")
}

func (e *engine) trackCommand(commandID string) {
	if commandID == "" {
		return
	}
	e.mu.Lock()
	e.pendingCommands[commandID] = struct{}{}
	e.mu.Unlock()
}

func (e *engine) untrackCommand(commandID string) {
	e.mu.Lock()
	delete(e.pendingCommands, commandID)
	e.mu.Unlock()
}

func (e *engine) failPendingCommands(ctx context.Context, reason string) {
	e.mu.Lock()
	commandIDs := make([]string, 0, len(e.pendingCommands))
	for commandID := range e.pendingCommands {
		commandIDs = append(commandIDs, commandID)
	}
	e.pendingCommands = map[string]struct{}{}
	e.mu.Unlock()
	if e.deps.Commands == nil {
		return
	}
	for _, commandID := range commandIDs {
		var err error
		if err = e.withPersistence(ctx, func(persistCtx context.Context) error {
			_, commandErr := e.deps.Commands.Fail(persistCtx, commandID, reason, e.scope.Actor())
			return commandErr
		}); err != nil {
			e.logger.Warn("fail simulated command after scenario termination", "command_id", commandID, "error", err)
		}
	}
}

// Pause freezes virtual time.
func (e *engine) Pause() {
	e.mu.Lock()
	if !e.paused && !e.stopped && !e.finished {
		e.virtualBaseMs = e.currentVirtualLocked()
		e.paused = true
	}
	e.mu.Unlock()
	e.signal()
}

func (e *engine) markFinished() {
	e.mu.Lock()
	e.finished = true
	e.mu.Unlock()
}

// Resume continues a paused run.
func (e *engine) Resume() {
	e.mu.Lock()
	if e.paused && !e.stopped && !e.finished {
		e.paused = false
		e.lastResume = time.Now()
	}
	e.mu.Unlock()
	e.signal()
}

// SetSpeed changes playback speed, preserving virtual time.
func (e *engine) SetSpeed(speed float64) {
	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return
	}
	e.mu.Lock()
	if !e.paused && !e.stopped && !e.finished {
		e.virtualBaseMs = e.currentVirtualLocked()
		e.lastResume = time.Now()
	}
	e.speed = speed
	e.mu.Unlock()
	e.signal()
}

// Stop terminates the run.
func (e *engine) Stop() {
	e.mu.Lock()
	e.stopped = true
	e.mu.Unlock()
	e.signal()
}

// VirtualTimeMs returns the current virtual timestamp.
func (e *engine) VirtualTimeMs() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return virtualTimeMs(e.currentVirtualLocked())
}

// IsPaused reports pause state.
func (e *engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

// IsStopped reports whether Stop was called.
func (e *engine) IsStopped() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stopped
}

func (e *engine) currentVirtualLocked() float64 {
	if e.paused || e.stopped {
		return e.virtualBaseMs
	}
	elapsedMs := float64(time.Since(e.lastResume)) / float64(time.Millisecond)
	value := e.virtualBaseMs + elapsedMs*e.speed
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if math.IsInf(value, 1) || value >= float64(maxInt64Value) {
		return float64(maxInt64Value)
	}
	return value
}

func virtualTimeMs(value float64) int64 {
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	if math.IsInf(value, 1) || value >= float64(maxInt64Value) {
		return maxInt64Value
	}
	return int64(value)
}

func (e *engine) waitUntil(ctx context.Context, atMs int64) bool {
	for {
		e.mu.Lock()
		if e.stopped {
			e.mu.Unlock()
			return false
		}
		if !e.paused {
			current := virtualTimeMs(e.currentVirtualLocked())
			if current >= atMs {
				e.mu.Unlock()
				return true
			}
			remaining := atMs - current
			waitMs := float64(remaining) / e.speed
			wait := 100 * time.Millisecond
			if waitMs < 100 {
				wait = time.Duration(waitMs * float64(time.Millisecond))
				if wait <= 0 {
					wait = time.Nanosecond
				}
			}
			e.mu.Unlock()

			select {
			case <-ctx.Done():
				return false
			case <-time.After(wait):
			}
			continue
		}
		e.mu.Unlock()

		select {
		case <-ctx.Done():
			return false
		case <-e.wake:
		}
	}
}

func (e *engine) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *engine) reportProgress(force bool) {
	if e.onProgress == nil {
		return
	}
	if !force && time.Since(e.lastProgress) < time.Second {
		return
	}
	e.lastProgress = time.Now()
	e.onProgress(e.VirtualTimeMs())
}

func (e *engine) jitter(p geo.Point, jitterM float64) geo.Point {
	if jitterM <= 0 {
		return p
	}
	latOffset := (e.rng.Float64()*2 - 1) * jitterM / 111320.0
	mPerDegLng := 111320.0 * math.Cos(p.Lat*math.Pi/180)
	if mPerDegLng < 1 {
		mPerDegLng = 1
	}
	lngOffset := (e.rng.Float64()*2 - 1) * jitterM / mPerDegLng
	return geo.Point{Lat: p.Lat + latOffset, Lng: p.Lng + lngOffset}
}

func (e *engine) emitObservation(ctx context.Context, ev EventSpec, trackRef string, position *geo.Point, seqNo int) error {
	if e.deps.Observations == nil {
		return apperr.Internal(fmt.Errorf("scenario observation dependency is not configured"))
	}
	observedAt := time.Now().UTC()
	if staleMs, _ := ParseDurationMs(ev.StaleFor); staleMs > 0 {
		observedAt = observedAt.Add(-time.Duration(staleMs) * time.Millisecond)
	} else if ev.Fault == "stale" {
		observedAt = observedAt.Add(-2 * time.Minute)
	}

	payload := map[string]any{
		"scenario":          e.sc.Name,
		"scenarioRunId":     e.scope.RunID,
		"resourceNamespace": e.scope.ResourceNamespace,
		"trackRef":          trackRef,
		"qualifiedTrackRef": e.scope.Track(trackRef),
	}
	for key, value := range ev.Payload {
		payload[key] = value
	}
	if ev.Fault != "" {
		payload["fault"] = ev.Fault
	}
	result, err := e.deps.Observations.Ingest(ctx, observations.CreateInput{
		ID:         e.scope.Observation(fmt.Sprintf("%s__%d", trackRef, seqNo)),
		SourceID:   e.scope.Source(ev.Source),
		Type:       "track.observation",
		ObservedAt: observedAt,
		Position:   position,
		TrackHint:  e.scope.Track(trackRef),
		Payload:    payload,
		Quality:    ev.Quality,
	})
	if err != nil {
		return err
	}
	if result.Duplicate {
		e.logger.Debug("scenario emitted duplicate observation", "track", trackRef, "observation_id", result.Observation.ID)
	}
	e.mu.Lock()
	e.observationIDs[trackRef] = result.Observation.ID
	e.mu.Unlock()
	return nil
}

func (e *engine) emitTelemetry(ctx context.Context, assetID string, position *geo.Point, connectionState, health string, sequence int) error {
	if e.deps.Telemetry == nil {
		return apperr.Internal(fmt.Errorf("scenario telemetry dependency is not configured"))
	}
	payload := map[string]any{
		"scenario":          e.sc.Name,
		"scenarioRunId":     e.scope.RunID,
		"resourceNamespace": e.scope.ResourceNamespace,
		"logicalAssetId":    assetID,
	}
	_, err := e.deps.Telemetry.Ingest(ctx, telemetry.CreateInput{
		ID:              e.scope.Telemetry(fmt.Sprintf("%s__%d", assetID, sequence)),
		AssetID:         e.scope.Asset(assetID),
		ObservedAt:      time.Now().UTC(),
		Position:        position,
		ConnectionState: connectionState,
		Health:          health,
		Payload:         payload,
	})
	return err
}

func (e *engine) emitClassification(ctx context.Context, ev EventSpec) error {
	if e.deps.Tracks == nil || e.deps.Classifications == nil {
		return apperr.Internal(fmt.Errorf("scenario classification dependencies are not configured"))
	}
	track, err := e.deps.Tracks.FindByExternalRef(ctx, e.scope.Track(ev.Track))
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			e.logger.Warn("classification skipped: track not observed yet", "track", ev.Track)
			return nil
		}
		return err
	}
	created, err := e.deps.Classifications.Create(ctx, classifications.CreateInput{
		TrackID:         track.ID,
		Label:           ev.Label,
		Confidence:      ev.Confidence,
		Method:          classifications.MethodScenario,
		SourceReference: e.scope.Actor(),
		CreatedBy:       e.scope.Actor(),
	})
	if err == nil {
		e.mu.Lock()
		e.classificationIDs[ev.Track] = created.ID
		e.mu.Unlock()
	}
	return err
}

func (e *engine) emitAssessment(ctx context.Context, ev EventSpec) error {
	if e.deps.Assessments == nil {
		return apperr.Internal(fmt.Errorf("scenario assessment dependency is not configured"))
	}
	evidence := make([]assessments.Evidence, 0, len(ev.Evidence))
	for _, item := range ev.Evidence {
		evidence = append(evidence, assessments.Evidence{
			Type: assessments.EvidenceType(item.Type),
			ID:   e.qualifyReference(item.Type, item.ID),
		})
	}
	subjectID := e.qualifyReference(ev.SubjectType, ev.SubjectID)
	_, err := e.deps.Assessments.Create(ctx, assessments.CreateInput{
		SubjectType: assessments.SubjectType(ev.SubjectType),
		SubjectID:   subjectID,
		Type:        ev.AssessmentType,
		Conclusion:  ev.Conclusion,
		Confidence:  ev.Confidence,
		Method:      assessments.Method(ev.Method),
		CreatedBy:   e.scope.Actor(),
		Evidence:    evidence,
	})
	return err
}

func (e *engine) qualifyReference(kind, logicalID string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	logicalID = strings.TrimSpace(logicalID)
	if logicalID == "" {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	switch kind {
	case "source":
		return e.scope.Source(logicalID)
	case "asset":
		return e.scope.Asset(logicalID)
	case "track":
		return e.scope.Track(logicalID)
	case "observation":
		if id := e.observationIDs[logicalID]; id != "" {
			return id
		}
		return e.scope.Observation(logicalID)
	case "classification":
		if id := e.classificationIDs[logicalID]; id != "" {
			return id
		}
		return e.scope.Classification(logicalID)
	case "incident":
		if id := e.incidentRefs[logicalID]; id != "" {
			return id
		}
		return e.scope.ID("incident", logicalID)
	case "alert":
		return e.scope.ID("alert", logicalID)
	default:
		return e.scope.ID(kind, logicalID)
	}
}

func (e *engine) emitAssetStatus(ctx context.Context, ev EventSpec) error {
	if e.deps.Assets == nil {
		return apperr.Internal(fmt.Errorf("scenario asset dependency is not configured"))
	}
	_, err := e.deps.Assets.UpdateStatus(ctx, e.scope.Asset(ev.Asset), assets.Status(ev.Status))
	return err
}

func (e *engine) emitIncident(ctx context.Context, ev EventSpec) error {
	if e.deps.Incidents == nil {
		return apperr.Internal(fmt.Errorf("scenario incident dependency is not configured"))
	}
	ref := ev.IncidentRef
	incidentID := e.incidentRefs[ref]
	actor := e.scope.Actor()
	if incidentID == "" {
		created, err := e.deps.Incidents.Create(ctx, incidents.CreateInput{
			Title:       firstNonEmpty(ev.IncidentTitle, ref, "Scenario incident"),
			Description: ev.IncidentDescription,
			Priority:    incidents.Priority(ev.IncidentPriority),
			Actor:       actor,
		})
		if err != nil {
			return err
		}
		incidentID = created.ID
		if ref != "" {
			e.incidentRefs[ref] = incidentID
		}
	}
	if ev.IncidentStatus != "" && ev.IncidentStatus != string(incidents.StatusOpen) {
		_, err := e.deps.Incidents.UpdateStatus(ctx, incidentID, incidents.Status(ev.IncidentStatus), actor)
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (e *engine) emitCommand(ctx context.Context, ev EventSpec) error {
	if e.deps.Commands == nil {
		return apperr.Internal(fmt.Errorf("scenario command dependency is not configured"))
	}
	payload := map[string]any{
		"scenarioRunId":     e.scope.RunID,
		"resourceNamespace": e.scope.ResourceNamespace,
		"logicalAssetId":    ev.Asset,
	}
	for key, value := range ev.Payload {
		payload[key] = value
	}
	_, err := e.deps.Commands.Issue(ctx, commands.IssueInput{
		AssetID: e.scope.Asset(ev.Asset),
		Type:    ev.Type,
		Payload: payload,
		Actor:   e.scope.Actor(),
	})
	return err
}

// simulateCommand schedules acknowledgments for commands issued against assets
// owned by this run. Called by the application service for every CommandIssued.
func (e *engine) simulateCommand(ctx context.Context, ev events.CommandIssued) {
	if e.deps.Commands == nil {
		e.logger.Error("cannot simulate command without command dependency", "command_id", ev.CommandID)
		return
	}
	if !e.ownsAsset(ev.AssetID) {
		return
	}

	behavior := "normal"
	ackDelay := int64(defaultAckDelayMs)
	completeDelay := int64(defaultCompleteDelay)
	if e.sc.CommandSimulation != nil {
		if e.sc.CommandSimulation.Behavior != "" {
			behavior = e.sc.CommandSimulation.Behavior
		}
		if ms, err := ParseDurationMs(e.sc.CommandSimulation.AcknowledgeAfter); err == nil && ms > 0 {
			ackDelay = ms
		}
		if ms, err := ParseDurationMs(e.sc.CommandSimulation.CompleteAfter); err == nil && ms > 0 {
			completeDelay = ms
		}
	}
	actor := e.scope.Actor()
	commandID := ev.CommandID
	e.trackCommand(commandID)

	switch behavior {
	case "reject":
		if err := e.scheduleAfter(ctx, ackDelay, "command.reject", func(ctx context.Context) error {
			defer e.untrackCommand(commandID)
			_, err := e.deps.Commands.Reject(ctx, commandID, "rejected by simulated asset", actor)
			return err
		}); err != nil {
			e.failCapacity(ctx, commandID, actor, err)
		}
	case "fail":
		if err := e.scheduleAfter(ctx, ackDelay, "command.fail", func(ctx context.Context) error {
			defer e.untrackCommand(commandID)
			_, err := e.deps.Commands.Fail(ctx, commandID, "execution failed on simulated asset", actor)
			return err
		}); err != nil {
			e.failCapacity(ctx, commandID, actor, err)
		}
	case "timeout":
		if err := e.scheduleAfter(ctx, completeDelay, "command.timeout", func(ctx context.Context) error {
			defer e.untrackCommand(commandID)
			_, err := e.deps.Commands.Timeout(ctx, commandID, "simulated acknowledgment timeout", actor)
			return err
		}); err != nil {
			e.failCapacity(ctx, commandID, actor, err)
		}
	default:
		err := e.scheduleAfterMany(ctx, []dynamicAction{
			{DelayMs: ackDelay, Name: "command.acknowledge", Run: func(ctx context.Context) error {
				_, err := e.deps.Commands.Acknowledge(ctx, commandID, actor)
				if err != nil {
					e.untrackCommand(commandID)
				}
				return err
			}},
			{DelayMs: completeDelay, Name: "command.complete", Run: func(ctx context.Context) error {
				defer e.untrackCommand(commandID)
				_, err := e.deps.Commands.Complete(ctx, commandID, actor)
				return err
			}},
		})
		if err != nil {
			e.failCapacity(ctx, commandID, actor, err)
		}
	}
}

func (e *engine) failCapacity(ctx context.Context, commandID, actor string, cause error) {
	e.untrackCommand(commandID)
	if e.deps.Commands == nil {
		e.logger.Error("cannot fail command after scenario capacity exhaustion", "command_id", commandID, "cause", cause)
		return
	}
	reason := "scenario command simulation could not be scheduled"
	if cause != nil {
		reason = fmt.Sprintf("%s: %v", reason, cause)
	}
	if err := e.withPersistence(ctx, func(persistCtx context.Context) error {
		_, commandErr := e.deps.Commands.Fail(persistCtx, commandID, reason, actor)
		return commandErr
	}); err != nil {
		e.logger.Error("fail command after scenario capacity exhaustion", "command_id", commandID, "cause", cause, "error", err)
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func addMilliseconds(base, delta int64) (int64, bool) {
	if base < 0 || delta < 0 || base > maxInt64Value-delta {
		return 0, false
	}
	return base + delta, true
}
