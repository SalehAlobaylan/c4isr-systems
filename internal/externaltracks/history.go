package externaltracks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
)

const historyLimit = 200

type HistoryItem struct {
	Association
	AdmissionID      string    `json:"admissionId"`
	ProviderRecordID string    `json:"providerRecordId"`
	ProviderRevision string    `json:"providerRevision"`
	ContentSHA256    string    `json:"contentSha256"`
	AvailableAt      time.Time `json:"availableAt"`
	RetrievedAt      time.Time `json:"retrievedAt"`
}

type Gap struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Reason string    `json:"reason"`
}

type History struct {
	TrackID        string        `json:"trackId"`
	AreaRevisionID string        `json:"areaRevisionId"`
	From           time.Time     `json:"from"`
	To             time.Time     `json:"to"`
	Cutoff         *time.Time    `json:"cutoff,omitempty"`
	Items          []HistoryItem `json:"items"`
	Gaps           []Gap         `json:"gaps"`
	Truncated      bool          `json:"truncated"`
	Meaning        string        `json:"meaning"`
}

func (s Service) ReadHistory(ctx context.Context, id, area string, from, to time.Time, cutoff *time.Time) (out History, err error) {
	err = platformdb.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
		out, err = ReadHistoryIn(ctx, tx, id, area, from, to, cutoff)
		return err
	})
	return
}

// ReadHistoryIn is also used by freeze to re-establish exact membership inside
// its transaction. It never reads current-state positions or synthesizes reports.
func ReadHistoryIn(ctx context.Context, tx pgx.Tx, id, area string, from, to time.Time, cutoff *time.Time) (History, error) {
	out := History{TrackID: id, AreaRevisionID: area, From: from.UTC(), To: to.UTC(), Cutoff: cutoff, Items: []HistoryItem{}, Gaps: []Gap{},
		Meaning: "Retained provider reports only; no verified identity, interpolation, or 3D altitude. Unreported time and area are unknown."}
	if err := require(ctx); err != nil {
		return out, err
	}
	if err := access.Require(ctx, "observations.read"); err != nil {
		return out, err
	}
	if id == "" || len(id) > 128 || area == "" || len(area) > 128 || from.IsZero() || !to.After(from) || to.Sub(from) > 6*time.Hour || to.After(time.Now().Add(5*time.Second)) || cutoff != nil && cutoff.After(time.Now().Add(5*time.Second)) {
		return out, apperr.Validation("select an exact area and an elapsed history window of at most six hours")
	}
	ctx = platformdb.UseTx(ctx, tx)
	var kind string
	err := tx.QueryRow(ctx, `SELECT s.kind FROM external_track_segments s CROSS JOIN area_revisions a WHERE s.track_id=$1 AND a.id=$2 AND ST_Dimension(a.geometry)=2`, id, area).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, apperr.NotFound("external track or area", "unavailable")
	}
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT x.document,e.document,c.published_at
		FROM external_track_associations x
		JOIN public_evidence e ON (e.id,e.scope_id)=(x.evidence_id,x.scope_id)
		JOIN public_evidence_catalog c ON (c.evidence_id,c.scope_id)=(x.evidence_id,x.scope_id)
		CROSS JOIN area_revisions a
		WHERE x.track_id=$1 AND a.id=$2 AND x.observed_at >= $3 AND x.observed_at < $4
		AND ($5::timestamptz IS NULL OR c.published_at <= $5)
		AND ST_Covers(a.geometry, ST_SetSRID(ST_MakePoint((x.document#>>'{reported,position,lng}')::float8,(x.document#>>'{reported,position,lat}')::float8),4326))
		ORDER BY x.observed_at,x.evidence_id LIMIT $6`, id, area, from, to, cutoff, historyLimit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var assoc, raw []byte
		var item HistoryItem
		var evidence publicingest.Evidence
		if err := rows.Scan(&assoc, &raw, &item.AvailableAt); err != nil {
			return out, err
		}
		if err := json.Unmarshal(assoc, &item.Association); err != nil {
			return out, err
		}
		if err := json.Unmarshal(raw, &evidence); err != nil {
			return out, err
		}
		item.AdmissionID, item.ProviderRecordID, item.ProviderRevision = evidence.AdmissionID, evidence.ProviderRecordID, evidence.ProviderRevision
		item.ContentSHA256, item.RetrievedAt = evidence.ContentHash, evidence.RetrievedAt
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	admissions := map[string]bool{}
	for _, item := range out.Items {
		if admissions[item.AdmissionID] {
			continue
		}
		p, err := publicingest.LockPolicy(ctx, tx, item.SourceID, item.AdmissionID)
		if err != nil {
			return out, err
		}
		if !p.Rights.AllowAnalysis {
			return out, apperr.Forbidden("source does not permit moving history analysis")
		}
		admissions[item.AdmissionID] = true
	}
	out.Truncated = len(out.Items) > historyLimit
	if out.Truncated {
		out.Items = out.Items[:historyLimit]
	}
	out.Gaps = SelectedGaps(out.Items, out.From, out.To, kind, out.Truncated)
	return out, nil
}

func SelectedGaps(items []HistoryItem, from, to time.Time, kind string, truncated bool) []Gap {
	max := 2 * time.Minute
	if kind == "vessel" {
		max = 5 * time.Minute
	}
	var last time.Time
	quarantined := false
	gaps := []Gap{}
	for _, item := range items {
		if item.Decision == "QUARANTINED" {
			quarantined = true
			continue
		}
		if last.IsZero() {
			if item.ObservedAt.After(from) {
				gaps = append(gaps, Gap{from, item.ObservedAt, "window_edge_unknown"})
			}
		} else if quarantined || item.ObservedAt.Sub(last) > max {
			reason := "report_gap"
			if quarantined {
				reason = "quarantined_report"
			}
			if item.ObservedAt.After(last) {
				gaps = append(gaps, Gap{last, item.ObservedAt, reason})
			}
		}
		last, quarantined = item.ObservedAt, false
	}
	if last.IsZero() {
		last = from
	}
	reason := "window_edge_unknown"
	if truncated {
		reason = "result_cap"
	}
	if last.Before(to) {
		gaps = append(gaps, Gap{last, to, reason})
	}
	return gaps
}
