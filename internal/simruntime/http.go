package simruntime

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
func (b *Broker) Mount(r chi.Router) {
	r.Get("/simulation-runtime/capabilities", func(w http.ResponseWriter, r *http.Request) {
		v, e := b.Capabilities(r.Context())
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Get("/simulation-runtime/queue", func(w http.ResponseWriter, r *http.Request) {
		v, e := b.Queue(r.Context())
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Get("/simulation-runtime/runs", func(w http.ResponseWriter, r *http.Request) {
		v, e := b.List(r.Context())
		reply(w, map[string]any{"items": v}, e, 200)
	})
	r.Post("/simulation-runtime/runs", func(w http.ResponseWriter, r *http.Request) {
		var in Start
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := b.Start(r.Context(), in)
		reply(w, v, e, 201)
	})
	r.Get("/simulation-runtime/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := b.Get(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
	for _, action := range []string{"pause", "resume", "stop"} {
		r.Post("/simulation-runtime/runs/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
			v, e := b.Control(r.Context(), chi.URLParam(r, "id"), action)
			reply(w, v, e, 200)
		})
	}
	r.Post("/simulation-runtime/runs/{id}/attach", func(w http.ResponseWriter, r *http.Request) {
		var in Attach
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := b.Attach(r.Context(), chi.URLParam(r, "id"), in)
		reply(w, v, e, 200)
	})
	r.Post("/simulation-runtime/runs/{id}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var in Beat
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := b.Heartbeat(r.Context(), chi.URLParam(r, "id"), in)
		reply(w, v, e, 200)
	})
	r.Get("/simulation-runtime/runs/{id}/truth", func(w http.ResponseWriter, r *http.Request) {
		v, e := b.Truth(r.Context(), chi.URLParam(r, "id"))
		reply(w, v, e, 200)
	})
}
