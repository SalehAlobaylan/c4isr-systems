package simgateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SessionCommand represents a live operator command applied during an active
// interactive Salvo exercise.
type SessionCommand struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"` // SET_POLICY | HOLD_FIRE_ZONE | CLEAR_HOLD_FIRE | PRIORITIZE_TRACK | ENGAGE_TRACK | TOGGLE_SENSOR | TOGGLE_BATTERY
	Payload map[string]any `json:"payload"`
	ActorID string         `json:"actorId"`
	At      string         `json:"at"`
}

// LiveSessionSnapshot is the EXERCISE_OBSERVATION-only tick frame broadcast over SSE.
type LiveSessionSnapshot struct {
	SessionID   string           `json:"sessionId"`
	Status      string           `json:"status"` // running | paused | completed | stopped
	Tick        int              `json:"tick"`
	VirtualS    float64          `json:"virtualTimeS"`
	DurationS   float64          `json:"durationS"`
	TimeScale   float64          `json:"timeScale"`
	Policy      string           `json:"policy"`
	HoldFire    bool             `json:"holdFire"`
	ViewScope   string           `json:"viewScope"`
	Observation json.RawMessage  `json:"observation,omitempty"`
	Commands    []SessionCommand `json:"recentCommands,omitempty"`
	RunID       string           `json:"finalRunId,omitempty"`
}

type liveSession struct {
	mu          sync.Mutex
	id          string
	status      string
	tick        int
	virtualS    float64
	durationS   float64
	timeScale   float64
	policy      string
	holdFire    bool
	lastObs     json.RawMessage
	pendingCmds []SessionCommand
	recentCmds  []SessionCommand
	finalRunID  string
	cancel      context.CancelFunc
	subs        map[chan LiveSessionSnapshot]struct{}
	wake        chan struct{}
}

// SessionCoordinator uses Go's goroutine virtual clock to drive step-by-step
// Python SalvoStepSession ticks while handling concurrent multi-operator
// commands and SSE fan-out cleanly.
type SessionCoordinator struct {
	worker *WorkerClient
	store  *Store
	logger *slog.Logger

	mu       sync.RWMutex
	sessions map[string]*liveSession
}

func NewSessionCoordinator(worker *WorkerClient, store *Store, logger *slog.Logger) *SessionCoordinator {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionCoordinator{
		worker:   worker,
		store:    store,
		logger:   logger,
		sessions: make(map[string]*liveSession),
	}
}

func randomID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func (c *SessionCoordinator) Start(ctx context.Context, payload map[string]any, actorID string) (LiveSessionSnapshot, error) {
	status, raw, err := c.worker.DoJSON(ctx, "POST", "/api/salvo/sessions", payload)
	if err != nil {
		return LiveSessionSnapshot{}, err
	}
	if status >= 400 {
		return LiveSessionSnapshot{}, fmt.Errorf("failed to start live session (HTTP %d): %s", status, string(raw))
	}
	var initDoc struct {
		SessionID   string          `json:"sessionId"`
		DurationS   float64         `json:"durationS"`
		Policy      string          `json:"policy"`
		Tick        int             `json:"tick"`
		VirtualS    float64         `json:"virtualTimeS"`
		Observation json.RawMessage `json:"observation"`
	}
	if err := json.Unmarshal(raw, &initDoc); err != nil || initDoc.SessionID == "" {
		return LiveSessionSnapshot{}, fmt.Errorf("invalid live session response from worker")
	}

	pausedStart, _ := payload["startPaused"].(bool)
	speed := 1.0
	if sp, ok := payload["timeScale"].(float64); ok && sp >= 0.25 && sp <= 4.0 {
		speed = sp
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	sess := &liveSession{
		id:        initDoc.SessionID,
		status:    "running",
		tick:      initDoc.Tick,
		virtualS:  initDoc.VirtualS,
		durationS: initDoc.DurationS,
		timeScale: speed,
		policy:    initDoc.Policy,
		lastObs:   initDoc.Observation,
		cancel:    cancel,
		subs:      make(map[chan LiveSessionSnapshot]struct{}),
		wake:      make(chan struct{}, 1),
	}
	if pausedStart {
		sess.status = "paused"
	}

	c.mu.Lock()
	c.sessions[sess.id] = sess
	c.mu.Unlock()

	go c.clockLoop(runCtx, sess)
	return sess.snapshot(), nil
}

func (s *liveSession) snapshot() LiveSessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := append([]SessionCommand(nil), s.recentCmds...)
	return LiveSessionSnapshot{
		SessionID:   s.id,
		Status:      s.status,
		Tick:        s.tick,
		VirtualS:    s.virtualS,
		DurationS:   s.durationS,
		TimeScale:   s.timeScale,
		Policy:      s.policy,
		HoldFire:    s.holdFire,
		ViewScope:   "EXERCISE_OBSERVATION",
		Observation: s.lastObs,
		Commands:    recent,
		RunID:       s.finalRunID,
	}
}

func (s *liveSession) broadcast() {
	snap := s.snapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- snap:
		default:
		}
	}
}

func (c *SessionCoordinator) clockLoop(ctx context.Context, sess *liveSession) {
	for {
		sess.mu.Lock()
		st := sess.status
		scale := sess.timeScale
		sess.mu.Unlock()

		if st == "completed" || st == "stopped" {
			return
		}
		if st == "paused" {
			select {
			case <-ctx.Done():
				return
			case <-sess.wake:
				continue
			case <-time.After(150 * time.Millisecond):
				continue
			}
		}

		// Step 1 virtual tick (0.25 s simulated) every 250ms / timeScale wall-clock time.
		interval := time.Duration(float64(250*time.Millisecond) / scale)
		if interval < 40*time.Millisecond {
			interval = 40 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			if _, err := c.Step(ctx, sess.id, 1); err != nil {
				sess.mu.Lock()
				sess.status = "stopped"
				sess.mu.Unlock()
				sess.broadcast()
				return
			}
		}
	}
}

func (c *SessionCoordinator) Step(ctx context.Context, sessionID string, ticks int) (LiveSessionSnapshot, error) {
	sess, ok := c.get(sessionID)
	if !ok {
		return LiveSessionSnapshot{}, fmt.Errorf("session not found")
	}
	if ticks < 1 {
		ticks = 1
	}
	if ticks > 40 {
		ticks = 40
	}

	sess.mu.Lock()
	if sess.status == "completed" || sess.status == "stopped" {
		sess.mu.Unlock()
		return sess.snapshot(), nil
	}
	cmds := append([]SessionCommand(nil), sess.pendingCmds...)
	sess.pendingCmds = nil
	sess.mu.Unlock()

	req := map[string]any{
		"ticks":    ticks,
		"commands": cmds,
	}
	status, raw, err := c.worker.DoJSON(ctx, "POST", fmt.Sprintf("/api/salvo/sessions/%s/step", sessionID), req)
	if err != nil {
		return LiveSessionSnapshot{}, err
	}
	if status >= 400 {
		return LiveSessionSnapshot{}, fmt.Errorf("worker step error (HTTP %d): %s", status, string(raw))
	}
	var resp struct {
		Tick        int             `json:"tick"`
		VirtualS    float64         `json:"virtualTimeS"`
		Completed   bool            `json:"completed"`
		Policy      string          `json:"policy"`
		HoldFire    bool            `json:"holdFire"`
		Observation json.RawMessage `json:"observation"`
		FinalRunID  string          `json:"finalRunId"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return LiveSessionSnapshot{}, err
	}

	sess.mu.Lock()
	sess.tick = resp.Tick
	sess.virtualS = resp.VirtualS
	if resp.Policy != "" {
		sess.policy = resp.Policy
	}
	sess.holdFire = resp.HoldFire
	sess.lastObs = resp.Observation
	if resp.FinalRunID != "" {
		sess.finalRunID = resp.FinalRunID
	}
	if resp.Completed {
		sess.status = "completed"
	}
	sess.mu.Unlock()

	sess.broadcast()
	return sess.snapshot(), nil
}

func (c *SessionCoordinator) EnqueueCommand(ctx context.Context, sessionID, kind string, payload map[string]any, actorID string) (SessionCommand, error) {
	sess, ok := c.get(sessionID)
	if !ok {
		return SessionCommand{}, fmt.Errorf("session not found")
	}
	cmd := SessionCommand{
		ID:      randomID("cmd_"),
		Kind:    kind,
		Payload: payload,
		ActorID: actorID,
		At:      time.Now().UTC().Format(time.RFC3339),
	}
	sess.mu.Lock()
	tick := sess.tick
	vt := sess.virtualS
	sess.pendingCmds = append(sess.pendingCmds, cmd)
	sess.recentCmds = append(sess.recentCmds, cmd)
	if len(sess.recentCmds) > 25 {
		sess.recentCmds = sess.recentCmds[len(sess.recentCmds)-25:]
	}
	sess.mu.Unlock()

	c.store.RecordSessionCommand(ctx, cmd.ID, sessionID, tick, vt, kind, payload, actorID)
	sess.broadcast()
	return cmd, nil
}

func (c *SessionCoordinator) Control(ctx context.Context, sessionID, action string, timeScale float64) (LiveSessionSnapshot, error) {
	sess, ok := c.get(sessionID)
	if !ok {
		return LiveSessionSnapshot{}, fmt.Errorf("session not found")
	}
	sess.mu.Lock()
	switch action {
	case "pause":
		if sess.status == "running" {
			sess.status = "paused"
		}
	case "resume":
		if sess.status == "paused" {
			sess.status = "running"
		}
	case "speed":
		if timeScale >= 0.25 && timeScale <= 4.0 {
			sess.timeScale = timeScale
		}
	case "stop":
		sess.status = "stopped"
		if sess.cancel != nil {
			sess.cancel()
		}
	}
	sess.mu.Unlock()
	select {
	case sess.wake <- struct{}{}:
	default:
	}
	if action == "stop" {
		_, _, _ = c.worker.DoJSON(ctx, "POST", fmt.Sprintf("/api/salvo/sessions/%s/stop", sessionID), nil)
	}
	sess.broadcast()
	return sess.snapshot(), nil
}

func (c *SessionCoordinator) get(sessionID string) (*liveSession, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sessions[sessionID]
	return s, ok
}

func (c *SessionCoordinator) Get(sessionID string) (LiveSessionSnapshot, bool) {
	s, ok := c.get(sessionID)
	if !ok {
		return LiveSessionSnapshot{}, false
	}
	return s.snapshot(), true
}

func (c *SessionCoordinator) List() []LiveSessionSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]LiveSessionSnapshot, 0, len(c.sessions))
	for _, s := range c.sessions {
		out = append(out, s.snapshot())
	}
	return out
}

func (c *SessionCoordinator) Subscribe(sessionID string) (chan LiveSessionSnapshot, func(), bool) {
	s, ok := c.get(sessionID)
	if !ok {
		return nil, nil, false
	}
	ch := make(chan LiveSessionSnapshot, 16)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	unsub := func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
	return ch, unsub, true
}

func (c *SessionCoordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.sessions {
		if s.cancel != nil {
			s.cancel()
		}
	}
}
