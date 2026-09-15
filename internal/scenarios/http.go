package scenarios

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes scenario definitions and run control over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the scenario service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers scenario routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/scenarios", func(r chi.Router) {
		r.Get("/", h.listScenarios)
		r.Get("/runs", h.listRuns)
		r.Get("/runs/{id}", h.getRun)
		r.Post("/runs/{id}/pause", h.pause)
		r.Post("/runs/{id}/resume", h.resume)
		r.Post("/runs/{id}/stop", h.stop)
		r.Post("/runs/{id}/speed", h.setSpeed)
		r.Get("/{name}", h.getScenario)
		r.Post("/{name}/start", h.start)
	})
}

type startRequest struct {
	Speed float64 `json:"speed"`
	Seed  *int64  `json:"seed"`
}

type speedRequest struct {
	Speed float64 `json:"speed"`
}

type runResponse struct {
	ID            string     `json:"id"`
	ScenarioName  string     `json:"scenarioName"`
	Seed          int64      `json:"seed"`
	Status        string     `json:"status"`
	PlaybackSpeed float64    `json:"playbackSpeed"`
	VirtualTimeMs int64      `json:"virtualTimeMs"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
	Error         string     `json:"error,omitempty"`
}

func (h *Handler) listScenarios(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.svc.ListScenarios()
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Summary]{Items: summaries, Total: len(summaries)})
}

func (h *Handler) getScenario(w http.ResponseWriter, r *http.Request) {
	sc, err := h.svc.GetScenario(chi.URLParam(r, "name"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sc)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}
	}
	run, err := h.svc.Start(r.Context(), chi.URLParam(r, "name"), StartOptions{Speed: req.Speed, Seed: req.Seed})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toRunResponse(run))
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	runs, total, err := h.svc.ListRuns(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[runResponse]{
		Items: make([]runResponse, 0, len(runs)),
		Total: total,
	}
	for _, run := range runs {
		resp.Items = append(resp.Items, toRunResponse(run))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.GetRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toRunResponse(run))
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Pause(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toRunResponse(run))
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Resume(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toRunResponse(run))
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Stop(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toRunResponse(run))
}

func (h *Handler) setSpeed(w http.ResponseWriter, r *http.Request) {
	var req speedRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	run, err := h.svc.SetSpeed(r.Context(), chi.URLParam(r, "id"), req.Speed)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toRunResponse(run))
}

func toRunResponse(run Run) runResponse {
	return runResponse{
		ID:            run.ID,
		ScenarioName:  run.ScenarioName,
		Seed:          run.Seed,
		Status:        run.Status,
		PlaybackSpeed: run.PlaybackSpeed,
		VirtualTimeMs: run.VirtualTimeMs,
		StartedAt:     run.StartedAt,
		EndedAt:       run.EndedAt,
		Error:         run.Error,
	}
}
