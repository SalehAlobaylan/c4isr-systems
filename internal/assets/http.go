package assets

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes assets over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the asset service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers asset routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/assets", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Post("/{id}/status", h.updateStatus)
	})
}

type createRequest struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Status       string         `json:"status"`
	Capabilities []string       `json:"capabilities"`
	Metadata     map[string]any `json:"metadata"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type assetResponse struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	Status          string         `json:"status"`
	Capabilities    []string       `json:"capabilities"`
	Metadata        map[string]any `json:"metadata"`
	Position        *geo.Point     `json:"position,omitempty"`
	Speed           *float64       `json:"speed"`
	Heading         *float64       `json:"heading"`
	Health          string         `json:"health"`
	ConnectionState string         `json:"connectionState"`
	LastSeenAt      *time.Time     `json:"lastSeenAt"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	asset, err := h.svc.Create(r.Context(), CreateInput{
		ID:           req.ID,
		Name:         req.Name,
		Type:         req.Type,
		Status:       Status(req.Status),
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(asset))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	asset, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(asset))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[assetResponse]{
		Items: make([]assetResponse, 0, len(items)),
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
	asset, err := h.svc.UpdateStatus(r.Context(), chi.URLParam(r, "id"), Status(req.Status))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(asset))
}

func toResponse(a Asset) assetResponse {
	capabilities := a.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	metadata := a.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return assetResponse{
		ID:              a.ID,
		Name:            a.Name,
		Type:            a.Type,
		Status:          string(a.Status),
		Capabilities:    capabilities,
		Metadata:        metadata,
		Position:        a.Position,
		Speed:           a.Speed,
		Heading:         a.Heading,
		Health:          a.Health,
		ConnectionState: a.ConnectionState,
		LastSeenAt:      a.LastSeenAt,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}
