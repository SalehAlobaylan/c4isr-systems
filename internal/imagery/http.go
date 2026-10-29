package imagery

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct{ Service *Service }

func (h Handler) Mount(r chi.Router) {
	h.mountCaptures(r)
	h.mountPreflight(r)
	r.Get("/imagery/scenes", func(w http.ResponseWriter, r *http.Request) {
		var b *publicingest.Bounds
		if raw := r.URL.Query().Get("bbox"); raw != "" {
			p := strings.Split(raw, ",")
			if len(p) != 4 {
				httpx.Error(w, apperr.Validation("four AOI coordinates required"))
				return
			}
			v := make([]float64, 4)
			for i := range p {
				n, e := strconv.ParseFloat(p[i], 64)
				if e != nil {
					httpx.Error(w, apperr.Validation("invalid AOI"))
					return
				}
				v[i] = n
			}
			b = &publicingest.Bounds{West: v[0], South: v[1], East: v[2], North: v[3]}
		}
		items, e := h.Service.Scenes(r.Context(), b, r.URL.Query().Get("region_id"), r.URL.Query().Get("geometry"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(items) > 100
		if truncated {
			items = items[:100]
		}
		httpx.JSON(w, 200, queryscope.ListMetadata(r.Context(), "scenes", legacyCatalogPage(items, len(items), truncated, "scenes")))
	})
	r.Get("/imagery/scenes/{id}", func(w http.ResponseWriter, r *http.Request) {
		scene, e := h.Service.Scene(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, scene)
	})
	r.Get("/imagery/jobs", func(w http.ResponseWriter, r *http.Request) {
		items, e := h.Service.Jobs(r.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(items) > 100
		if truncated {
			items = items[:100]
		}
		httpx.JSON(w, 200, queryscope.ListMetadata(r.Context(), "imagery", legacyCatalogPage(items, len(items), truncated, "jobs")))
	})
	r.Post("/imagery/jobs", func(w http.ResponseWriter, r *http.Request) {
		var in SubmitInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		j, e := h.Service.Submit(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, j)
	})
	r.Get("/imagery/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		j, e := h.Service.Job(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, j)
	})
	r.Get("/imagery/jobs/{id}/result", func(w http.ResponseWriter, r *http.Request) {
		j, e := h.Service.Job(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if j.Status != "SUCCEEDED" || j.Input.Kind != "NDVI_CHANGE" || j.Result == nil {
			httpx.Error(w, apperr.Conflict("analysis result is not ready"))
			return
		}
		a, b, e := h.Service.Artifacts.Read(r.Context(), j.Result.ArtifactID)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if a.SHA256 != j.Result.SHA256 {
			httpx.Error(w, apperr.Conflict("derived hash mismatch"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(200)
		_, _ = w.Write(b)
	})
	r.Get("/imagery/jobs/{id}/attempts", func(w http.ResponseWriter, r *http.Request) {
		items, e := h.Service.Attempts(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items)})
	})
	r.Post("/imagery/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		j, e := h.Service.Cancel(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, j)
	})
	r.Post("/imagery/jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		j, e := h.Service.Retry(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, j)
	})
}

// Compatibility lists disclose their ceiling. Saved catalogs are the exact,
// stable discovery path; a returned page length is never an omitted total.
func legacyCatalogPage(items any, returned int, more bool, kind string) map[string]any {
	var total any = returned
	if more {
		total = nil
	}
	return map[string]any{"items": items, "total": total, "returned": returned, "truncated": more, "hasMore": more, "representation": "legacy_bounded_first_page", "countMeaning": "Total is unknown when truncated. Open a saved catalog read for an exact count and stable continuation.", "nextCatalogRead": map[string]string{"method": "POST", "path": "/api/v1/catalog/reads", "kind": kind}}
}
