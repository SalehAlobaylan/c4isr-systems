package externaltracks

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Track struct {
	SourceLastAttemptAt *time.Time                     `json:"sourceLastAttemptAt,omitempty"`
	SourceLastSuccessAt *time.Time                     `json:"sourceLastSuccessAt,omitempty"`
	SourceFailureCode   string                         `json:"sourceFailureCode,omitempty"`
	CoverageTo          *time.Time                     `json:"coverageTo,omitempty"`
	ID                  string                         `json:"id"`
	SourceID            string                         `json:"sourceId"`
	Kind                string                         `json:"kind"`
	IdentityClaim       string                         `json:"identityClaim"`
	FirstSeenAt         time.Time                      `json:"firstSeenAt"`
	LastSeenAt          time.Time                      `json:"lastSeenAt"`
	LatestEvidenceID    string                         `json:"latestEvidenceId"`
	Reported            publicingest.MovingObservation `json:"reported"`
	Status              string                         `json:"status"`
	AgeSeconds          float64                        `json:"ageSeconds"`
	CoverageState       string                         `json:"coverageState"`
	OriginalProducer    string                         `json:"originalProducer"`
	UpstreamProvider    string                         `json:"upstreamProvider"`
	Attribution         string                         `json:"attribution"`
	Meaning             string                         `json:"meaning"`
	Controllable        bool                           `json:"controllable"`
}
type Service struct{ Pool *pgxpool.Pool }

const trackSelect = `SELECT s.track_id,s.source_id,s.kind,s.identity_claim,s.first_seen_at,s.last_seen_at,s.latest_evidence_id,s.document,t.status,a.document,src.status,c.last_attempt_at,c.last_success_at,c.last_failure_code,c.coverage_to FROM external_track_segments s JOIN tracks t ON t.id=s.track_id AND t.scope_id=s.scope_id JOIN public_evidence latest_original ON latest_original.id=s.latest_evidence_id JOIN retrieval_profile_revisions original_profile ON original_profile.admission_id=latest_original.admission_id JOIN retrieval_profile_state c ON c.source_id=s.source_id AND c.scope_id=s.scope_id AND c.profile_key=original_profile.profile_key JOIN source_admissions a ON a.id=c.admission_id JOIN sources src ON src.id=s.source_id`

func require(ctx context.Context) error {
	if err := access.Require(ctx, "tracks.read"); err != nil {
		return err
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("external tracks require Public Context")
	}
	return nil
}
func scan(row pgx.Row) (Track, error) {
	var t Track
	var doc, admission []byte
	var sourceStatus string
	err := row.Scan(&t.ID, &t.SourceID, &t.Kind, &t.IdentityClaim, &t.FirstSeenAt, &t.LastSeenAt, &t.LatestEvidenceID, &doc, &t.Status, &admission, &sourceStatus, &t.SourceLastAttemptAt, &t.SourceLastSuccessAt, &t.SourceFailureCode, &t.CoverageTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperr.NotFound("external track", "unavailable")
	}
	if err != nil {
		return t, err
	}
	if err = json.Unmarshal(doc, &t.Reported); err != nil {
		return t, err
	}
	var a publicingest.Admission
	if err = json.Unmarshal(admission, &a); err != nil {
		return t, err
	}
	t.FirstSeenAt = t.FirstSeenAt.UTC()
	t.LastSeenAt = t.LastSeenAt.UTC()
	for _, stamp := range []*time.Time{t.SourceLastAttemptAt, t.SourceLastSuccessAt, t.CoverageTo} {
		if stamp != nil {
			*stamp = stamp.UTC()
		}
	}
	t.AgeSeconds = math.Max(0, time.Since(t.LastSeenAt).Seconds())
	t.CoverageState = "admitted_bounded_capture"
	if t.SourceFailureCode != "" {
		t.CoverageState = "partial_capture_or_source_failure"
	} else if t.SourceLastSuccessAt == nil || time.Since(*t.SourceLastSuccessAt) > time.Duration(a.Policy.UnavailableAfterSeconds)*time.Second {
		t.CoverageState = "capture_outdated_or_not_completed"
	}
	if sourceStatus != "active" {
		t.CoverageState = "source_marked_" + sourceStatus
	}
	if !a.Enabled || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now())) {
		t.CoverageState = "source_disabled_or_rights_expired"
	}
	if t.AgeSeconds > SegmentGap.Seconds() {
		t.Status = "closed"
	}
	if t.Status != "closed" {
		t.Status = "live"
		if t.AgeSeconds > float64(a.Policy.FreshForSeconds) {
			t.Status = "stale"
		}
		if t.AgeSeconds > float64(a.Policy.UnavailableAfterSeconds) {
			t.Status = "lost"
		}
	}
	t.OriginalProducer = a.OriginalProducer
	t.UpstreamProvider = a.UpstreamProvider
	t.Attribution = a.Rights.Attribution
	t.Meaning = "Public observed hypothesis; identity/classification are provider claims, affiliation unassessed. Bounded reception gaps are not evidence of absence. No control actions."
	return t, nil
}
func (s Service) Get(ctx context.Context, id string) (Track, error) {
	if err := require(ctx); err != nil {
		return Track{}, err
	}
	return scan(platformdb.Scoped(s.Pool).QueryRow(ctx, trackSelect+` WHERE s.track_id=$1`, id))
}
func (s Service) List(ctx context.Context, b *publicingest.Bounds, source string, regions ...string) ([]Track, bool, error) {
	if err := require(ctx); err != nil {
		return nil, false, err
	}
	region := ""
	if len(regions) > 0 {
		region = regions[0]
	}
	if len(region) > 256 {
		return nil, false, apperr.Validation("invalid region")
	}
	geometry := ""
	scopeGeometry, from, to, unsupported := queryscope.Catalog(ctx, "aircraft")
	if unsupported {
		return []Track{}, false, nil
	}
	if len(regions) > 1 {
		geometry = regions[1]
	}
	if geometry != "" {
		if e := publicevents.ValidateStudyGeometry(geometry, false); e != nil {
			return nil, false, e
		}
	}
	bounds := publicingest.Bounds{}
	if b != nil {
		bounds = *b
		if err := b.Validate(); err != nil {
			return nil, false, err
		}
	}
	if scopeGeometry != "" {
		geometry = scopeGeometry
	}
	rows, err := platformdb.Scoped(s.Pool).Query(ctx, trackSelect+` LEFT JOIN track_state st ON st.track_id=s.track_id WHERE ($1::boolean OR ST_Intersects(st.position::geometry,ST_MakeEnvelope($2,$3,$4,$5,4326))) AND ($6::text='' OR s.source_id=$6) AND ($7::text='' OR EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$7 AND ST_Covers(g.geometry,st.position::geometry))) AND ($8::text='' OR ST_Intersects(st.position::geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($8,'')),4326))) AND ($9::timestamptz IS NULL OR s.last_seen_at>=$9) AND ($10::timestamptz IS NULL OR s.last_seen_at<$10) ORDER BY s.last_seen_at DESC,s.track_id LIMIT 201`, b == nil, bounds.West, bounds.South, bounds.East, bounds.North, source, region, geometry, from, to)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []Track{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	cut := len(out) > 200
	if cut {
		out = out[:200]
	}
	return out, cut, nil
}
func (s Service) Associations(ctx context.Context, id string) ([]Association, bool, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, false, err
	}
	if err := access.Require(ctx, "observations.read"); err != nil {
		return nil, false, err
	}
	rows, err := platformdb.Scoped(s.Pool).Query(ctx, `SELECT document FROM external_track_associations WHERE track_id=$1 ORDER BY observed_at DESC,evidence_id LIMIT 101`, id)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []Association{}
	for rows.Next() {
		var raw []byte
		var a Association
		if err = rows.Scan(&raw); err != nil {
			return nil, false, err
		}
		if err = json.Unmarshal(raw, &a); err != nil {
			return nil, false, err
		}
		out = append(out, a)
	}
	cut := len(out) > 100
	if cut {
		out = out[:100]
	}
	return out, cut, rows.Err()
}
func (s Service) Mount(r chi.Router) {
	r.Get("/external-tracks", func(w http.ResponseWriter, r *http.Request) {
		var b *publicingest.Bounds
		if raw := r.URL.Query().Get("bbox"); raw != "" {
			parts := strings.Split(raw, ",")
			if len(parts) != 4 {
				httpx.Error(w, apperr.Validation("four AOI coordinates required"))
				return
			}
			v := make([]float64, 4)
			for i := range parts {
				n, e := strconv.ParseFloat(parts[i], 64)
				if e != nil {
					httpx.Error(w, apperr.Validation("invalid AOI"))
					return
				}
				v[i] = n
			}
			b = &publicingest.Bounds{West: v[0], South: v[1], East: v[2], North: v[3]}
		}
		items, cut, err := s.List(r.Context(), b, r.URL.Query().Get("source"), r.URL.Query().Get("region_id"), r.URL.Query().Get("geometry"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, queryscope.ListMetadata(r.Context(), "aircraft", map[string]any{"items": items, "total": len(items), "truncated": cut}))
	})
	r.Get("/external-tracks/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/external-tracks/{id}/associations", func(w http.ResponseWriter, r *http.Request) {
		v, cut, err := s.Associations(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": v, "total": len(v), "truncated": cut})
	})
	r.Get("/external-tracks/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from, fromErr := time.Parse(time.RFC3339Nano, q.Get("from"))
		to, toErr := time.Parse(time.RFC3339Nano, q.Get("to"))
		var cutoff *time.Time
		if raw := q.Get("cutoff"); raw != "" {
			t, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				httpx.Error(w, apperr.Validation("invalid history cutoff"))
				return
			}
			cutoff = &t
		}
		if fromErr != nil || toErr != nil {
			httpx.Error(w, apperr.Validation("history requires from and to timestamps with offsets"))
			return
		}
		v, err := s.ReadHistory(r.Context(), chi.URLParam(r, "id"), q.Get("areaRevisionId"), from, to, cutoff)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
}
