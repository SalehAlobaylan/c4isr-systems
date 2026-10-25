// Package readiness reports scoped feature activation, not merely a database
// ping. No credential, provider URL, hidden run, or private blob key is exposed.
package readiness

import (
	"context"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type Worker struct {
	Feature      string    `json:"feature"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
	State        string    `json:"state"`
	RuntimeBuild string    `json:"runtimeBuild,omitempty"`
}
type Feature struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Count      int    `json:"count"`
	NextAction string `json:"nextAction"`
}
type Report struct {
	CheckedAt time.Time       `json:"checkedAt"`
	Resource  access.Resource `json:"resource"`
	Features  []Feature       `json:"features"`
	Workers   []Worker        `json:"workers"`
}
type Service struct {
	Pool          *pgxpool.Pool
	Store         artifacts.Store
	RuntimeBuilds []string
}

func (s Service) Mount(r chi.Router) {
	r.Get("/platform/readiness", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Report(r.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
	r.Post("/platform/worker-heartbeats", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Feature      string `json:"feature"`
			RuntimeBuild string `json:"runtimeBuild"`
		}
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		if e := s.Beat(r.Context(), in.Feature, in.RuntimeBuild); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]string{"state": "READY"})
	})
}
func (s Service) Beat(ctx context.Context, feature, build string) error {
	p, r, _ := access.From(ctx)
	if p.Kind != "service" {
		return apperr.Forbidden("worker service identity required")
	}
	permissions := map[string]string{"connectors": "connectors.execute", "imagery": "imagery.jobs.execute", "assets": "sim.assets.import", "runtime": "sim.runtime.execute", "recordings": "sim.recordings.execute", "preparation": "imagery.capture.execute"}
	permission, ok := permissions[feature]
	if !ok {
		return apperr.Validation("unknown worker feature")
	}
	if e := access.Require(ctx, permission); e != nil {
		return e
	}
	if feature == "runtime" {
		admitted := false
		for _, v := range s.RuntimeBuilds {
			if v == build {
				admitted = true
			}
		}
		if !admitted {
			return apperr.Forbidden("runtime build is not admitted by deployment")
		}
	} else {
		build = ""
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO platform_worker_status(scope_id,principal_id,feature,runtime_build)VALUES($1,$2,$3,$4) ON CONFLICT(scope_id,principal_id,feature)DO UPDATE SET last_seen_at=now(),runtime_build=excluded.runtime_build`, r.ID(), p.ID, feature, build)
		return e
	})
}
func (s Service) Report(ctx context.Context) (Report, error) {
	if e := access.Require(ctx, "operational.read"); e != nil {
		return Report{}, e
	}
	_, r, _ := access.From(ctx)
	out := Report{CheckedAt: time.Now().UTC(), Resource: r, Features: []Feature{}, Workers: []Worker{}}
	queries := [][3]string{}
	switch r.Partition {
	case access.PublicEvidence:
		if access.Allows(ctx, "sources.read") {
			queries = append(queries, [3]string{"reference", "reference_datasets", "Install reviewed geographic layers"}, [3]string{"imagery", "imagery_scenes", "Retrieve admitted scene metadata"}, [3]string{"imagery-captures", "imagery_capture_jobs", "Capture and prepare real band windows"}, [3]string{"evidence", "public_evidence", "Retrieve an admitted public source"})
		}
	case access.ExerciseInput:
		if access.Allows(ctx, "sim.scenarios.author") {
			queries = append(queries, [3]string{"models", "model_asset_versions", "Install validated starter models"}, [3]string{"scenes", "simulation_scenes", "Prepare and freeze a scene"})
		}
	case access.ExerciseObservation:
		if access.Allows(ctx, "observations.read") {
			queries = append(queries, [3]string{"observations", "observations", "Start an authorized exercise"})
		}
	}
	// Table identifiers are fixed above, never request input.
	for _, q := range queries {
		f := Feature{ID: q[0], State: "NOT_INSTALLED", NextAction: q[2]}
		query := fmt.Sprintf("SELECT count(*) FROM %s", q[1])
		if q[0] == "imagery-captures" {
			query += " WHERE status='SUCCEEDED'"
		}
		e := db.Scoped(s.Pool).QueryRow(ctx, query).Scan(&f.Count)
		if e != nil {
			return out, e
		}
		if f.Count > 0 {
			f.State = "AVAILABLE"
			f.NextAction = ""
		}
		out.Features = append(out.Features, f)
	}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	raw, e := s.Store.Get(probe, "platform/readiness", int64(len("emad-ready")))
	cancel()
	f := Feature{ID: "artifact-storage", State: "UNAVAILABLE", NextAction: "Run local setup or check private object storage"}
	if e == nil && string(raw) == "emad-ready" {
		f.State = "AVAILABLE"
		f.NextAction = ""
	}
	out.Features = append(out.Features, f)
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT feature,last_seen_at,runtime_build FROM platform_worker_status ORDER BY feature,principal_id`)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Worker
		if e = rows.Scan(&v.Feature, &v.LastSeenAt, &v.RuntimeBuild); e != nil {
			return out, e
		}
		v.State = "STALE"
		if time.Since(v.LastSeenAt) < 15*time.Second {
			v.State = "READY"
		}
		out.Workers = append(out.Workers, v)
	}
	return out, rows.Err()
}
