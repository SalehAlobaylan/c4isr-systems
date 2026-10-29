package assessments

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (r *Reviews) Mount(router chi.Router) {
	router.Get("/assessments/{id}/revisions/{revisionID}", func(w http.ResponseWriter, req *http.Request) {
		v, e := r.Revision(req.Context(), chi.URLParam(req, "id"), chi.URLParam(req, "revisionID"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	router.Get("/assessments/reports", func(w http.ResponseWriter, req *http.Request) {
		items, e := r.ListReports(req.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(items) > 100
		if truncated {
			items = items[:100]
		}
		httpx.JSON(w, 200, queryscope.ListMetadata(req.Context(), "imagery", map[string]any{"items": items, "total": len(items), "truncated": truncated}))
	})
	router.Post("/assessments/report-proposals", func(w http.ResponseWriter, req *http.Request) {
		var in Proposal
		if e := httpx.DecodeJSON(req, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		if in.ReportEvidenceID == "" || in.JobID != "" {
			httpx.Error(w, apperr.Validation("report proposal requires report evidence"))
			return
		}
		v, e := r.Propose(req.Context(), in, "")
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	router.Get("/assessments/intelligence", func(w http.ResponseWriter, req *http.Request) {
		items, e := r.List(req.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(items) > 100
		if truncated {
			items = items[:100]
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items), "truncated": truncated})
	})
	router.Post("/assessments/imagery-proposals", func(w http.ResponseWriter, req *http.Request) {
		var in Proposal
		if e := httpx.DecodeJSON(req, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		if in.JobID == "" || in.ReportEvidenceID != "" {
			httpx.Error(w, apperr.Validation("imagery proposal requires an imagery job"))
			return
		}
		v, e := r.Propose(req.Context(), in, "")
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	router.Get("/assessments/{id}/revisions", func(w http.ResponseWriter, req *http.Request) {
		items, e := r.History(req.Context(), chi.URLParam(req, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		truncated := len(items) > 100
		if truncated {
			items = items[:100]
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items), "truncated": truncated})
	})
	router.Post("/assessments/{id}/revisions", func(w http.ResponseWriter, req *http.Request) {
		var in Proposal
		if e := httpx.DecodeJSON(req, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := r.Propose(req.Context(), in, chi.URLParam(req, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	for _, path := range []string{"review", "submit"} {
		actionPath := path
		router.Post("/assessments/{id}/"+path, func(w http.ResponseWriter, req *http.Request) {
			var in ReviewAction
			if e := httpx.DecodeJSON(req, &in); e != nil {
				httpx.Error(w, e)
				return
			}
			if actionPath == "submit" {
				in.Action = "submit"
			}
			v, e := r.Act(req.Context(), chi.URLParam(req, "id"), in)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			httpx.JSON(w, 201, v)
		})
	}
}
