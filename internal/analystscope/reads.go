package analystscope

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/reads"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type QueryFilters struct {
	SourceID   string `json:"sourceId,omitempty"`
	Quality    string `json:"quality,omitempty"`
	Phenomenon string `json:"phenomenon,omitempty"`
	Revisions  string `json:"revisions,omitempty"`
}
type ReadInput struct {
	Input
	Families      []string     `json:"families"`
	Filters       QueryFilters `json:"filters"`
	Zoom          int          `json:"zoom"`
	IncludeEvents bool         `json:"includeEvents"`
	Purpose       string       `json:"purpose,omitempty"`
}
type LayerError struct {
	Code          string `json:"code"`
	Meaning       string `json:"meaning"`
	RecoveryOwner string `json:"recoveryOwner"`
}
type ReadSnapshot struct {
	RecordSets       map[string][]string                `json:"recordSets,omitempty"`
	Errors           map[string]LayerError              `json:"errors"`
	EventState       string                             `json:"eventState,omitempty"`
	ID               string                             `json:"id"`
	QueryFingerprint string                             `json:"queryFingerprint"`
	ViewKey          string                             `json:"viewKey"`
	Scope            Snapshot                           `json:"scope"`
	ExpiresAt        time.Time                          `json:"expiresAt"`
	Layers           map[string]reference.MapCollection `json:"layers"`
	Events           *publicevents.QueryResult          `json:"events,omitempty"`
	CoverageReceipts []json.RawMessage                  `json:"coverageReceipts"`
	ReceiptMeaning   string                             `json:"receiptMeaning"`
}

// ViewKey identifies the logical view a refresh supersedes. It is stable
// across a zoom change of the same map view — that churn is what exhausted the
// read budget — and differs for a different area or period, another family
// set, another filter, or events instead of layers. The stored fingerprint
// keeps its own meaning: it identifies one exact query, zoom included.
func ViewKey(ctx context.Context, scope Snapshot, families []string, filters QueryFilters, includeEvents bool) string {
	sorted := append([]string(nil), families...)
	sort.Strings(sorted)
	raw, _ := json.Marshal(struct {
		Scope    string
		Content  string
		Families []string
		Filters  QueryFilters
		Events   bool
	}{scope.ScopeHash, access.ContentView(ctx), sorted, filters, includeEvents})
	return areas.Hash(raw)
}

func QueryValues(scope Snapshot, f QueryFilters) url.Values {
	t := scope.Temporal
	q := url.Values{"view": {t.RevisionView}, "observed_from": {t.Interval.From.Format(time.RFC3339Nano)}, "observed_to": {t.Interval.To.Format(time.RFC3339Nano)}}
	if t.AsOf != nil {
		q.Set("as_of", t.AsOf.Format(time.RFC3339Nano))
	}
	if len(scope.Area.Geometry) > 0 {
		q.Set("geometry", string(scope.Area.Geometry))
	}
	if f.SourceID != "" {
		q.Set("source_id", f.SourceID)
	}
	if f.Quality != "" {
		q.Set("quality", f.Quality)
	}
	return q
}
func (s Service) Read(ctx context.Context, in ReadInput) (ReadSnapshot, error) {
	var out ReadSnapshot
	if e := areas.Require(ctx); e != nil {
		return out, e
	}
	if len(in.Families) > 10 || len(in.Families) == 0 && !in.IncludeEvents || in.Zoom < 0 || in.Zoom > 24 ||
		in.Purpose != "" && in.Purpose != "geographic_preview" && in.Purpose != "geographic_handoff" ||
		(in.Purpose == "geographic_preview" && (in.IncludeEvents || len(in.Families) != 1 ||
			!slices.Contains([]string{"earthquakes", "thermal", "weather", "bulletins"}, in.Families[0]))) {
		return out, apperr.Validation("choose 1–10 supported families and a bounded zoom")
	}
	seen := map[string]bool{}
	for _, family := range in.Families {
		valid := false
		for _, known := range Families[:10] {
			valid = valid || family == known
		}
		if !valid || seen[family] {
			return out, apperr.Validation("unknown or duplicate read family")
		}
		seen[family] = true
	}
	sort.Strings(in.Families)
	now := time.Now().UTC()
	// Only expired ephemeral manifests are reclaimed here. Durable scope, area,
	// evidence and authored content are never subject to this housekeeping.
	if _, e := s.Pool.Exec(ctx, `DELETE FROM analyst_read_snapshots WHERE expires_at<clock_timestamp()`); e != nil {
		return out, e
	}
	e := db.RepeatableRead(ctx, s.Pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SET LOCAL statement_timeout='12s'`); e != nil {
			return e
		}
		scope, e := s.resolve(ctx, in.Input, now)
		if e != nil {
			return e
		}
		if scope.Area.Kind == "point" {
			return apperr.Validation("a place point is a navigation target; choose an area")
		}
		ctx = queryscope.With(ctx, scope.Temporal)
		q := QueryValues(scope, in.Filters)
		q.Set("zoom", strconv.Itoa(in.Zoom))
		// Validate shared filters even when the family is unsupported or empty.
		validation := QueryValues(scope, in.Filters)
		if in.Filters.Phenomenon != "" {
			validation.Set("phenomenon", in.Filters.Phenomenon)
		}
		if in.Filters.Revisions != "" {
			validation.Set("revisions", in.Filters.Revisions)
		}
		if _, e = publicevents.ParseScopeFilters(validation, now); e != nil {
			return e
		}
		if in.Filters.SourceID != "" {
			if e = access.RequireSource(ctx, in.Filters.SourceID); e != nil {
				return e
			}
		}
		out = ReadSnapshot{Errors: map[string]LayerError{}, RecordSets: map[string][]string{}, ID: ids.New("read"), Scope: scope, ExpiresAt: now.Add(5 * time.Minute), Layers: map[string]reference.MapCollection{}, CoverageReceipts: []json.RawMessage{}, ReceiptMeaning: "Pinned connector/admission state observed in this database snapshot. These receipts do not establish complete provider coverage or factual absence.", ViewKey: ViewKey(ctx, scope, in.Families, in.Filters, in.IncludeEvents)}
		if in.Purpose != "" {
			out.ViewKey = areas.Hash([]byte(out.ViewKey + "\x00" + in.Purpose))
		}
		// Creation, cleanup and page/citation validation use the same clock,
		// even when the application and database hosts have clock skew.
		if e = tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '5 minutes'`).Scan(&out.ExpiresAt); e != nil {
			return e
		}
		for _, family := range in.Families {
			if _, e = tx.Exec(ctx, `SAVEPOINT read_family`); e != nil {
				return e
			}
			v, queryErr := reference.NewService(s.Pool).MapFeaturesRead(ctx, family, q)
			if queryErr == nil && (family == "earthquakes" || family == "thermal" || family == "events") {
				out.RecordSets[family] = v.RecordIDs
				v.RecordIDs = nil
			}

			if queryErr != nil {
				if _, e = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT read_family`); e != nil {
					return e
				}
				code := "query_unavailable"
				if apperr.Is(queryErr, apperr.CodeForbidden) {
					code = "access_required"
				}
				out.Errors[family] = LayerError{Code: code, Meaning: "This family could not be evaluated. Its count is unknown; successful sibling results remain available.", RecoveryOwner: "source administrator"}
				v = reference.MapCollection{Type: "FeatureCollection", Features: []reference.Feature{}, State: "unavailable", CountMeaning: "Count unknown: query unavailable", EvaluatedAt: &scope.Temporal.EvaluatedAt}
			}
			if _, e = tx.Exec(ctx, `RELEASE SAVEPOINT read_family`); e != nil {
				return e
			}

			v.ScopeHash = scope.ScopeHash
			out.Layers[family] = v
		}
		if in.IncludeEvents {
			f, e := publicevents.ParseScopeFilters(validation, now)
			if e != nil {
				return e
			}
			if in.Filters.Revisions != "" && in.Filters.Revisions != "latest" && in.Filters.Revisions != "all" {
				return apperr.Validation("invalid revision selection")
			}
			if in.Filters.Revisions != "" {
				f.Revisions = in.Filters.Revisions
			}
			f.Phenomenon = in.Filters.Phenomenon
			f.Limit = 5001
			if _, e = tx.Exec(ctx, `SAVEPOINT read_events`); e != nil {
				return e
			}
			records, queryErr := publicevents.Query(ctx, s.Pool, f, "", false)
			if queryErr == nil && records.Total > 5000 {
				queryErr = apperr.Conflict("event read manifest exceeds 5000 revisions; narrow the scope")
			}
			if queryErr != nil {
				if _, e = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT read_events`); e != nil {
					return e
				}
				out.EventState = "unavailable"
				out.Errors["events"] = LayerError{Code: "query_unavailable", Meaning: "Event count unknown; narrow the scope or inspect source access, then explicitly refresh.", RecoveryOwner: "analyst"}
			} else {
				out.Events = &records
				out.EventState = "available"
			}
			if _, e = tx.Exec(ctx, `RELEASE SAVEPOINT read_events`); e != nil {
				return e
			}
		}
		rows, e := tx.Query(ctx, `SELECT jsonb_build_object('sourceId',c.source_id,'profileKey',c.profile_key,'profileRevisionId','rpr_'||c.admission_id,'admissionId',c.admission_id,'coverageFrom',c.coverage_from,'coverageTo',c.coverage_to,'lastSuccessAt',c.last_success_at,'freshForSeconds',a.document#>'{policy,freshForSeconds}','connectorRunId',v.id,'runState',v.document) FROM retrieval_profile_state c JOIN source_admissions a ON a.id=c.admission_id LEFT JOIN LATERAL(SELECT id,document FROM connector_runs v WHERE v.source_id=c.source_id AND v.admission_id=c.admission_id ORDER BY id DESC LIMIT 1)v ON true WHERE ($1::text='' OR c.source_id=$1) ORDER BY c.source_id,c.profile_key LIMIT 257`, in.Filters.SourceID)
		if e != nil {
			return e
		}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return e
			}
			out.CoverageReceipts = append(out.CoverageReceipts, json.RawMessage(raw))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(out.CoverageReceipts) > 256 {
			return apperr.Conflict("read receipt budget exceeded; select a source")
		}
		fingerprint, _ := json.Marshal(struct {
			Scope, Access, ContentView string
			Families                   []string
			Filters                    QueryFilters
			Zoom                       int
			Events                     bool
		}{scope.ScopeHash, scope.AuthorizationRevision, access.ContentView(ctx), in.Families, in.Filters, in.Zoom, in.IncludeEvents})
		out.QueryFingerprint = areas.Hash(fingerprint)
		raw, _ := json.Marshal(out)
		if len(raw) > 8<<20 {
			return apperr.Conflict("read manifest exceeds 8 MiB; narrow the query")
		}
		if in.Purpose == "geographic_preview" {
			// The renderer inspects only this response. It cannot page or freeze it.
			out.ID = ""
			return nil
		}
		// A refresh of the same logical view supersedes the retained read it
		// replaces. Removing it inside this transaction frees its budget slot
		// for the manifest stored immediately afterwards, so repeatedly
		// refreshing one view cannot exhaust the bounded read capacity.
		if e = reads.Supersede(ctx, tx, "map", out.ViewKey); e != nil {
			return e
		}
		e = reads.Save(ctx, tx, out.ID, "map", raw, out.ExpiresAt)
		return e
	})
	var pe *pgconn.PgError
	if errors.As(e, &pe) && (pe.Code == "23505" || pe.Code == "40001") {
		e = apperr.Conflict("concurrent read budget claim; retry the view")
	}
	out.RecordSets = nil
	return out, e
}
func (s Service) ReadPage(ctx context.Context, id, family string, offset, limit int) (map[string]any, error) {
	if e := areas.Require(ctx); e != nil {
		return nil, e
	}
	if offset < 0 || limit < 1 || limit > 200 {
		return nil, apperr.Validation("invalid read cursor")
	}
	var raw []byte
	var accessRevision string
	var expiry time.Time
	var active bool
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document,authorization_revision,expires_at,expires_at>clock_timestamp() FROM analyst_read_snapshots WHERE id=$1 AND snapshot_kind='map'`, id).Scan(&raw, &accessRevision, &expiry, &active)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperr.Conflict("read snapshot unavailable or expired; restart this view")
	}
	if e != nil {
		return nil, e
	}
	if !active || accessRevision != access.AccessRevision(ctx) {
		return nil, apperr.Conflict("read snapshot expired or authorization changed; restart this view")
	}
	var v ReadSnapshot
	if e = json.Unmarshal(raw, &v); e != nil {
		return nil, e
	}
	result := map[string]any{"snapshotId": id, "scopeHash": v.Scope.ScopeHash, "evaluatedAt": v.Scope.Temporal.EvaluatedAt, "expiresAt": expiry, "queryFingerprint": v.QueryFingerprint, "offset": offset}
	if family == "event_records" && v.Events != nil {
		items := v.Events.Items
		if offset > len(items) {
			return nil, apperr.Validation("cursor outside materialized membership")
		}
		end := min(offset+limit, len(items))
		result["items"] = items[offset:end]
		result["total"] = v.Events.Total
		result["materialized"] = len(items)
		result["hasMore"] = end < len(items)
		return result, nil
	}
	layer, ok := v.Layers[family]
	if !ok {
		return nil, apperr.Validation("family is not in this read snapshot")
	}
	if layer.State == "unavailable" || layer.State == "historical_unsupported" {
		return nil, apperr.Conflict("family query is unavailable; count is unknown; refresh or choose compatible scope")
	}
	if offset > len(layer.Features) {
		return nil, apperr.Validation("cursor outside materialized membership")
	}
	end := min(offset+limit, len(layer.Features))
	result["features"] = layer.Features[offset:end]
	result["total"] = layer.Total
	result["materialized"] = len(layer.Features)
	result["state"] = layer.State
	result["hasMore"] = end < len(layer.Features)
	result["requiresNarrowerScope"] = layer.State == "aggregated"
	return result, nil
}

// Release drops a retained read manifest so the budget slot it holds is
// available immediately instead of at expiry. The deletion runs inside a
// transaction bound to the caller's scope and principal, so row-level
// security prevents one principal from releasing another principal's read.
func (s Service) Release(ctx context.Context, id string) (bool, error) {
	if e := areas.Require(ctx); e != nil {
		return false, e
	}
	if id == "" || len(id) > 160 {
		return false, apperr.Validation("invalid read snapshot identity")
	}
	released := false
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var e error
		released, e = reads.Release(ctx, tx, id)
		return e
	})
	if e != nil {
		return false, e
	}
	return released, nil
}
func (s Service) mountReads(r chi.Router) {
	s.mountRecords(r)
	r.Post("/analyst-scope/read", func(w http.ResponseWriter, r *http.Request) {
		var in ReadInput
		if !areas.Decode(w, r, &in) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		v, e := s.Read(ctx, in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/analyst-scope/reads/{id}/release", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		id := chi.URLParam(r, "id")
		released, e := s.Release(ctx, id)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		if !released {
			httpx.Error(w, apperr.NotFound("read snapshot", id))
			return
		}
		httpx.JSON(w, 200, map[string]any{"id": id, "released": true})
	})
	r.Get("/analyst-scope/reads/{id}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for key, values := range q {
			if len(values) != 1 || key != "family" && key != "offset" && key != "limit" {
				httpx.Error(w, apperr.Validation("invalid read cursor"))
				return
			}
		}
		offset, limit := 0, 100
		var e error
		if q.Has("offset") {
			offset, e = strconv.Atoi(q.Get("offset"))
			if e != nil {
				httpx.Error(w, apperr.Validation("invalid offset"))
				return
			}
		}
		if q.Has("limit") {
			limit, e = strconv.Atoi(q.Get("limit"))
			if e != nil {
				httpx.Error(w, apperr.Validation("invalid limit"))
				return
			}
		}
		v, e := s.ReadPage(r.Context(), chi.URLParam(r, "id"), q.Get("family"), offset, limit)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}
