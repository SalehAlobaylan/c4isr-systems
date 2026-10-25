// Package publiccontext presents station weather and source reports without
// promoting them into tracks, affected-area assertions or reviewed intelligence.
package publiccontext

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
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

type Projector struct{ Next publicingest.Projector }

func (p Projector) Project(ctx context.Context, tx pgx.Tx, e publicingest.Evidence) error {
	if p.Next != nil {
		if err := p.Next.Project(ctx, tx, e); err != nil {
			return err
		}
	}
	if e.Context == nil {
		return nil
	}
	v := e.Context
	key := e.ProviderRecordID
	revision := v.From
	if v.Weather != nil {
		key = v.Weather.StationID
	}
	if e.ProviderUpdatedAt != nil {
		revision = *e.ProviderUpdatedAt
	}
	_, err := tx.Exec(ctx, `INSERT INTO public_context_records(evidence_id,source_id,kind,record_key,valid_from,valid_to,revision_at,known_at,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8,ST_SetSRID(ST_MakePoint($9,$10),4326)) ON CONFLICT(evidence_id) DO NOTHING`, e.ID, e.SourceID, v.Kind, key, v.From, v.To, revision, e.RetrievedAt, v.Position.Lng, v.Position.Lat)
	return err
}

type Entry struct {
	Evidence        publicingest.Evidence `json:"evidence"`
	AnalystQuestion string                `json:"analystQuestion"`
	Attribution     string                `json:"attribution"`
	Status          string                `json:"status"`
	AgeSeconds      float64               `json:"ageSeconds"`
	SourceState     string                `json:"sourceState"`
	LastAttempt     *time.Time            `json:"lastAttempt,omitempty"`
	LastSuccess     *time.Time            `json:"lastSuccess,omitempty"`
	FailureCode     string                `json:"failureCode,omitempty"`
}
type Service struct{ Pool *pgxpool.Pool }

func require(ctx context.Context) error {
	if err := access.Require(ctx, "sources.read"); err != nil {
		return err
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("weather and reports require Public Context")
	}
	return nil
}

const selectEntry = `SELECT e.document,original_admission.document,current_admission.document,c.last_attempt_at,c.last_success_at,c.last_failure_code,src.status FROM public_context_records r JOIN public_evidence e ON e.id=r.evidence_id AND e.scope_id=r.scope_id JOIN source_admissions original_admission ON original_admission.id=e.admission_id JOIN source_connector_state c ON c.source_id=r.source_id AND c.scope_id=r.scope_id JOIN source_admissions current_admission ON current_admission.id=c.admission_id JOIN sources src ON src.id=r.source_id `

func scan(row pgx.Row) (Entry, error) {
	var v Entry
	var raw, oldRaw, currentRaw []byte
	var sourceStatus string
	err := row.Scan(&raw, &oldRaw, &currentRaw, &v.LastAttempt, &v.LastSuccess, &v.FailureCode, &sourceStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, apperr.NotFound("context evidence", "unavailable")
	}
	if err != nil {
		return v, err
	}
	var old, a publicingest.Admission
	if err = json.Unmarshal(raw, &v.Evidence); err != nil {
		return v, err
	}
	if err = json.Unmarshal(oldRaw, &old); err != nil {
		return v, err
	}
	if err = json.Unmarshal(currentRaw, &a); err != nil {
		return v, err
	}
	for _, at := range []*time.Time{v.LastAttempt, v.LastSuccess} {
		if at != nil {
			*at = at.UTC()
		}
	}
	v.AnalystQuestion = old.AnalystQuestion
	v.Attribution = old.Rights.Attribution
	v.AgeSeconds = math.Max(0, time.Since(v.Evidence.Context.To).Seconds())
	v.Status = "current"
	if v.AgeSeconds > float64(a.Policy.FreshForSeconds) {
		v.Status = "stale"
	}
	if v.AgeSeconds > float64(a.Policy.UnavailableAfterSeconds) {
		v.Status = "outdated"
	}
	v.SourceState = "bounded_capture_completed"
	if v.LastSuccess == nil || time.Since(*v.LastSuccess) > time.Duration(a.Policy.UnavailableAfterSeconds)*time.Second {
		v.SourceState = "no_recent_completed_capture"
	}
	if v.FailureCode != "" {
		v.SourceState = "partial_or_failed_capture"
	}
	if sourceStatus != "active" {
		v.SourceState = "source_" + sourceStatus
	}
	if !a.Enabled || a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(time.Now()) {
		v.SourceState = "disabled_or_rights_expired"
	}
	return v, nil
}
func (s Service) Get(ctx context.Context, id string) (Entry, error) {
	if err := require(ctx); err != nil {
		return Entry{}, err
	}
	return scan(platformdb.Scoped(s.Pool).QueryRow(ctx, selectEntry+`WHERE r.evidence_id=$1`, id))
}
func (s Service) List(ctx context.Context, kind, bbox string, regions ...string) ([]Entry, error) {
	if err := require(ctx); err != nil {
		return nil, err
	}
	if kind != "" && kind != "WEATHER" && kind != "REPORT" {
		return nil, apperr.Validation("kind must be WEATHER or REPORT")
	}
	args := []any{kind}
	predicate := ""
	if bbox != "" {
		parts := strings.Split(bbox, ",")
		if len(parts) != 4 {
			return nil, apperr.Validation("invalid context AOI")
		}
		var b [4]float64
		for i, v := range parts {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, apperr.Validation("invalid context AOI")
			}
			b[i] = n
		}
		if (publicingest.Bounds{West: b[0], South: b[1], East: b[2], North: b[3]}).Validate() != nil {
			return nil, apperr.Validation("invalid context AOI")
		}
		predicate = ` AND ST_Intersects(r.position,ST_MakeEnvelope($2,$3,$4,$5,4326))`
		for _, n := range b {
			args = append(args, n)
		}
	}
	if len(regions) > 0 && regions[0] != "" {
		if len(regions[0]) > 256 {
			return nil, apperr.Validation("invalid region")
		}
		args = append(args, regions[0])
		predicate += ` AND EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$` + strconv.Itoa(len(args)) + ` AND ST_Covers(g.geometry,r.position))`
	}
	if len(regions) > 1 && regions[1] != "" {
		if e := publicevents.ValidateStudyGeometry(regions[1], false); e != nil {
			return nil, e
		}
		args = append(args, regions[1])
		predicate += ` AND ST_Intersects(r.position,ST_SetSRID(ST_GeomFromGeoJSON($` + strconv.Itoa(len(args)) + `),4326))`
	}
	// Choose current revision before spatial filtering so an older location cannot
	// reappear after its provider correction moved outside the selected AOI.
	sql := `WITH latest AS(SELECT DISTINCT ON(source_id,record_key) evidence_id FROM public_context_records ORDER BY source_id,record_key,revision_at DESC,known_at DESC,evidence_id DESC) ` + selectEntry + `JOIN latest l ON l.evidence_id=r.evidence_id WHERE ($1='' OR r.kind=$1)` + predicate + ` ORDER BY r.valid_from DESC,r.evidence_id LIMIT 101`
	rows, err := platformdb.Scoped(s.Pool).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s Service) History(ctx context.Context, id string) ([]Entry, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	rows, err := platformdb.Scoped(s.Pool).Query(ctx, selectEntry+`WHERE (r.source_id,r.record_key)=(SELECT source_id,record_key FROM public_context_records WHERE evidence_id=$1) ORDER BY r.revision_at DESC,r.known_at DESC,r.evidence_id DESC LIMIT 101`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func list(w http.ResponseWriter, items []Entry, err error) {
	if err != nil {
		httpx.Error(w, err)
		return
	}
	truncated := len(items) > 100
	if truncated {
		items = items[:100]
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "total": len(items), "truncated": truncated})
}
func (s Service) Mount(r chi.Router) {
	r.Get("/public-context/records", func(w http.ResponseWriter, q *http.Request) {
		v, e := s.List(q.Context(), q.URL.Query().Get("kind"), q.URL.Query().Get("bbox"), q.URL.Query().Get("region_id"), q.URL.Query().Get("geometry"))
		list(w, v, e)
	})
	r.Get("/public-context/records/{id}", func(w http.ResponseWriter, q *http.Request) {
		v, e := s.Get(q.Context(), chi.URLParam(q, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/public-context/records/{id}/history", func(w http.ResponseWriter, q *http.Request) {
		v, e := s.History(q.Context(), chi.URLParam(q, "id"))
		list(w, v, e)
	})
}
