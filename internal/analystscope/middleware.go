package analystscope

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
)

// CatalogQueries is an additive adapter. Exact retained reads do not rebind to
// discovery scope. Old requests retain their original interval semantics.
func (s Service) CatalogQueries(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !q.Has("scope") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == "GET" && !q.Has("scope_hash") && (r.URL.Path == "/api/v1/picture" || r.URL.Path == "/api/v1/picture/window") {
			// Picture scope is a legacy selector inside its authorized run.
			next.ServeHTTP(w, r)
			return
		}
		supported := map[string]bool{"/api/v1/imagery/scenes": true, "/api/v1/imagery/captures": true, "/api/v1/imagery/jobs": true, "/api/v1/assessments/intelligence": true, "/api/v1/assessments/reports": true, "/api/v1/public-context/records": true, "/api/v1/external-tracks": true, "/api/v1/public-events": true}
		if r.Method != "GET" || !supported[strings.TrimSuffix(r.URL.Path, "/")] || len(q["scope"]) != 1 || len(q["scope_hash"]) != 1 || len(q.Get("scope")) > 3<<20 {
			httpx.Error(w, apperr.Validation("unsupported or ambiguous scoped catalog request"))
			return
		}
		for _, key := range []string{"region_id", "geometry", "bbox", "view", "as_of", "observed_from", "observed_to"} {
			if q.Get(key) != "" {
				httpx.Error(w, apperr.Validation("conflicting legacy and versioned scope"))
				return
			}
		}
		var in Input
		dec := json.NewDecoder(strings.NewReader(q.Get("scope")))
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
			httpx.Error(w, apperr.Validation("invalid catalog scope"))
			return
		}
		if in.Temporal.Interval == nil || in.Temporal.RollingSeconds != 0 {
			httpx.Error(w, apperr.Validation("catalog calls require the server-evaluated fixed period from scope resolution"))
			return
		}
		v, e := s.Resolve(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if v.ScopeHash != q.Get("scope_hash") {
			httpx.Error(w, apperr.Conflict("scope fingerprint changed; resolve the scope again"))
			return
		}
		if v.Area.Kind == "point" {
			httpx.Error(w, apperr.Validation("place point is not an analytical area"))
			return
		}
		ctx := queryscope.With(r.Context(), v.Temporal)
		ctx = queryscope.WithResolution(ctx, queryscope.Resolution{Geometry: string(v.Area.Geometry), Kind: v.Area.Kind, ScopeHash: v.ScopeHash, AuthorizationRevision: v.AuthorizationRevision})
		q.Del("scope")
		q.Del("scope_hash")
		if strings.HasSuffix(r.URL.Path, "/public-events") {
			for key, values := range QueryValues(v, QueryFilters{}) {
				q[key] = values
			}
		}
		r.URL.RawQuery = q.Encode()
		w.Header().Set("X-Analyst-Scope-Hash", v.ScopeHash)
		w.Header().Set("X-Query-Evaluated-At", v.Temporal.EvaluatedAt.Format(time.RFC3339Nano))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
