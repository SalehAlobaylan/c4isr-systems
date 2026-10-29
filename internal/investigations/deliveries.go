package investigations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type EnvironmentInput = environmentprep.Input
type EnvironmentSnapshot = environmentprep.Snapshot
type DeliveryInput struct {
	Kind        string            `json:"kind"`
	Output      OutputInput       `json:"output"`
	Environment *EnvironmentInput `json:"environment,omitempty"`
}
type DeliveryJob struct {
	NextAttemptAt  time.Time  `json:"nextAttemptAt"`
	ID             string     `json:"id"`
	CaseID         string     `json:"caseId"`
	OutputID       string     `json:"outputId"`
	CreatorHumanID string     `json:"creatorHumanId"`
	Kind           string     `json:"kind"`
	State          string     `json:"state"`
	Attempt        int        `json:"attempt"`
	LeaseID        string     `json:"-"`
	LeaseUntil     *time.Time `json:"leaseUntil,omitempty"`
	WorkerBuild    string     `json:"workerBuild"`
	FailureReason  string     `json:"failureReason"`
	CreatedAt      time.Time  `json:"createdAt"`
}

const jobSelect = `SELECT id,case_id,output_id,creator_human_id,kind,state,attempt,lease_id,lease_until,worker_build,failure_reason,created_at,next_attempt_at FROM delivery_jobs `

func scanDelivery(row pgx.Row) (DeliveryJob, error) {
	var j DeliveryJob
	e := row.Scan(&j.ID, &j.CaseID, &j.OutputID, &j.CreatorHumanID, &j.Kind, &j.State, &j.Attempt, &j.LeaseID, &j.LeaseUntil, &j.WorkerBuild, &j.FailureReason, &j.CreatedAt, &j.NextAttemptAt)
	if errors.Is(e, pgx.ErrNoRows) {
		e = apperr.NotFound("delivery job", "unavailable")
	}
	return j, e
}
func (s Service) Delivery(ctx context.Context, id string) (DeliveryJob, error) {
	if e := require(ctx, "outputs.read"); e != nil {
		return DeliveryJob{}, e
	}
	return scanDelivery(db.Scoped(s.Pool).QueryRow(ctx, jobSelect+`WHERE id=$1`, id))
}
func (s Service) SubmitDelivery(ctx context.Context, caseID string, in DeliveryInput) (DeliveryJob, error) {
	if e := require(ctx, "outputs.create"); e != nil {
		return DeliveryJob{}, e
	}
	if in.Kind != "brief" && in.Kind != "environment" {
		return DeliveryJob{}, apperr.Validation("choose brief or environment")
	}
	if in.Output.RequestKey == "" || len(in.Output.RequestKey) > 128 {
		return DeliveryJob{}, apperr.Validation("bounded original request key required")
	}
	originalRaw, _ := jsoncanon.Marshal([]any{caseID, in})
	originalHash := areas.Hash(originalRaw)
	in.Output.DeliveryKind = in.Kind
	in.Output.Environment = in.Environment
	in.Output.StyleVersion = "emad.reader-print.v1"
	in.Output.RendererVersion = BriefRenderer
	if in.Kind == "brief" {
		in.Output.ExpectedBytes = 8 << 20
	}
	if in.Kind == "brief" {
		if in.Environment != nil || in.Output.Purpose == "environment" {
			return DeliveryJob{}, apperr.Validation("brief purpose must be internal or distribution")
		}
	}
	if in.Kind == "environment" {

		if in.Environment == nil {
			return DeliveryJob{}, apperr.Validation("environment qualification required")
		}
		in.Output.Purpose = "environment"
		in.Output.RendererVersion = environmentprep.Version
		in.Output.ExpectedBytes = int64(in.Environment.Qualification.MaxBytes)
	}
	raw, _ := jsoncanon.Marshal(map[string]any{"original": json.RawMessage(originalRaw), "normalized": in})
	hash := originalHash
	var id string
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		principal, resource, _ := access.From(ctx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/delivery/"+principal.HumanID+in.Output.RequestKey); e != nil {
			return e
		}
		var priorHash string
		e := tx.QueryRow(ctx, `SELECT input_hash,job_id FROM delivery_intents WHERE human_id=$1 AND request_key=$2`, principal.HumanID, in.Output.RequestKey).Scan(&priorHash, &id)
		if e == nil {
			if priorHash != originalHash {
				return apperr.Conflict("original delivery request key reused with changed input")
			}
			j, e := scanDelivery(tx.QueryRow(ctx, jobSelect+`WHERE id=$1`, id))
			if e != nil {
				return e
			}
			allowed, e := s.liveHuman(ctx, tx, principal.HumanID, resource, "outputs.read", j.CaseID)
			if e != nil {
				return e
			}
			if !allowed {
				return apperr.Forbidden("live case/source access required to recover the original job")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if in.Kind == "environment" {
			if in.Output.SourceUses == nil {
				in.Output.SourceUses = map[string]string{}
			}
			var required []byte
			if err := tx.QueryRow(ctx, `SELECT required_sources FROM investigations WHERE id=$1`, caseID).Scan(&required); err != nil {
				return err
			}
			var sources []string
			_ = json.Unmarshal(required, &sources)
			for _, source := range sources {
				if in.Output.SourceUses[source] == "" {
					in.Output.SourceUses[source] = "environment"
				}
			}
		}
		o, e := s.CreateOutput(ctx, caseID, in.Output)
		if e != nil {
			return e
		}
		id = "delivery_" + areas.Hash([]byte(o.ID+in.Kind))
		var old string
		e = tx.QueryRow(ctx, `SELECT input_hash FROM delivery_jobs WHERE id=$1`, id).Scan(&old)
		if e == nil {
			if old != hash {
				return apperr.Conflict("delivery intent changed")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		p, _, _ := access.From(ctx)
		var active int
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE state IN('QUEUED','RUNNING')`).Scan(&active); e != nil {
			return e
		}
		if active >= 16 {
			return apperr.Conflict("delivery queue full; reconcile existing work")
		}
		_, e = tx.Exec(ctx, `INSERT INTO delivery_jobs(id,case_id,output_id,creator_human_id,kind,input_hash,document) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, caseID, o.ID, p.HumanID, in.Kind, hash, raw)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO delivery_intents(human_id,request_key,input_hash,job_id)VALUES($1,$2,$3,$4)`, p.HumanID, in.Output.RequestKey, originalHash, id)
		return e
	})
	if e != nil {
		return DeliveryJob{}, e
	}
	return s.Delivery(ctx, id)
}
func (s Service) freezeEnvironment(ctx context.Context, tx pgx.Tx, c Case, in EnvironmentInput) (EnvironmentSnapshot, error) {
	data := []reference.Dataset{}
	allowed := map[string]bool{}
	for _, r := range c.Revision.Evidence {
		if r.Kind == "reference" {
			allowed[r.ID] = true
		}
	}
	for _, f := range c.Revision.Findings {
		for _, r := range f.Evidence {
			if r.Kind == "reference" {
				allowed[r.ID] = true
			}
		}
	}
	if len(in.DatasetIDs) > 16 {
		return EnvironmentSnapshot{}, apperr.Validation("at most 16 datasets")
	}
	for _, id := range in.DatasetIDs {
		if !allowed[id] {
			return EnvironmentSnapshot{}, apperr.Validation("environment datasets must be exactly cited by the current case")
		}
		var doc, raw []byte
		if e := tx.QueryRow(ctx, `SELECT document,artifact FROM reference_datasets WHERE id=$1 AND emad_content_visible('reference',id)`, id).Scan(&doc, &raw); e != nil {
			return EnvironmentSnapshot{}, e
		}
		var d reference.Dataset
		if e := json.Unmarshal(doc, &d); e != nil {
			return EnvironmentSnapshot{}, e
		}
		d.Artifact = raw
		data = append(data, d)
	}
	var clipVersion string
	if e := tx.QueryRow(ctx, `SELECT PostGIS_Full_Version()`).Scan(&clipVersion); e != nil {
		return EnvironmentSnapshot{}, e
	}
	snap, e := environmentprep.Preflight(c.Revision.Scope.Area, in, data)
	if e != nil {
		return snap, apperr.Validation(e.Error())
	}
	snap.ClippingVersion = clipVersion
	return snap, nil
}
func (s Service) deliveryEvent(ctx context.Context, tx pgx.Tx, j DeliveryJob, action, reason string) error {
	raw, _ := json.Marshal(map[string]any{"action": action, "attempt": j.Attempt, "leaseId": j.LeaseID, "workerBuild": j.WorkerBuild, "reason": reason, "recordedAt": time.Now().UTC()})
	_, e := tx.Exec(ctx, `INSERT INTO delivery_attempt_events(id,case_id,job_id,document)VALUES($1,$2,$3,$4)`, ids.New("delivery_event"), j.CaseID, j.ID, raw)
	return e
}
func (s Service) ChangeDelivery(ctx context.Context, id, action string, expected int) (DeliveryJob, error) {
	if e := require(ctx, "outputs.create"); e != nil {
		return DeliveryJob{}, e
	}
	p, _, _ := access.From(ctx)
	e := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		j, e := scanDelivery(tx.QueryRow(ctx, jobSelect+`WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return e
		}
		if p.HumanID != j.CreatorHumanID {
			var owner string
			if action != "cancel" {
				return apperr.Forbidden("only the original job creator can retry")
			}
			if e := tx.QueryRow(ctx, `SELECT owner_human_id FROM investigations WHERE id=$1`, j.CaseID).Scan(&owner); e != nil {
				return e
			}
			if owner != p.HumanID {
				return apperr.Forbidden("only the creator or current case owner can cancel")
			}
		}
		if j.Attempt != expected {
			return apperr.Conflict("job attempt changed")
		}
		state := ""
		if action == "cancel" {
			if j.State == "SUCCEEDED" || j.State == "CANCELED" {
				return apperr.Conflict("job is terminal")
			}
			state = "CANCELED"
		} else if action == "retry" {
			if j.State != "FAILED" && !(j.State == "RUNNING" && j.LeaseUntil != nil && j.LeaseUntil.Before(time.Now())) {
				return apperr.Conflict("retry requires failure or expired lease")
			}
			if j.Attempt >= 3 {
				return apperr.Conflict("attempt limit reached; create a new reviewed intent")
			}
			o, e := s.output(ctx, j.OutputID)
			if e != nil {
				return e
			}
			if e = s.outputEligibility(ctx, tx, o, false); e != nil {
				return e
			}
			state = "QUEUED"
		} else {
			return apperr.Validation("unsupported job action")
		}
		if _, e = tx.Exec(ctx, `UPDATE delivery_jobs SET state=$2,lease_id='',lease_until=NULL,failure_reason='',next_attempt_at=clock_timestamp() WHERE id=$1`, id, state); e != nil {
			return e
		}
		return s.deliveryEvent(ctx, tx, j, state, "Operator action")
	})
	if e != nil {
		return DeliveryJob{}, e
	}
	return s.Delivery(ctx, id)
}

// WorkDelivery is called only from the independent worker command, never HTTP.
// Its credential retains the original canonical author's live all-source case
// access. Automation grants no new membership and cannot perform human review.
func (s Service) WorkDelivery(auth context.Context, build string) (bool, error) {
	ctx, e := access.Refresh(auth)
	if e != nil {
		return false, e
	}
	if e = require(ctx, "outputs.create"); e != nil {
		return false, e
	}
	if len(build) != 64 {
		return false, apperr.Validation("worker binary SHA-256 required")
	}
	p, _, _ := access.From(ctx)
	var j DeliveryJob
	var blocked error
	e = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		var e error
		j, e = scanDelivery(tx.QueryRow(ctx, jobSelect+`WHERE creator_human_id=$1 AND (state='QUEUED' AND next_attempt_at<=clock_timestamp() OR state='RUNNING' AND lease_until<clock_timestamp()) AND attempt<3 ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, p.HumanID))
		if e != nil {
			return e
		}
		o, e := s.output(ctx, j.OutputID)
		if e != nil {
			return e
		}
		if e = s.outputEligibility(ctx, tx, o, false); e != nil {
			blocked = e
			if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET state='FAILED',failure_reason='current_support_unavailable: reassessment required' WHERE id=$1`, j.ID); err != nil {
				return err
			}
			return s.deliveryEvent(ctx, tx, j, "FAILED", "Current support/rights unavailable before processing")
		}
		if j.State == "RUNNING" {
			if e := s.deliveryEvent(ctx, tx, j, "LEASE_EXPIRED", "Prior attempt did not finish; original frozen input retained"); e != nil {
				return e
			}
		}
		j.Attempt++
		j.LeaseID = ids.New("delivery_lease")
		j.WorkerBuild = build
		if _, e = tx.Exec(ctx, `UPDATE delivery_jobs SET state='RUNNING',attempt=$2,lease_id=$3,lease_until=clock_timestamp()+interval '150 seconds',worker_build=$4 WHERE id=$1`, j.ID, j.Attempt, j.LeaseID, build); e != nil {
			return e
		}
		return s.deliveryEvent(ctx, tx, j, "RUNNING", "")
	})
	if apperr.Is(e, apperr.CodeNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if blocked != nil {
		return true, blocked
	}
	work, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	o, e := s.output(work, j.OutputID)
	var raw []byte
	media := "text/html"
	if e == nil && j.Kind == "brief" {
		raw, e = s.renderBrief(work, o)
	} else if e == nil {
		media = "application/zip"
		if o.Manifest.Environment == nil || o.Manifest.RendererVersion != environmentprep.Version {
			e = fmt.Errorf("missing frozen environment input")
		} else {
			snap := *o.Manifest.Environment
			processing, stop := context.WithTimeout(work, time.Duration(snap.Input.Qualification.MaxSeconds)*time.Second)
			defer stop()
			work = processing
			snap.OutputMetadata, _ = json.Marshal(map[string]any{"caseRevisionId": o.RevisionID, "outputId": o.ID, "contentDigest": o.ContentDigest, "rendererVersion": o.Manifest.RendererVersion, "workerBuild": j.WorkerBuild, "dependencies": o.Manifest.Dependencies, "assets": o.Manifest.Assets})
			if _, e = environmentprep.Preflight(snap.Area, snap.Input, snap.Datasets); e == nil {
				for _, asset := range o.Manifest.Assets {
					_, data, err := artifacts.NewService(s.Pool, s.Store).Read(work, asset.ID)
					if err != nil {
						e = err
						break
					}
					tile, err := imageryanalysis.Decode(data)
					if err != nil {
						e = err
						break
					}
					interval := o.Manifest.Case.Scope.Temporal.Interval
					if interval != nil && (tile.AcquiredAt.Before(interval.From) || !tile.AcquiredAt.Before(interval.To)) && !o.Manifest.Case.Scope.Temporal.BackgroundImagery {
						e = fmt.Errorf("imagery acquisition outside study period; explicitly adopt background imagery or another retained input")
						break
					}
					snap.Imagery = append(snap.Imagery, tile)
					snap.ImageryOriginals = append(snap.ImageryOriginals, data)
				}
				if e == nil {
					snap.NormalizedVectors, e = s.clipEnvironmentVectors(work, snap)
				}
				if e == nil {
					raw, _, e = environmentprep.Build(work, snap)
				}
			}
		}
	}
	if e != nil && work.Err() == nil {
		e = apperr.Validation(e.Error())
	}
	if e == nil {
		fresh, err := access.RefreshWithContext(auth, work)
		if err != nil {
			e = err
		} else {
			fresh = context.WithValue(fresh, deliveryLeaseKey{}, j)
			_, e = s.stageOutput(fresh, j.OutputID, media, raw, true)
		}
	}

	if e != nil {
		state := "FAILED"
		reason := "delivery_failed: retained inputs, renderer, storage or live authorization require inspection"
		delay := time.Duration((1<<j.Attempt)+int(j.ID[len(j.ID)-1])%3) * time.Second
		if j.Attempt < 3 && (apperr.Is(e, apperr.CodeConflict) || errors.Is(e, context.DeadlineExceeded)) {
			state = "QUEUED"
			reason = "retry_wait: bounded retry after transient storage/lease/deadline failure"
		}
		_ = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
			ctx := db.UseTx(ctx, tx)
			tag, err := tx.Exec(ctx, `UPDATE delivery_jobs SET state=$4,failure_reason=$3,lease_id='',lease_until=NULL,next_attempt_at=$5 WHERE id=$1 AND lease_id=$2 AND state='RUNNING'`, j.ID, j.LeaseID, reason, state, time.Now().Add(delay))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				return s.deliveryEvent(ctx, tx, j, state, reason)
			}
			return nil
		})
		return true, e
	}
	return true, nil
}
func (s Service) mountDeliveries(r chi.Router) {
	r.Post("/investigations/{id}/delivery-jobs", func(w http.ResponseWriter, r *http.Request) {
		var in DeliveryInput
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.SubmitDelivery(r.Context(), chi.URLParam(r, "id"), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 202, v)
	})
	r.Get("/investigations/{id}/delivery-jobs", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "outputs.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(s.Pool).Query(r.Context(), jobSelect+`WHERE case_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []DeliveryJob{}
		for rows.Next() {
			j, e := scanDelivery(rows)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, j)
		}
		if e := rows.Err(); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items, "limit": 100})
	})
	r.Get("/delivery-jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Delivery(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/delivery-jobs/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedAttempt int `json:"expectedAttempt"`
		}
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := s.ChangeDelivery(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "action"), in.ExpectedAttempt)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/outputs/{id}/bundle", func(w http.ResponseWriter, r *http.Request) {
		raw, e := s.BriefBundle(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="analyst-brief.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(raw)
	})
	r.Get("/outputs/{id}/attestation", func(w http.ResponseWriter, r *http.Request) {
		o, _, e := s.ReadOutput(r.Context(), chi.URLParam(r, "id"), true)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		v, e := s.latestReview(r.Context(), o.CaseID)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}

func (s Service) clipEnvironmentVectors(ctx context.Context, snap EnvironmentSnapshot) (json.RawMessage, error) {
	var version string
	if e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT PostGIS_Full_Version()`).Scan(&version); e != nil {
		return nil, e
	}
	if version != snap.ClippingVersion {
		return nil, apperr.Conflict("geospatial implementation changed; preserve the old package and create a new delivery intent")
	}
	features := []json.RawMessage{}
	for _, d := range snap.Datasets {
		if d.Kind != "vector" {
			continue
		}
		var raw []byte
		e := db.Scoped(s.Pool).QueryRow(ctx, `WITH f AS(SELECT value FROM jsonb_array_elements($1::jsonb->'features')), clipped AS(SELECT value,ST_Intersection(ST_SetSRID(ST_GeomFromGeoJSON(value->'geometry'),4326),ST_SetSRID(ST_GeomFromGeoJSON($2::jsonb),4326)) geom FROM f)SELECT jsonb_build_object('type','FeatureCollection','features',COALESCE(jsonb_agg(jsonb_build_object('type','Feature','id',value->'id','properties',value->'properties','geometry',ST_AsGeoJSON(geom)::jsonb))FILTER(WHERE NOT ST_IsEmpty(geom)),'[]'::jsonb)) FROM clipped`, d.Artifact, snap.Area.Geometry).Scan(&raw)
		if e != nil {
			return nil, e
		}
		var fc struct {
			Features []json.RawMessage `json:"features"`
		}
		if e = json.Unmarshal(raw, &fc); e != nil {
			return nil, e
		}
		features = append(features, fc.Features...)
	}
	return json.Marshal(map[string]any{"type": "FeatureCollection", "features": features})
}

type deliveryLeaseKey struct{}

func (s Service) guardDelivery(ctx context.Context, tx pgx.Tx, outputID string) error {
	j, ok := ctx.Value(deliveryLeaseKey{}).(DeliveryJob)
	if !ok || j.OutputID != outputID {
		return apperr.Forbidden("durable renderer lease required")
	}
	var state, lease string
	var until *time.Time
	if e := tx.QueryRow(ctx, `SELECT state,lease_id,lease_until FROM delivery_jobs WHERE id=$1 FOR UPDATE`, j.ID).Scan(&state, &lease, &until); e != nil {
		return e
	}
	if state != "RUNNING" || lease != j.LeaseID || until == nil || until.Before(time.Now()) {
		return apperr.Conflict("stale or canceled delivery lease")
	}
	return nil
}
func (s Service) finishDelivery(ctx context.Context, tx pgx.Tx) error {
	j, ok := ctx.Value(deliveryLeaseKey{}).(DeliveryJob)
	if !ok {
		return apperr.Forbidden("delivery lease required")
	}
	if _, e := tx.Exec(ctx, `UPDATE delivery_jobs SET state='SUCCEEDED',lease_id='',lease_until=NULL WHERE id=$1`, j.ID); e != nil {
		return e
	}
	return s.deliveryEvent(ctx, tx, j, "SUCCEEDED", "")
}
