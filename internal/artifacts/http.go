package artifacts

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

type Handler struct{ Service *Service }

func (h Handler) Mount(r chi.Router) {
	r.Post("/artifacts", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		a, e := h.Service.Reserve(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, a)
	})
	r.Get("/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, e := h.Service.Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, a)
	})
	r.Put("/artifacts/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBytes))
		if e != nil {
			httpx.Error(w, apperr.Validation("artifact exceeds byte limit"))
			return
		}
		a, e := h.Service.Upload(r.Context(), chi.URLParam(r, "id"), b)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, a)
	})
	r.Get("/artifacts/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		a, b, e := h.Service.Read(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", a.MediaType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+a.ID+`"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(200)
		_, _ = w.Write(b)
	})
	r.Delete("/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e := h.Service.Delete(r.Context(), chi.URLParam(r, "id")); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.NoContent(w)
	})
}
