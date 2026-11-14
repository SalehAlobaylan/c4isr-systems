package contextpackage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/externaltracks"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
)

const geographicSchema = "emad.geographic-context.v1"

var handoffKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
var handoffDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

type GeographicInput struct {
	RequestKey        string                   `json:"requestKey"`
	Geography         geoenv.ContextDependency `json:"geography"`
	ReadID            string                   `json:"readId"`
	EvidenceIDs       []string                 `json:"evidenceIds"`
	MovingTrackID     string                   `json:"movingTrackId,omitempty"`
	MovingEvidenceIDs []string                 `json:"movingEvidenceIds,omitempty"`
	MissingData       string                   `json:"missingData"`
}

type FrozenMoving struct {
	TrackID        string                       `json:"trackId"`
	AreaRevisionID string                       `json:"areaRevisionId"`
	Items          []externaltracks.HistoryItem `json:"items"`
	Gaps           []externaltracks.Gap         `json:"gaps"`
	Meaning        string                       `json:"meaning"`
}

type GeographicObservation struct {
	EvidenceID        string                           `json:"evidenceId"`
	SourceID          string                           `json:"sourceId"`
	AdmissionID       string                           `json:"admissionId"`
	ProviderRecordID  string                           `json:"providerRecordId"`
	ProviderRevision  string                           `json:"providerRevision"`
	ContentSHA256     string                           `json:"contentSha256"`
	RetrievedAt       time.Time                        `json:"retrievedAt"`
	AvailableAt       time.Time                        `json:"availableAt"`
	ProviderUpdatedAt *time.Time                       `json:"providerUpdatedAt,omitempty"`
	Reported          publicingest.ReportedObservation `json:"reported"`
	ProviderReport    *publicingest.ContextRecord      `json:"providerReport,omitempty"`
}

type GeographicHandoff struct {
	Schema             string                   `json:"schema"`
	ID                 string                   `json:"id"`
	IntentSHA256       string                   `json:"intentSha256"`
	Geography          geoenv.ContextDependency `json:"geography"`
	Scope              analystscope.Snapshot    `json:"scope"`
	ReadID             string                   `json:"readId"`
	QueryFingerprint   string                   `json:"queryFingerprint"`
	MissingData        string                   `json:"missingData"`
	SelectionMeaning   string                   `json:"selectionMeaning"`
	ObservedWindow     Window                   `json:"observedWindow"`
	AvailabilityCutoff *time.Time               `json:"availabilityCutoff,omitempty"`
	CreatedAt          time.Time                `json:"createdAt"`
	ExpiresAt          time.Time                `json:"expiresAt"`
	Observations       []GeographicObservation  `json:"observations"`
	Moving             *FrozenMoving            `json:"moving,omitempty"`
	Terms              []geoenv.ContextTerms    `json:"terms"`
}

func (s *Service) FreezeGeographic(ctx context.Context, in GeographicInput) (out GeographicHandoff, err error) {
	ctx, err = refreshExport(ctx)
	if err != nil {
		return out, err
	}
	if !handoffKey.MatchString(in.RequestKey) || !handoffKey.MatchString(in.Geography.ReleaseID) ||
		!handoffDigest.MatchString(in.Geography.ManifestSHA256) || !handoffKey.MatchString(in.ReadID) ||
		len(in.Geography.Displays) > 6 || len(in.EvidenceIDs)+len(in.MovingEvidenceIDs) > 200 || in.MissingData != "explicit_unavailable" ||
		(len(in.MovingEvidenceIDs) > 0 && !handoffKey.MatchString(in.MovingTrackID)) || (len(in.MovingEvidenceIDs) == 0 && in.MovingTrackID != "") {
		return out, apperr.Validation("provide a scoped read, exact dependencies and the explicit unavailable policy")
	}
	slices.Sort(in.EvidenceIDs)
	slices.Sort(in.MovingEvidenceIDs)
	for i, id := range in.MovingEvidenceIDs {
		if !handoffKey.MatchString(id) || i > 0 && id == in.MovingEvidenceIDs[i-1] {
			return out, apperr.Validation("moving revisions must be unique exact IDs")
		}
	}
	slices.SortFunc(in.Geography.Displays, func(a, b geoenv.ContextDisplay) int { return strings.Compare(a.ID, b.ID) })
	for i, id := range in.EvidenceIDs {
		if !handoffKey.MatchString(id) || i > 0 && id == in.EvidenceIDs[i-1] {
			return out, apperr.Validation("observation revisions must be unique exact IDs")
		}
	}
	for i, ref := range in.Geography.Displays {
		if !handoffKey.MatchString(ref.ID) || !handoffDigest.MatchString(ref.SHA256) || i > 0 && ref.ID == in.Geography.Displays[i-1].ID {
			return out, apperr.Validation("display dependencies must have unique IDs and exact digests")
		}
	}
	if in.EvidenceIDs == nil {
		in.EvidenceIDs = []string{}
	}
	if in.Geography.Displays == nil {
		in.Geography.Displays = []geoenv.ContextDisplay{}
	}
	intent, _ := json.Marshal(in)
	principal, resource, _ := access.From(ctx)
	id := "geoctx_" + artifacts.Hash([]byte(resource.ID()+"\x00"+principal.ID+"\x00"+in.RequestKey))
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/context-packages"); err != nil {
			return err
		}
		out, err = readGeographic(ctx, tx, id)
		if err == nil {
			if out.IntentSHA256 != artifacts.Hash(intent) {
				return apperr.Conflict("handoff request key already identifies another intent")
			}
			return s.recheckGeographic(ctx, tx, out)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := access.Require(ctx, "observations.read"); err != nil {
			return err
		}
		var raw []byte
		var revision string
		err = tx.QueryRow(ctx, `SELECT document,authorization_revision FROM analyst_read_snapshots
			WHERE id=$1 AND snapshot_kind='map' AND expires_at>clock_timestamp()`, in.ReadID).Scan(&raw, &revision)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && revision != access.AccessRevision(ctx) {
			return apperr.Conflict("scoped observation read expired or authorization changed; run the selection again")
		}
		if err != nil {
			return err
		}
		var read analystscope.ReadSnapshot
		if err := json.Unmarshal(raw, &read); err != nil {
			return err
		}
		temporal := read.Scope.Temporal
		if temporal.RevisionView != "latest" && temporal.RevisionView != "known_at" || temporal.LegacyEndInclusive ||
			temporal.RevisionView == "known_at" && temporal.AsOf == nil {
			return apperr.Validation("choose occurrence time or known by cutoff with a start-inclusive, end-exclusive window")
		}
		if read.Events == nil || read.Events.Truncated || read.EventState != "available" {
			return apperr.Conflict("the scoped observation membership is unavailable or incomplete")
		}
		g := geoenv.Service{Pool: s.pool}
		manifest, expiry, terms, err := g.ContextDependencies(ctx, tx, in.Geography)
		if err != nil {
			return err
		}
		if manifest.AreaRevisionID != read.Scope.Area.RevisionID {
			return apperr.Conflict("the observation read must use the exact geographic release area revision")
		}
		out = GeographicHandoff{Schema: geographicSchema, ID: id, IntentSHA256: artifacts.Hash(intent), Geography: in.Geography,
			Scope: read.Scope, ReadID: in.ReadID, QueryFingerprint: read.QueryFingerprint, MissingData: in.MissingData,
			SelectionMeaning: "Explicit selected reported revisions. Gaps are unknown. Occurrence is within [from,to); known-at also requires platform publication at or before the cutoff.",
			ObservedWindow:   Window{From: temporal.Interval.From, To: temporal.Interval.To}, AvailabilityCutoff: temporal.AsOf,
			CreatedAt: time.Now().UTC(), ExpiresAt: expiry, Observations: []GeographicObservation{}, Terms: terms}
		selected := make(map[string]publicevents.Event, len(in.EvidenceIDs))
		selectedReports := map[string]time.Time{}
		if reports, ok := read.Layers["bulletins"]; ok && !reports.Truncated && reports.State != "unavailable" && reports.State != "historical_unsupported" {
			for _, item := range reports.Features {
				if !slices.Contains(in.EvidenceIDs, item.ID) || item.Properties["reportHistory"] == nil {
					continue
				}
				at, err := time.Parse(time.RFC3339Nano, fmt.Sprint(item.Properties["date"]))
				if err != nil {
					return err
				}
				selectedReports[item.ID] = at
			}
		}
		for _, item := range read.Events.Items {
			var event publicevents.Event
			if err := json.Unmarshal(item, &event); err != nil {
				return err
			}
			if _, found := slices.BinarySearch(in.EvidenceIDs, event.EvidenceID); found {
				selected[event.EvidenceID] = event
			}
		}
		for _, key := range in.EvidenceIDs {
			event, found := selected[key]
			reportAt, isReport := selectedReports[key]
			if !isReport && (!found || event.KnownAt == nil || event.Reported.ObservedAt.Before(out.ObservedWindow.From) ||
				!event.Reported.ObservedAt.Before(out.ObservedWindow.To) || out.AvailabilityCutoff != nil && event.KnownAt.After(*out.AvailabilityCutoff)) {
				return apperr.Conflict("selected revision is outside the pinned area, occurrence window or availability cutoff")
			}
			observation, term, err := s.geographicObservation(ctx, tx, key)
			if err != nil {
				return err
			}
			if isReport {
				if observation.ProviderReport == nil || !observation.Reported.ObservedAt.Equal(reportAt) || reportAt.Before(out.ObservedWindow.From) ||
					!reportAt.Before(out.ObservedWindow.To) || out.AvailabilityCutoff != nil && observation.AvailableAt.After(*out.AvailabilityCutoff) {
					return apperr.Conflict("selected report dependency changed")
				}
			} else if !observation.AvailableAt.Equal(*event.KnownAt) || observation.ProviderRevision != event.ProviderRevision ||
				!observation.Reported.ObservedAt.Equal(event.Reported.ObservedAt) || observation.SourceID != event.SourceID {
				return apperr.Conflict("selected observation dependency changed")
			}
			if term.ExpiresAt != nil && term.ExpiresAt.Before(out.ExpiresAt) {
				out.ExpiresAt = *term.ExpiresAt
			}
			out.Observations = append(out.Observations, observation)
			if !slices.ContainsFunc(out.Terms, func(t geoenv.ContextTerms) bool {
				return t.SourceID == term.SourceID && t.AdmissionID == term.AdmissionID
			}) {
				out.Terms = append(out.Terms, term)
			}
		}
		if in.MovingTrackID != "" {
			h, err := externaltracks.ReadHistoryIn(ctx, tx, in.MovingTrackID, manifest.AreaRevisionID, out.ObservedWindow.From, out.ObservedWindow.To, out.AvailabilityCutoff)
			if err != nil {
				return err
			}
			if h.Truncated {
				return apperr.Conflict("narrow the incomplete moving history before freezing")
			}
			out.Moving = &FrozenMoving{TrackID: h.TrackID, AreaRevisionID: h.AreaRevisionID, Meaning: h.Meaning, Items: []externaltracks.HistoryItem{}}
			for _, item := range h.Items {
				if !slices.Contains(in.MovingEvidenceIDs, item.EvidenceID) {
					continue
				}
				term, err := movingTerms(ctx, tx, item)
				if err != nil {
					return err
				}
				if term.ExpiresAt != nil && term.ExpiresAt.Before(out.ExpiresAt) {
					out.ExpiresAt = *term.ExpiresAt
				}
				if !slices.ContainsFunc(out.Terms, func(t geoenv.ContextTerms) bool {
					return t.SourceID == term.SourceID && t.AdmissionID == term.AdmissionID
				}) {
					out.Terms = append(out.Terms, term)
				}
				out.Moving.Items = append(out.Moving.Items, item)
			}
			if len(out.Moving.Items) != len(in.MovingEvidenceIDs) {
				return apperr.Conflict("selected moving revision is outside the pinned track, area, occurrence window or cutoff")
			}
			out.Moving.Gaps = externaltracks.SelectedGaps(out.Moving.Items, h.From, h.To, out.Moving.Items[0].Reported.Kind, false)
			out.Moving.Meaning += " Only selected revisions are included. Gaps describe the selection, which can omit retained reports."
		}
		if !out.ExpiresAt.After(out.CreatedAt) {
			return apperr.Conflict("handoff dependencies have expired")
		}
		slices.SortFunc(out.Observations, func(a, b GeographicObservation) int {
			if c := a.Reported.ObservedAt.Compare(b.Reported.ObservedAt); c != 0 {
				return c
			}
			return strings.Compare(a.EvidenceID, b.EvidenceID)
		})
		encoded, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if len(encoded) > 4<<20 {
			return apperr.Conflict("handoff exceeds the 4 MiB manifest budget")
		}
		var count, bytes int64
		if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(octet_length(bundle)),0) FROM context_packages`).Scan(&count, &bytes); err != nil {
			return err
		}
		if count >= 64 || bytes+int64(len(encoded)) > 64<<20 {
			return apperr.Conflict("context package budget exceeded")
		}
		sources := make([]map[string]string, len(out.Terms))
		for i, term := range out.Terms {
			sources[i] = map[string]string{"sourceId": term.SourceID}
		}
		doc, _ := json.Marshal(map[string]any{"schema": geographicSchema, "scope": out.Scope, "assets": sources})
		_, err = tx.Exec(ctx, `INSERT INTO context_packages(id,document,bundle) VALUES($1,$2,$3)`, id, doc, encoded)
		return err
	})
	return out, err
}

func readGeographic(ctx context.Context, tx pgx.Tx, id string) (GeographicHandoff, error) {
	var out GeographicHandoff
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT bundle FROM context_packages WHERE id=$1 AND document->>'schema'=$2`, id, geographicSchema).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, err
}

func (s *Service) LoadGeographic(ctx context.Context, id string) (out GeographicHandoff, err error) {
	if err = require(ctx); err != nil {
		return out, err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		out, err = readGeographic(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("geographic context handoff", "unavailable")
		}
		if err != nil {
			return err
		}
		return s.recheckGeographic(ctx, tx, out)
	})
	return out, err
}

func (s *Service) recheckGeographic(ctx context.Context, tx pgx.Tx, out GeographicHandoff) error {
	if !out.ExpiresAt.After(time.Now()) {
		return apperr.Conflict("handoff dependencies have expired")
	}
	if err := access.Require(ctx, "observations.read"); err != nil {
		return err
	}
	if _, _, _, err := (geoenv.Service{Pool: s.pool}).ContextDependencies(ctx, tx, out.Geography); err != nil {
		return err
	}
	for _, pinned := range out.Observations {
		current, _, err := s.geographicObservation(ctx, tx, pinned.EvidenceID)
		if err != nil || current.ContentSHA256 != pinned.ContentSHA256 || current.ProviderRevision != pinned.ProviderRevision ||
			!current.AvailableAt.Equal(pinned.AvailableAt) {
			return apperr.Conflict("pinned observation dependency is unavailable under current policy")
		}
	}
	if out.Moving != nil {
		if err := access.Require(ctx, "tracks.read"); err != nil {
			return err
		}
		for _, item := range out.Moving.Items {
			if _, err := movingTerms(ctx, tx, item); err != nil {
				return err
			}
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM external_track_associations x JOIN public_evidence e ON (e.id,e.scope_id)=(x.evidence_id,x.scope_id) JOIN public_evidence_catalog c ON (c.evidence_id,c.scope_id)=(x.evidence_id,x.scope_id) WHERE x.evidence_id=$1 AND x.track_id=$2 AND e.document->>'contentHash'=$3 AND c.published_at=$4)`, item.EvidenceID, out.Moving.TrackID, item.ContentSHA256, item.AvailableAt).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return apperr.Conflict("pinned moving revision is unavailable")
			}
		}
	}
	return nil
}

func movingTerms(ctx context.Context, tx pgx.Tx, item externaltracks.HistoryItem) (geoenv.ContextTerms, error) {
	p, err := publicingest.LockPolicy(ctx, tx, item.SourceID, item.AdmissionID)
	if err != nil {
		return geoenv.ContextTerms{}, err
	}
	r := p.Rights
	if !handoffDigest.MatchString(item.ContentSHA256) || !r.AllowAnalysis || r.OutputPermissions == nil || r.OutputPermissions.GeographicContextExport == nil || !*r.OutputPermissions.GeographicContextExport {
		return geoenv.ContextTerms{}, apperr.Forbidden("moving source lacks geographic context export permission")
	}
	return geoenv.ContextTerms{SourceID: item.SourceID, AdmissionID: item.AdmissionID, CurrentAdmissionID: p.CurrentAdmissionID, RightsID: r.ID, Attribution: r.Attribution, TermsURL: r.TermsURL, LegalBasis: r.LegalBasis, ExpiresAt: r.ExpiresAt}, nil
}

func (s *Service) geographicObservation(ctx context.Context, tx pgx.Tx, id string) (GeographicObservation, geoenv.ContextTerms, error) {
	var out GeographicObservation
	var terms geoenv.ContextTerms
	var document []byte
	var withdrawn bool
	err := tx.QueryRow(ctx, `SELECT e.document, c.published_at,
		COALESCE((head.document#>>'{reported,withdrawn}')::boolean,false)
		FROM public_event_revisions r JOIN public_evidence e ON (e.id,e.scope_id)=(r.evidence_id,r.scope_id)
		JOIN public_evidence_catalog c ON (c.evidence_id,c.scope_id)=(r.evidence_id,r.scope_id)
		LEFT JOIN public_event_current h ON (h.source_id,h.scope_id,h.provider_record_id)=(r.source_id,r.scope_id,r.provider_record_id)
		LEFT JOIN public_event_revisions head ON (head.evidence_id,head.scope_id)=(h.evidence_id,h.scope_id)
		WHERE r.evidence_id=$1`, id).Scan(&document, &out.AvailableAt, &withdrawn)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT e.document,c.published_at FROM public_context_records r
			JOIN public_evidence e ON (e.id,e.scope_id)=(r.evidence_id,r.scope_id)
			JOIN public_evidence_catalog c ON (c.evidence_id,c.scope_id)=(r.evidence_id,r.scope_id)
			WHERE r.evidence_id=$1 AND r.kind='REPORT' AND e.document#>>'{context,extractionMethod}'='nasa-eonet-floods-v1'`, id).Scan(&document, &out.AvailableAt)
	}
	if errors.Is(err, pgx.ErrNoRows) || withdrawn {
		return out, terms, apperr.Conflict("selected observation is unavailable or withdrawn")
	}
	if err != nil {
		return out, terms, err
	}
	var evidence publicingest.Evidence
	if err := json.Unmarshal(document, &evidence); err != nil {
		return out, terms, err
	}
	isReport := evidence.Context != nil && evidence.Context.Report != nil && evidence.Context.Report.EventHistory != nil &&
		evidence.Context.ExtractionMethod == publicingest.EONETAdapter
	if !isReport && (evidence.Observation == nil || evidence.Observation.Withdrawn) || !handoffDigest.MatchString(evidence.ContentHash) {
		return out, terms, apperr.Conflict("selected observation is unavailable or withdrawn")
	}
	policy, err := publicingest.LockPolicy(ctx, tx, evidence.SourceID, evidence.AdmissionID)
	if err != nil {
		return out, terms, err
	}
	r := policy.Rights
	if !r.AllowAnalysis || r.OutputPermissions == nil || r.OutputPermissions.GeographicContextExport == nil || !*r.OutputPermissions.GeographicContextExport {
		return out, terms, apperr.Forbidden("observation source lacks context handoff permission")
	}
	out.EvidenceID, out.SourceID, out.AdmissionID = evidence.ID, evidence.SourceID, evidence.AdmissionID
	out.ProviderRecordID, out.ProviderRevision, out.ContentSHA256 = evidence.ProviderRecordID, evidence.ProviderRevision, evidence.ContentHash
	out.RetrievedAt, out.ProviderUpdatedAt = evidence.RetrievedAt, evidence.ProviderUpdatedAt
	if isReport {
		out.ProviderReport = evidence.Context
		out.Reported = publicingest.ReportedObservation{SemanticKind: "point_event", Type: "event.provider-flood-report", Title: evidence.Context.Title,
			ObservedAt: evidence.Context.From, Position: &evidence.Context.Position, GeometryRole: "reported_location",
			Quality: map[string]any{"meaning": evidence.Context.LocationBasis}}
	} else {
		out.Reported = *evidence.Observation
	}
	terms = geoenv.ContextTerms{SourceID: out.SourceID, AdmissionID: out.AdmissionID, CurrentAdmissionID: policy.CurrentAdmissionID,
		RightsID: r.ID, Attribution: r.Attribution, TermsURL: r.TermsURL, LegalBasis: r.LegalBasis, ExpiresAt: r.ExpiresAt}
	return out, terms, nil
}
