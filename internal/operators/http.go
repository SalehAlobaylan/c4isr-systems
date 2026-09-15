package operators

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes operators over HTTP. Handlers only translate transport input
// into application calls.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the operator service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers operator routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/operators", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
	})
}

type createRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type operatorResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	operator, err := h.svc.Create(r.Context(), CreateInput{
		ID:   req.ID,
		Name: req.Name,
		Role: Role(req.Role),
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(operator))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	operator, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(operator))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	resp := httpx.ListResponse[operatorResponse]{
		Items: make([]operatorResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(operator Operator) operatorResponse {
	return operatorResponse{
		ID:        operator.ID,
		Name:      operator.Name,
		Role:      string(operator.Role),
		CreatedAt: operator.CreatedAt,
	}
}
