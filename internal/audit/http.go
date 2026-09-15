package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes audit entries over HTTP. Handlers only translate transport
// input into application calls.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the audit service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers audit routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/audit", func(r chi.Router) {
		r.Get("/", h.list)
	})
}

type entryResponse struct {
	ID            string         `json:"id"`
	OccurredAt    time.Time      `json:"occurredAt"`
	ActorType     string         `json:"actorType"`
	ActorID       string         `json:"actorId"`
	Action        string         `json:"action"`
	SubjectType   string         `json:"subjectType"`
	SubjectID     string         `json:"subjectId"`
	CorrelationID string         `json:"correlationId"`
	Data          map[string]any `json:"data"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := ListFilter{
		SubjectType: r.URL.Query().Get("subject_type"),
		SubjectID:   r.URL.Query().Get("subject_id"),
		Action:      r.URL.Query().Get("action"),
	}
	if raw := r.URL.Query().Get("since"); raw != "" {
		since, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.Error(w, apperr.BadRequest("since must be an RFC3339 timestamp"))
			return
		}
		since = since.UTC()
		filter.Since = &since
	}

	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	resp := httpx.ListResponse[entryResponse]{
		Items: make([]entryResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(entry Entry) entryResponse {
	data := entry.Data
	if data == nil {
		data = map[string]any{}
	}
	return entryResponse{
		ID:            entry.ID,
		OccurredAt:    entry.OccurredAt,
		ActorType:     string(entry.ActorType),
		ActorID:       entry.ActorID,
		Action:        entry.Action,
		SubjectType:   entry.SubjectType,
		SubjectID:     entry.SubjectID,
		CorrelationID: entry.CorrelationID,
		Data:          data,
	}
}
