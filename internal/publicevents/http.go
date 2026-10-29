package publicevents

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool} }
func (h *Handler) Mount(r chi.Router) {
	r.Get("/public-events", h.list)
	r.Get("/public-events/{id}", h.detail)
	r.Get("/public-events/{id}/revisions", h.history)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	f, err := ScopeFilters(r.Context(), r.URL.Query())
	if err != nil {
		httpx.Error(w, err)
		return
	}
	result, err := Query(r.Context(), h.pool, f, "", false)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, result)
}
func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	f, err := ParseFilters(r.URL.Query(), time.Now().UTC())
	if err != nil {
		httpx.Error(w, err)
		return
	}
	f.Revisions = "all"
	result, err := Query(r.Context(), h.pool, f, chi.URLParam(r, "id"), false)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if len(result.Items) == 0 {
		httpx.Error(w, apperr.NotFound("published public event", "unavailable in selected view"))
		return
	}
	httpx.JSON(w, 200, result.Items[0])
}
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	f, err := ParseFilters(r.URL.Query(), time.Now().UTC())
	if err != nil {
		httpx.Error(w, err)
		return
	}
	f.Revisions = "all"
	result, err := Query(r.Context(), h.pool, f, chi.URLParam(r, "id"), true)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, result)
}
