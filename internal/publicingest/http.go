package publicingest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func (h *Handler) Mount(r chi.Router) {
	h.mountProfiles(r)
	r.Post("/sources/{id}/scheduler/run", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.service.RunScheduled(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/sources/{id}/acquisition-metrics", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.service.AcquisitionMetrics(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/sources/{id}/activation", func(w http.ResponseWriter, r *http.Request) {
		v, e := h.service.Activation(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
	r.Post("/sources/{id}/admission", func(w http.ResponseWriter, r *http.Request) {
		var in AdmissionInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.Error(w, err)
			return
		}
		a, err := h.service.Admit(r.Context(), chi.URLParam(r, "id"), in)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, a)
	})
	r.Get("/sources/{id}/admission", func(w http.ResponseWriter, r *http.Request) {
		a, err := h.service.Admission(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, a)
	})
	r.Get("/sources/{id}/admissions/{admissionID}", func(w http.ResponseWriter, r *http.Request) {
		a, err := h.service.AdmissionVersion(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "admissionID"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, a)
	})
	r.Get("/sources/{id}/health", func(w http.ResponseWriter, r *http.Request) {
		a, err := h.service.Health(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, a)
	})
	r.Post("/sources/{id}/connector-runs", func(w http.ResponseWriter, r *http.Request) {
		var window *Window
		var in Window
		if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
			if e := httpx.DecodeJSON(r, &in); e != nil && !errors.Is(e, io.EOF) {
				httpx.Error(w, e)
				return
			}
			if !in.From.IsZero() || !in.To.IsZero() {
				window = &in
			}
		}
		a, err := h.service.ExecuteWindow(r.Context(), chi.URLParam(r, "id"), window)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, a)
	})
	r.Get("/sources/{id}/connector-runs", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageBounds(r)
		items, total, err := h.service.Runs(r.Context(), chi.URLParam(r, "id"), limit, offset)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, httpx.ListResponse[Run]{Items: items, Total: total})
	})
	r.Get("/sources/{id}/evidence", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageBounds(r)
		items, total, err := h.service.EvidenceList(r.Context(), chi.URLParam(r, "id"), limit, offset)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, httpx.ListResponse[Evidence]{Items: items, Total: total})
	})
	r.Get("/sources/{id}/evidence/{evidenceID}", func(w http.ResponseWriter, r *http.Request) {
		a, err := h.service.Evidence(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "evidenceID"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, a)
	})
	r.Get("/sources/{id}/evidence/{evidenceID}/original", func(w http.ResponseWriter, r *http.Request) {
		e, err := h.service.Evidence(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "evidenceID"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		raw := []byte(e.OriginalRecord)
		mediaType := "application/json"
		extension := "json"
		if e.OriginalMediaType == "text/csv" {
			raw = e.OriginalBytes
			mediaType = "text/csv"
			extension = "csv"
		}
		if e.OriginalMediaType == "application/xml" {
			raw = e.OriginalBytes
			mediaType = "application/xml"
			extension = "xml"
		}
		if len(raw) == 0 {
			httpx.Error(w, apperr.NotFound("retained original", "use the permitted provider reference"))
			return
		}
		w.Header().Set("Content-Type", mediaType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="`+e.ID+`.`+extension+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
	r.Post("/sources/{id}/publications/retry", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		count, err := h.service.Replay(ctx, chi.URLParam(r, "id"), 1000)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]int{"published": count})
	})
}
func pageBounds(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	return min(limit, 100), max(offset, 0)
}
