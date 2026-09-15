package tracks

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes tracks over HTTP. Handlers only translate transport input
// into application calls.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the track service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers track routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/tracks", func(r chi.Router) {
		r.Get("/", h.list)
		r.Get("/{id}", h.get)
		r.Get("/{id}/history", h.history)
	})
}

type trackResponse struct {
	ID               string         `json:"id"`
	ExternalRef      string         `json:"externalRef"`
	Status           string         `json:"status"`
	FirstSeenAt      time.Time      `json:"firstSeenAt"`
	LastSeenAt       time.Time      `json:"lastSeenAt"`
	Position         *geo.Point     `json:"position"`
	Speed            *float64       `json:"speed"`
	Heading          *float64       `json:"heading"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	ClosedAt         *time.Time     `json:"closedAt"`
	ObservationCount int            `json:"observationCount"`
}

type historyPointResponse struct {
	ID         string     `json:"id"`
	ObservedAt time.Time  `json:"observedAt"`
	Position   *geo.Point `json:"position"`
	Speed      *float64   `json:"speed"`
	Heading    *float64   `json:"heading"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[trackResponse]{
		Items: make([]trackResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	track, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(track))
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	points, err := h.svc.History(r.Context(), chi.URLParam(r, "id"), limit)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[historyPointResponse]{
		Items: make([]historyPointResponse, 0, len(points)),
		Total: len(points),
	}
	for _, point := range points {
		resp.Items = append(resp.Items, historyPointResponse{
			ID:         point.ID,
			ObservedAt: point.ObservedAt,
			Position:   point.Position,
			Speed:      point.Speed,
			Heading:    point.Heading,
		})
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(t Track) trackResponse {
	metadata := t.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return trackResponse{
		ID:               t.ID,
		ExternalRef:      t.ExternalRef,
		Status:           string(t.Status),
		FirstSeenAt:      t.FirstSeenAt,
		LastSeenAt:       t.LastSeenAt,
		Position:         t.Position,
		Speed:            t.Speed,
		Heading:          t.Heading,
		Metadata:         metadata,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		ClosedAt:         t.ClosedAt,
		ObservationCount: t.ObservationCount,
	}
}
