package scenarios

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
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
	sc     *Scenario
	deps   EngineDeps
	logger *slog.Logger
	rng    *rand.Rand

	assetIDs  map[string]bool
	trackRefs map[string]bool

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

	seq          map[string]int
	lastProgress time.Time
	onProgress   func(ms int64)
}

func newEngine(runID string, sc *Scenario, seed int64, speed float64, deps EngineDeps, onProgress func(int64)) *engine {
	if speed <= 0 {
		speed = defaultPlaybackSpeed
	}
	return &engine{
		runID:      runID,
		sc:         sc,
		deps:       deps,
		logger:     deps.Logger.With("run_id", runID, "scenario", sc.Name),
		rng:        rand.New(rand.NewSource(seed)),
		assetIDs:   map[string]bool{},
		trackRefs:  map[string]bool{},
		speed:      speed,
		wake:       make(chan struct{}, 1),
		seq:        map[string]int{},
		onProgress: onProgress,
	}
}

// ownsAsset reports whether the run simulates the given asset.
func (e *engine) ownsAsset(assetID string) bool { return e.assetIDs[assetID] }

// build precomputes the entire deterministic timeline.
func (e *engine) build() error {
	for _, a := range e.sc.Assets {
		e.assetIDs[a.ID] = true
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
	add := func(atMs int64, name string, run func(context.Context) error) {
		seq++
		actions = append(actions, scheduledAction{AtMs: atMs, Seq: seq, Name: name, Run: run})
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
				at += int64(dist / speed * 1000)
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
			add(0, "telemetry."+assetID, func(ctx context.Context) error {
				return e.emitTelemetry(ctx, assetID, &start, "", "")
			})
		}
		if m == nil {
			continue
		}
		for at := int64(defaultTelemetryMs); at <= m.lastAt(); at += defaultTelemetryMs {
			if pos, ok := m.at(at); ok {
				p := pos
				add(at, "telemetry."+assetID, func(ctx context.Context) error {
					return e.emitTelemetry(ctx, assetID, &p, "", "")
				})
			}
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
					untilMs = startMs + defaultObserveUntilMs
				}
				lastMs = untilMs
			}
			motion := motions["track:"+ev.Track]
			trackRef := ev.Track
			for at := startMs; at <= lastMs; at += max64(intervalMs, 1) {
				pos, hasPos := motion.at(at)
				var point *geo.Point
				if hasPos {
					p := e.jitter(pos, ev.JitterM)
					point = &p
				}
				e.seq[trackRef]++
				seqNo := e.seq[trackRef]
				add(at+delayMs, "observe."+trackRef, func(ctx context.Context) error {
					return e.emitObservation(ctx, ev, trackRef, point, seqNo)
				})
				if ev.Duplicate {
					add(at+delayMs+500, "observe.duplicate."+trackRef, func(ctx context.Context) error {
						return e.emitObservation(ctx, ev, trackRef, point, seqNo)
					})
				}
				if intervalMs <= 0 {
					break
				}
			}

		case ActionClassify:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			add(at, "classify."+ev.Track, func(ctx context.Context) error {
				return e.emitClassification(ctx, evCopy)
			})

		case ActionConnection:
			at, _ := ParseDurationMs(ev.After)
			motion := motions["asset:"+ev.Asset]
			pos, hasPos := motion.at(at)
			var point *geo.Point
			if hasPos {
				point = &pos
			}
			assetID, state := ev.Asset, ev.State
			add(at, "connection."+assetID, func(ctx context.Context) error {
				return e.emitTelemetry(ctx, assetID, point, state, "")
			})

		case ActionIssueCommand:
			at, _ := ParseDurationMs(ev.After)
			evCopy := ev
			add(at, "command."+ev.Asset, func(ctx context.Context) error {
				return e.emitCommand(ctx, evCopy)
			})
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
			return nil
		}
		e.popNext()
		e.mu.Lock()
		e.virtualBaseMs = float64(next.AtMs)
		e.lastResume = time.Now()
		e.mu.Unlock()

		if err := next.Run(ctx); err != nil {
			e.logger.Warn("scenario action failed", "action", next.Name, "error", err)
			if apperr.Is(err, apperr.CodeInternal) {
				return err
			}
		}
		e.reportProgress(false)
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
		if candidate == nil || p.AtMs < candidate.AtMs {
			candidate = p
		}
	}
	return candidate
}

func (e *engine) popNext() {
	e.mu.Lock()
	defer e.mu.Unlock()
	usePending := false
	if e.nextStatic < len(e.static) {
		if len(e.pending) == 0 || e.static[e.nextStatic].AtMs <= e.pending[0].AtMs {
			e.nextStatic++
			return
		}
	}
	if len(e.pending) > 0 {
		usePending = true
	}
	if usePending {
		e.pending = e.pending[1:]
	}
}

// scheduleAfter inserts a dynamic action relative to the current virtual time.
func (e *engine) scheduleAfter(delayMs int64, name string, fn func(ctx context.Context) error) {
	e.mu.Lock()
	at := int64(e.currentVirtualLocked()) + delayMs
	e.pending = append(e.pending, scheduledAction{AtMs: at, Name: name, Run: fn})
	sort.SliceStable(e.pending, func(i, j int) bool { return e.pending[i].AtMs < e.pending[j].AtMs })
	e.mu.Unlock()
	e.signal()
}

// Pause freezes virtual time.
func (e *engine) Pause() {
	e.mu.Lock()
	if !e.paused && !e.stopped {
		e.virtualBaseMs = e.currentVirtualLocked()
		e.paused = true
	}
	e.mu.Unlock()
	e.signal()
}

// Resume continues a paused run.
func (e *engine) Resume() {
	e.mu.Lock()
	if e.paused && !e.stopped {
		e.paused = false
		e.lastResume = time.Now()
	}
	e.mu.Unlock()
	e.signal()
}

// SetSpeed changes playback speed, preserving virtual time.
func (e *engine) SetSpeed(speed float64) {
	e.mu.Lock()
	if !e.paused && !e.stopped {
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
	return int64(e.currentVirtualLocked())
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
	return e.virtualBaseMs + elapsedMs*e.speed
}

func (e *engine) waitUntil(ctx context.Context, atMs int64) bool {
	for {
		e.mu.Lock()
		if e.stopped {
			e.mu.Unlock()
			return false
		}
		if !e.paused {
			remaining := atMs - int64(e.currentVirtualLocked())
			if remaining <= 0 {
				e.mu.Unlock()
				return true
			}
			wait := time.Duration(float64(remaining)/e.speed) * time.Millisecond
			if wait > 100*time.Millisecond {
				wait = 100 * time.Millisecond
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
	observedAt := time.Now().UTC()
	if staleMs, _ := ParseDurationMs(ev.StaleFor); staleMs > 0 {
		observedAt = observedAt.Add(-time.Duration(staleMs) * time.Millisecond)
	}

	result, err := e.deps.Observations.Ingest(ctx, observations.CreateInput{
		ID:         fmt.Sprintf("obs_%s_%s_%d", e.runID, trackRef, seqNo),
		SourceID:   ev.Source,
		Type:       "track.observation",
		ObservedAt: observedAt,
		Position:   position,
		TrackHint:  trackRef,
		Payload: map[string]any{
			"scenario": e.sc.Name,
			"trackRef": trackRef,
		},
	})
	if err != nil {
		return err
	}
	if result.Duplicate {
		e.logger.Debug("scenario emitted duplicate observation", "track", trackRef, "observation_id", result.Observation.ID)
	}
	return nil
}

func (e *engine) emitTelemetry(ctx context.Context, assetID string, position *geo.Point, connectionState, health string) error {
	_, err := e.deps.Telemetry.Ingest(ctx, telemetry.CreateInput{
		AssetID:         assetID,
		ObservedAt:      time.Now().UTC(),
		Position:        position,
		ConnectionState: connectionState,
		Health:          health,
		Payload:         map[string]any{"scenario": e.sc.Name},
	})
	return err
}

func (e *engine) emitClassification(ctx context.Context, ev EventSpec) error {
	track, err := e.deps.Tracks.FindByExternalRef(ctx, ev.Track)
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			e.logger.Warn("classification skipped: track not observed yet", "track", ev.Track)
			return nil
		}
		return err
	}
	_, err = e.deps.Classifications.Create(ctx, classifications.CreateInput{
		TrackID:         track.ID,
		Label:           ev.Label,
		Confidence:      ev.Confidence,
		Method:          classifications.MethodScenario,
		SourceReference: "scenario:" + e.sc.Name,
		CreatedBy:       "scenario:" + e.sc.Name,
	})
	return err
}

func (e *engine) emitCommand(ctx context.Context, ev EventSpec) error {
	_, err := e.deps.Commands.Issue(ctx, commands.IssueInput{
		AssetID: ev.Asset,
		Type:    ev.Type,
		Payload: ev.Payload,
		Actor:   "scenario:" + e.sc.Name,
	})
	return err
}

// simulateCommand schedules acknowledgments for commands issued against assets
// owned by this run. Called by the application service for every CommandIssued.
func (e *engine) simulateCommand(ev events.CommandIssued) {
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
	actor := "scenario:" + e.sc.Name
	commandID := ev.CommandID

	switch behavior {
	case "reject":
		e.scheduleAfter(ackDelay, "command.reject", func(ctx context.Context) error {
			_, err := e.deps.Commands.Reject(ctx, commandID, "rejected by simulated asset", actor)
			return err
		})
	case "fail":
		e.scheduleAfter(ackDelay, "command.fail", func(ctx context.Context) error {
			_, err := e.deps.Commands.Fail(ctx, commandID, "execution failed on simulated asset", actor)
			return err
		})
	case "timeout":
		e.scheduleAfter(completeDelay, "command.timeout", func(ctx context.Context) error {
			_, err := e.deps.Commands.Timeout(ctx, commandID, "simulated acknowledgment timeout", actor)
			return err
		})
	default:
		e.scheduleAfter(ackDelay, "command.acknowledge", func(ctx context.Context) error {
			_, err := e.deps.Commands.Acknowledge(ctx, commandID, actor)
			return err
		})
		e.scheduleAfter(completeDelay, "command.complete", func(ctx context.Context) error {
			_, err := e.deps.Commands.Complete(ctx, commandID, actor)
			return err
		})
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
