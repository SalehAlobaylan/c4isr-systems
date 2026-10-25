package modelassets

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

func reply(w http.ResponseWriter, v any, e error, status int) {
	if e != nil {
		httpx.Error(w, e)
		return
	}
	httpx.JSON(w, status, v)
}
func (s *Service) Mount(r chi.Router) {
	r.Post("/simulation-assets/uploads", func(w http.ResponseWriter, r *http.Request) {
		var in UploadInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Reserve(r.Context(), in)
		reply(w, v, e, 201)
	})
	for _, family := range []string{"uploads", "models", "templates", "environments"} {
		r.Get("/simulation-assets/"+family, func(w http.ResponseWriter, r *http.Request) {
			items, e := s.List(r.Context(), family)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			truncated := len(items) > 100
			if truncated {
				items = items[:100]
			}
			reply(w, map[string]any{"items": items, "total": len(items), "truncated": truncated}, nil, 200)
		})
	}
	r.Get("/simulation-assets/uploads/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
	r.Get("/simulation-assets/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Model(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
	r.Get("/simulation-assets/models/{id}/manifest", func(w http.ResponseWriter, r *http.Request) {
		b, e := s.Manifest(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(b)
	})
	r.Get("/simulation-assets/uploads/{id}/attempts", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Attempts(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(v) > 100
		if truncated {
			v = v[:100]
		}
		reply(w, map[string]any{"items": v, "total": len(v), "truncated": truncated}, nil, 200)
	})
	r.Put("/simulation-assets/uploads/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBytes))
		if e != nil {
			httpx.Error(w, apperr.Validation("asset body exceeds import budget"))
			return
		}
		v, e := s.Upload(r.Context(), chi.URLParam(r, "id"), b)
		reply(w, v, e, 200)
	})
	r.Get("/simulation-assets/uploads/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		_, b, e := s.Content(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="model-source.bin"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(b)
	})
	for _, family := range []string{"templates", "environments"} {
		r.Get("/simulation-assets/"+family+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			v, e := s.Definition(r.Context(), family, chi.URLParam(r, "id"))
			reply(w, v, e, 200)
		})
	}
	r.Post("/simulation-assets/templates", func(w http.ResponseWriter, r *http.Request) {
		var in ActorTemplate
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Template(r.Context(), in)
		reply(w, v, e, 201)
	})
	r.Post("/simulation-assets/environments", func(w http.ResponseWriter, r *http.Request) {
		var in EnvironmentAsset
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Environment(r.Context(), in)
		reply(w, v, e, 201)
	})
}
