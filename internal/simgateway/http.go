package simgateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service wires together the Go Simulation Gateway: bounded concurrent job
// queue, SSE streaming, wire-level EXERCISE_OBSERVATION vs EXERCISE_TRUTH
// enforcement, PostGIS/S3 persistence, and live interactive sessions.
type Service struct {
	Worker               *WorkerClient
	Store                *Store
	Scheduler            *Scheduler
	Sessions             *SessionCoordinator
	Logger               *slog.Logger
	AllowLocalMapDebrief bool
}

func NewService(
	workerURL, workerSecret string,
	maxConcurrent int,
	allowLocalMapDebrief bool,
	pool *pgxpool.Pool,
	blobs artifacts.Store,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	worker := NewWorkerClient(workerURL, workerSecret)
	store := NewStore(pool, blobs, logger)
	scheduler := NewScheduler(worker, store, maxConcurrent, logger)
	sessions := NewSessionCoordinator(worker, store, logger)
	return &Service{
		Worker:               worker,
		Store:                store,
		Scheduler:            scheduler,
		Sessions:             sessions,
		Logger:               logger,
		AllowLocalMapDebrief: allowLocalMapDebrief,
	}
}

func (s *Service) Close() {
	if s.Sessions != nil {
		s.Sessions.Close()
	}
}

func (s *Service) Mount(r chi.Router) {
	r.Route("/sim-platform", func(sr chi.Router) {
		sr.Get("/health", s.handleHealth)
		sr.Get("/biomes", s.proxyWorkerGET("/api/biomes"))
		sr.Get("/biomes/{id}/terrain.glb", s.handleBiomeAsset("terrain.glb"))
		sr.Get("/biomes/{id}/imagery.json", s.handleBiomeAsset("imagery.json"))
		sr.Get("/networks", s.handleListNetworks)
		sr.Post("/networks", s.handleExportNetwork)

		// Unified run queries
		sr.Get("/runs", s.handleListAllRuns)
		sr.Get("/runs/{id}", s.handleGetAnyRun)
		sr.Get("/runs/{id}/events", s.handleRunSSE(""))
		sr.Get("/runs/{id}/replay", s.handleGetReplay(""))
		sr.Get("/runs/{id}/replay.json", s.handleGetReplay(""))
		sr.Post("/runs/{id}/cancel", s.handleCancelRun(""))

		// Engine-scoped routes (sim, coverage, salvo)
		for _, eng := range []string{"sim", "coverage", "salvo"} {
			engine := eng
			sr.Route("/"+engine, func(er chi.Router) {
				er.Get("/health", s.proxyWorkerGET("/api/"+engine+"/health"))
				er.Get("/environments", s.proxyWorkerGET("/api/"+engine+"/environments"))
				er.Get("/networks", s.handleListNetworks)
				er.Get("/runs", s.handleListEngineRuns(engine))
				er.Post("/runs", s.handleStartRun(engine))
				er.Get("/runs/{id}", s.handleGetEngineRun(engine))
				er.Get("/runs/{id}/events", s.handleRunSSE(engine))
				er.Get("/runs/{id}/replay", s.handleGetReplay(engine))
				er.Get("/runs/{id}/replay.json", s.handleGetReplay(engine))
				er.Get("/runs/{id}/terrain.glb", s.handleRunStaticAsset(engine, "terrain.glb"))
				er.Get("/runs/{id}/imagery.json", s.handleRunStaticAsset(engine, "imagery.json"))
				er.Post("/runs/{id}/cancel", s.handleCancelRun(engine))
				if engine == "coverage" {
					er.Post("/los-profile", s.proxyWorkerPOST("/api/coverage/los-profile"))
					er.Post("/preview-sensor", s.proxyWorkerPOST("/api/coverage/preview-sensor"))
					er.Post("/adversarial-route", s.proxyWorkerPOST("/api/coverage/adversarial-route"))
					er.Post("/export-network", s.handleExportNetwork)
				}
			})
		}

		// Benchmark & Monte Carlo routes
		sr.Route("/bench", func(br chi.Router) {
			br.Get("/health", s.proxyWorkerGET("/api/bench/health"))
			br.Get("/environments", s.proxyWorkerGET("/api/bench/environments"))
			br.Get("/models", s.proxyWorkerGET("/api/bench/models"))
			br.Get("/leaderboard", s.proxyWorkerGET("/api/bench/leaderboard"))
			br.Post("/monte-carlo", s.proxyWorkerPOST("/api/bench/monte-carlo"))
			br.Post("/benchmark", s.proxyWorkerPOST("/api/bench/benchmark"))
		})

		// Live interactive command-in-the-loop sessions
		sr.Route("/sessions", func(lr chi.Router) {
			lr.Get("/", s.handleListSessions)
			lr.Post("/", s.handleStartSession)
			lr.Get("/{id}", s.handleGetSession)
			lr.Get("/{id}/events", s.handleSessionSSE)
			lr.Post("/{id}/commands", s.handleSessionCommand)
			lr.Post("/{id}/control", s.handleSessionControl)
			lr.Post("/{id}/step", s.handleSessionStep)
			lr.Post("/{id}/stop", s.handleSessionStop)
		})
	})
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"status":          "ok",
		"gateway":         "emad-go-simgateway",
		"workerUrl":       s.Worker.BaseURL,
		"workerReachable": false,
		"activeJobs":      s.Scheduler.ActiveCount(),
		"maxConcurrent":   s.Scheduler.Capacity(),
		"canReadDebrief":  CanReadDebriefTruth(r.Context(), s.AllowLocalMapDebrief),
	}
	if s.Worker.Configured() {
		st, raw, err := s.Worker.DoJSON(r.Context(), "GET", "/api/health", nil)
		if err == nil && st == 200 {
			var wh map[string]any
			if json.Unmarshal(raw, &wh) == nil {
				out["workerReachable"] = true
				out["worker"] = wh
			}
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (s *Service) proxyWorkerGET(targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := targetPath
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		st, ctype, raw, err := s.Worker.Do(r.Context(), "GET", path, nil)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "simulation compute worker unavailable: "+err.Error()))
			return
		}
		if ctype == "" {
			ctype = "application/json; charset=utf-8"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(st)
		_, _ = w.Write(raw)
	}
}

func (s *Service) proxyWorkerPOST(targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
		if err != nil {
			httpx.Error(w, apperr.BadRequest("failed to read request body"))
			return
		}
		st, ctype, raw, err := s.Worker.Do(r.Context(), "POST", targetPath, body)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "simulation compute worker unavailable: "+err.Error()))
			return
		}
		if ctype == "" {
			ctype = "application/json; charset=utf-8"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(st)
		_, _ = w.Write(raw)
	}
}

func (s *Service) handleBiomeAsset(asset string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := chi.URLParam(r, "id")
		st, ctype, raw, err := s.Worker.Do(r.Context(), "GET", fmt.Sprintf("/api/biomes/%s/%s", envID, asset), nil)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
			return
		}
		if ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		if st == 200 {
			w.Header().Set("Cache-Control", "private, max-age=86400")
		}
		w.WriteHeader(st)
		_, _ = w.Write(raw)
	}
}

func (s *Service) handleRunStaticAsset(engine, asset string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		st, ctype, raw, err := s.Worker.Do(r.Context(), "GET", fmt.Sprintf("/api/%s/runs/%s/%s", engine, jobID, asset), nil)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
			return
		}
		if ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		if st == 200 && asset == "terrain.glb" {
			w.Header().Set("Cache-Control", "private, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.WriteHeader(st)
		_, _ = w.Write(raw)
	}
}

func (s *Service) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	path := "/api/networks"
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	st, _, raw, err := s.Worker.Do(r.Context(), "GET", path, nil)
	if err != nil {
		httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(st)
	_, _ = w.Write(raw)
}

func (s *Service) handleExportNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		httpx.Error(w, apperr.BadRequest("invalid body"))
		return
	}
	st, _, raw, err := s.Worker.Do(r.Context(), "POST", "/api/coverage/export-network", body)
	if err != nil {
		httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
		return
	}
	if st >= 200 && st < 300 {
		var doc map[string]any
		if json.Unmarshal(raw, &doc) == nil {
			s.Store.SaveSensorNetwork(r.Context(), doc, httpx.GetOperatorID(r.Context()))
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(st)
	_, _ = w.Write(raw)
}

func (s *Service) handleStartRun(engine string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
		if err != nil {
			httpx.Error(w, apperr.BadRequest("invalid request body"))
			return
		}
		payload := map[string]any{}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &payload); err != nil {
				httpx.Error(w, apperr.BadRequest("request body must be a JSON object"))
				return
			}
		}
		syncMode, _ := payload["_sync"].(bool)
		idempKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		rec, err := s.Scheduler.Submit(r.Context(), engine, payload, httpx.GetOperatorID(r.Context()), idempKey, syncMode)
		if err != nil {
			httpx.Error(w, apperr.BadRequest(err.Error()))
			return
		}
		httpx.JSON(w, http.StatusAccepted, rec)
	}
}

func (s *Service) handleListAllRuns(w http.ResponseWriter, r *http.Request) {
	eng := strings.TrimSpace(r.URL.Query().Get("engine"))
	httpx.JSON(w, http.StatusOK, map[string]any{"runs": s.Store.ListRuns(r.Context(), eng)})
}

func (s *Service) handleListEngineRuns(engine string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Merge gateway records with any historical disk runs known to the Python hub
		gwRuns := s.Store.ListRuns(r.Context(), engine)
		seen := make(map[string]bool, len(gwRuns))
		for _, g := range gwRuns {
			seen[g.JobID] = true
		}
		st, raw, err := s.Worker.DoJSON(r.Context(), "GET", fmt.Sprintf("/api/%s/runs", engine), nil)
		if err == nil && st == 200 {
			var doc struct {
				Runs []RunRecord `json:"runs"`
			}
			if json.Unmarshal(raw, &doc) == nil {
				for _, wr := range doc.Runs {
					if !seen[wr.JobID] {
						wr.Engine = engine
						wr.ViewerURL = viewerPathForEngine(engine, wr.JobID)
						gwRuns = append(gwRuns, wr)
					}
				}
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"runs": gwRuns})
	}
}

func (s *Service) handleGetAnyRun(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	rec, ok := s.Store.GetRun(r.Context(), jobID)
	if !ok {
		httpx.Error(w, apperr.NotFound("simulation run", jobID))
		return
	}
	httpx.JSON(w, http.StatusOK, rec)
}

func (s *Service) handleGetEngineRun(engine string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		if rec, ok := s.Store.GetRun(r.Context(), jobID); ok {
			httpx.JSON(w, http.StatusOK, rec)
			return
		}
		st, raw, err := s.Worker.DoJSON(r.Context(), "GET", fmt.Sprintf("/api/%s/runs/%s", engine, jobID), nil)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
			return
		}
		if st == 404 {
			httpx.Error(w, apperr.NotFound("simulation run", jobID))
			return
		}
		var rec RunRecord
		if json.Unmarshal(raw, &rec) == nil {
			rec.Engine = engine
			rec.ViewerURL = viewerPathForEngine(engine, rec.JobID)
			httpx.JSON(w, http.StatusOK, rec)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(st)
		_, _ = w.Write(raw)
	}
}

func (s *Service) handleCancelRun(engine string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		eng := engine
		if eng == "" {
			if rec, ok := s.Store.GetRun(r.Context(), jobID); ok {
				eng = rec.Engine
			} else {
				switch {
				case strings.HasPrefix(jobID, "covjob_"):
					eng = "coverage"
				case strings.HasPrefix(jobID, "salvjob_"):
					eng = "salvo"
				default:
					eng = "sim"
				}
			}
		}
		rec, ok := s.Scheduler.Cancel(r.Context(), eng, jobID)
		if !ok {
			httpx.Error(w, apperr.NotFound("simulation run", jobID))
			return
		}
		httpx.JSON(w, http.StatusOK, rec)
	}
}

func (s *Service) handleGetReplay(engineHint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		rec, found := s.Store.GetRun(r.Context(), jobID)
		engine := engineHint
		if engine == "" && found {
			engine = rec.Engine
		}
		if engine == "" {
			switch {
			case strings.HasPrefix(jobID, "covjob_"):
				engine = "coverage"
			case strings.HasPrefix(jobID, "salvjob_"):
				engine = "salvo"
			default:
				engine = "sim"
			}
		}

		// Wire-level 4-partition authorization check!
		requestedView := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
		canDebrief := CanReadDebriefTruth(r.Context(), s.AllowLocalMapDebrief)
		effectiveView := requestedView
		if engine == "sim" || engine == "salvo" {
			if requestedView == "debrief" {
				if !canDebrief {
					access.Deny(r.Context(), "sim.truth.read", "debrief truth requested without sim.truth.read grant")
					httpx.Error(w, apperr.Forbidden("debrief truth requires sim.truth.read authorization"))
					return
				}
				s.Logger.Info("sim_platform.debrief.unlocked",
					"jobId", jobID,
					"engine", engine,
					"operatorId", httpx.GetOperatorID(r.Context()),
				)
			} else if requestedView == "" {
				if canDebrief {
					effectiveView = "debrief"
				} else {
					effectiveView = "operator"
				}
			} else if requestedView != "operator" {
				httpx.Error(w, apperr.Validation("view must be operator or debrief"))
				return
			}
		}

		// Try S3 / gateway cache first
		if found {
			if cached, ok := s.Store.GetReplayBlob(r.Context(), rec, effectiveView); ok {
				filtered, err := FilterReplayJSON(engine, cached, effectiveView)
				if err == nil {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("X-EMAD-View-Scope", scopeHeader(effectiveView))
					w.Header().Set("Cache-Control", "no-store")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(filtered)
					return
				}
			}
		}

		// Fallback to Python worker and enforce Go-side FilterReplayJSON defense-in-depth!
		st, _, raw, err := s.Worker.Do(r.Context(), "GET", fmt.Sprintf("/api/%s/runs/%s/replay.json?view=%s", engine, jobID, effectiveView), nil)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.CodeInternal, "compute worker unavailable"))
			return
		}
		if st != http.StatusOK {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(st)
			_, _ = w.Write(raw)
			return
		}
		filtered, err := FilterReplayJSON(engine, raw, effectiveView)
		if err != nil {
			httpx.Error(w, apperr.Internal(err))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-EMAD-View-Scope", scopeHeader(effectiveView))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(filtered)
	}
}

func scopeHeader(view string) string {
	if view == "operator" {
		return "EXERCISE_OBSERVATION"
	}
	return "EXERCISE_TRUTH"
}

func (s *Service) handleRunSSE(engine string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		flusher, ok := w.(http.Flusher)
		if !ok {
			rc := http.NewResponseController(w)
			if err := rc.Flush(); err != nil {
				httpx.Error(w, apperr.New(apperr.CodeInternal, "streaming unsupported"))
				return
			}
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		writeEvent := func(name string, payload any) {
			data, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
			if flusher != nil {
				flusher.Flush()
			} else {
				_ = http.NewResponseController(w).Flush()
			}
		}

		ch, unsub := s.Scheduler.Subscribe(jobID)
		defer unsub()

		// Send current snapshot immediately so late joiners never miss terminal state.
		if rec, found := s.Store.GetRun(r.Context(), jobID); found {
			evName := "stage"
			if rec.Status == "completed" || rec.Status == "failed" || rec.Status == "cancelled" {
				evName = rec.Status
			}
			writeEvent(evName, RunEvent{Event: evName, Job: rec, At: time.Now().UTC().Format(time.RFC3339)})
			if rec.Status == "completed" || rec.Status == "failed" || rec.Status == "cancelled" {
				return
			}
		}

		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-keepalive.C:
				_, _ = fmt.Fprint(w, ": keepalive\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			case ev := <-ch:
				writeEvent(ev.Event, ev)
				if ev.Job.Status == "completed" || ev.Job.Status == "failed" || ev.Job.Status == "cancelled" {
					return
				}
			}
		}
	}
}

// --- Live Interactive Sessions (Phase 4 Go Event Coordinator) ---

func (s *Service) handleListSessions(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": s.Sessions.List()})
}

func (s *Service) handleStartSession(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		httpx.Error(w, apperr.BadRequest("invalid body"))
		return
	}
	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			httpx.Error(w, apperr.BadRequest("request body must be JSON object"))
			return
		}
	}
	snap, err := s.Sessions.Start(r.Context(), payload, httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, apperr.BadRequest(err.Error()))
		return
	}
	httpx.JSON(w, http.StatusCreated, snap)
}

func (s *Service) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, ok := s.Sessions.Get(id)
	if !ok {
		httpx.Error(w, apperr.NotFound("live session", id))
		return
	}
	httpx.JSON(w, http.StatusOK, snap)
}

func (s *Service) handleSessionCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Kind    string         `json:"kind"`
		Payload map[string]any `json:"payload"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if strings.TrimSpace(req.Kind) == "" {
		httpx.Error(w, apperr.Validation("command kind is required"))
		return
	}
	cmd, err := s.Sessions.EnqueueCommand(r.Context(), id, req.Kind, req.Payload, httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, apperr.NotFound("live session", id))
		return
	}
	httpx.JSON(w, http.StatusAccepted, cmd)
}

func (s *Service) handleSessionControl(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Action    string  `json:"action"`
		TimeScale float64 `json:"timeScale"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	snap, err := s.Sessions.Control(r.Context(), id, req.Action, req.TimeScale)
	if err != nil {
		httpx.Error(w, apperr.NotFound("live session", id))
		return
	}
	httpx.JSON(w, http.StatusOK, snap)
}

func (s *Service) handleSessionStep(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Ticks int `json:"ticks"`
	}
	_ = httpx.DecodeJSON(r, &req)
	snap, err := s.Sessions.Step(r.Context(), id, req.Ticks)
	if err != nil {
		httpx.Error(w, apperr.BadRequest(err.Error()))
		return
	}
	httpx.JSON(w, http.StatusOK, snap)
}

func (s *Service) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := s.Sessions.Control(r.Context(), id, "stop", 1.0)
	if err != nil {
		httpx.Error(w, apperr.NotFound("live session", id))
		return
	}
	httpx.JSON(w, http.StatusOK, snap)
}

func (s *Service) handleSessionSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, unsub, ok := s.Sessions.Subscribe(id)
	if !ok {
		httpx.Error(w, apperr.NotFound("live session", id))
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)
	writeTick := func(snap LiveSessionSnapshot) {
		data, _ := json.Marshal(snap)
		_, _ = fmt.Fprintf(w, "event: tick\ndata: %s\n\n", data)
		_ = rc.Flush()
	}

	if snap, found := s.Sessions.Get(id); found {
		writeTick(snap)
		if snap.Status == "completed" || snap.Status == "stopped" {
			return
		}
	}
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			_ = rc.Flush()
		case snap := <-ch:
			writeTick(snap)
			if snap.Status == "completed" || snap.Status == "stopped" {
				return
			}
		}
	}
}
