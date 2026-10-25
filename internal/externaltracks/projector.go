// Package externaltracks projects admitted moving claims into non-controllable
// public track hypotheses. Provider identity claims are never asset identities.
package externaltracks

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"time"
)

const Method = "source-bound-motion-v1"
const SegmentGap = 30 * time.Minute

type Projector struct{ Next publicingest.Projector }
type Association struct {
	EvidenceID    string                         `json:"evidenceId"`
	ObservationID string                         `json:"observationId"`
	TrackID       string                         `json:"trackId"`
	SourceID      string                         `json:"sourceId"`
	Decision      string                         `json:"decision"`
	Rationale     string                         `json:"rationale"`
	Method        string                         `json:"method"`
	ObservedAt    time.Time                      `json:"observedAt"`
	KnownAt       time.Time                      `json:"knownAt"`
	Reported      publicingest.MovingObservation `json:"reported"`
}

func (p Projector) Project(ctx context.Context, tx pgx.Tx, e publicingest.Evidence) error {
	if p.Next != nil {
		if err := p.Next.Project(ctx, tx, e); err != nil {
			return err
		}
	}
	if e.Moving == nil {
		return nil
	}
	if e.NormalizerVersion != publicingest.OpenSkyAdapter && e.NormalizerVersion != publicingest.AISAdapter {
		return apperr.Validation("moving projection requires a supported native normalizer")
	}
	o := *e.Moving
	if err := o.Validate(e.RetrievedAt); err != nil {
		return err
	}
	_, r, _ := access.From(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":"+e.SourceID+":"+o.Kind+":"+o.IdentityClaim); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM external_track_associations WHERE evidence_id=$1)`, e.ID).Scan(&exists); err != nil || exists {
		return err
	}
	var trackID string
	var first, last time.Time
	err := tx.QueryRow(ctx, `SELECT track_id,first_seen_at,last_seen_at FROM external_track_segments WHERE source_id=$1 AND kind=$2 AND identity_claim=$3 AND first_seen_at<=$4 ORDER BY first_seen_at DESC,track_id LIMIT 1 FOR UPDATE`, e.SourceID, o.Kind, o.IdentityClaim, o.ObservedAt).Scan(&trackID, &first, &last)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	decision, reason := "APPLIED", "Source-bound identity claim within the declared temporal/motion gate; identity remains unverified."
	newSegment := errors.Is(err, pgx.ErrNoRows) || o.ObservedAt.Sub(last) > SegmentGap
	if newSegment {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM external_track_segments WHERE source_id=$1`, e.SourceID).Scan(&count); err != nil {
			return err
		}
		if count >= 2000 {
			return apperr.Conflict("source external-track segment budget exhausted")
		}
		if trackID != "" {
			if _, err := tx.Exec(ctx, `UPDATE tracks SET status='closed',closed_at=now(),updated_at=now() WHERE id=$1`, trackID); err != nil {
				return err
			}
			reason = "New segment after a gap greater than 30 minutes; no assumed continuity across missing coverage."
		} else {
			reason = "New source-bound identity-claim hypothesis; no cross-provider merge or verified identity."
		}
		trackID = ids.New("ext")
		first = o.ObservedAt
		last = o.ObservedAt
		metadata, _ := json.Marshal(map[string]any{"externalSourceId": e.SourceID, "kind": o.Kind, "identityClaim": o.IdentityClaim, "authority": "public_observed_hypothesis", "controllable": false, "associationMethod": Method, "originalProducer": e.OriginalProducer, "upstreamProvider": e.UpstreamProvider})
		if _, err := tx.Exec(ctx, `INSERT INTO tracks(id,external_ref,status,first_seen_at,last_seen_at,metadata,external_source_id)VALUES($1,$2,'active',$3,$3,$4,$5)`, trackID, "public-moving/"+trackID, o.ObservedAt, metadata, e.SourceID); err != nil {
			return err
		}
		raw, _ := json.Marshal(o)
		if _, err := tx.Exec(ctx, `INSERT INTO external_track_segments(track_id,source_id,kind,identity_claim,first_seen_at,last_seen_at,latest_evidence_id,document)VALUES($1,$2,$3,$4,$5,$5,$6,$7)`, trackID, e.SourceID, o.Kind, o.IdentityClaim, o.ObservedAt, e.ID, raw); err != nil {
			return err
		}
	} else if !o.ObservedAt.After(last) {
		decision = "LATE"
		reason = "Late or same-time report retained without rewinding the current state. Repeated source lineage is not independent corroboration."
	} else {
		// Geography and the motion gate are authoritative PostGIS operations.
		limit, slack := 1500.0, 15000.0
		if o.Kind == "vessel" {
			limit = 100
			slack = 2000
		}
		var plausible bool
		if err := tx.QueryRow(ctx, `SELECT COALESCE(ST_Distance(position,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography)<=$4,true) FROM track_state WHERE track_id=$1`, trackID, o.Position.Lng, o.Position.Lat, o.ObservedAt.Sub(last).Seconds()*limit+slack).Scan(&plausible); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !plausible {
			decision = "QUARANTINED"
			reason = "Position jump exceeds the declared source-bound motion gate; candidate association is recorded but state is not updated."
		}
	}
	observationID := "moving_obs_" + e.ID
	payload, _ := json.Marshal(map[string]any{"evidenceId": e.ID, "identityClaim": o.IdentityClaim, "claims": o.Claims, "associationMethod": Method})
	quality, _ := json.Marshal(o.Quality)
	if _, err := tx.Exec(ctx, `INSERT INTO observations(id,source_id,observation_type,observed_at,received_at,processed_at,position,payload,quality,semantic_kind,geometry_role)VALUES($1,$2,$3,$4,$5,now(),ST_SetSRID(ST_MakePoint($6,$7),4326)::geography,$8,$9,'moving','reported_location') ON CONFLICT(id)DO NOTHING`, observationID, e.SourceID, "external."+o.Kind+".position", o.ObservedAt, e.RetrievedAt, o.Position.Lng, o.Position.Lat, payload, quality); err != nil {
		return err
	}
	a := Association{e.ID, observationID, trackID, e.SourceID, decision, reason, Method, o.ObservedAt, e.RetrievedAt, o}
	raw, _ := json.Marshal(a)
	if _, err := tx.Exec(ctx, `INSERT INTO external_track_associations(evidence_id,source_id,track_id,observation_id,observed_at,document)VALUES($1,$2,$3,$4,$5,$6)`, e.ID, e.SourceID, trackID, observationID, o.ObservedAt, raw); err != nil {
		return err
	}
	if decision != "QUARANTINED" {
		if _, err := tx.Exec(ctx, `INSERT INTO track_observations(track_id,observation_id,source_id,observed_at,position)VALUES($1,$2,$3,$4,ST_SetSRID(ST_MakePoint($5,$6),4326)::geography)ON CONFLICT DO NOTHING`, trackID, observationID, e.SourceID, o.ObservedAt, o.Position.Lng, o.Position.Lat); err != nil {
			return err
		}
	}
	if decision == "APPLIED" {
		doc, _ := json.Marshal(o)
		if _, err := tx.Exec(ctx, `UPDATE external_track_segments SET last_seen_at=$2,latest_evidence_id=$3,document=$4 WHERE track_id=$1`, trackID, o.ObservedAt, e.ID, doc); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tracks SET last_seen_at=$2,status='active',closed_at=NULL,updated_at=now() WHERE id=$1`, trackID, o.ObservedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO track_state(track_id,position,speed,heading)VALUES($1,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography,$4,$5)ON CONFLICT(track_id)DO UPDATE SET position=excluded.position,speed=excluded.speed,heading=excluded.heading,updated_at=now()`, trackID, o.Position.Lng, o.Position.Lat, o.SpeedMPS, o.Course); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO track_history(id,track_id,observed_at,position,speed,heading)VALUES($1,$2,$3,ST_SetSRID(ST_MakePoint($4,$5),4326)::geography,$6,$7)`, "ext_history_"+e.ID, trackID, o.ObservedAt, o.Position.Lng, o.Position.Lat, o.SpeedMPS, o.Course)
		return err
	}
	return nil
}
func (p Projector) Maintain(ctx context.Context, tx pgx.Tx, source string, now time.Time) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT a.document FROM source_connector_state c JOIN source_admissions a ON a.id=c.admission_id WHERE c.source_id=$1`, source).Scan(&raw); err != nil {
		return err
	}
	var admission publicingest.Admission
	if err := json.Unmarshal(raw, &admission); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE tracks SET status=CASE WHEN last_seen_at<$3 THEN 'closed' ELSE 'lost' END,closed_at=CASE WHEN last_seen_at<$3 THEN COALESCE(closed_at,$4) ELSE closed_at END,updated_at=$4 WHERE external_source_id=$1 AND last_seen_at<$2 AND status<>'closed'`, source, now.Add(-time.Duration(admission.Policy.UnavailableAfterSeconds)*time.Second), now.Add(-SegmentGap), now)
	return err
}
