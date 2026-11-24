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
	"strings"
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
		rows, e := tx.Query(ctx, `SELECT s.id,s.name,s.status,COALESCE(s.metadata,'{}'::jsonb),COALESCE(jsonb_agg(jsonb_build_object('key',p.profile_key,'revision',p.head_revision_id) ORDER BY p.profile_key)FILTER(WHERE p.profile_key IS NOT NULL),'[]'::jsonb) FROM sources s LEFT JOIN retrieval_profiles p ON p.source_id=s.id AND p.scope_id=s.scope_id WHERE ($1='' OR s.id=$1) GROUP BY s.id,s.name,s.status,s.metadata ORDER BY s.id LIMIT 257`, in.SourceID)
		if e != nil {
			return e
		}
		type sourceRow struct {
			id, name, status string
			metadata         map[string]any
			profiles         []struct {
				Key      string `json:"key"`
				Revision string `json:"revision"`
			}
		}
		entries := []sourceRow{}
		for rows.Next() {
			var v sourceRow
			var metaRaw, raw []byte
			if e = rows.Scan(&v.id, &v.name, &v.status, &metaRaw, &raw); e != nil {
				rows.Close()
				return e
			}
			_ = json.Unmarshal(metaRaw, &v.metadata)
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
			category, _ := entry.metadata["category"].(string)
			if len(entry.profiles) == 0 {
				if category == "GEOGRAPHIC_REFERENCE" {
					if e = evaluateReferenceSource(ctx, tx, scope, entry.id, entry.metadata, &v); e != nil {
						return e
					}
					out.Sources = append(out.Sources, v)
					continue
				}
				evaluateUnadmittedSource(ctx, s.Ingestion, entry.id, entry.metadata, &v)
				out.Sources = append(out.Sources, v)
				continue
			}

			footprints := []publicingest.Bounds{}
			policies := make([]publicingest.AcquisitionPolicy, 0, len(entry.profiles))
			hasConfiguredProfile := false
			var maxPollGrace time.Duration
			historicalRequest := scope.Temporal.Mode == "historical" || scope.Temporal.RevisionView != "latest"
			for _, p := range entry.profiles {
				policy, e := s.Ingestion.AcquisitionPolicy(ctx, entry.id, p.Key)
				if e != nil {
					return e
				}
				policies = append(policies, policy)
				if policy.Gate != "not_configured" {
					hasConfiguredProfile = true
					v.Capability = policy.Capability
					if policy.Capability.Family != "" {
						v.Family = policy.Capability.Family
					}
				} else if v.Capability.Adapter == "" && policy.Capability.Adapter != "" {
					v.Capability = policy.Capability
				}
				var relation string
				if scope.Area.Kind == "world" {
					relation = "configured_footprint"
				} else {
					relation, e = spatial(ctx, tx, scope, []publicingest.Bounds{policy.Admission.Policy.AOI})
					if e != nil {
						return e
					}
				}
				v.Profiles = append(v.Profiles, Profile{Key: p.Key, RevisionID: p.Revision, Gate: policy.Gate, SpatialRelation: relation, ExecutionGranted: policy.ExecutionGranted, Rights: policy.Admission.Rights})
				if policy.Gate == "ready" {
					footprints = append(footprints, policy.Admission.Policy.AOI)
					grace := max(
						time.Duration(policy.Admission.Policy.MinIntervalSeconds)*time.Second,
						time.Duration(policy.Admission.Policy.FreshForSeconds)*time.Second,
					)
					if grace > maxPollGrace {
						maxPollGrace = grace
					}
				}
				kind, reason, owner := profileAction(ctx, scope, policy, relation, historicalRequest)
				v.Actions = append(v.Actions, Action{Kind: kind, SourceID: entry.id, ProfileKey: p.Key, ProfileRevisionID: p.Revision, Owner: owner, Reason: reason})
			}
			v.Support = "admitted"
			if scope.Area.Kind == "world" {
				if len(footprints) > 0 {
					v.SpatialRelation = "configured_footprint"
				} else {
					v.SpatialRelation = "unknown"
				}
			} else {
				v.SpatialRelation, e = spatial(ctx, tx, scope, footprints)
				if e != nil {
					return e
				}
			}
			if v.Family == "reference" || v.Family == "terrain" {
				if e = evaluateReferenceSource(ctx, tx, scope, entry.id, entry.metadata, &v); e != nil {
					return e
				}
				out.Sources = append(out.Sources, v)
				continue
			}
			if !hasConfiguredProfile && (v.Family == "" || v.Family == "unknown") {
				v.Support = "not_configured"
				v.SupportReason = "No trusted server connector binding is configured for this admission."
				v.Retrieval = "not_configured"
				v.DataResult = "not_configured"
				out.Sources = append(out.Sources, v)
				continue
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
			completeReceiptAOIs := []publicingest.Bounds{}
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
				if len(scope.Area.Geometry) > 0 {
					relation, e := spatial(ctx, tx, scope, []publicingest.Bounds{*receipt.AOI})
					if e != nil {
						return e
					}
					if relation == "none" {
						continue
					}
				}
				relevant = append(relevant, receipt)
				if receipt.QueryComplete {
					completeReceiptAOIs = append(completeReceiptAOIs, *receipt.AOI)
				}
				if receipt.Status == "SUCCEEDED" && receipt.Pages > 0 && (v.LatestRetrieval == nil || receipt.CompletedAt.After(*v.LatestRetrieval)) {
					at := receipt.CompletedAt
					v.LatestRetrieval = &at
				}
			}
			v.Receipts = relevant
			if v.LatestRetrieval != nil {
				v.Freshness = "stale"
				for _, policy := range policies {
					if v.LatestRetrieval.After(out.EvaluatedAt.Add(-time.Duration(policy.Admission.Policy.FreshForSeconds) * time.Second)) {
						v.Freshness = "fresh"
					}
				}
			}
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
			// In live Monitoring mode, respect scheduled polling cadence:
			//  1. Current-polling adapters (METAR, GDACS, OpenSky) are complete when a
			//     fresh completed poll covers the selected area (or configured footprint on World).
			//  2. Historical-capable polling adapters are complete when the only trailing
			//     gap is within their normal polling/freshness grace window.
			if scope.Temporal.Mode == "monitoring" && v.Freshness == "fresh" && !failedOnly && len(completeReceiptAOIs) > 0 {
				if !v.Capability.Historical {
					coversArea := len(scope.Area.Geometry) == 0
					if !coversArea {
						rel, e := spatial(ctx, tx, scope, completeReceiptAOIs)
						if e != nil {
							return e
						}
						coversArea = rel == "full"
					}
					if coversArea {
						v.Retrieval = "complete_query_footprint"
						v.MissingWindows = []Window{}
					}
				} else if len(v.CompletedWindows) > 0 && len(v.MissingWindows) == 1 && maxPollGrace > 0 {
					tail := v.MissingWindows[0]
					if tail.To.Equal(scope.Temporal.Interval.To) && tail.To.Sub(tail.From) <= maxPollGrace {
						v.Retrieval = "complete_query_footprint"
						v.MissingWindows = []Window{}
					}
				}
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

func profileAction(ctx context.Context, scope analystscope.Snapshot, policy publicingest.AcquisitionPolicy, relation string, historicalRequest bool) (string, string, string) {
	switch {
	case policy.Gate == "not_configured":
		return "inspect_source", "No trusted server connector binding is configured for this profile.", "source administrator"
	case policy.Gate == "disabled_or_expired":
		return "inspect_source", "This source or admission is disabled or expired.", "source administrator"
	case policy.Gate != "ready":
		return "inspect_source", "Review this profile's provider prerequisites and rights.", "source administrator"
	case !policy.Capability.Historical && historicalRequest:
		return "choose_compatible_dates", "This adapter uses current polling and cannot replay a frozen acquisition window.", "analyst"
	case relation == "none":
		return "request_coverage_review", "The selected area is outside this approved profile.", "source administrator"
	case relation == "partial":
		if !policy.Capability.Historical {
			return "request_coverage_review", "This current-polling profile covers part of the selected area; narrow the area or request wider coverage.", "source administrator"
		}
		return "request_coverage_review", "The selected area exceeds this approved profile.", "source administrator"
	case scope.Area.Kind == "world":
		return "select_area", "Configured for its approved regional footprint. Select a bounded area to queue deliberate acquisition.", "analyst"
	case !policy.Capability.Historical:
		return "inspect_source", "Current-polling adapter refreshes on its approved schedule; on-demand historical replay is unavailable.", "connector operator"
	case policy.ExecutionGranted && relation == "full" && access.Allows(ctx, "acquisitions.request"):
		return "request_acquisition", "Queue a deliberate acquisition using this approved revision.", "analyst"
	default:
		return "inspect_source", "Profile revision requires an explicit execution grant before automated retrieval.", "source administrator"
	}
}

func extractCoverageBounds(metadata map[string]any) []publicingest.Bounds {
	rawCov, ok := metadata["coverage"]
	if !ok || rawCov == nil {
		return nil
	}
	encoded, err := json.Marshal(rawCov)
	if err != nil {
		return nil
	}
	var b publicingest.Bounds
	if err := json.Unmarshal(encoded, &b); err != nil || b.Validate() != nil {
		return nil
	}
	return []publicingest.Bounds{b}
}

func evaluateUnadmittedSource(ctx context.Context, ingestion *publicingest.Service, sourceID string, metadata map[string]any, v *Source) {
	capInfo, gateState, gateAction, hasBinding := ingestion.SourceBinding(ctx, sourceID)
	if hasBinding {
		v.Capability = capInfo
		if capInfo.Family != "" {
			v.Family = capInfo.Family
		}
	}
	activation, _ := metadata["activation"].(map[string]any)
	metaState, _ := activation["state"].(string)
	metaAction, _ := activation["nextAction"].(string)
	if gateState == "" {
		gateState = metaState
	}
	if gateAction == "" {
		gateAction = metaAction
	}
	v.Support = "not_admitted"
	v.Retrieval = "not_admitted"
	v.DataResult = "not_admitted"
	v.MissingWindows = []Window{}
	switch gateState {
	case "NEEDS_PERMISSION":
		v.SupportReason = "Provider access is not admitted yet (requires reviewed use/retention permission)."
	case "NEEDS_CREDENTIALS":
		v.SupportReason = "Provider access is not admitted yet (requires server credentials)."
	default:
		v.SupportReason = "Source has no admitted retrieval profile in this workspace."
	}
	reason := gateAction
	if reason == "" {
		reason = "Review provider prerequisites and create an admission profile to activate this source."
	}
	v.Actions = append(v.Actions, Action{
		Kind:     "inspect_source",
		SourceID: sourceID,
		Owner:    "source administrator",
		Reason:   reason,
	})
}

func evaluateReferenceSource(ctx context.Context, tx pgx.Tx, scope analystscope.Snapshot, sourceID string, metadata map[string]any, v *Source) error {
	rows, err := tx.Query(ctx, `SELECT dataset_key,kind,document FROM reference_datasets WHERE source_id=$1 AND emad_content_visible('reference',id) AND COALESCE((document#>>'{rights,expiresAt}')::timestamptz>clock_timestamp(),true) ORDER BY dataset_key,version,id LIMIT 256`, sourceID)
	if err != nil {
		return err
	}
	type refDoc struct {
		AOI          *publicingest.Bounds `json:"aoi"`
		RetrievedAt  *time.Time           `json:"retrievedAt"`
		FeatureCount int                  `json:"featureCount"`
	}
	datasetCount := 0
	worldFeatures := 0
	nonVectorBounds := []publicingest.Bounds{}
	refBounds := []publicingest.Bounds{}
	isTerrain := strings.Contains(strings.ToLower(sourceID), "dem") || strings.Contains(strings.ToLower(v.Name), "elevation") || strings.Contains(strings.ToLower(v.Name), "terrain")
	for rows.Next() {
		var key, kind string
		var raw []byte
		if err = rows.Scan(&key, &kind, &raw); err != nil {
			rows.Close()
			return err
		}
		datasetCount++
		var doc refDoc
		_ = json.Unmarshal(raw, &doc)
		lowerKey := strings.ToLower(key)
		lowerKind := strings.ToLower(kind)
		if lowerKind == "terrain" || lowerKind == "elevation" || lowerKind == "grid" || strings.Contains(lowerKey, "terrain") || strings.Contains(lowerKey, "dem") || strings.Contains(lowerKey, "elevation") {
			isTerrain = true
		}
		if doc.AOI != nil && doc.AOI.Validate() == nil {
			refBounds = append(refBounds, *doc.AOI)
			if lowerKind != "vector" {
				nonVectorBounds = append(nonVectorBounds, *doc.AOI)
			}
		}
		if lowerKind == "vector" {
			worldFeatures += doc.FeatureCount
		} else {
			worldFeatures++
		}
		if doc.RetrievedAt != nil && !doc.RetrievedAt.IsZero() && (v.LatestEvidenceAt == nil || doc.RetrievedAt.After(*v.LatestEvidenceAt)) {
			at := doc.RetrievedAt.UTC()
			v.LatestEvidenceAt = &at
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(refBounds) == 0 {
		refBounds = extractCoverageBounds(metadata)
	}

	var regionCount int
	if len(scope.Area.Geometry) == 0 {
		err = tx.QueryRow(ctx, `SELECT count(*) FROM geographic_regions WHERE source_id=$1`, sourceID).Scan(&regionCount)
	} else {
		err = tx.QueryRow(ctx, `SELECT count(*) FROM geographic_regions WHERE source_id=$1 AND ST_Intersects(geometry,ST_SetSRID(ST_GeomFromGeoJSON($2),4326))`, sourceID, scope.Area.Geometry).Scan(&regionCount)
	}
	if err != nil {
		return err
	}

	if v.Family == "" || v.Family == "unknown" {
		if isTerrain {
			v.Family = "terrain"
		} else {
			v.Family = "reference"
		}
	}
	if v.Capability.Adapter == "" {
		if v.Family == "terrain" {
			v.Capability = publicingest.Capability("dem-import-v1")
		} else {
			v.Capability = publicingest.Capability("reference-import-v1")
		}
	}

	if scope.Area.Kind == "world" {
		if len(refBounds) > 0 || datasetCount > 0 || regionCount > 0 {
			v.SpatialRelation = "configured_footprint"
		}
	} else if len(refBounds) > 0 {
		v.SpatialRelation, err = spatial(ctx, tx, scope, refBounds)
		if err != nil {
			return err
		}
	} else if regionCount > 0 {
		v.SpatialRelation = "partial"
	}

	retainedCount := 0
	if scope.Area.Kind == "world" {
		retainedCount = worldFeatures + regionCount
	} else {
		var featureCount int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM reference_features f JOIN reference_datasets d ON d.id=f.dataset_id AND d.scope_id=f.scope_id WHERE f.source_id=$1 AND emad_content_visible('reference',d.id) AND COALESCE((d.document#>>'{rights,expiresAt}')::timestamptz>clock_timestamp(),true) AND ST_Intersects(f.geometry,ST_SetSRID(ST_GeomFromGeoJSON($2),4326))`, sourceID, scope.Area.Geometry).Scan(&featureCount)
		if err != nil {
			return err
		}
		retainedCount = featureCount + regionCount
		for _, b := range nonVectorBounds {
			rel, relErr := spatial(ctx, tx, scope, []publicingest.Bounds{b})
			if relErr != nil {
				return relErr
			}
			if rel != "none" {
				retainedCount++
			}
		}
	}
	v.UsableRetainedRecords = &retainedCount
	v.MissingWindows = []Window{}

	support := queryscope.Capability(v.Family, scope.Temporal)
	if support.State == "unsupported" {
		v.Support = "unsupported"
		v.SupportReason = support.Reason
		v.DataResult = "unavailable_query"
		return nil
	}

	v.Support = "retained_reference"
	if datasetCount == 0 && regionCount == 0 {
		v.Retrieval = "no_retrieval"
		v.DataResult = "no_retrieval"
		v.Freshness = "unknown"
		v.SupportReason = "No retained geographic reference artifacts are installed for this source."
		v.Actions = append(v.Actions, Action{
			Kind:     "inspect_source",
			SourceID: sourceID,
			Owner:    "source administrator",
			Reason:   "Import reviewed reference artifacts for this source.",
		})
		return nil
	}
	v.Freshness = "retained_edition"
	if scope.Area.Kind != "world" && v.SpatialRelation == "none" && retainedCount == 0 {
		v.Retrieval = "outside_area"
		v.DataResult = "outside_retained_area"
		v.SupportReason = "Versioned geographic reference artifacts are installed for another footprint and do not intersect the selected area."
		v.Actions = append(v.Actions, Action{
			Kind:     "inspect_source",
			SourceID: sourceID,
			Owner:    "analyst",
			Reason:   "Select an area within the retained reference footprint or import reference depth for this area.",
		})
		return nil
	}
	v.Retrieval = "retained_reference"
	v.DataResult = "usable_retained_records"
	v.SupportReason = "Versioned geographic reference artifacts are installed; static edition, not a live polling feed."
	return nil
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
	raw, _ := json.Marshal(receipts)
	var rows pgx.Rows
	var e error
	if len(scope.Area.Geometry) == 0 {
		// World is a discovery scope: report time-slab coverage of completed receipts
		// over each source's own configured regional footprint.
		rows, e = tx.Query(ctx, `WITH eligible AS(SELECT GREATEST((x->>'from')::timestamptz,$2)f,LEAST((x->>'to')::timestamptz,$3)t FROM jsonb_array_elements($1::jsonb)x WHERE (x->>'queryComplete')::boolean AND x->'aoi'<>'null'::jsonb),edges AS(SELECT $2::timestamptz d UNION SELECT $3::timestamptz UNION SELECT f FROM eligible UNION SELECT t FROM eligible),slabs AS(SELECT d f,lead(d)OVER(ORDER BY d)t FROM edges) SELECT s.f,s.t,EXISTS(SELECT 1 FROM eligible e WHERE e.f<=s.f AND e.t>=s.t) FROM slabs s WHERE s.t>s.f ORDER BY s.f`, raw, scope.Temporal.Interval.From, scope.Temporal.Interval.To)
	} else {
		rows, e = tx.Query(ctx, `WITH eligible AS(SELECT GREATEST((x->>'from')::timestamptz,$3)f,LEAST((x->>'to')::timestamptz,$4)t,ST_MakeEnvelope((x#>>'{aoi,west}')::float8,(x#>>'{aoi,south}')::float8,(x#>>'{aoi,east}')::float8,(x#>>'{aoi,north}')::float8,4326)g FROM jsonb_array_elements($2::jsonb)x WHERE (x->>'queryComplete')::boolean AND x->'aoi'<>'null'::jsonb),edges AS(SELECT $3::timestamptz d UNION SELECT $4::timestamptz UNION SELECT f FROM eligible UNION SELECT t FROM eligible),slabs AS(SELECT d f,lead(d)OVER(ORDER BY d)t FROM edges) SELECT s.f,s.t,COALESCE(ST_Covers((SELECT ST_UnaryUnion(ST_Collect(g)) FROM eligible e WHERE e.f<=s.f AND e.t>=s.t),ST_SetSRID(ST_GeomFromGeoJSON($1),4326)),false) FROM slabs s WHERE s.t>s.f ORDER BY s.f`, scope.Area.Geometry, raw, scope.Temporal.Interval.From, scope.Temporal.Interval.To)
	}
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
	r.Get("/coverage/inventory", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Inventory(r.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
}
