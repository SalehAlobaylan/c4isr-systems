package simgateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
)

var validJobIDPattern = regexp.MustCompile(`^(simjob|covjob|salvjob)_[a-f0-9]{12}$`)

// RunEvent is pushed over SSE whenever a simulation job transitions state or stage.
type RunEvent struct {
	Event string    `json:"event"`
	Job   RunRecord `json:"job"`
	At    string    `json:"at"`
}

// Scheduler coordinates bounded concurrent compute jobs against the Python
// sim_core hub, deduplicates idempotent submissions, and fans out SSE updates.
type Scheduler struct {
	worker *WorkerClient
	store  *Store
	logger *slog.Logger
	slots  chan struct{}

	mu      sync.Mutex
	subs    map[string]map[chan RunEvent]struct{}
	cancels map[string]context.CancelFunc
	idemp   map[string]string
}

func NewScheduler(worker *WorkerClient, store *Store, maxConcurrent int, logger *slog.Logger) *Scheduler {
	if maxConcurrent < 1 {
		maxConcurrent = 2
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		worker:  worker,
		store:   store,
		logger:  logger,
		slots:   make(chan struct{}, maxConcurrent),
		subs:    make(map[string]map[chan RunEvent]struct{}),
		cancels: make(map[string]context.CancelFunc),
		idemp:   make(map[string]string),
	}
}

func newJobID(engine string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	prefix := "simjob_"
	switch engine {
	case "coverage":
		prefix = "covjob_"
	case "salvo":
		prefix = "salvjob_"
	}
	return prefix + hex.EncodeToString(b[:])
}

func viewerPathForEngine(engine, jobID string) string {
	return fmt.Sprintf("/sim-viewers/%s/?run=%s", engine, jobID)
}

func (s *Scheduler) Subscribe(jobID string) (chan RunEvent, func()) {
	ch := make(chan RunEvent, 16)
	s.mu.Lock()
	if s.subs[jobID] == nil {
		s.subs[jobID] = make(map[chan RunEvent]struct{})
	}
	s.subs[jobID][ch] = struct{}{}
	s.mu.Unlock()
	unsub := func() {
		s.mu.Lock()
		if m := s.subs[jobID]; m != nil {
			delete(m, ch)
			if len(m) == 0 {
				delete(s.subs, jobID)
			}
		}
		s.mu.Unlock()
	}
	return ch, unsub
}

func (s *Scheduler) publish(jobID, eventName string, rec RunRecord) {
	ev := RunEvent{
		Event: eventName,
		Job:   rec,
		At:    time.Now().UTC().Format(time.RFC3339),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs[jobID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *Scheduler) Submit(
	ctx context.Context,
	engine string,
	payload map[string]any,
	actorID string,
	idempotencyKey string,
	syncMode bool,
) (RunRecord, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return RunRecord{}, err
	}
	scenarioHash := artifacts.Hash(rawPayload)

	if idempotencyKey != "" {
		dedupKey := engine + "|" + actorID + "|" + idempotencyKey
		s.mu.Lock()
		existingID, ok := s.idemp[dedupKey]
		s.mu.Unlock()
		if ok {
			if rec, found := s.store.GetRun(ctx, existingID); found {
				return rec, nil
			}
		}
	}

	jobID := newJobID(engine)
	if reqID, ok := payload["jobId"].(string); ok && reqID != "" {
		if !validJobIDPattern.MatchString(reqID) {
			return RunRecord{}, fmt.Errorf("invalid jobId format")
		}
		jobID = reqID
	}
	envID, _ := payload["environmentId"].(string)
	if envID == "" {
		envID = "riyadh-observation-v1"
	}
	title, _ := payload["title"].(string)
	mode, _ := payload["mode"].(string)
	var seed int64
	if f, ok := payload["seed"].(float64); ok {
		seed = int64(f)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	rec := RunRecord{
		JobID:          jobID,
		Engine:         engine,
		EnvironmentID:  envID,
		Title:          title,
		Mode:           mode,
		Seed:           seed,
		Status:         "queued",
		Stage:          "Queued in Go concurrency scheduler…",
		WorkerJobID:    jobID,
		CreatedBy:      actorID,
		CreatedAt:      now,
		ViewerURL:      viewerPathForEngine(engine, jobID),
		ScenarioSha256: scenarioHash,
	}
	s.store.SaveRun(ctx, rec)
	s.publish(jobID, "stage", rec)

	if idempotencyKey != "" {
		dedupKey := engine + "|" + actorID + "|" + idempotencyKey
		s.mu.Lock()
		s.idemp[dedupKey] = jobID
		s.mu.Unlock()
	}

	// Copy payload and inject pinned jobId so Go & Python share exact run identity.
	workerPayload := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		workerPayload[k] = v
	}
	workerPayload["jobId"] = jobID
	delete(workerPayload, "_sync")

	// Detach background execution context from incoming HTTP request cancel, but keep auth values.
	bgCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.cancels[jobID] = cancel
	s.mu.Unlock()

	runFn := func() {
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.cancels, jobID)
			s.mu.Unlock()
		}()
		s.execute(bgCtx, engine, jobID, workerPayload, rec)
	}

	if syncMode {
		runFn()
		if updated, ok := s.store.GetRun(ctx, jobID); ok {
			return updated, nil
		}
		return rec, nil
	}

	go runFn()
	return rec, nil
}

func (s *Scheduler) execute(ctx context.Context, engine, jobID string, payload map[string]any, rec RunRecord) {
	fail := func(msg string) {
		now := time.Now().UTC().Format(time.RFC3339)
		rec.Status = "failed"
		rec.Stage = "Failed"
		rec.CompletedAt = &now
		rec.Error = &msg
		s.store.SaveRun(ctx, rec)
		s.publish(jobID, "failed", rec)
	}
	markCancelled := func() {
		now := time.Now().UTC().Format(time.RFC3339)
		rec.Status = "cancelled"
		rec.Stage = "Cancelled by operator"
		rec.CompletedAt = &now
		rec.Error = nil
		s.store.SaveRun(ctx, rec)
		s.publish(jobID, "cancelled", rec)
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		markCancelled()
		return
	}

	rec.Status = "running"
	rec.Stage = "Dispatching to unified Python sim_core compute worker…"
	s.store.SaveRun(ctx, rec)
	s.publish(jobID, "stage", rec)

	status, raw, err := s.worker.DoJSON(ctx, "POST", fmt.Sprintf("/api/%s/runs", engine), payload)
	if err != nil {
		if ctx.Err() != nil {
			markCancelled()
			return
		}
		fail("compute worker unreachable: " + err.Error())
		return
	}
	if status >= 400 {
		var errDoc map[string]any
		if json.Unmarshal(raw, &errDoc) == nil {
			if msg, ok := errDoc["error"].(string); ok && msg != "" {
				fail(msg)
				return
			}
		}
		fail(fmt.Sprintf("worker rejected scenario (HTTP %d)", status))
		return
	}

	// Merge initial worker fields (title, environmentTitle, hasGlb, etc.)
	mergeWorkerRecord(&rec, raw, engine)
	s.store.SaveRun(ctx, rec)
	s.publish(jobID, "stage", rec)

	// Poll worker status cleanly until terminal state, forwarding every stage change over SSE.
	lastStage := rec.Stage
	ticker := time.NewTicker(180 * time.Millisecond)
	defer ticker.Stop()
	for rec.Status == "running" || rec.Status == "queued" {
		select {
		case <-ctx.Done():
			_, _, _ = s.worker.DoJSON(context.Background(), "POST", fmt.Sprintf("/api/%s/runs/%s/cancel", engine, jobID), nil)
			markCancelled()
			return
		case <-ticker.C:
			st, body, pollErr := s.worker.DoJSON(ctx, "GET", fmt.Sprintf("/api/%s/runs/%s", engine, jobID), nil)
			if pollErr != nil {
				if ctx.Err() != nil {
					markCancelled()
					return
				}
				fail("lost connection to compute worker: " + pollErr.Error())
				return
			}
			if st >= 400 {
				fail(fmt.Sprintf("worker status error (HTTP %d)", st))
				return
			}
			mergeWorkerRecord(&rec, body, engine)
			if rec.Stage != lastStage || rec.Status != "running" {
				lastStage = rec.Stage
				s.store.SaveRun(ctx, rec)
				if rec.Status == "running" {
					s.publish(jobID, "stage", rec)
				}
			}
		}
	}

	if rec.Status == "failed" {
		s.store.SaveRun(ctx, rec)
		s.publish(jobID, "failed", rec)
		return
	}
	if rec.Status == "cancelled" {
		s.store.SaveRun(ctx, rec)
		s.publish(jobID, "cancelled", rec)
		return
	}

	// Persist both operator-redacted and debrief replay blobs to content-addressed S3 + memory cache!
	stRep, _, rawReplay, repErr := s.worker.Do(ctx, "GET", fmt.Sprintf("/api/%s/runs/%s/replay.json?view=debrief", engine, jobID), nil)
	if repErr == nil && stRep == 200 && len(rawReplay) > 0 {
		obsBytes, obsErr := FilterReplayJSON(engine, rawReplay, "operator")
		debBytes, debErr := FilterReplayJSON(engine, rawReplay, "debrief")
		if obsErr != nil {
			obsBytes = nil
		}
		if debErr != nil {
			debBytes = rawReplay
		}
		s.store.PersistReplays(ctx, &rec, obsBytes, debBytes)
	}
	s.store.SaveRun(ctx, rec)
	s.publish(jobID, "completed", rec)
}

func (s *Scheduler) Cancel(ctx context.Context, engine, jobID string) (RunRecord, bool) {
	s.mu.Lock()
	cancel, ok := s.cancels[jobID]
	s.mu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
	if engine != "" {
		_, _, _ = s.worker.DoJSON(ctx, "POST", fmt.Sprintf("/api/%s/runs/%s/cancel", engine, jobID), nil)
	}
	rec, found := s.store.GetRun(ctx, jobID)
	if !found {
		return RunRecord{}, false
	}
	if rec.Status == "queued" || rec.Status == "running" {
		now := time.Now().UTC().Format(time.RFC3339)
		rec.Status = "cancelled"
		rec.Stage = "Cancelled by operator"
		rec.CompletedAt = &now
		s.store.SaveRun(ctx, rec)
		s.publish(jobID, "cancelled", rec)
	}
	return rec, true
}

func (s *Scheduler) ActiveCount() int {
	return len(s.slots)
}

func (s *Scheduler) Capacity() int {
	return cap(s.slots)
}

func mergeWorkerRecord(rec *RunRecord, raw []byte, engine string) {
	var parsed RunRecord
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	if parsed.EnvironmentID != "" {
		rec.EnvironmentID = parsed.EnvironmentID
	}
	if parsed.EnvironmentTitle != "" {
		rec.EnvironmentTitle = parsed.EnvironmentTitle
	}
	if parsed.Title != "" {
		rec.Title = parsed.Title
	}
	if parsed.Mode != "" {
		rec.Mode = parsed.Mode
	}
	if parsed.Seed != 0 {
		rec.Seed = parsed.Seed
	}
	if parsed.Status != "" {
		rec.Status = parsed.Status
	}
	if parsed.Stage != "" {
		rec.Stage = parsed.Stage
	}
	if parsed.CompletedAt != nil {
		rec.CompletedAt = parsed.CompletedAt
	}
	if parsed.Error != nil {
		rec.Error = parsed.Error
	}
	rec.HasGlb = parsed.HasGlb
	rec.HasImagery = parsed.HasImagery
	if parsed.Comparison != nil {
		rec.Comparison = parsed.Comparison
	}
	if parsed.Summary != nil {
		rec.Summary = parsed.Summary
	}
	if parsed.ResultOverlay != nil {
		rec.ResultOverlay = parsed.ResultOverlay
	}
	if len(parsed.Policies) > 0 {
		rec.Policies = parsed.Policies
	}
	if parsed.ElapsedS > 0 {
		rec.ElapsedS = parsed.ElapsedS
	}
	if parsed.SensorCount > 0 {
		rec.SensorCount = parsed.SensorCount
	}
	if parsed.MapLayerAglM > 0 {
		rec.MapLayerAglM = parsed.MapLayerAglM
	}
	if parsed.ThreatCount > 0 {
		rec.ThreatCount = parsed.ThreatCount
	}
	if parsed.BatteryCount > 0 {
		rec.BatteryCount = parsed.BatteryCount
	}
	rec.ViewerURL = viewerPathForEngine(engine, rec.JobID)
}
