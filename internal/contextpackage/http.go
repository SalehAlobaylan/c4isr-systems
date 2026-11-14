package contextpackage

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) Mount(r chi.Router) {
	r.Route("/context-packages", func(r chi.Router) {
		r.Post("/", h.build)
		r.Post("/geographic", func(w http.ResponseWriter, r *http.Request) {
			var in GeographicInput
			if err := httpx.DecodeJSON(r, &in); err != nil {
				httpx.Error(w, err)
				return
			}
			v, err := h.s.FreezeGeographic(r.Context(), in)
			if err != nil {
				httpx.Error(w, err)
				return
			}
			w.Header().Set("Location", "/api/v1/context-packages/"+v.ID+"/geographic")
			httpx.JSON(w, http.StatusCreated, v)
		})
		r.Get("/{id}/geographic", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.s.LoadGeographic(r.Context(), chi.URLParam(r, "id"))
			if err != nil {
				httpx.Error(w, err)
				return
			}
			httpx.JSON(w, http.StatusOK, v)
		})
		r.Post("/preview", func(w http.ResponseWriter, r *http.Request) {
			var in BuildInput
			if err := httpx.DecodeJSON(r, &in); err != nil {
				httpx.Error(w, err)
				return
			}
			v, err := h.s.Preview(r.Context(), in)
			if err != nil {
				httpx.Error(w, err)
				return
			}
			httpx.JSON(w, 200, v)
		})
		r.Get("/", h.list)
		r.Get("/{id}", h.get)
		r.Get("/{id}/bundle", h.bundle)
	})
}
func (h *Handler) build(w http.ResponseWriter, r *http.Request) {
	var in BuildInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.Error(w, err)
		return
	}
	b, err := h.s.Build(r.Context(), in)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": b.ID, "manifest": b.Manifest})
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.s.List(r.Context())
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items)})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	b, err := h.s.Load(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": b.ID, "manifest": b.Manifest})
}
func (h *Handler) bundle(w http.ResponseWriter, r *http.Request) {
	b, err := h.s.Load(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="context-package.json"`)
	httpx.JSON(w, 200, b)
}
