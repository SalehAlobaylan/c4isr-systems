package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes telemetry over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the telemetry service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers telemetry routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/telemetry", h.ingest)
	r.Get("/assets/{id}/telemetry", h.listByAsset)
}

type ingestRequest struct {
	ID              string         `json:"id"`
	MessageID       string         `json:"messageId"`
	AssetID         string         `json:"assetId"`
	SourceID        string         `json:"sourceId"`
	ObservedAt      *time.Time     `json:"observedAt"`
	ReceivedAt      *time.Time     `json:"receivedAt"`
	Position        *geo.Point     `json:"position"`
	Speed           *float64       `json:"speed"`
	Heading         *float64       `json:"heading"`
	Health          string         `json:"health"`
	ConnectionState string         `json:"connectionState"`
	Payload         map[string]any `json:"payload"`
}

type sampleResponse struct {
	ID              string         `json:"id,omitempty"`
	MessageID       string         `json:"messageId,omitempty"`
	AssetID         string         `json:"assetId,omitempty"`
	SourceID        string         `json:"sourceId,omitempty"`
	ObservedAt      time.Time      `json:"observedAt,omitzero"`
	ReceivedAt      time.Time      `json:"receivedAt,omitzero"`
	Position        *geo.Point     `json:"position,omitempty"`
	Speed           *float64       `json:"speed,omitempty"`
	Heading         *float64       `json:"heading,omitempty"`
	Health          string         `json:"health,omitempty"`
	ConnectionState string         `json:"connectionState,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
	Stale           bool           `json:"stale,omitempty"`
	Duplicate       bool           `json:"duplicate,omitempty"`
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	in := CreateInput{
		ID:              req.ID,
		MessageID:       req.MessageID,
		AssetID:         req.AssetID,
		SourceID:        req.SourceID,
		Position:        req.Position,
		Speed:           req.Speed,
		Heading:         req.Heading,
		Health:          req.Health,
		ConnectionState: req.ConnectionState,
		Payload:         req.Payload,
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
	if result.Duplicate {
		httpx.JSON(w, http.StatusOK, sampleResponse{Duplicate: true})
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(result.Sample, result.Stale))
}

func (h *Handler) listByAsset(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.ListByAsset(r.Context(), chi.URLParam(r, "id"), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[sampleResponse]{
		Items: make([]sampleResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item, false))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(sample Sample, stale bool) sampleResponse {
	return sampleResponse{
		ID:              sample.ID,
		MessageID:       sample.MessageID,
		AssetID:         sample.AssetID,
		SourceID:        sample.SourceID,
		ObservedAt:      sample.ObservedAt,
		ReceivedAt:      sample.ReceivedAt,
		Position:        sample.Position,
		Speed:           sample.Speed,
		Heading:         sample.Heading,
		Health:          sample.Health,
		ConnectionState: sample.ConnectionState,
		Payload:         sample.Payload,
		Stale:           stale,
	}
}
