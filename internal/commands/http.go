package commands

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes commands over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the command service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers command routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/commands", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.issue)
		r.Get("/{id}", h.get)
		r.Post("/{id}/transition", h.transition)
	})
}

type issueRequest struct {
	AssetID       string         `json:"assetId"`
	MissionID     string         `json:"missionId"`
	IncidentID    string         `json:"incidentId"`
	Type          string         `json:"type"`
	Payload       map[string]any `json:"payload"`
	CorrelationID string         `json:"correlationId"`
}

type transitionRequest struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type commandResponse struct {
	ID             string         `json:"id"`
	AssetID        string         `json:"assetId"`
	MissionID      string         `json:"missionId,omitempty"`
	IncidentID     string         `json:"incidentId,omitempty"`
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	State          string         `json:"state"`
	CreatedBy      string         `json:"createdBy,omitempty"`
	CorrelationID  string         `json:"correlationId,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	QueuedAt       *time.Time     `json:"queuedAt,omitempty"`
	SentAt         *time.Time     `json:"sentAt,omitempty"`
	AcknowledgedAt *time.Time     `json:"acknowledgedAt,omitempty"`
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	FailureReason  string         `json:"failureReason,omitempty"`
}

func (h *Handler) issue(w http.ResponseWriter, r *http.Request) {
	var req issueRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	correlationID := req.CorrelationID
	if correlationID == "" {
		correlationID = httpx.GetRequestID(r.Context())
	}
	command, err := h.svc.Issue(r.Context(), IssueInput{
		AssetID:       req.AssetID,
		MissionID:     req.MissionID,
		IncidentID:    req.IncidentID,
		Type:          req.Type,
		Payload:       req.Payload,
		CorrelationID: correlationID,
		Actor:         httpx.GetOperatorID(r.Context()),
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(command))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	command, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(command))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := ListFilter{
		AssetID:   r.URL.Query().Get("asset_id"),
		State:     r.URL.Query().Get("state"),
		MissionID: r.URL.Query().Get("mission_id"),
	}
	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[commandResponse]{
		Items: make([]commandResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	var req transitionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	command, err := h.svc.Transition(r.Context(),
		chi.URLParam(r, "id"),
		State(req.State),
		req.Reason,
		httpx.GetOperatorID(r.Context()),
	)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(command))
}

func toResponse(command Command) commandResponse {
	payload := command.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return commandResponse{
		ID:             command.ID,
		AssetID:        command.AssetID,
		MissionID:      command.MissionID,
		IncidentID:     command.IncidentID,
		Type:           command.Type,
		Payload:        payload,
		State:          string(command.State),
		CreatedBy:      command.CreatedBy,
		CorrelationID:  command.CorrelationID,
		CreatedAt:      command.CreatedAt,
		UpdatedAt:      command.UpdatedAt,
		QueuedAt:       command.QueuedAt,
		SentAt:         command.SentAt,
		AcknowledgedAt: command.AcknowledgedAt,
		CompletedAt:    command.CompletedAt,
		FailureReason:  command.FailureReason,
	}
}
