package publicevents

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/eventrules"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	DisplayPosition  json.RawMessage                  `json:"displayPosition,omitempty"`
	CurrentValidity  *Validity                        `json:"currentValidity,omitempty"`
	KnownAt          *time.Time                       `json:"knownAt,omitempty"`
	EffectiveAt      *time.Time                       `json:"effectiveAt,omitempty"`
	EffectiveBasis   string                           `json:"effectiveBasis,omitempty"`
	QualityClass     string                           `json:"qualityClass,omitempty"`
	EvidenceID       string                           `json:"evidenceId"`
	SourceID         string                           `json:"sourceId"`
	ProviderRecordID string                           `json:"providerRecordId"`
	ProviderRevision string                           `json:"providerRevision"`
	AdmissionID      string                           `json:"admissionId"`
	ObservationID    string                           `json:"observationId,omitempty"`
	SupersedesID     string                           `json:"supersedesId,omitempty"`
	RevisionAt       time.Time                        `json:"revisionAt"`
	ReceivedAt       time.Time                        `json:"receivedAt"`
	Reported         publicingest.ReportedObservation `json:"reported"`
}
type Projector struct{}

// Project is called only by the admitted publication boundary, inside its
// transaction. Evidence, observations, current projection and spatial-alert
// corrections therefore remain recoverable together.
func (Projector) Project(ctx context.Context, tx pgx.Tx, e publicingest.Evidence) error {
	if e.Observation == nil {
		return nil
	}
	o := *e.Observation
	revision := e.RetrievedAt
	if e.ProviderUpdatedAt != nil {
		revision = *e.ProviderUpdatedAt
	}
	ev := Event{EvidenceID: e.ID, SourceID: e.SourceID, ProviderRecordID: e.ProviderRecordID, ProviderRevision: e.ProviderRevision, AdmissionID: e.AdmissionID, RevisionAt: revision, ReceivedAt: e.RetrievedAt, Reported: o}
	var previous Event
	var doc []byte
	err := tx.QueryRow(ctx, `SELECT r.document FROM public_event_current c JOIN public_event_revisions r ON r.evidence_id=c.evidence_id WHERE c.source_id=$1 AND c.provider_record_id=$2 FOR UPDATE OF c`, e.SourceID, e.ProviderRecordID).Scan(&doc)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		if err = json.Unmarshal(doc, &previous); err != nil {
			return err
		}
	}
	current := previous.EvidenceID == "" || revision.After(previous.RevisionAt) || (revision.Equal(previous.RevisionAt) && e.ID > previous.EvidenceID)
	if current {
		ev.SupersedesID = previous.EvidenceID
	}
	if !o.Withdrawn {
		ev.ObservationID = "event_obs_" + e.ID
		var area any
		var areaKind any
		var lng, lat any
		role := o.GeometryRole
		if role == "" {
			role = "reported_location"
			if o.Area != nil {
				role = "reported_extent"
			}
		}
		if o.Position != nil {
			lng = o.Position.Lng
			lat = o.Position.Lat
		}
		if o.Area != nil {
			a := observations.Area{Kind: o.Area.Kind, Polygon: o.Area.Polygon}
			area = a.GeoJSON()
			areaKind = string(a.Kind)
		}
		payload, _ := json.Marshal(map[string]any{"evidenceId": e.ID, "providerRecordId": e.ProviderRecordID, "providerRevision": e.ProviderRevision, "properties": o.Properties, "reportedTitle": o.Title})
		quality, _ := json.Marshal(o.Quality)
		_, err = tx.Exec(ctx, `INSERT INTO observations(id,source_id,observation_type,observed_at,received_at,processed_at,position,area,area_kind,payload,quality,semantic_kind,geometry_role)
 VALUES($1,$2,$3,$4,$5,now(),CASE WHEN $6::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6,$7),4326)::geography END,CASE WHEN $8::text IS NULL THEN NULL ELSE ST_SetSRID(ST_GeomFromGeoJSON($8),4326)::geography END,$9,$10,$11,$12,$13) ON CONFLICT(id) DO NOTHING`, ev.ObservationID, e.SourceID, o.Type, o.ObservedAt, e.RetrievedAt, lng, lat, area, areaKind, payload, quality, o.SemanticKind, role)
		if err != nil {
			return err
		}
	}
	encoded, _ := json.Marshal(ev)
	var supersedes any
	var obsID any
	if ev.SupersedesID != "" {
		supersedes = ev.SupersedesID
	}
	if ev.ObservationID != "" {
		obsID = ev.ObservationID
	}
	_, err = tx.Exec(ctx, `INSERT INTO public_event_revisions(evidence_id,source_id,provider_record_id,revision_at,observation_id,supersedes_id,document) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(evidence_id) DO NOTHING`, e.ID, e.SourceID, e.ProviderRecordID, revision, obsID, supersedes, encoded)
	if err != nil || !current {
		return err
	}
	if previous.EvidenceID != "" {
		_, err = tx.Exec(ctx, `UPDATE alerts SET state='RESOLVED',resolved_at=now(),resolved_by='provider-revision',updated_at=now(),source_reference=source_reference||jsonb_build_object('supersededByEvidenceId',$2::text,'correctionReason',$3::text) WHERE type='public.event.spatial' AND source_reference->>'evidenceId'=$1 AND state<>'RESOLVED'`, previous.EvidenceID, e.ID, map[bool]string{true: "withdrawn", false: "corrected"}[o.Withdrawn])
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public_event_current(source_id,provider_record_id,evidence_id) VALUES($1,$2,$3) ON CONFLICT(scope_id,source_id,provider_record_id) DO UPDATE SET evidence_id=excluded.evidence_id`, e.SourceID, e.ProviderRecordID, e.ID)
	if err != nil {
		return err
	}
	if !o.Withdrawn {
		return eventrules.Evaluate(ctx, tx, ev.ObservationID, e.ID)
	}
	return nil
}
