package reference

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) Mount(r chi.Router) {
	h.mountGeography(r)
	h.mountMap(r)
	h.mountSpatial(r)
	r.Get("/reference-features", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Query().Get("bbox"), ",")
		if len(parts) != 4 {
			httpx.Error(w, apperr.Validation("four viewport bounds required"))
			return
		}
		v := make([]float64, 4)
		for i := range v {
			n, e := strconv.ParseFloat(parts[i], 64)
			if e != nil {
				httpx.Error(w, apperr.Validation("invalid viewport bounds"))
				return
			}
			v[i] = n
		}
		offset, e := strconv.Atoi(r.URL.Query().Get("offset"))
		if r.URL.Query().Get("offset") == "" {
			offset = 0
			e = nil
		}
		if e != nil {
			httpx.Error(w, apperr.Validation("invalid viewport offset"))
			return
		}
		out, e := h.s.Viewport(r.Context(), publicingest.Bounds{West: v[0], South: v[1], East: v[2], North: v[3]}, strings.Split(r.URL.Query().Get("datasets"), ","), offset)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, out)
	})
	r.Get("/reference-datasets", func(w http.ResponseWriter, r *http.Request) {
		items, err := h.s.List(r.Context(), r.URL.Query().Get("eligible") == "true")
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items)})
	})
	r.Post("/reference-datasets", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&in); err != nil {
			httpx.Error(w, apperr.Validation("invalid or oversized reference import"))
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			httpx.Error(w, apperr.Validation("trailing reference import data"))
			return
		}
		d, duplicate, err := h.s.Import(r.Context(), in)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		status := 201
		if duplicate {
			status = 200
		}
		httpx.JSON(w, status, d)
	})
	r.Get("/reference-datasets/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, err := h.s.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, d)
	})
	r.Get("/reference-datasets/{id}/features", func(w http.ResponseWriter, r *http.Request) {
		c, err := h.s.Features(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, c)
	})
	r.Get("/reference-datasets/{id}/artifact", func(w http.ResponseWriter, r *http.Request) {
		raw, err := h.s.Artifact(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", `attachment; filename="reference.json"`)
		w.WriteHeader(200)
		_, _ = w.Write(raw)
	})
}
