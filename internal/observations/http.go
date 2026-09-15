package observations

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes observations over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the observation service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers observation routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/observations", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.ingest)
		r.Get("/{id}", h.get)
	})
}

type ingestRequest struct {
	ID         string         `json:"id"`
	SourceID   string         `json:"sourceId"`
	Type       string         `json:"type"`
	ObservedAt *time.Time     `json:"observedAt"`
	ReceivedAt *time.Time     `json:"receivedAt"`
	Position   *geo.Point     `json:"position"`
	Payload    map[string]any `json:"payload"`
	Quality    map[string]any `json:"quality"`
	TrackHint  string         `json:"trackHint"`
}

type observationResponse struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"sourceId"`
	Type        string         `json:"type"`
	ObservedAt  time.Time      `json:"observedAt"`
	ReceivedAt  time.Time      `json:"receivedAt"`
	ProcessedAt *time.Time     `json:"processedAt,omitempty"`
	Position    *geo.Point     `json:"position,omitempty"`
	Payload     map[string]any `json:"payload"`
	Quality     map[string]any `json:"quality,omitempty"`
	TrackHint   string         `json:"trackHint,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	Duplicate   bool           `json:"duplicate,omitempty"`
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	in := CreateInput{
		ID:        req.ID,
		SourceID:  req.SourceID,
		Type:      req.Type,
		Position:  req.Position,
		Payload:   req.Payload,
		Quality:   req.Quality,
		TrackHint: req.TrackHint,
	}
	if req.ObservedAt != nil {
		in.ObservedAt = *req.ObservedAt
	}
	if req.ReceivedAt != nil {
		in.ReceivedAt = *req.ReceivedAt
	}

	result, err := h.svc.Ingest(r.Context(), in)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}
	httpx.JSON(w, status, toResponse(result.Observation, result.Duplicate))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	obs, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(obs, false))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := ListFilter{TrackID: r.URL.Query().Get("track_id")}
	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[observationResponse]{
		Items: make([]observationResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item, false))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(obs Observation, duplicate bool) observationResponse {
	payload := obs.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return observationResponse{
		ID:          obs.ID,
		SourceID:    obs.SourceID,
		Type:        obs.Type,
		ObservedAt:  obs.ObservedAt,
		ReceivedAt:  obs.ReceivedAt,
		ProcessedAt: obs.ProcessedAt,
		Position:    obs.Position,
		Payload:     payload,
		Quality:     obs.Quality,
		TrackHint:   obs.TrackHint,
		CreatedAt:   obs.CreatedAt,
		Duplicate:   duplicate,
	}
}
