package recordings

import (
	"net/http"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func respond(w http.ResponseWriter, v any, e error, status int) {
	if e != nil {
		httpx.Error(w, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, status, v)
}
func (s *Service) Mount(r chi.Router) {
	r.Route("/simulation-recordings/{id}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			v, e := s.Index(r.Context(), chi.URLParam(r, "id"))
			respond(w, v, e, 200)
		})
		r.Post("/archive", func(w http.ResponseWriter, r *http.Request) {
			v, e := s.Request(r.Context(), chi.URLParam(r, "id"))
			respond(w, v, e, 202)
		})
		r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
			after, e := parseInt(r.URL.Query().Get("after"), 0)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.Events(r.Context(), chi.URLParam(r, "id"), after)
			respond(w, map[string]any{"items": v}, e, 200)
		})
		r.Get("/frame", func(w http.ResponseWriter, r *http.Request) {
			ms, e := parseInt(r.URL.Query().Get("elapsedMs"), 0)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			seq, e := parseInt(r.URL.Query().Get("sequence"), 0)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.Frame(r.Context(), chi.URLParam(r, "id"), int64(ms), seq)
			respond(w, v, e, 200)
		})
		r.Get("/checkpoint", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			c, e := s.authorize(r.Context(), id, "sim.recordings.read")
			if e != nil {
				httpx.Error(w, e)
				return
			}
			idx, e := s.index(c)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			if idx.Status != "READY" || idx.Manifest == nil {
				httpx.Error(w, apperr.Conflict("recording is not ready"))
				return
			}
			raw, e := s.blob(c, idx.Manifest.Checkpoint)
			var v Checkpoint
			if e == nil {
				v, e = DecodeCheckpoint(raw, *idx.Manifest)
			}
			if e == nil {
				e = s.audit(c, "simulation.recording.checkpoint.read", id)
			}
			respond(w, v, e, 200)
		})
		r.Get("/evaluation", func(w http.ResponseWriter, r *http.Request) {
			v, e := s.Evaluation(r.Context(), chi.URLParam(r, "id"))
			respond(w, v, e, 200)
		})
		r.Get("/spatial", func(w http.ResponseWriter, r *http.Request) {
			ms, e := parseInt(r.URL.Query().Get("elapsedMs"), 0)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.Spatial(r.Context(), chi.URLParam(r, "id"), int64(ms), r.URL.Query().Get("reason"))
			respond(w, v, e, 200)
		})
		r.Post("/release", func(w http.ResponseWriter, r *http.Request) {
			var in Release
			if e := httpx.DecodeJSON(r, &in); e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.Release(r.Context(), chi.URLParam(r, "id"), in)
			respond(w, v, e, 201)
		})
		r.Get("/report", func(w http.ResponseWriter, r *http.Request) {
			v, e := s.Released(r.Context(), chi.URLParam(r, "id"))
			respond(w, v, e, 200)
		})
	})
}
