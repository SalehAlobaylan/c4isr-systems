package imagery

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h Handler) mountCaptures(r chi.Router) {
	answer := func(w http.ResponseWriter, value any, e error, status int) {
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, status, value)
	}
	r.Get("/imagery/captures", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.Service.Captures(r.Context(), r.URL.Query().Get("region_id"), r.URL.Query().Get("geometry"))
		more := len(v) > 100
		if more {
			v = v[:100]
		}
		answer(w, queryscope.ListMetadata(r.Context(), "captures", legacyCatalogPage(v, len(v), more, "captures")), e, 200)
	})
	r.Post("/imagery/captures", func(w http.ResponseWriter, r *http.Request) {
		var in CaptureInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := h.Service.SubmitCapture(r.Context(), in)
		answer(w, v, e, 201)
	})
	r.Post("/imagery/captures/claim", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.Service.ClaimCapture(r.Context())
		if apperr.Is(e, apperr.CodeNotFound) {
			answer(w, map[string]any{"job": nil}, nil, 200)
			return
		}
		answer(w, map[string]any{"job": v, "lease": v.Lease}, e, 200)
	})
	r.Get("/imagery/captures/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.Service.Capture(r.Context(), chi.URLParam(r, "id"))
		answer(w, v, e, 200)
	})
	r.Post("/imagery/captures/{id}/progress", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Lease    string         `json:"lease"`
			Progress string         `json:"progress"`
			Result   *CaptureResult `json:"result"`
			Failed   bool           `json:"failed"`
		}
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		e := h.Service.UpdateCapture(r.Context(), chi.URLParam(r, "id"), in.Lease, in.Progress, in.Result, in.Failed)
		answer(w, map[string]string{"state": "ACCEPTED"}, e, 200)
	})
	for _, action := range []string{"retry", "cancel"} {
		r.Post("/imagery/captures/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
			v, e := h.Service.ControlCapture(r.Context(), chi.URLParam(r, "id"), action)
			answer(w, v, e, 200)
		})
	}
}
