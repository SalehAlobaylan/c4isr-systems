package sources

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes sources over HTTP. Handlers only translate transport input
// into application calls.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the source service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers source routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/sources", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Post("/{id}/status", h.updateStatus)
	})
}

type createRequest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type sourceResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	source, err := h.svc.Create(r.Context(), CreateInput{
		ID:       req.ID,
		Name:     req.Name,
		Type:     Type(req.Type),
		Status:   Status(req.Status),
		Metadata: req.Metadata,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(source))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	source, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(source))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[sourceResponse]{
		Items: make([]sourceResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req updateStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	source, err := h.svc.UpdateStatus(r.Context(), chi.URLParam(r, "id"), Status(req.Status))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(source))
}

func toResponse(s Source) sourceResponse {
	metadata := s.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return sourceResponse{
		ID:        s.ID,
		Name:      s.Name,
		Type:      string(s.Type),
		Status:    string(s.Status),
		Metadata:  metadata,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
