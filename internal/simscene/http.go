package simscene

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func reply(w http.ResponseWriter, v any, e error, status int) {
	if e != nil {
		httpx.Error(w, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, status, v)
}
func (s *Service) Mount(r chi.Router) {
	s.mountEnvironmentView(r)
	r.Get("/simulation-studio/sensor-presets", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Presets(r.Context())
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Post("/simulation-studio/sensor-presets", func(w http.ResponseWriter, r *http.Request) {
		var in PresetInput
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.CreatePreset(r.Context(), in)
		reply(w, v, e, 201)
	})
	r.Get("/simulation-studio/sensor-presets/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Preset(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
	r.Get("/simulation-studio/context-packages", func(w http.ResponseWriter, r *http.Request) {
		c, e := s.Context(r.Context())
		if e != nil {
			reply(w, nil, e, 200)
			return
		}
		v, e := s.Packages.List(c)
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Get("/simulation-studio/scenes", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.List(r.Context())
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Post("/simulation-studio/scenes", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Create(r.Context(), in)
		reply(w, v, e, 201)
	})
	r.Get("/simulation-studio/scenes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e := Require(r.Context(), "sim.scenarios.author", false); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.Load(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
}
