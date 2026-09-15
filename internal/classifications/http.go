package classifications

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes classifications over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the classification service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers classification routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/classifications", h.create)
	r.Get("/classifications/{id}", h.get)
	r.Get("/tracks/{id}/classifications", h.listByTrack)
}

type createRequest struct {
	TrackID         string   `json:"trackId"`
	Label           string   `json:"label"`
	Confidence      *float64 `json:"confidence"`
	Method          string   `json:"method"`
	SourceReference string   `json:"sourceReference"`
	CreatedBy       string   `json:"createdBy"`
}

type classificationResponse struct {
	ID              string    `json:"id"`
	TrackID         string    `json:"trackId"`
	Label           string    `json:"label"`
	Confidence      *float64  `json:"confidence"`
	Method          string    `json:"method"`
	SourceReference string    `json:"sourceReference"`
	CreatedBy       string    `json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	classification, err := h.svc.Create(r.Context(), CreateInput{
		TrackID:         req.TrackID,
		Label:           req.Label,
		Confidence:      req.Confidence,
		Method:          Method(req.Method),
		SourceReference: req.SourceReference,
		CreatedBy:       req.CreatedBy,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(classification))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	classification, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(classification))
}

func (h *Handler) listByTrack(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.ListByTrack(r.Context(), chi.URLParam(r, "id"), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[classificationResponse]{
		Items: make([]classificationResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(c Classification) classificationResponse {
	return classificationResponse{
		ID:              c.ID,
		TrackID:         c.TrackID,
		Label:           c.Label,
		Confidence:      c.Confidence,
		Method:          string(c.Method),
		SourceReference: c.SourceReference,
		CreatedBy:       c.CreatedBy,
		CreatedAt:       c.CreatedAt,
	}
}
