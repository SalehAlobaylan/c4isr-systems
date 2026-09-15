package alerts

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes alerts over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the alert service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers alert routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/alerts", func(r chi.Router) {
		r.Get("/", h.list)
		r.Get("/{id}", h.get)
		r.Post("/{id}/acknowledge", h.acknowledge)
		r.Post("/{id}/resolve", h.resolve)
	})
}

type alertResponse struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	Severity        string         `json:"severity"`
	State           string         `json:"state"`
	Title           string         `json:"title"`
	Message         string         `json:"message"`
	SourceReference map[string]any `json:"sourceReference"`
	TrackID         string         `json:"trackId"`
	AssetID         string         `json:"assetId"`
	GeofenceID      string         `json:"geofenceId"`
	IncidentID      string         `json:"incidentId"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	AcknowledgedAt  *time.Time     `json:"acknowledgedAt"`
	AcknowledgedBy  string         `json:"acknowledgedBy"`
	ResolvedAt      *time.Time     `json:"resolvedAt"`
	ResolvedBy      string         `json:"resolvedBy"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(alert))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := ListFilter{
		State:      r.URL.Query().Get("state"),
		Severity:   r.URL.Query().Get("severity"),
		TrackID:    r.URL.Query().Get("track_id"),
		IncidentID: r.URL.Query().Get("incident_id"),
	}
	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[alertResponse]{
		Items: make([]alertResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) acknowledge(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Acknowledge(r.Context(), chi.URLParam(r, "id"), httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(alert))
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Resolve(r.Context(), chi.URLParam(r, "id"), httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(alert))
}

func toResponse(alert Alert) alertResponse {
	sourceReference := alert.SourceReference
	if sourceReference == nil {
		sourceReference = map[string]any{}
	}
	return alertResponse{
		ID:              alert.ID,
		Type:            alert.Type,
		Severity:        string(alert.Severity),
		State:           string(alert.State),
		Title:           alert.Title,
		Message:         alert.Message,
		SourceReference: sourceReference,
		TrackID:         alert.TrackID,
		AssetID:         alert.AssetID,
		GeofenceID:      alert.GeofenceID,
		IncidentID:      alert.IncidentID,
		CreatedAt:       alert.CreatedAt,
		UpdatedAt:       alert.UpdatedAt,
		AcknowledgedAt:  alert.AcknowledgedAt,
		AcknowledgedBy:  alert.AcknowledgedBy,
		ResolvedAt:      alert.ResolvedAt,
		ResolvedBy:      alert.ResolvedBy,
	}
}
