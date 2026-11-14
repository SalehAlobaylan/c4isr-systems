// Package coverage describes task readiness for a resolved area and period.
// Operational health, provider retrieval, reference depth and observations are
// independent dimensions. Missing records never prove factual absence.
package coverage

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type Input struct {
	Scope    analystscope.Input `json:"scope"`
	SourceID string             `json:"sourceId,omitempty"`
}
type Action struct {
	Kind              string `json:"kind"`
	SourceID          string `json:"sourceId,omitempty"`
	ProfileKey        string `json:"profileKey,omitempty"`
	ProfileRevisionID string `json:"profileRevisionId,omitempty"`
	RequestID         string `json:"requestId,omitempty"`
	Owner             string `json:"owner"`
	Reason            string `json:"reason"`
}
type Profile struct {
	Key              string              `json:"key"`
	RevisionID       string              `json:"revisionId"`
	Gate             string              `json:"gate"`
	SpatialRelation  string              `json:"spatialRelation"`
	ExecutionGranted bool                `json:"executionGranted"`
	Rights           publicingest.Rights `json:"rights"`
}
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
type Source struct {
	ID                      string                         `json:"id"`
	Name                    string                         `json:"name"`
	Family                  string                         `json:"family"`
	Support                 string                         `json:"support"`
	SupportReason           string                         `json:"supportReason"`
	Capability              publicingest.AdapterCapability `json:"capability"`
	Profiles                []Profile                      `json:"profiles"`
	SpatialRelation         string                         `json:"spatialRelation"`
	Retrieval               string                         `json:"retrieval"`
	DataResult              string                         `json:"dataResult"`
	UsableRetainedRecords   *int                           `json:"usableRetainedRecords"`
	CompletedWindows        []Window                       `json:"completedWindows"`
	MissingWindows          []Window                       `json:"missingWindows"`
	LatestRetrieval         *time.Time                     `json:"latestRetrieval,omitempty"`
	LatestEvidenceAt        *time.Time                     `json:"latestEvidenceAt,omitempty"`
	Freshness               string                         `json:"freshness"`
	ObservationCompleteness string                         `json:"observationCompleteness"`
	Receipts                []publicingest.Receipt         `json:"receipts"`
	ReceiptsTruncated       bool                           `json:"receiptsTruncated"`
	Actions                 []Action                       `json:"actions"`
}
type Result struct {
	Schema         string                `json:"schema"`
	Scope          analystscope.Snapshot `json:"scope"`
	EvaluatedAt    time.Time             `json:"evaluatedAt"`
	Sources        []Source              `json:"sources"`
	ReferenceDepth any                   `json:"referenceDepth,omitempty"`
	Storage        string                `json:"storage"`
	Meaning        string                `json:"meaning"`
	RecoveryOwner  string                `json:"recoveryOwner"`
}
type Service struct {
	Metrics   *observability.Metrics
	Pool      *pgxpool.Pool
	Ingestion *publicingest.Service
}

func (s Service) Query(ctx context.Context, in Input) (Result, error) {
	var out Result
	if e := areas.Require(ctx); e != nil {
		return out, e
	}
	if in.SourceID != "" {
		if e := access.RequireSource(ctx, in.SourceID); e != nil {
			return out, e
		}
	}
	out = Result{Schema: "emad.coverage.v1", EvaluatedAt: time.Now().UTC(), Sources: []Source{}, Storage: "unknown", Meaning: "Authorized retained evidence and provider query receipts for this exact scope. Retrieval completeness is separate from observation completeness; empty results do not establish absence.", RecoveryOwner: "source administrator"}
	e := db.RepeatableRead(ctx, s.Pool, func(ctx context.Context, tx pgx.Tx) error {
		scope, e := (analystscope.Service{Pool: s.Pool}).Resolve(ctx, in.Scope)
		if e != nil {
			return e
		}
		out.Scope = scope
		out.EvaluatedAt = scope.Temporal.EvaluatedAt
		rows, e := tx.Query(ctx, `SELECT s.id,s.name,COALESCE(jsonb_agg(jsonb_build_object('key',p.profile_key,'revision',p.head_revision_id))FILTER(WHERE p.profile_key IS NOT NULL),'[]'::jsonb) FROM sources s LEFT JOIN retrieval_profiles p ON p.source_id=s.id AND p.scope_id=s.scope_id WHERE ($1='' OR s.id=$1) GROUP BY s.id,s.name ORDER BY s.id LIMIT 257`, in.SourceID)
		if e != nil {
			return e
		}
		type sourceRow struct {
			id, name string
			profiles []struct {
				Key      string `json:"key"`
				Revision string `json:"revision"`
			}
		}
		entries := []sourceRow{}
		for rows.Next() {
			var v sourceRow
			var raw []byte
			if e = rows.Scan(&v.id, &v.name, &raw); e != nil {
				rows.Close()
				return e
			}
			if e = json.Unmarshal(raw, &v.profiles); e != nil {
				rows.Close()
				return e
			}
			entries = append(entries, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(entries) > 256 {
			return apperr.Conflict("coverage source budget exceeded; choose a source")
		}
		for _, entry := range entries {
			v := Source{ID: entry.id, Name: entry.name, Family: "unknown", Support: "not_admitted", Profiles: []Profile{}, SpatialRelation: "unknown", Retrieval: "no_retrieval", DataResult: "no_retrieval", Freshness: "unknown", ObservationCompleteness: "unknown", Receipts: []publicingest.Receipt{}, CompletedWindows: []Window{}, MissingWindows: []Window{}, Actions: []Action{}}
			footprints := []publicingest.Bounds{}
			for _, p := range entry.profiles {
				policy, e := s.Ingestion.AcquisitionPolicy(ctx, entry.id, p.Key)
				if e != nil {
					return e
				}
				v.Capability = policy.Capability
				v.Family = policy.Capability.Family
				relation, e := spatial(ctx, tx, scope, []publicingest.Bounds{policy.Admission.Policy.AOI})
				if e != nil {
					return e
				}
				v.Profiles = append(v.Profiles, Profile{Key: p.Key, RevisionID: p.Revision, Gate: policy.Gate, SpatialRelation: relation, ExecutionGranted: policy.ExecutionGranted, Rights: policy.Admission.Rights})
				if policy.Gate == "ready" {
					footprints = append(footprints, policy.Admission.Policy.AOI)
				}
				kind, reason, owner := "inspect_source", "Review this profile's provider prerequisites and rights.", "source administrator"
				if policy.Gate == "ready" && policy.ExecutionGranted && relation == "full" && access.Allows(ctx, "acquisitions.request") {
					kind, reason, owner = "request_acquisition", "Queue a deliberate acquisition using this approved revision.", "analyst"
				}
				if relation == "none" || relation == "partial" {
					kind, reason = "request_coverage_review", "The selected area exceeds this approved profile."
				}
				if !policy.Capability.Historical {
					kind, reason = "choose_compatible_dates", "This adapter uses current polling and cannot replay a frozen acquisition window."
				}
				v.Actions = append(v.Actions, Action{Kind: kind, SourceID: entry.id, ProfileKey: p.Key, ProfileRevisionID: p.Revision, Owner: owner, Reason: reason})
			}
			if len(entry.profiles) > 0 {
				v.Support = "admitted"
				v.SpatialRelation, e = spatial(ctx, tx, scope, footprints)
				if e != nil {
					return e
				}
			}
			support := queryscope.Capability(v.Family, scope.Temporal)
			if support.State == "unsupported" {
				v.Support = "unsupported"
				v.SupportReason = support.Reason
				v.DataResult = "unavailable_query"
			}
			// Only retained receipts establish prior coverage. Today's profile AOI is
			// never substituted for an earlier query's attempted footprint.
			rr, e := tx.Query(ctx, `SELECT document FROM retrieval_receipts WHERE source_id=$1 AND (document->>'from')::timestamptz<$3 AND (document->>'to')::timestamptz>$2 ORDER BY (document->>'completedAt')::timestamptz DESC,run_id LIMIT 257`, entry.id, scope.Temporal.Interval.From, scope.Temporal.Interval.To)
			if e != nil {
				return e
			}
			for rr.Next() {
				var raw []byte
				var receipt publicingest.Receipt
				if e = rr.Scan(&raw); e != nil {
					rr.Close()
					return e
				}
				if e = json.Unmarshal(raw, &receipt); e != nil {
					rr.Close()
					return e
				}
				v.Receipts = append(v.Receipts, receipt)
			}
			e = rr.Err()
			rr.Close()
			if e != nil {
				return e
			}
			if len(v.Receipts) > 256 {
				v.ReceiptsTruncated = true
				v.Receipts = v.Receipts[:256]
			}
			// Exclude spatially disjoint receipts from both status and union coverage.
			relevant := []publicingest.Receipt{}
			for _, receipt := range v.Receipts {
				receipt.RecordedUnpublishedRecords = receipt.UnpublishedRecords
				if receipt.Status == "SUCCEEDED" && receipt.Pages > 0 && receipt.RejectedRecords == 0 && !receipt.ProviderTruncated {
					pending, e := publicationGaps(ctx, tx, receipt)
					if e != nil {
						return e
					}
					receipt.PublicationReconciled = pending != receipt.UnpublishedRecords
					receipt.UnpublishedRecords = pending
					receipt.QueryComplete = pending == 0
				}
				if receipt.AOI == nil {
					continue
				}
				relation, e := spatial(ctx, tx, scope, []publicingest.Bounds{*receipt.AOI})
				if e != nil {
					return e
				}
				if relation == "none" {
					continue
				}
				relevant = append(relevant, receipt)
				if receipt.Status == "SUCCEEDED" && receipt.Pages > 0 && (v.LatestRetrieval == nil || receipt.CompletedAt.After(*v.LatestRetrieval)) {
					at := receipt.CompletedAt
					v.LatestRetrieval = &at
				}
			}
			v.Receipts = relevant
			v.CompletedWindows, v.MissingWindows, e = windows(ctx, tx, scope, v.Receipts)
			if e != nil {
				return e
			}
			if len(v.Receipts) > 0 {
				v.Retrieval = "partial_retrieval"
			}
			failedOnly := len(v.Receipts) > 0
			for _, receipt := range v.Receipts {
				if receipt.Status != "FAILED" || receipt.Pages > 0 {
					failedOnly = false
				}
			}
			if failedOnly {
				v.Retrieval = "unavailable_query"
			}
			if len(v.CompletedWindows) > 0 && len(v.MissingWindows) == 0 {
				v.Retrieval = "complete_query_footprint"
			}
			if v.Support != "unsupported" && v.Family != "unknown" {
				q := analystscope.QueryValues(scope, analystscope.QueryFilters{SourceID: entry.id})
				bound := queryscope.With(ctx, scope.Temporal)
				if _, e = tx.Exec(ctx, `SAVEPOINT coverage_records`); e != nil {
					return e
				}
				collection, queryErr := reference.NewService(s.Pool).MapFeatures(bound, v.Family, q)
				if queryErr != nil {
					if _, e = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT coverage_records`); e != nil {
						return e
					}
					v.DataResult = "unavailable_query"
				} else {
					count := collection.Total
					v.UsableRetainedRecords = &count
					for _, feature := range collection.Features {
						var properties struct {
							Date time.Time `json:"date"`
						}
						raw, _ := json.Marshal(feature.Properties)
						_ = json.Unmarshal(raw, &properties)
						if !properties.Date.IsZero() && (v.LatestEvidenceAt == nil || properties.Date.After(*v.LatestEvidenceAt)) {
							at := properties.Date
							v.LatestEvidenceAt = &at
						}
					}
					var age *time.Duration
					if v.LatestEvidenceAt != nil {
						elapsed := out.EvaluatedAt.Sub(*v.LatestEvidenceAt)
						age = &elapsed
					}
					failed := len(v.Receipts) > 0 && v.Receipts[0].FailureCode != ""
					s.Metrics.ObserveContextCoverage(failed, collection.Truncated || collection.State == "aggregated", age)
					if count > 0 {
						v.DataResult = "usable_retained_records"
					} else if v.Retrieval == "complete_query_footprint" {
						v.DataResult = "successful_empty_retrieval"
						for _, receipt := range v.Receipts {
							if receipt.RetainedRecords > 0 {
								v.DataResult = "no_usable_retained_records"
							}
						}
					} else {
						v.DataResult = v.Retrieval
					}
				}
				if _, e = tx.Exec(ctx, `RELEASE SAVEPOINT coverage_records`); e != nil {
					return e
				}
			}
			if v.LatestRetrieval != nil {
				v.Freshness = "stale"
				for _, p := range entry.profiles {
					policy, e := s.Ingestion.AcquisitionPolicy(ctx, entry.id, p.Key)
					if e != nil {
						return e
					}
					if v.LatestRetrieval.After(out.EvaluatedAt.Add(-time.Duration(policy.Admission.Policy.FreshForSeconds) * time.Second)) {
						v.Freshness = "fresh"
					}
				}
			}
			work, e := tx.Query(ctx, `SELECT id,status,document->>'recoveryOwner' FROM acquisition_requests WHERE source_id=$1 AND scope_snapshot_id IN(SELECT id FROM analyst_scope_snapshots WHERE document->>'scopeHash'=$2) ORDER BY created_at DESC LIMIT 20`, entry.id, scope.ScopeHash)
			if e != nil {
				return e
			}
			for work.Next() {
				var id, status, owner string
				if e = work.Scan(&id, &status, &owner); e != nil {
					work.Close()
					return e
				}
				v.Actions = append(v.Actions, Action{Kind: "inspect_request", SourceID: entry.id, RequestID: id, Owner: owner, Reason: status})
			}
			e = work.Err()
			work.Close()
			if e != nil {
				return e
			}
			out.Sources = append(out.Sources, v)
		}
		if scope.Area.Geography != nil {
			depth, e := reference.NewService(s.Pool).GeographyAvailability(ctx, scope.Area.Geography.RegionID, scope.Area.Geography.ReleaseID)
			if e != nil {
				return e
			}
			out.ReferenceDepth = depth
		}
		return nil
	})
	if e != nil {
		return out, e
	}
	// The capacity check uses a separate control-plane transaction so its
	// globally serialized storage lock never nests inside the read snapshot.
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if e = capacity.Check(ctx, tx, 0, ""); e != nil {
		out.Storage = "blocked"
	} else {
		out.Storage = "available"
	}
	return out, nil
}

// A publication retry can close an original immutable receipt's outbox gap
// without refetching provider bytes. The original gap is retained separately in
// the response; a frozen read keeps the annotation observed at its own read.
func publicationGaps(ctx context.Context, tx pgx.Tx, r publicingest.Receipt) (int, error) {
	raw, _ := json.Marshal(r.AOI)
	var pending int
	e := tx.QueryRow(ctx, `SELECT count(DISTINCT o.evidence_id) FROM retrieval_occurrences o JOIN connector_runs c ON c.id=o.run_id AND c.scope_id=o.scope_id JOIN public_evidence_outbox p ON p.evidence_id=o.evidence_id AND p.scope_id=o.scope_id WHERE c.source_id=$1 AND c.admission_id=$2 AND (c.document->>'queryFrom')::timestamptz=$3 AND (c.document->>'queryTo')::timestamptz=$4 AND c.document->'queryAoi'=$5::jsonb AND COALESCE(c.document->>'acquisitionRequestId','')=$6 AND p.delivered_at IS NULL`, r.SourceID, r.AdmissionID, r.From, r.To, raw, r.RequestID).Scan(&pending)
	return pending, e
}

func spatial(ctx context.Context, tx pgx.Tx, scope analystscope.Snapshot, bounds []publicingest.Bounds) (string, error) {
	if len(scope.Area.Geometry) == 0 || len(bounds) == 0 {
		return "unknown", nil
	}
	raw, _ := json.Marshal(bounds)
	var relation string
	e := tx.QueryRow(ctx, `WITH a AS(SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326)g),b AS(SELECT ST_UnaryUnion(ST_Collect(ST_MakeEnvelope((x->>'west')::float8,(x->>'south')::float8,(x->>'east')::float8,(x->>'north')::float8,4326)))g FROM jsonb_array_elements($2::jsonb)x) SELECT CASE WHEN b.g IS NULL THEN 'none' WHEN ST_Covers(b.g,a.g)THEN 'full' WHEN ST_Intersects(b.g,a.g)THEN 'partial' ELSE 'none' END FROM a,b`, scope.Area.Geometry, raw).Scan(&relation)
	return relation, e
}
func windows(ctx context.Context, tx pgx.Tx, scope analystscope.Snapshot, receipts []publicingest.Receipt) ([]Window, []Window, error) {
	complete, missing := []Window{}, []Window{}
	if len(scope.Area.Geometry) == 0 {
		return complete, []Window{{scope.Temporal.Interval.From, scope.Temporal.Interval.To}}, nil
	}
	raw, _ := json.Marshal(receipts)
	rows, e := tx.Query(ctx, `WITH eligible AS(SELECT GREATEST((x->>'from')::timestamptz,$3)f,LEAST((x->>'to')::timestamptz,$4)t,ST_MakeEnvelope((x#>>'{aoi,west}')::float8,(x#>>'{aoi,south}')::float8,(x#>>'{aoi,east}')::float8,(x#>>'{aoi,north}')::float8,4326)g FROM jsonb_array_elements($2::jsonb)x WHERE (x->>'queryComplete')::boolean AND x->'aoi'<>'null'::jsonb),edges AS(SELECT $3::timestamptz d UNION SELECT $4::timestamptz UNION SELECT f FROM eligible UNION SELECT t FROM eligible),slabs AS(SELECT d f,lead(d)OVER(ORDER BY d)t FROM edges) SELECT s.f,s.t,COALESCE(ST_Covers((SELECT ST_UnaryUnion(ST_Collect(g)) FROM eligible e WHERE e.f<=s.f AND e.t>=s.t),ST_SetSRID(ST_GeomFromGeoJSON($1),4326)),false) FROM slabs s WHERE s.t>s.f ORDER BY s.f`, scope.Area.Geometry, raw, scope.Temporal.Interval.From, scope.Temporal.Interval.To)
	if e != nil {
		return nil, nil, e
	}
	defer rows.Close()
	appendWindow := func(items []Window, v Window) []Window {
		if len(items) > 0 && items[len(items)-1].To.Equal(v.From) {
			items[len(items)-1].To = v.To
			return items
		}
		return append(items, v)
	}
	for rows.Next() {
		var v Window
		var covered bool
		if e = rows.Scan(&v.From, &v.To, &covered); e != nil {
			return nil, nil, e
		}
		if covered {
			complete = appendWindow(complete, v)
		} else {
			missing = appendWindow(missing, v)
		}
	}
	return complete, missing, rows.Err()
}
func (s Service) Mount(r chi.Router) {
	r.Post("/coverage/query", func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.Query(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
	r.Get("/coverage/adapters", func(w http.ResponseWriter, r *http.Request) {
		if e := areas.Require(r.Context()); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": publicingest.CapabilityInventory(), "meaning": "Installed adapter operations; source rights and credentials are independent."})
	})
}
